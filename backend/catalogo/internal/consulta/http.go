// Package consulta expone la consulta interna de disponibilidad que usan Listas
// e IA. Vive fuera de /api/v1 para que el Gateway no la publique y comparte el
// token de servicio de la ingesta.
package consulta

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/middleware"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
)

// Config agrupa las dependencias de la ruta interna. Si Token está vacío, la
// ruta no se registra y Catálogo queda en modo sólo lectura pública.
type Config struct {
	Service      services.DisponibilidadService
	Token        string
	MaxBodyBytes int
}

// Register monta POST /internal/v1/productos/disponibilidad sobre el router ya
// construido. El token se valida en tiempo constante y se rechaza cualquier
// petición con Origin de navegador.
func Register(enrutador *gin.Engine, cfg Config, logger *slog.Logger) {
	if cfg.Token == "" || cfg.Service == nil {
		return
	}
	if cfg.MaxBodyBytes < 1 {
		panic("consulta.Config.MaxBodyBytes debe ser positivo")
	}
	if logger == nil {
		logger = slog.Default()
	}

	group := enrutador.Group("/internal/v1", middleware.RequireInternalToken(cfg.Token))
	group.POST("/productos/disponibilidad", func(contexto *gin.Context) {
		var consulta domain.ConsultaDisponibilidadDTO
		if !decodificarCuerpo(contexto, &consulta, cfg.MaxBodyBytes) {
			return
		}

		respuesta, errorConsulta := cfg.Service.Consultar(contexto.Request.Context(), consulta.ProductoIDs)
		if errorConsulta != nil {
			responderError(contexto, errorConsulta, logger)
			return
		}
		contexto.JSON(http.StatusOK, respuesta)
	})
}

// responderError traduce los errores de validación del servicio a 400 y deja
// los fallos de persistencia como 500 genérico, sin filtrar SQL ni detalles.
func responderError(contexto *gin.Context, errorConsulta error, logger *slog.Logger) {
	switch {
	case errors.Is(errorConsulta, services.ErrLoteDisponibilidadVacio),
		errors.Is(errorConsulta, services.ErrLoteDisponibilidadExcesivo):
		contexto.JSON(http.StatusBadRequest, gin.H{"error": "invalid_batch"})
	case errors.Is(errorConsulta, services.ErrIDProductoDisponibilidad):
		contexto.JSON(http.StatusBadRequest, gin.H{"error": "invalid_product_id"})
	default:
		logger.Error("catalogo_disponibilidad_error",
			"request_id", contexto.Writer.Header().Get("X-Request-ID"),
			"error", errorConsulta)
		contexto.JSON(http.StatusInternalServerError, gin.H{"error": "catalogo_internal_error"})
	}
}

// decodificarCuerpo lee un único objeto JSON estricto y corta el cuerpo en
// maximoBytes. Rechaza campos desconocidos y contenido extra para evitar que un
// cliente envíe datos fuera del contrato.
func decodificarCuerpo(contexto *gin.Context, destino any, maximoBytes int) bool {
	contexto.Request.Body = http.MaxBytesReader(contexto.Writer, contexto.Request.Body, int64(maximoBytes))
	decodificador := json.NewDecoder(contexto.Request.Body)
	decodificador.DisallowUnknownFields()
	errorDecodificacion := decodificador.Decode(destino)
	if errorDecodificacion == nil {
		var contenidoExtra any
		if decodificador.Decode(&contenidoExtra) != io.EOF {
			errorDecodificacion = errors.New("contenido JSON extra")
		}
	}
	if errorDecodificacion != nil {
		var errorTamano *http.MaxBytesError
		estado := http.StatusBadRequest
		if errors.As(errorDecodificacion, &errorTamano) {
			estado = http.StatusRequestEntityTooLarge
		}
		contexto.AbortWithStatusJSON(estado, gin.H{"error": "invalid_payload"})
		return false
	}
	return true
}
