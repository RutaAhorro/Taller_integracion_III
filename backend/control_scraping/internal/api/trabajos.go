// Handlers del ciclo de vida de trabajos: iniciar, consultar, finalizar y
// encolar su ejecución para Scraper Go.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/gin-gonic/gin"
)

// ===== Spiders y payload de ejecución =====

// spiderPredeterminado se usa si la solicitud no indica spider; el bot siempre
// lo envía (ver SUP-394).
const spiderPredeterminado = "jumbo_rsc"

// spidersPermitidos son los spiders que Scraper Go registra en spiders.json.
var spidersPermitidos = map[string]bool{
	"jumbo_rsc":        true,
	"santa_isabel_rsc": true,
	"lider_rsc":        true,
	"acuenta_rsc":      true,
	"cugat_rsc":        true,
}

// solicitudEjecucion son los parámetros opcionales para encolar un spider.
type solicitudEjecucion struct {
	Spider      string  `json:"spider,omitempty"`
	ProductoURL *string `json:"producto_url,omitempty"`
	ProductoID  *string `json:"producto_id,omitempty"`
	AddURL      *string `json:"add_url,omitempty"`
	EnrichEAN   *bool   `json:"enrich_ean,omitempty"`
}

// ===== Ejecución =====

// EjecutarTrabajo godoc
// @Summary      Encola un trabajo para ejecución
// @Description  Guarda la ejecución en el outbox SQL; Control la publica en Redis Streams para que Scraper Go ejecute el spider solicitado.
// @Tags         scraper
// @Security     TokenServicio
// @Accept       json
// @Produce      json
// @Param        id     path      string              true  "UUID del trabajo"
// @Param        input  body      solicitudEjecucion  true  "Spider y parámetros opcionales"
// @Success      202    {object}  map[string]interface{}
// @Failure      400    {object}  map[string]interface{}
// @Failure      401    {object}  RespuestaError
// @Failure      404    {object}  map[string]interface{}
// @Failure      409    {object}  map[string]interface{}
// @Failure      503    {object}  map[string]interface{}
// @Failure      504    {object}  map[string]interface{}
// @Router       /api/v1/scraper/trabajos/{id}/ejecutar [post]
func (modulo *Modulo) EjecutarTrabajo(contexto *gin.Context) {
	contextoPeticion := contextoConIDPeticion(contexto)
	trabajo, errorObtenido := modulo.trabajos.ObtenerTrabajo(contextoPeticion, contexto.Param("id"))
	if errorObtenido != nil {
		switch {
		case errors.Is(errorObtenido, domain.ErrTrabajoNoEncontrado):
			responderFallo(contexto, http.StatusNotFound, errorObtenido.Error(), "")
		case errors.Is(errorObtenido, domain.ErrUUIDInvalido):
			responderFallo(contexto, http.StatusBadRequest, errorObtenido.Error(), "")
		default:
			responderFallo(contexto, http.StatusInternalServerError, "Error interno al consultar trabajo", "")
		}
		return
	}
	if trabajo.Estado != domain.EstadoEnProgreso {
		responderFallo(contexto, http.StatusConflict, "El trabajo no está disponible para ejecución", "")
		return
	}
	cadena, errorObtenido := modulo.trabajos.ObtenerCadena(contextoPeticion, trabajo.CadenaID, trabajo.CadenaNombre)
	if errorObtenido != nil {
		if errors.Is(errorObtenido, domain.ErrCadenaNoEncontrada) {
			responderFallo(contexto, http.StatusNotFound, errorObtenido.Error(), "")
			return
		}
		if responderErrorCatalogo(contexto, errorObtenido) {
			return
		}
		slog.Error("scraper_chain_lookup_error", "request_id", contexto.GetHeader(cabeceraIDPeticion), "trabajo_id", trabajo.ID, "error", errorObtenido)
		responderFallo(contexto, http.StatusInternalServerError, "Error interno al consultar la fuente", "")
		return
	}
	if !cadena.EstaActiva {
		responderFallo(contexto, http.StatusConflict, domain.ErrFuenteDeshabilitada.Error(), codigoFuenteDeshabilitada)
		return
	}
	var entrada solicitudEjecucion
	if errorObtenido := contexto.ShouldBindJSON(&entrada); errorObtenido != nil {
		responderPayloadInvalido(contexto, "Payload inválido para ejecutar scraper", errorObtenido)
		return
	}
	carga, mensajeInvalido := construirCargaEjecucion(trabajo.ID, &entrada)
	if mensajeInvalido != "" {
		responderFallo(contexto, http.StatusBadRequest, mensajeInvalido, "")
		return
	}
	if errorObtenido := modulo.ejecuciones.Encolar(contexto.Request.Context(), trabajo.ID, carga); errorObtenido != nil {
		if errors.Is(errorObtenido, domain.ErrIdempotencia) || errors.Is(errorObtenido, domain.ErrTrabajoYaFinalizado) {
			responderFallo(contexto, http.StatusConflict, errorObtenido.Error(), codigoConflictoEjecucion)
			return
		}
		slog.Error("scraper_encolar_error", "trabajo_id", trabajo.ID, "error", errorObtenido)
		responderFallo(contexto, http.StatusServiceUnavailable, "No se pudo registrar el trabajo", "")
		return
	}
	contexto.Header(cabeceraCola, valorCabeceraCola)
	contexto.JSON(http.StatusAccepted, gin.H{"trabajo_id": trabajo.ID, "estado": "encolado", "spider": entrada.Spider})
}

// construirCargaEjecucion valida la solicitud y arma el payload del stream.
// Las claves product_url y product_id están en inglés porque las lee Scraper Go.
// Normaliza el spider en la solicitud (la respuesta 202 lo informa) y
// devuelve un mensaje para el 400 si la solicitud es inválida.
func construirCargaEjecucion(trabajoID string, entrada *solicitudEjecucion) ([]byte, string) {
	entrada.Spider = strings.TrimSpace(entrada.Spider)
	if entrada.Spider == "" {
		entrada.Spider = spiderPredeterminado
	}
	if !spidersPermitidos[entrada.Spider] {
		return nil, "Spider no permitido"
	}
	if entrada.ProductoID != nil && entrada.ProductoURL == nil {
		return nil, "producto_id requiere producto_url"
	}
	datosTrabajo := map[string]any{"trabajo_id": trabajoID, "spider": entrada.Spider}
	if entrada.ProductoURL != nil {
		datosTrabajo["product_url"] = *entrada.ProductoURL
	}
	if entrada.ProductoID != nil {
		datosTrabajo["product_id"] = *entrada.ProductoID
	}
	if entrada.AddURL != nil {
		// El worker valida host y esquema por cadena; aquí sólo se evita
		// encolar un valor vacío.
		urlAdicional := strings.TrimSpace(*entrada.AddURL)
		if urlAdicional == "" {
			return nil, "add_url no puede estar vacío"
		}
		datosTrabajo["add_url"] = urlAdicional
	}
	if entrada.EnrichEAN != nil {
		datosTrabajo["enrich_ean"] = *entrada.EnrichEAN
	}
	carga, _ := json.Marshal(datosTrabajo)
	return carga, ""
}

// ===== Ciclo de vida =====

// IniciarTrabajo godoc
// @Summary      Inicia una sesión de scraping
// @Description  Registra el inicio de un trabajo para auditar su ciclo de vida y métricas
// @Tags         scraper
// @Security     TokenServicio
// @Accept       json
// @Produce      json
// @Param        input  body      domain.IniciarTrabajoDTO  true  "Datos de inicio de la sesión"
// @Success      201    {object}  domain.TrabajoScraperDTO
// @Failure      400    {object}  map[string]interface{}
// @Failure      401    {object}  RespuestaError
// @Failure      404    {object}  map[string]interface{}
// @Failure      409    {object}  map[string]interface{}
// @Failure      500    {object}  map[string]interface{}
// @Failure      503    {object}  map[string]interface{}
// @Failure      504    {object}  map[string]interface{}
// @Router       /api/v1/scraper/trabajos [post]
func (modulo *Modulo) IniciarTrabajo(contexto *gin.Context) {
	var entrada domain.IniciarTrabajoDTO
	if errorObtenido := contexto.ShouldBindJSON(&entrada); errorObtenido != nil {
		responderPayloadInvalido(contexto, "Payload inválido para iniciar trabajo", errorObtenido)
		return
	}
	trabajo, errorObtenido := modulo.trabajos.IniciarTrabajo(contextoConIDPeticion(contexto), entrada)
	if errorObtenido != nil {
		switch {
		case errors.Is(errorObtenido, domain.ErrCadenaNoEncontrada):
			responderFallo(contexto, http.StatusNotFound, errorObtenido.Error(), "")
		case errors.Is(errorObtenido, domain.ErrFuenteDeshabilitada):
			responderFallo(contexto, http.StatusConflict, errorObtenido.Error(), codigoFuenteDeshabilitada)
		default:
			if responderErrorCatalogo(contexto, errorObtenido) {
				return
			}
			slog.Error("scraper_start_error", "request_id", contexto.GetHeader(cabeceraIDPeticion), "error", errorObtenido)
			responderFallo(contexto, http.StatusInternalServerError, "Error interno al iniciar trabajo", "")
		}
		return
	}
	contexto.JSON(http.StatusCreated, trabajo)
}

// FinalizarTrabajo godoc
// @Summary      Finaliza una sesión de scraping
// @Description  Marca el trabajo como completado o fallido. Repetir el mismo cierre devuelve el trabajo sin cambios.
// @Tags         scraper
// @Security     TokenServicio
// @Accept       json
// @Produce      json
// @Param        id     path      string                      true  "UUID del trabajo de scraping"
// @Param        input  body      domain.FinalizarTrabajoDTO  true  "Datos de cierre de la sesión"
// @Success      200    {object}  domain.TrabajoScraperDTO
// @Failure      400    {object}  map[string]interface{}
// @Failure      401    {object}  RespuestaError
// @Failure      404    {object}  map[string]interface{}
// @Failure      409    {object}  map[string]interface{}
// @Failure      500    {object}  map[string]interface{}
// @Router       /api/v1/scraper/trabajos/{id}/finalizar [put]
func (modulo *Modulo) FinalizarTrabajo(contexto *gin.Context) {
	identificador := contexto.Param("id")
	var entrada domain.FinalizarTrabajoDTO
	if errorObtenido := contexto.ShouldBindJSON(&entrada); errorObtenido != nil {
		responderPayloadInvalido(contexto, "Payload inválido para finalizar trabajo", errorObtenido)
		return
	}
	trabajo, errorObtenido := modulo.trabajos.FinalizarTrabajo(contextoConIDPeticion(contexto), identificador, entrada)
	if errorObtenido != nil {
		switch {
		case errors.Is(errorObtenido, domain.ErrTrabajoNoEncontrado):
			responderFallo(contexto, http.StatusNotFound, errorObtenido.Error(), "")
		case errors.Is(errorObtenido, domain.ErrTrabajoYaFinalizado):
			responderFallo(contexto, http.StatusConflict, errorObtenido.Error(), "")
		case errors.Is(errorObtenido, domain.ErrEntregaPendiente):
			responderFallo(contexto, http.StatusConflict, errorObtenido.Error(), codigoEntregasPendientes)
		case errors.Is(errorObtenido, domain.ErrEntregaFallida):
			responderFallo(contexto, http.StatusConflict, errorObtenido.Error(), codigoEntregasFallidas)
		case errors.Is(errorObtenido, domain.ErrEstadoTrabajoInvalido), errors.Is(errorObtenido, domain.ErrUUIDInvalido):
			responderFallo(contexto, http.StatusBadRequest, errorObtenido.Error(), "")
		default:
			slog.Error("scraper_finalizar_error", "trabajo_id", identificador, "error", errorObtenido)
			responderFallo(contexto, http.StatusInternalServerError, "Error interno al finalizar trabajo", "")
		}
		return
	}
	contexto.JSON(http.StatusOK, trabajo)
}

// ObtenerTrabajo godoc
// @Summary      Consulta el estado de una sesión de scraping
// @Description  Devuelve detalles, duración, conteo y errores de un trabajo (lo usa el bot de Discord)
// @Tags         scraper
// @Security     TokenServicio
// @Produce      json
// @Param        id   path      string  true  "UUID del trabajo"
// @Success      200  {object}  domain.TrabajoScraperDTO
// @Failure      400  {object}  map[string]interface{}
// @Failure      401  {object}  RespuestaError
// @Failure      404  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /api/v1/scraper/trabajos/{id} [get]
func (modulo *Modulo) ObtenerTrabajo(contexto *gin.Context) {
	identificador := contexto.Param("id")
	trabajo, errorObtenido := modulo.trabajos.ObtenerTrabajo(contextoConIDPeticion(contexto), identificador)
	if errorObtenido != nil {
		switch {
		case errors.Is(errorObtenido, domain.ErrTrabajoNoEncontrado):
			responderFallo(contexto, http.StatusNotFound, errorObtenido.Error(), "")
		case errors.Is(errorObtenido, domain.ErrUUIDInvalido):
			responderFallo(contexto, http.StatusBadRequest, errorObtenido.Error(), "")
		default:
			slog.Error("scraper_consultar_error", "trabajo_id", identificador, "error", errorObtenido)
			responderFallo(contexto, http.StatusInternalServerError, "Error interno al consultar trabajo", "")
		}
		return
	}
	contexto.JSON(http.StatusOK, trabajo)
}
