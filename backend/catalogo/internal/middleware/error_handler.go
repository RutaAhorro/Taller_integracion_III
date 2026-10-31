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

func (e ErrorValidacion) Error() string { return e.Mensaje }

// ErrorHandler es el middleware global que intercepta panics no controlados y errores del contexto
func ErrorHandler(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *gin.Context) {
		c.Set("catalogo_error_logger", logger)
		defer func() {
			if r := recover(); r != nil {
				LogInternalError(c, fmt.Errorf("panic: %v", r))
				if !c.Writer.Written() {
					c.JSON(http.StatusInternalServerError, RespuestaError{
						Estado:  http.StatusInternalServerError,
						Mensaje: "Ocurrió un error interno en el servidor.",
					})
				}
				c.Abort()
			}
		}()

		c.Next()

		// Procesar errores acumulados en el contexto Gin si aún no se ha escrito la respuesta
		if len(c.Errors) > 0 && !c.Writer.Written() {
			err := c.Errors.Last().Err
			var validation ErrorValidacion
			if errors.As(err, &validation) {
				ResponderError(c, http.StatusBadRequest, "Solicitud inválida.", validation)
			} else {
				ResponderError(c, http.StatusInternalServerError, "Ocurrió un error interno en el servidor.", err)
			}
		}
	}
}

// NotFoundHandler genera respuestas estandarizadas 404 para rutas inexistentes
func NotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotFound, RespuestaError{
			Estado:  http.StatusNotFound,
			Mensaje: "La ruta o recurso solicitado no existe.",
			Detalle: fmt.Sprintf("La ruta '%s %s' no está registrada en el servidor.", c.Request.Method, c.Request.URL.Path),
		})
	}
}

// MethodNotAllowedHandler genera respuestas estandarizadas 405 para métodos HTTP no permitidos
func MethodNotAllowedHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, RespuestaError{
			Estado:  http.StatusMethodNotAllowed,
			Mensaje: "El método HTTP utilizado no está permitido para este recurso.",
			Detalle: fmt.Sprintf("El método '%s' no es válido para '%s'.", c.Request.Method, c.Request.URL.Path),
		})
	}
}

// ResponderError es una función utilitaria exportada para emitir errores estandarizados desde cualquier handler
func ResponderError(c *gin.Context, estado int, mensaje string, err error) {
	detalle := ""
	var validation ErrorValidacion
	if estado == http.StatusBadRequest && errors.As(err, &validation) {
		detalle = validation.Mensaje
	}
	if estado >= http.StatusInternalServerError {
		LogInternalError(c, err)
		mensaje = "Ocurrió un error interno en el servidor."
	}

	c.JSON(estado, RespuestaError{
		Estado:  estado,
		Mensaje: mensaje,
		Detalle: detalle,
	})
	c.Abort()
}

// LogInternalError conserva el diagnóstico sólo en logs, junto a la correlación.
func LogInternalError(c *gin.Context, err error) {
	logger, ok := c.Get("catalogo_error_logger")
	log, valid := logger.(*slog.Logger)
	if !ok || !valid {
		log = slog.Default()
	}
	path := c.FullPath()
	if path == "" {
		path = "unmatched"
	}
	log.ErrorContext(c.Request.Context(), "http_internal_error", "request_id", c.Writer.Header().Get("X-Request-ID"),
		"method", c.Request.Method, "path", path, "error", err)
}
