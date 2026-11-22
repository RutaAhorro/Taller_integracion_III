// Fuentes: listado y activación de cadenas de supermercado desde el panel de
// administración.

package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
)

// ===== Handlers =====

// manejadorFuentes agrupa los handlers de fuentes.
type manejadorFuentes struct {
	servicio *services.ServicioFuentes
}

// listar atiende GET /api/v1/admin/productos/fuentes.
func (manejador manejadorFuentes) listar(contexto *gin.Context) {
	fuentes, errorListado := manejador.servicio.ListarFuentes(contexto.Request.Context())
	if errorListado != nil {
		registrarErrorInterno(contexto, errorListado)
		contexto.JSON(http.StatusInternalServerError, gin.H{"error": "Error interno al obtener fuentes"})
		return
	}
	contexto.JSON(http.StatusOK, fuentes)
}

// actualizar atiende PUT /api/v1/admin/productos/fuentes/:id con {habilitada}.
func (manejador manejadorFuentes) actualizar(contexto *gin.Context) {
	fuenteID, errorID := strconv.Atoi(contexto.Param("id"))
	if errorID != nil || fuenteID <= 0 {
		contexto.JSON(http.StatusBadRequest, gin.H{"error": "ID de fuente inválido"})
		return
	}
	var cambio domain.ActualizarFuenteDTO
	if errorCuerpo := contexto.ShouldBindJSON(&cambio); errorCuerpo != nil {
		contexto.JSON(http.StatusBadRequest, gin.H{"error": "Payload inválido"})
		return
	}
	fuente, errorActualizacion := manejador.servicio.ActualizarFuente(contexto.Request.Context(), fuenteID, cambio)
	switch {
	case errorActualizacion == nil:
		contexto.JSON(http.StatusOK, fuente)
	case errors.Is(errorActualizacion, services.ErrFuenteNoEncontrada):
		contexto.JSON(http.StatusNotFound, gin.H{"error": "Fuente no encontrada"})
	case errors.Is(errorActualizacion, services.ErrEstadoFuenteRequerido):
		contexto.JSON(http.StatusBadRequest, gin.H{"error": errorActualizacion.Error()})
	default:
		registrarErrorInterno(contexto, errorActualizacion)
		contexto.JSON(http.StatusInternalServerError, gin.H{"error": "Error interno al actualizar fuente"})
	}
}
