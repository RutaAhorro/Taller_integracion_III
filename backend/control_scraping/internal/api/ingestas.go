// Handlers de recepción de lotes de productos (con trabajo o directos) y la
// traducción de errores de ingesta y de Catálogo a respuestas HTTP.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ===== Endpoints de ingesta =====

// IngestarProductosConTrabajo godoc
// @Summary      Registra y entrega un lote con trabajo a Catálogo
// @Description  Conserva la entrega durable en Control, usa una clave estable en Catálogo y confirma el contador una sola vez. Responde 202 con Retry-After si el lote sigue en curso.
// @Tags         scraper
// @Security     TokenServicio
// @Accept       json
// @Produce      json
// @Param        id               path      string                 true   "UUID del trabajo de scraping"
// @Param        Idempotency-Key  header    string                 false  "UUID estable del lote"
// @Param        input            body      domain.IngestaLoteDTO  true   "Lote de productos extraídos"
// @Success      200    {object}  domain.IngestaResultadoDTO
// @Success      202    {object}  map[string]interface{}
// @Failure      400    {object}  map[string]interface{}
// @Failure      401    {object}  RespuestaError
// @Failure      404    {object}  map[string]interface{}
// @Failure      409    {object}  map[string]interface{}
// @Failure      500    {object}  map[string]interface{}
// @Failure      503    {object}  map[string]interface{}
// @Failure      504    {object}  map[string]interface{}
// @Router       /api/v1/scraper/trabajos/{id}/productos [post]
func (modulo *Modulo) IngestarProductosConTrabajo(contexto *gin.Context) {
	trabajoID := contexto.Param("id")
	modulo.ingestar(contexto, &trabajoID)
}

// IngestarProductosDirecto godoc
// @Summary      Registra y entrega un lote sin trabajo a Catálogo
// @Description  Igual que la ingesta con trabajo; la cadena se resuelve por supermercado o por la sucursal indicada.
// @Tags         scraper
// @Security     TokenServicio
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header    string                 false  "UUID estable del lote"
// @Param        input            body      domain.IngestaLoteDTO  true   "Lote de productos extraídos"
// @Success      200    {object}  domain.IngestaResultadoDTO
// @Success      202    {object}  map[string]interface{}
// @Failure      400    {object}  map[string]interface{}
// @Failure      401    {object}  RespuestaError
// @Failure      404    {object}  map[string]interface{}
// @Failure      409    {object}  map[string]interface{}
// @Failure      500    {object}  map[string]interface{}
// @Failure      503    {object}  map[string]interface{}
// @Failure      504    {object}  map[string]interface{}
// @Router       /api/v1/scraper/productos [post]
func (modulo *Modulo) IngestarProductosDirecto(contexto *gin.Context) {
	modulo.ingestar(contexto, nil)
}

// ingestar asigna la clave del lote, valida el cuerpo y delega en el servicio.
func (modulo *Modulo) ingestar(contexto *gin.Context, trabajoID *string) {
	var entrada domain.IngestaLoteDTO
	if !asignarClaveLote(contexto, &entrada) {
		return
	}
	if errorObtenido := contexto.ShouldBindJSON(&entrada); errorObtenido != nil {
		responderPayloadInvalido(contexto, "Payload inválido para ingesta de lote", errorObtenido)
		return
	}
	resultado, errorObtenido := modulo.entregas.IngestarProductos(contextoConIDPeticion(contexto), trabajoID, entrada)
	if errorObtenido != nil {
		responderErrorIngesta(contexto, errorObtenido)
		return
	}
	contexto.JSON(http.StatusOK, resultado)
}

// asignarClaveLote toma el Idempotency-Key (o genera uno) y lo devuelve en la
// respuesta para que el cliente reintente con la misma clave.
func asignarClaveLote(contexto *gin.Context, entrada *domain.IngestaLoteDTO) bool {
	clave := uuid.New()
	if recibida := contexto.GetHeader(cabeceraClaveIdempotencia); recibida != "" {
		analizada, errorObtenido := uuid.Parse(recibida)
		if errorObtenido != nil {
			responderFallo(contexto, http.StatusBadRequest, cabeceraClaveIdempotencia+" debe ser un UUID", "")
			return false
		}
		clave = analizada
	}
	entrada.LoteID = clave.String()
	contexto.Header(cabeceraClaveIdempotencia, entrada.LoteID)
	return true
}

// ===== Traducción de errores =====

// responderErrorIngesta traduce fallos de dominio y de Catálogo a HTTP.
func responderErrorIngesta(contexto *gin.Context, errorObtenido error) {
	if responderErrorCatalogo(contexto, errorObtenido) {
		return
	}
	mensaje := errorObtenido.Error()
	switch {
	case errors.Is(errorObtenido, domain.ErrFuenteDeshabilitada):
		responderFallo(contexto, http.StatusConflict, mensaje, codigoFuenteDeshabilitada)
	case errors.Is(errorObtenido, domain.ErrEntregaPendiente):
		contexto.Header(cabeceraReintento, segundosReintentoEntrega)
		responderFallo(contexto, http.StatusAccepted, mensaje, codigoEntregaPendiente)
	case errors.Is(errorObtenido, domain.ErrEntregaFallida):
		responderFallo(contexto, http.StatusConflict, mensaje, codigoEntregaFallida)
	case errors.Is(errorObtenido, domain.ErrIdempotencia):
		responderFallo(contexto, http.StatusConflict, mensaje, codigoConflictoClave)
	case errors.Is(errorObtenido, domain.ErrCatalogoRechazo):
		responderFallo(contexto, http.StatusBadRequest, mensaje, domain.CodigoLoteRechazado)
	case errors.Is(errorObtenido, domain.ErrContadorPendiente):
		slog.Error("scraper_counter_pending", "request_id", contexto.GetHeader(cabeceraIDPeticion), "trabajo_id", contexto.Param("id"))
		contexto.JSON(http.StatusConflict, gin.H{"error": mensaje, "codigo": domain.CodigoContadorPendiente, campoCatalogoConfirmado: true})
	case errors.Is(errorObtenido, domain.ErrTrabajoNoEncontrado), errors.Is(errorObtenido, domain.ErrSucursalNoEncontrada):
		responderFallo(contexto, http.StatusNotFound, mensaje, "")
	case errors.Is(errorObtenido, domain.ErrTrabajoYaFinalizado):
		responderFallo(contexto, http.StatusConflict, mensaje, "")
	case errors.Is(errorObtenido, domain.ErrLoteVacio), errors.Is(errorObtenido, domain.ErrLoteExcedeMaximo), errors.Is(errorObtenido, domain.ErrUUIDInvalido):
		responderFallo(contexto, http.StatusBadRequest, mensaje, "")
	default:
		slog.Error("scraper_ingesta_error", "request_id", contexto.GetHeader(cabeceraIDPeticion), "error", errorObtenido)
		responderFallo(contexto, http.StatusInternalServerError, mensajeErrorIngestaInterno, "")
	}
}

// responderErrorCatalogo responde 503 o 504 si Catálogo no estuvo disponible
// y devuelve true si escribió la respuesta.
func responderErrorCatalogo(contexto *gin.Context, errorObtenido error) bool {
	switch {
	case errors.Is(errorObtenido, domain.ErrCatalogoTimeout):
		responderFallo(contexto, http.StatusGatewayTimeout, errorObtenido.Error(), codigoTimeoutCatalogo)
	case errors.Is(errorObtenido, domain.ErrCatalogoNoDisponible):
		responderFallo(contexto, http.StatusServiceUnavailable, errorObtenido.Error(), domain.CodigoCatalogoNoDisponible)
	default:
		return false
	}
	return true
}

// contextoConIDPeticion propaga a Catálogo el X-Request-ID recibido.
func contextoConIDPeticion(contexto *gin.Context) context.Context {
	return domain.ConIDPeticion(contexto.Request.Context(), contexto.GetHeader(cabeceraIDPeticion))
}
