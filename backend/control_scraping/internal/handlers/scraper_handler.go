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
	servicio     services.ScraperService
	clienteRedis *redis.Client
	cola         *services.QueueService
}

// SetQueue conecta la cola durable que utiliza el endpoint de ejecución.
func (manejador *ScraperHandler) SetQueue(cola *services.QueueService) { manejador.cola = cola }

// NewScraperHandler inicializa el handler con el servicio y el cliente Redis opcional.
func NewScraperHandler(servicio services.ScraperService, clienteRedis ...*redis.Client) *ScraperHandler {
	var cliente *redis.Client
	if len(clienteRedis) > 0 {
		cliente = clienteRedis[0]
	}
	return &ScraperHandler{servicio: servicio, clienteRedis: cliente}
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
	if manejador.cola == nil {
		contextoHTTP.JSON(http.StatusServiceUnavailable, gin.H{"error": "La cola durable no está configurada"})
		return
	}

	identificador := contextoHTTP.Param("id")
	trabajo, errorObtenido := manejador.servicio.ObtenerTrabajo(repositories.WithRequestID(contextoHTTP.Request.Context(), contextoHTTP.GetHeader("X-Request-ID")), identificador)
	if errorObtenido != nil {
		switch {
		case errors.Is(errorObtenido, services.ErrTrabajoNoEncontrado):
			contextoHTTP.JSON(http.StatusNotFound, gin.H{"error": errorObtenido.Error()})
		case errors.Is(errorObtenido, services.ErrUUIDInvalido):
			contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": errorObtenido.Error()})
		default:
			contextoHTTP.JSON(http.StatusInternalServerError, gin.H{"error": "Error interno al consultar trabajo"})
		}
		return
	}
	if trabajo.Estado != "en_progreso" {
		contextoHTTP.JSON(http.StatusConflict, gin.H{"error": "El trabajo no está disponible para ejecución"})
		return
	}

	cadena, errorObtenido := manejador.servicio.ObtenerCadena(repositories.WithRequestID(contextoHTTP.Request.Context(), contextoHTTP.GetHeader("X-Request-ID")), trabajo.CadenaID, trabajo.CadenaNombre)
	if errorObtenido != nil {
		if errors.Is(errorObtenido, services.ErrCadenaNoEncontrada) {
			contextoHTTP.JSON(http.StatusNotFound, gin.H{"error": errorObtenido.Error()})
			return
		}
		if manejador.responderErrorCatalogo(contextoHTTP, errorObtenido) {
			return
		}
		slog.Error("scraper_chain_lookup_error", "request_id", contextoHTTP.GetHeader("X-Request-ID"), "trabajo_id", trabajo.ID, "error", errorObtenido)
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
	identificador = trabajo.ID

	var entrada ejecutarScraperRequest
	if errorObtenido := contextoHTTP.ShouldBindJSON(&entrada); errorObtenido != nil {
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": "Payload inválido para ejecutar scraper", "detalle": errorObtenido.Error()})
		return
	}
	entrada.Spider = strings.TrimSpace(entrada.Spider)
	if entrada.Spider == "" {
		entrada.Spider = "jumbo_rsc"
	}
	spidersPermitidos := map[string]bool{
		"jumbo_rsc":        true,
		"santa_isabel_rsc": true,
		"lider_rsc":        true,
		"acuenta_rsc":      true,
		"cugat_rsc":        true,
	}
	if !spidersPermitidos[entrada.Spider] {
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": "Spider no permitido"})
		return
	}
	if entrada.ProductoID != nil && entrada.ProductoURL == nil {
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": "producto_id requiere producto_url"})
		return
	}

	datosTrabajo := map[string]interface{}{
		"trabajo_id": identificador,
		"spider":     entrada.Spider,
	}
	if entrada.ProductoURL != nil {
		datosTrabajo["product_url"] = *entrada.ProductoURL
	}
	if entrada.ProductoID != nil {
		datosTrabajo["product_id"] = *entrada.ProductoID
	}
	if entrada.AddURL != nil {
		urlAdicional := strings.TrimSpace(*entrada.AddURL)
		if urlAdicional == "" {
			contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": "add_url no puede estar vacío"})
			return
		}
		// El worker valida host y esquema por cadena antes de escribir la
		// categoría en el volumen compartido. Aquí sólo se evita encolar un
		// valor vacío.
		datosTrabajo["add_url"] = urlAdicional
	}
	if entrada.EnrichEAN != nil {
		datosTrabajo["enrich_ean"] = *entrada.EnrichEAN
	}
	carga, errorObtenido := json.Marshal(datosTrabajo)
	if errorObtenido != nil {
		contextoHTTP.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo serializar el trabajo"})
		return
	}

	if errorObtenido := manejador.cola.Encolar(contextoHTTP.Request.Context(), identificador, carga); errorObtenido != nil {
		if errors.Is(errorObtenido, services.ErrIdempotencia) || errors.Is(errorObtenido, services.ErrTrabajoYaFinalizado) {
			contextoHTTP.JSON(409, gin.H{"error": errorObtenido.Error(), "codigo": "ejecucion_conflict"})
			return
		}
		slog.Error("scraper_encolar_error", "trabajo_id", identificador, "error", errorObtenido)
		contextoHTTP.JSON(http.StatusServiceUnavailable, gin.H{"error": "No se pudo registrar el trabajo"})
		return
	}

	contextoHTTP.Header("X-Scraper-Queue", "durable-stream")
	contextoHTTP.JSON(http.StatusAccepted, gin.H{
		"trabajo_id": identificador,
		"estado":     "encolado",
		"spider":     entrada.Spider,
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
	var entrada domain.IniciarTrabajoDTO
	if errorObtenido := contextoHTTP.ShouldBindJSON(&entrada); errorObtenido != nil {
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{
			"error":   "Payload inválido para iniciar trabajo",
			"detalle": errorObtenido.Error(),
		})
		return
	}

	trabajo, errorObtenido := manejador.servicio.IniciarTrabajo(repositories.WithRequestID(contextoHTTP.Request.Context(), contextoHTTP.GetHeader("X-Request-ID")), entrada)
	if errorObtenido != nil {
		if errors.Is(errorObtenido, services.ErrCadenaNoEncontrada) {
			contextoHTTP.JSON(http.StatusNotFound, gin.H{"error": errorObtenido.Error()})
			return
		}
		if errors.Is(errorObtenido, services.ErrFuenteDeshabilitada) {
			contextoHTTP.JSON(http.StatusConflict, gin.H{"error": errorObtenido.Error(), "codigo": "fuente_deshabilitada"})
			return
		}
		if manejador.responderErrorCatalogo(contextoHTTP, errorObtenido) {
			return
		}
		slog.Error("scraper_start_error", "request_id", contextoHTTP.GetHeader("X-Request-ID"), "error", errorObtenido)
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
	identificador := contextoHTTP.Param("id")

	var entrada domain.FinalizarTrabajoDTO
	if errorObtenido := contextoHTTP.ShouldBindJSON(&entrada); errorObtenido != nil {
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{
			"error":   "Payload inválido para finalizar trabajo",
			"detalle": errorObtenido.Error(),
		})
		return
	}

	trabajo, errorObtenido := manejador.servicio.FinalizarTrabajo(repositories.WithRequestID(contextoHTTP.Request.Context(), contextoHTTP.GetHeader("X-Request-ID")), identificador, entrada)
	if errorObtenido != nil {
		switch {
		case errors.Is(errorObtenido, services.ErrTrabajoNoEncontrado):
			contextoHTTP.JSON(http.StatusNotFound, gin.H{"error": errorObtenido.Error()})
		case errors.Is(errorObtenido, services.ErrTrabajoYaFinalizado):
			contextoHTTP.JSON(http.StatusConflict, gin.H{"error": errorObtenido.Error()})
		case errors.Is(errorObtenido, services.ErrEntregaPendiente):
			contextoHTTP.JSON(http.StatusConflict, gin.H{"error": errorObtenido.Error(), "codigo": "entregas_pendientes"})
		case errors.Is(errorObtenido, services.ErrEntregaFallida):
			contextoHTTP.JSON(http.StatusConflict, gin.H{"error": errorObtenido.Error(), "codigo": "entregas_fallidas"})
		case errors.Is(errorObtenido, services.ErrEstadoTrabajoInvalido), errors.Is(errorObtenido, services.ErrUUIDInvalido):
			contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": errorObtenido.Error()})
		default:
			slog.Error("scraper_finalizar_error", "trabajo_id", identificador, "error", errorObtenido)
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
	identificador := contextoHTTP.Param("id")

	trabajo, errorObtenido := manejador.servicio.ObtenerTrabajo(repositories.WithRequestID(contextoHTTP.Request.Context(), contextoHTTP.GetHeader("X-Request-ID")), identificador)
	if errorObtenido != nil {
		if errors.Is(errorObtenido, services.ErrTrabajoNoEncontrado) {
			contextoHTTP.JSON(http.StatusNotFound, gin.H{"error": errorObtenido.Error()})
			return
		}
		if errors.Is(errorObtenido, services.ErrUUIDInvalido) {
			contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": errorObtenido.Error()})
			return
		}
		slog.Error("scraper_consultar_error", "trabajo_id", identificador, "error", errorObtenido)
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
	identificador := contextoHTTP.Param("id")

	var entrada domain.IngestaLoteDTO
	if !prepararLote(contextoHTTP, &entrada) {
		return
	}
	if errorObtenido := contextoHTTP.ShouldBindJSON(&entrada); errorObtenido != nil {
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{
			"error":   "Payload inválido para ingesta de lote",
			"detalle": errorObtenido.Error(),
		})
		return
	}

	resultado, errorObtenido := manejador.servicio.IngestarProductos(repositories.WithRequestID(contextoHTTP.Request.Context(), contextoHTTP.GetHeader("X-Request-ID")), &identificador, entrada)
	if errorObtenido != nil {
		manejador.manejarErrorIngesta(contextoHTTP, errorObtenido)
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
	var entrada domain.IngestaLoteDTO
	if !prepararLote(contextoHTTP, &entrada) {
		return
	}
	if errorObtenido := contextoHTTP.ShouldBindJSON(&entrada); errorObtenido != nil {
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{
			"error":   "Payload inválido para ingesta de lote",
			"detalle": errorObtenido.Error(),
		})
		return
	}

	resultado, errorObtenido := manejador.servicio.IngestarProductos(repositories.WithRequestID(contextoHTTP.Request.Context(), contextoHTTP.GetHeader("X-Request-ID")), nil, entrada)
	if errorObtenido != nil {
		manejador.manejarErrorIngesta(contextoHTTP, errorObtenido)
		return
	}

	contextoHTTP.JSON(http.StatusOK, resultado)
}

// ===== Validación y respuestas de ingesta =====

// manejarErrorIngesta traduce fallos de dominio y de Catálogo a respuestas HTTP.
func (manejador *ScraperHandler) manejarErrorIngesta(contextoHTTP *gin.Context, errorObtenido error) {
	if manejador.responderErrorCatalogo(contextoHTTP, errorObtenido) {
		return
	}
	switch {
	case errors.Is(errorObtenido, services.ErrFuenteDeshabilitada):
		contextoHTTP.JSON(http.StatusConflict, gin.H{"error": errorObtenido.Error(), "codigo": "fuente_deshabilitada"})
	case errors.Is(errorObtenido, services.ErrEntregaPendiente):
		contextoHTTP.Header("Retry-After", "2")
		contextoHTTP.JSON(http.StatusAccepted, gin.H{"error": errorObtenido.Error(), "codigo": "entrega_pendiente"})
	case errors.Is(errorObtenido, services.ErrEntregaFallida):
		contextoHTTP.JSON(http.StatusConflict, gin.H{"error": errorObtenido.Error(), "codigo": "entrega_fallida"})
	case errors.Is(errorObtenido, services.ErrIdempotencia):
		contextoHTTP.JSON(http.StatusConflict, gin.H{"error": errorObtenido.Error(), "codigo": "idempotency_conflict"})
	case errors.Is(errorObtenido, services.ErrCatalogoRechazo):
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": errorObtenido.Error(), "codigo": "lote_rechazado"})
	case errors.Is(errorObtenido, services.ErrContadorPendiente):
		slog.Error("scraper_counter_pending", "request_id", contextoHTTP.GetHeader("X-Request-ID"), "trabajo_id", contextoHTTP.Param("id"))
		contextoHTTP.JSON(http.StatusConflict, gin.H{"error": errorObtenido.Error(), "codigo": "contador_pendiente", "catalogo_confirmado": true})
	case errors.Is(errorObtenido, services.ErrTrabajoNoEncontrado), errors.Is(errorObtenido, services.ErrSucursalNoEncontrada):
		contextoHTTP.JSON(http.StatusNotFound, gin.H{"error": errorObtenido.Error()})
	case errors.Is(errorObtenido, services.ErrTrabajoYaFinalizado):
		contextoHTTP.JSON(http.StatusConflict, gin.H{"error": errorObtenido.Error()})
	case errors.Is(errorObtenido, services.ErrLoteVacio), errors.Is(errorObtenido, services.ErrLoteExcedeMaximo), errors.Is(errorObtenido, services.ErrUUIDInvalido):
		contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": errorObtenido.Error()})
	default:
		slog.Error("scraper_ingesta_error", "request_id", contextoHTTP.GetHeader("X-Request-ID"), "error", errorObtenido)
		contextoHTTP.JSON(http.StatusInternalServerError, gin.H{"error": "Error interno durante la ingesta"})
	}
}

// prepararLote asigna una clave idempotente y limita el tamaño de la petición.
func prepararLote(contextoHTTP *gin.Context, entrada *domain.IngestaLoteDTO) bool {
	clave := contextoHTTP.GetHeader("Idempotency-Key")
	identificador := uuid.New()
	if clave != "" {
		var errorObtenido error
		identificador, errorObtenido = uuid.Parse(clave)
		if errorObtenido != nil {
			contextoHTTP.JSON(400, gin.H{"error": "Idempotency-Key debe ser un UUID"})
			return false
		}
	}
	entrada.LoteID = identificador.String()
	contextoHTTP.Header("Idempotency-Key", entrada.LoteID)
	contextoHTTP.Request.Body = http.MaxBytesReader(contextoHTTP.Writer, contextoHTTP.Request.Body, 8<<20)
	return true
}

// responderErrorCatalogo traduce errores de disponibilidad de Catálogo a HTTP.
func (manejador *ScraperHandler) responderErrorCatalogo(contextoHTTP *gin.Context, errorObtenido error) bool {
	estadoHTTP, codigo := 0, ""
	if errors.Is(errorObtenido, services.ErrCatalogoNoDisponible) {
		estadoHTTP, codigo = http.StatusServiceUnavailable, "catalogo_unavailable"
	}
	if errors.Is(errorObtenido, services.ErrCatalogoTimeout) {
		estadoHTTP, codigo = http.StatusGatewayTimeout, "catalogo_timeout"
	}
	if estadoHTTP == 0 {
		return false
	}
	contextoHTTP.JSON(estadoHTTP, gin.H{"error": errorObtenido.Error(), "codigo": codigo})
	return true
}
