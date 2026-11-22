// Formato de errores HTTP. Conviven dos formatos por compatibilidad: los
// middlewares (CORS, token, 404, 405, panic) responden RespuestaError y los
// endpoints de scraping responden {"error", "codigo"}, que leen el bot y
// Scraper Go. Unificarlos es un cambio de contrato (ver SUP-394).
package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ===== Errores de middlewares =====

// mensajeErrorInterno nunca incluye el detalle del fallo.
const mensajeErrorInterno = "Ocurrió un error interno en el servidor."

// RespuestaError es el cuerpo de error de los middlewares y rutas inexistentes.
type RespuestaError struct {
	Estado  int    `json:"estado"`
	Mensaje string `json:"mensaje"`
	Detalle string `json:"detalle,omitempty"`
}

// responderError escribe un RespuestaError y detiene la cadena de handlers.
func responderError(contexto *gin.Context, estado int, mensaje, detalle string) {
	contexto.AbortWithStatusJSON(estado, RespuestaError{Estado: estado, Mensaje: mensaje, Detalle: detalle})
}

// recuperarPanicos responde 500 ante un panic sin filtrar su contenido.
func recuperarPanicos() gin.HandlerFunc {
	return func(contexto *gin.Context) {
		defer func() {
			if panicoRecuperado := recover(); panicoRecuperado != nil {
				slog.Error("scraping_panic", "request_id", contexto.GetString(claveIDPeticion), "error", panicoRecuperado)
				if !contexto.Writer.Written() {
					responderError(contexto, http.StatusInternalServerError, mensajeErrorInterno, "")
				}
				contexto.Abort()
			}
		}()
		contexto.Next()
	}
}

// rutaNoEncontrada responde 404 con la ruta pedida.
func rutaNoEncontrada(contexto *gin.Context) {
	responderError(contexto, http.StatusNotFound, "La ruta o recurso solicitado no existe.",
		fmt.Sprintf("La ruta '%s %s' no está registrada en el servidor.", contexto.Request.Method, contexto.Request.URL.Path))
}

// metodoNoPermitido responde 405 con el método rechazado.
func metodoNoPermitido(contexto *gin.Context) {
	responderError(contexto, http.StatusMethodNotAllowed, "El método HTTP utilizado no está permitido para este recurso.",
		fmt.Sprintf("El método '%s' no es válido para '%s'.", contexto.Request.Method, contexto.Request.URL.Path))
}

// ===== Errores de endpoints de scraping =====

// Códigos del campo "codigo" que no provienen de domain.
const (
	codigoFuenteDeshabilitada  = "fuente_deshabilitada"
	codigoEntregasPendientes   = "entregas_pendientes"
	codigoEntregasFallidas     = "entregas_fallidas"
	codigoEntregaPendiente     = "entrega_pendiente"
	codigoEntregaFallida       = "entrega_fallida"
	codigoConflictoClave       = "idempotency_conflict"
	codigoConflictoEjecucion   = "ejecucion_conflict"
	codigoTimeoutCatalogo      = "catalogo_timeout"
	segundosReintentoEntrega   = "2"
	cabeceraReintento          = "Retry-After"
	cabeceraCola               = "X-Scraper-Queue"
	valorCabeceraCola          = "durable-stream"
	cabeceraClaveIdempotencia  = "Idempotency-Key"
	cabeceraIDPeticion         = "X-Request-ID"
	campoCatalogoConfirmado    = "catalogo_confirmado"
	mensajeErrorIngestaInterno = "Error interno durante la ingesta"
)

// responderFallo escribe {"error": mensaje} y agrega "codigo" si se indica.
func responderFallo(contexto *gin.Context, estado int, mensaje, codigo string) {
	cuerpo := gin.H{"error": mensaje}
	if codigo != "" {
		cuerpo["codigo"] = codigo
	}
	contexto.JSON(estado, cuerpo)
}

// responderPayloadInvalido escribe el 400 de un cuerpo JSON que no valida.
func responderPayloadInvalido(contexto *gin.Context, mensaje string, errorObtenido error) {
	contexto.JSON(http.StatusBadRequest, gin.H{"error": mensaje, "detalle": errorObtenido.Error()})
}
