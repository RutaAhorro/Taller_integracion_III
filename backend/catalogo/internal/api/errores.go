// Errores HTTP: formato común {estado, mensaje, detalle}, recuperación de
// panics y registro de fallos internos sin filtrarlos al cliente.

package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ===== Formato =====

// mensajeErrorInterno es el único texto que ve el cliente ante un 5xx.
const mensajeErrorInterno = "Ocurrió un error interno en el servidor."

// claveRegistrador guarda el logger en el contexto Gin para los handlers.
const claveRegistrador = "catalogo_registrador"

// RespuestaError es el cuerpo JSON de error de las rutas públicas y admin.
type RespuestaError struct {
	Estado  int    `json:"estado"`
	Mensaje string `json:"mensaje"`
	Detalle string `json:"detalle,omitempty"`
}

// responderError escribe el error y corta la cadena de middlewares. Un 5xx
// reemplaza el mensaje por uno genérico; el detalle sólo se publica en 4xx.
func responderError(contexto *gin.Context, estado int, mensaje, detalle string) {
	if estado >= http.StatusInternalServerError {
		mensaje, detalle = mensajeErrorInterno, ""
	}
	contexto.AbortWithStatusJSON(estado, RespuestaError{Estado: estado, Mensaje: mensaje, Detalle: detalle})
}

// ===== Middlewares =====

// manejarErrores recupera panics como 500 genérico y deja el diagnóstico sólo
// en los logs, junto a la correlación.
func manejarErrores(registrador *slog.Logger) gin.HandlerFunc {
	return func(contexto *gin.Context) {
		contexto.Set(claveRegistrador, registrador)
		defer func() {
			if valorRecuperado := recover(); valorRecuperado != nil {
				registrarErrorInterno(contexto, fmt.Errorf("panic: %v", valorRecuperado))
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

// ===== Registro =====

// registrarErrorInterno escribe el error con request_id y ruta registrada.
func registrarErrorInterno(contexto *gin.Context, errorInterno error) {
	registrador, _ := contexto.MustGet(claveRegistrador).(*slog.Logger)
	registrador.ErrorContext(contexto.Request.Context(), "http_internal_error", "request_id", contexto.Writer.Header().Get("X-Request-ID"),
		"method", contexto.Request.Method, "path", rutaRegistrada(contexto), "error", errorInterno)
}
