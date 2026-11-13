package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RespuestaError representa la estructura estandarizada de respuestas de error en JSON para la API
type RespuestaError struct {
	Estado  int    `json:"estado"`            // Código numérico del estado HTTP (400, 404, 500, etc.)
	Mensaje string `json:"mensaje"`           // Mensaje descriptivo amigable para el cliente
	Detalle string `json:"detalle,omitempty"` // Sólo información aprobada para el cliente
}

// ErrorValidacion identifica mensajes de validación seguros para publicar.
// Nunca debe construirse con errores de infraestructura ni datos sensibles.
type ErrorValidacion struct{ Mensaje string }

func (errorValidacion ErrorValidacion) Error() string { return errorValidacion.Mensaje }

// ErrorHandler es el middleware global que intercepta panics no controlados y errores del contexto
func ErrorHandler(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(contexto *gin.Context) {
		contexto.Set("catalogo_error_logger", logger)
		defer func() {
			if valorRecuperado := recover(); valorRecuperado != nil {
				LogInternalError(contexto, fmt.Errorf("panic: %v", valorRecuperado))
				if !contexto.Writer.Written() {
					contexto.JSON(http.StatusInternalServerError, RespuestaError{
						Estado:  http.StatusInternalServerError,
						Mensaje: "Ocurrió un error interno en el servidor.",
					})
				}
				contexto.Abort()
			}
		}()

		contexto.Next()

		// Procesar errores acumulados en el contexto Gin si aún no se ha escrito la respuesta
		if len(contexto.Errors) > 0 && !contexto.Writer.Written() {
			err := contexto.Errors.Last().Err
			var validation ErrorValidacion
			if errors.As(err, &validation) {
				ResponderError(contexto, http.StatusBadRequest, "Solicitud inválida.", validation)
			} else {
				ResponderError(contexto, http.StatusInternalServerError, "Ocurrió un error interno en el servidor.", err)
			}
		}
	}
}

// NotFoundHandler genera respuestas estandarizadas 404 para rutas inexistentes
func NotFoundHandler() gin.HandlerFunc {
	return func(contexto *gin.Context) {
		contexto.JSON(http.StatusNotFound, RespuestaError{
			Estado:  http.StatusNotFound,
			Mensaje: "La ruta o recurso solicitado no existe.",
			Detalle: fmt.Sprintf("La ruta '%s %s' no está registrada en el servidor.", contexto.Request.Method, contexto.Request.URL.Path),
		})
	}
}

// MethodNotAllowedHandler genera respuestas estandarizadas 405 para métodos HTTP no permitidos
func MethodNotAllowedHandler() gin.HandlerFunc {
	return func(contexto *gin.Context) {
		contexto.JSON(http.StatusMethodNotAllowed, RespuestaError{
			Estado:  http.StatusMethodNotAllowed,
			Mensaje: "El método HTTP utilizado no está permitido para este recurso.",
			Detalle: fmt.Sprintf("El método '%s' no es válido para '%s'.", contexto.Request.Method, contexto.Request.URL.Path),
		})
	}
}

// ResponderError es una función utilitaria exportada para emitir errores estandarizados desde cualquier handler
func ResponderError(contexto *gin.Context, estado int, mensaje string, err error) {
	detalle := ""
	var validation ErrorValidacion
	if estado == http.StatusBadRequest && errors.As(err, &validation) {
		detalle = validation.Mensaje
	}
	if estado >= http.StatusInternalServerError {
		LogInternalError(contexto, err)
		mensaje = "Ocurrió un error interno en el servidor."
	}

	contexto.JSON(estado, RespuestaError{
		Estado:  estado,
		Mensaje: mensaje,
		Detalle: detalle,
	})
	contexto.Abort()
}

// LogInternalError conserva el diagnóstico sólo en logs, junto a la correlación.
func LogInternalError(contexto *gin.Context, err error) {
	logger, ok := contexto.Get("catalogo_error_logger")
	log, valid := logger.(*slog.Logger)
	if !ok || !valid {
		log = slog.Default()
	}
	path := contexto.FullPath()
	if path == "" {
		path = "unmatched"
	}
	log.ErrorContext(contexto.Request.Context(), "http_internal_error", "request_id", contexto.Writer.Header().Get("X-Request-ID"),
		"method", contexto.Request.Method, "path", path, "error", err)
}
