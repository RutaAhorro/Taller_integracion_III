package handlers

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
)

// ===== Tipos y dependencias del handler =====

// ScraperHandler gestiona las peticiones HTTP del microservicio de scraping y bots
type ScraperHandler struct {
	service services.ScraperService
	rdb     *redis.Client
	queue   *services.QueueService
}

// SetQueue conecta la cola durable que utiliza el endpoint de ejecución.
func (manejador *ScraperHandler) SetQueue(queue *services.QueueService) { manejador.queue = queue }

// NewScraperHandler inicializa el handler con el servicio y el cliente Redis opcional.
func NewScraperHandler(service services.ScraperService, rdb ...*redis.Client) *ScraperHandler {
	var client *redis.Client
	if len(rdb) > 0 {
		client = rdb[0]
	}
	return &ScraperHandler{service: service, rdb: client}
}

// ejecutarScraperRequest describe los parámetros opcionales para encolar un spider.
type ejecutarScraperRequest struct {
	Spider      string  `json:"spider,omitempty"`
	ProductoURL *string `json:"producto_url,omitempty"`
	ProductoID  *string `json:"producto_id,omitempty"`
	AddURL      *string `json:"add_url,omitempty"`
	EnrichEAN   *bool   `json:"enrich_ean,omitempty"`
}

// ===== Ejecución y ciclo de vida de trabajos =====

// EjecutarTrabajo godoc
// @Summary      Encola un trabajo para ejecución
// @Description  Envía el trabajo a Redis para que el worker Scrapy ejecute el spider solicitado
// @Tags         scraper
// @Accept       json
// @Produce      json
// @Param        id      path      string                 true  "UUID del trabajo"
// @Param        input   body      ejecutarScraperRequest true  "Spider y parámetros opcionales"
// @Success      202     {object}  map[string]interface{}
// @Failure      400     {object}  map[string]interface{}
// @Failure      404     {object}  map[string]interface{}
// @Failure      409     {object}  map[string]interface{}
// @Failure      503     {object}  map[string]interface{}
// @Router       /api/v1/scraper/trabajos/{id}/ejecutar [post]
func (manejador *ScraperHandler) EjecutarTrabajo(contextoHTTP *gin.Context) {
	if manejador.queue == nil {
		contextoHTTP.JSON(http.StatusServiceUnavailable, gin.H{"error": "La cola durable no está configurada"})
		return
	}

	id := contextoHTTP.Param("id")
	trabajo, err := manejador.service.ObtenerTrabajo(repositories.WithRequestID(contextoHTTP.Request.Context(), contextoHTTP.GetHeader("X-Request-ID")), id)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTrabajoNoEncontrado):
			contextoHTTP.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, services.ErrUUIDInvalido):
			contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			contextoHTTP.JSON(http.StatusInternalServerError, gin.H{"error": "Error interno al consultar trabajo"})
		}
		return
	}
	if trabajo.Estado != "en_progreso" {
		contextoHTTP.JSON(http.StatusConflict, gin.H{"error": "El trabajo no está disponible para ejecución"})
		return
	}

	cadena, err := manejador.service.ObtenerCadena(repositories.WithRequestID(contextoHTTP.Request.Context(), contextoHTTP.GetHeader("X-Request-ID")), trabajo.CadenaID, trabajo.CadenaNombre)
	if err != nil {
		if errors.Is(err, services.ErrCadenaNoEncontrada) {
			contextoHTTP.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if manejador.responderErrorCatalogo(contextoHTTP, err) {
			return
		}
		slog.Error("scraper_chain_lookup_error", "request_id", contextoHTTP.GetHeader("X-Request-ID"), "trabajo_id", trabajo.ID, "error", err)
		contextoHTTP.JSON(http.StatusInternalServerError, gin.H{"error": "Error interno al consultar la fuente"})
		return
	}
	if cadena == nil {
		contextoHTTP.JSON(http.StatusNotFound, gin.H{"error": services.ErrCadenaNoEncontrada.Error()})
		return
	}
	if !cadena.EstaActiva {
		contextoHTTP.JSON(http.StatusConflict, gin.H{"error": services.ErrFuenteDeshabilitada.Error(), "codigo": "fuente_deshabilitada"})
		return
	}
	id = trabajo.ID

	var input ejecutarScraperRequest
	if err := contextoHTTP.ShouldBindJSON(&input); err != nil {
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": "Payload inválido para ejecutar scraper", "detalle": err.Error()})
		return
	}
	input.Spider = strings.TrimSpace(input.Spider)
	if input.Spider == "" {
		input.Spider = "jumbo_rsc"
	}
	spidersPermitidos := map[string]bool{
		"jumbo_rsc":        true,
		"santa_isabel_rsc": true,
		"lider_rsc":        true,
		"acuenta_rsc":      true,
		"cugat_rsc":        true,
	}
	if !spidersPermitidos[input.Spider] {
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": "Spider no permitido"})
		return
	}
	if input.ProductoID != nil && input.ProductoURL == nil {
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": "producto_id requiere producto_url"})
		return
	}

	job := map[string]interface{}{
		"trabajo_id": id,
		"spider":     input.Spider,
	}
	if input.ProductoURL != nil {
		job["product_url"] = *input.ProductoURL
	}
	if input.ProductoID != nil {
		job["product_id"] = *input.ProductoID
	}
	if input.AddURL != nil {
		addURL := strings.TrimSpace(*input.AddURL)
		if addURL == "" {
			contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": "add_url no puede estar vacío"})
			return
		}
		// El worker valida host y esquema por cadena antes de escribir la
		// categoría en el volumen compartido. Aquí sólo se evita encolar un
		// valor vacío.
		job["add_url"] = addURL
	}
	if input.EnrichEAN != nil {
		job["enrich_ean"] = *input.EnrichEAN
	}
	payload, err := json.Marshal(job)
	if err != nil {
		contextoHTTP.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo serializar el trabajo"})
		return
	}

	if err := manejador.queue.Encolar(contextoHTTP.Request.Context(), id, payload); err != nil {
		if errors.Is(err, services.ErrIdempotencia) || errors.Is(err, services.ErrTrabajoYaFinalizado) {
			contextoHTTP.JSON(409, gin.H{"error": err.Error(), "codigo": "ejecucion_conflict"})
			return
		}
		slog.Error("scraper_encolar_error", "trabajo_id", id, "error", err)
		contextoHTTP.JSON(http.StatusServiceUnavailable, gin.H{"error": "No se pudo registrar el trabajo"})
		return
	}

	contextoHTTP.Header("X-Scraper-Queue", "durable-stream")
	contextoHTTP.JSON(http.StatusAccepted, gin.H{
		"trabajo_id": id,
		"estado":     "encolado",
		"spider":     input.Spider,
	})
}

// IniciarTrabajo godoc
// @Summary      Inicia una sesión de scraping
// @Description  Registra el inicio de una araña de scraping para auditar su ciclo de vida y métricas
// @Tags         scraper
// @Accept       json
// @Produce      json
// @Param        input  body      domain.IniciarTrabajoDTO  true  "Datos de inicio de la sesión"
// @Success      201    {object}  domain.TrabajoScraperDTO
// @Failure      400    {object}  map[string]interface{}
// @Failure      404    {object}  map[string]interface{}
// @Failure      500    {object}  map[string]interface{}
// @Router       /api/v1/scraper/trabajos [post]
func (manejador *ScraperHandler) IniciarTrabajo(contextoHTTP *gin.Context) {
	var input domain.IniciarTrabajoDTO
	if err := contextoHTTP.ShouldBindJSON(&input); err != nil {
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{
			"error":   "Payload inválido para iniciar trabajo",
			"detalle": err.Error(),
		})
		return
	}

	trabajo, err := manejador.service.IniciarTrabajo(repositories.WithRequestID(contextoHTTP.Request.Context(), contextoHTTP.GetHeader("X-Request-ID")), input)
	if err != nil {
		if errors.Is(err, services.ErrCadenaNoEncontrada) {
			contextoHTTP.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, services.ErrFuenteDeshabilitada) {
			contextoHTTP.JSON(http.StatusConflict, gin.H{"error": err.Error(), "codigo": "fuente_deshabilitada"})
			return
		}
		if manejador.responderErrorCatalogo(contextoHTTP, err) {
			return
		}
		slog.Error("scraper_start_error", "request_id", contextoHTTP.GetHeader("X-Request-ID"), "error", err)
		contextoHTTP.JSON(http.StatusInternalServerError, gin.H{"error": "Error interno al iniciar trabajo"})
		return
	}

	contextoHTTP.JSON(http.StatusCreated, trabajo)
}

// ===== Ingesta y consulta de trabajos =====

// FinalizarTrabajo godoc
// @Summary      Finaliza una sesión de scraping
// @Description  Marca el término del trabajo como completado o fallido, registrando métricas y errores
// @Tags         scraper
// @Accept       json
// @Produce      json
// @Param        id     path      string                   true  "UUID del trabajo de scraping"
// @Param        input  body      domain.FinalizarTrabajoDTO true  "Datos de cierre de la sesión"
// @Success      200    {object}  domain.TrabajoScraperDTO
// @Failure      400    {object}  map[string]interface{}
// @Failure      404    {object}  map[string]interface{}
// @Failure      409    {object}  map[string]interface{}
// @Failure      500    {object}  map[string]interface{}
// @Router       /api/v1/scraper/trabajos/{id}/finalizar [put]
func (manejador *ScraperHandler) FinalizarTrabajo(contextoHTTP *gin.Context) {
	id := contextoHTTP.Param("id")

	var input domain.FinalizarTrabajoDTO
	if err := contextoHTTP.ShouldBindJSON(&input); err != nil {
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{
			"error":   "Payload inválido para finalizar trabajo",
			"detalle": err.Error(),
		})
		return
	}

	trabajo, err := manejador.service.FinalizarTrabajo(repositories.WithRequestID(contextoHTTP.Request.Context(), contextoHTTP.GetHeader("X-Request-ID")), id, input)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTrabajoNoEncontrado):
			contextoHTTP.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, services.ErrTrabajoYaFinalizado):
			contextoHTTP.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, services.ErrEntregaPendiente):
			contextoHTTP.JSON(http.StatusConflict, gin.H{"error": err.Error(), "codigo": "entregas_pendientes"})
		case errors.Is(err, services.ErrEntregaFallida):
			contextoHTTP.JSON(http.StatusConflict, gin.H{"error": err.Error(), "codigo": "entregas_fallidas"})
		case errors.Is(err, services.ErrEstadoTrabajoInvalido), errors.Is(err, services.ErrUUIDInvalido):
			contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			slog.Error("scraper_finalizar_error", "trabajo_id", id, "error", err)
			contextoHTTP.JSON(http.StatusInternalServerError, gin.H{"error": "Error interno al finalizar trabajo"})
		}
		return
	}

	contextoHTTP.JSON(http.StatusOK, trabajo)
}

// ObtenerTrabajo godoc
// @Summary      Consulta el estado de una sesión de scraping
// @Description  Retorna los detalles, duración, conteo y errores de un trabajo (ideal para bots de Discord)
// @Tags         scraper
// @Produce      json
// @Param        id   path      string  true  "UUID del trabajo"
// @Success      200  {object}  domain.TrabajoScraperDTO
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /api/v1/scraper/trabajos/{id} [get]
func (manejador *ScraperHandler) ObtenerTrabajo(contextoHTTP *gin.Context) {
	id := contextoHTTP.Param("id")

	trabajo, err := manejador.service.ObtenerTrabajo(repositories.WithRequestID(contextoHTTP.Request.Context(), contextoHTTP.GetHeader("X-Request-ID")), id)
	if err != nil {
		if errors.Is(err, services.ErrTrabajoNoEncontrado) {
			contextoHTTP.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, services.ErrUUIDInvalido) {
			contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		slog.Error("scraper_consultar_error", "trabajo_id", id, "error", err)
		contextoHTTP.JSON(http.StatusInternalServerError, gin.H{"error": "Error interno al consultar trabajo"})
		return
	}

	contextoHTTP.JSON(http.StatusOK, trabajo)
}

// ===== Recepción de lotes de productos =====

// IngestarProductosConTrabajo godoc
// @Summary      Registra y entrega un lote con trabajo a Catálogo
// @Description  Conserva la entrega durable en Control, usa una clave estable en Catálogo y confirma el contador una sola vez.
// @Tags         scraper
// @Accept       json
// @Produce      json
// @Param        id     path      string                true  "UUID del trabajo de scraping"
// @Param        input  body      domain.IngestaLoteDTO true  "Lote de productos extraídos"
// @Success      200    {object}  domain.IngestaResultadoDTO
// @Failure      400    {object}  map[string]interface{}
// @Failure      404    {object}  map[string]interface{}
// @Failure      409    {object}  map[string]interface{}
// @Failure      500    {object}  map[string]interface{}
// @Router       /api/v1/scraper/trabajos/{id}/productos [post]
func (manejador *ScraperHandler) IngestarProductosConTrabajo(contextoHTTP *gin.Context) {
	id := contextoHTTP.Param("id")

	var input domain.IngestaLoteDTO
	if !prepararLote(contextoHTTP, &input) {
		return
	}
	if err := contextoHTTP.ShouldBindJSON(&input); err != nil {
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{
			"error":   "Payload inválido para ingesta de lote",
			"detalle": err.Error(),
		})
		return
	}

	resultado, err := manejador.service.IngestarProductos(repositories.WithRequestID(contextoHTTP.Request.Context(), contextoHTTP.GetHeader("X-Request-ID")), &id, input)
	if err != nil {
		manejador.manejarErrorIngesta(contextoHTTP, err)
		return
	}

	contextoHTTP.JSON(http.StatusOK, resultado)
}

// IngestarProductosDirecto godoc
// @Summary      Almacena un lote de productos de forma directa
// @Description  Guarda productos crudos y capturas de precio sin requerir una sesión de trabajo previa
// @Tags         scraper
// @Accept       json
// @Produce      json
// @Param        input  body      domain.IngestaLoteDTO true  "Lote de productos extraídos"
// @Success      200    {object}  domain.IngestaResultadoDTO
// @Failure      400    {object}  map[string]interface{}
// @Failure      404    {object}  map[string]interface{}
// @Failure      500    {object}  map[string]interface{}
// @Router       /api/v1/scraper/productos [post]
func (manejador *ScraperHandler) IngestarProductosDirecto(contextoHTTP *gin.Context) {
	var input domain.IngestaLoteDTO
	if !prepararLote(contextoHTTP, &input) {
		return
	}
	if err := contextoHTTP.ShouldBindJSON(&input); err != nil {
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{
			"error":   "Payload inválido para ingesta de lote",
			"detalle": err.Error(),
		})
		return
	}

	resultado, err := manejador.service.IngestarProductos(repositories.WithRequestID(contextoHTTP.Request.Context(), contextoHTTP.GetHeader("X-Request-ID")), nil, input)
	if err != nil {
		manejador.manejarErrorIngesta(contextoHTTP, err)
		return
	}

	contextoHTTP.JSON(http.StatusOK, resultado)
}

// ===== Validación y respuestas de ingesta =====

// manejarErrorIngesta traduce fallos de dominio y de Catálogo a respuestas HTTP.
func (manejador *ScraperHandler) manejarErrorIngesta(contextoHTTP *gin.Context, err error) {
	if manejador.responderErrorCatalogo(contextoHTTP, err) {
		return
	}
	switch {
	case errors.Is(err, services.ErrFuenteDeshabilitada):
		contextoHTTP.JSON(http.StatusConflict, gin.H{"error": err.Error(), "codigo": "fuente_deshabilitada"})
	case errors.Is(err, services.ErrEntregaPendiente):
		contextoHTTP.Header("Retry-After", "2")
		contextoHTTP.JSON(http.StatusAccepted, gin.H{"error": err.Error(), "codigo": "entrega_pendiente"})
	case errors.Is(err, services.ErrEntregaFallida):
		contextoHTTP.JSON(http.StatusConflict, gin.H{"error": err.Error(), "codigo": "entrega_fallida"})
	case errors.Is(err, services.ErrIdempotencia):
		contextoHTTP.JSON(http.StatusConflict, gin.H{"error": err.Error(), "codigo": "idempotency_conflict"})
	case errors.Is(err, services.ErrCatalogoRechazo):
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "codigo": "lote_rechazado"})
	case errors.Is(err, services.ErrContadorPendiente):
		slog.Error("scraper_counter_pending", "request_id", contextoHTTP.GetHeader("X-Request-ID"), "trabajo_id", contextoHTTP.Param("id"))
		contextoHTTP.JSON(http.StatusConflict, gin.H{"error": err.Error(), "codigo": "contador_pendiente", "catalogo_confirmado": true})
	case errors.Is(err, services.ErrTrabajoNoEncontrado), errors.Is(err, services.ErrSucursalNoEncontrada):
		contextoHTTP.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrTrabajoYaFinalizado):
		contextoHTTP.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrLoteVacio), errors.Is(err, services.ErrLoteExcedeMaximo), errors.Is(err, services.ErrUUIDInvalido):
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		slog.Error("scraper_ingesta_error", "request_id", contextoHTTP.GetHeader("X-Request-ID"), "error", err)
		contextoHTTP.JSON(http.StatusInternalServerError, gin.H{"error": "Error interno durante la ingesta"})
	}
}

// prepararLote asigna una clave idempotente y limita el tamaño de la petición.
func prepararLote(contextoHTTP *gin.Context, input *domain.IngestaLoteDTO) bool {
	key := contextoHTTP.GetHeader("Idempotency-Key")
	id := uuid.New()
	if key != "" {
		var err error
		id, err = uuid.Parse(key)
		if err != nil {
			contextoHTTP.JSON(400, gin.H{"error": "Idempotency-Key debe ser un UUID"})
			return false
		}
	}
	input.LoteID = id.String()
	contextoHTTP.Header("Idempotency-Key", input.LoteID)
	contextoHTTP.Request.Body = http.MaxBytesReader(contextoHTTP.Writer, contextoHTTP.Request.Body, 8<<20)
	return true
}

// responderErrorCatalogo traduce errores de disponibilidad de Catálogo a HTTP.
func (manejador *ScraperHandler) responderErrorCatalogo(contextoHTTP *gin.Context, err error) bool {
	status, code := 0, ""
	if errors.Is(err, services.ErrCatalogoNoDisponible) {
		status, code = http.StatusServiceUnavailable, "catalogo_unavailable"
	}
	if errors.Is(err, services.ErrCatalogoTimeout) {
		status, code = http.StatusGatewayTimeout, "catalogo_timeout"
	}
	if status == 0 {
		return false
	}
	contextoHTTP.JSON(status, gin.H{"error": err.Error(), "codigo": code})
	return true
}
