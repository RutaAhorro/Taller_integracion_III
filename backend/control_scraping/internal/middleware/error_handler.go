package middleware

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RespuestaError representa la estructura estandarizada de respuestas de error en JSON para la API
type RespuestaError struct {
	Estado  int    `json:"estado"`            // Código numérico del estado HTTP (400, 404, 500, etc.)
	Mensaje string `json:"mensaje"`           // Mensaje descriptivo amigable para el cliente
	Detalle string `json:"detalle,omitempty"` // Información técnica u origen del error
}

// ErrorHandler es el middleware global que intercepta panics no controlados y errores del contexto
func ErrorHandler() gin.HandlerFunc {
	return func(contextoHTTP *gin.Context) {
		defer func() {
			if panicoRecuperado := recover(); panicoRecuperado != nil {
				// Captura cualquier panic no controlado en el servidor
				slog.Error("scraping_panic", "request_id", contextoHTTP.GetString("request_id"), "error", panicoRecuperado)

				// Responder con estado HTTP 500 estandarizado
				if !contextoHTTP.Writer.Written() {
					contextoHTTP.JSON(http.StatusInternalServerError, RespuestaError{
						Estado:  http.StatusInternalServerError,
						Mensaje: "Ocurrió un error interno en el servidor.",
					})
				}

				// Detener la ejecución del pipeline de handlers
				contextoHTTP.Abort()
			}
		}()

		contextoHTTP.Next()

		// Procesar errores acumulados en el contexto Gin si aún no se ha escrito la respuesta
		if len(contextoHTTP.Errors) > 0 && !contextoHTTP.Writer.Written() {
			err := contextoHTTP.Errors.Last()
			slog.Error("scraping_context_error", "request_id", contextoHTTP.GetString("request_id"), "error", err.Err)

			contextoHTTP.JSON(http.StatusInternalServerError, RespuestaError{
				Estado:  http.StatusInternalServerError,
				Mensaje: "Ocurrió un error interno en el servidor.",
			})
		}
	}
}

// NotFoundHandler genera respuestas estandarizadas 404 para rutas inexistentes
func NotFoundHandler() gin.HandlerFunc {
	return func(contextoHTTP *gin.Context) {
		contextoHTTP.JSON(http.StatusNotFound, RespuestaError{
			Estado:  http.StatusNotFound,
			Mensaje: "La ruta o recurso solicitado no existe.",
			Detalle: fmt.Sprintf("La ruta '%s %s' no está registrada en el servidor.", contextoHTTP.Request.Method, contextoHTTP.Request.URL.Path),
		})
	}
}

// MethodNotAllowedHandler genera respuestas estandarizadas 405 para métodos HTTP no permitidos
func MethodNotAllowedHandler() gin.HandlerFunc {
	return func(contextoHTTP *gin.Context) {
		contextoHTTP.JSON(http.StatusMethodNotAllowed, RespuestaError{
			Estado:  http.StatusMethodNotAllowed,
			Mensaje: "El método HTTP utilizado no está permitido para este recurso.",
			Detalle: fmt.Sprintf("El método '%s' no es válido para '%s'.", contextoHTTP.Request.Method, contextoHTTP.Request.URL.Path),
		})
	}
}

// ResponderError es una función utilitaria exportada para emitir errores estandarizados desde cualquier handler
func ResponderError(contextoHTTP *gin.Context, estado int, mensaje string, err error) {
	detalle := ""
	if err != nil && estado == http.StatusBadRequest {
		detalle = err.Error()
	}

	if err != nil && estado >= 500 {
		slog.Error("scraping_http_error", "request_id", contextoHTTP.GetString("request_id"), "status", estado, "error", err)
	}
	contextoHTTP.JSON(estado, RespuestaError{
		Estado:  estado,
		Mensaje: mensaje,
		Detalle: detalle,
	})
	contextoHTTP.Abort()
}
