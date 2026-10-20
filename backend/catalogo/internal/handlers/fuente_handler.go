package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/middleware"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
)

// FuenteHandler maneja las peticiones HTTP de administración de fuentes de scraping.
type FuenteHandler struct {
	service services.FuenteService
}

// NewFuenteHandler crea una nueva instancia de FuenteHandler
func NewFuenteHandler(service services.FuenteService) *FuenteHandler {
	return &FuenteHandler{service: service}
}

// ObtenerFuentes godoc
// @Summary      Lista las fuentes de scraping (cadenas de supermercado)
// @Description  Obtiene la lista de cadenas y su estado de activación para el panel de administración
// @Tags         admin
// @Produce      json
// @Success      200  {array}   domain.FuenteDTO
// @Failure      401  {object}  map[string]interface{}
// @Failure      403  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /api/v1/admin/productos/fuentes [get]
func (h *FuenteHandler) ObtenerFuentes(c *gin.Context) {
	fuentes, err := h.service.ListarFuentes(c.Request.Context())
	if err != nil {
		middleware.LogInternalError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error interno al obtener fuentes",
		})
		return
	}
	c.JSON(http.StatusOK, fuentes)
}

// ActualizarFuente godoc
// @Summary      Habilita o deshabilita una fuente de scraping
// @Description  Actualiza el flag de activación de una cadena de supermercado
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id     path      int                        true  "ID de la fuente/cadena"
// @Param        input  body      domain.ActualizarFuenteDTO true  "Estado nuevo de la fuente"
// @Success      200    {object}  domain.FuenteDTO
// @Failure      400    {object}  map[string]interface{}
// @Failure      401    {object}  map[string]interface{}
// @Failure      403    {object}  map[string]interface{}
// @Failure      404    {object}  map[string]interface{}
// @Failure      500    {object}  map[string]interface{}
// @Router       /api/v1/admin/productos/fuentes/{id} [put]
func (h *FuenteHandler) ActualizarFuente(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "ID de fuente inválido",
		})
		return
	}

	var input domain.ActualizarFuenteDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Payload inválido",
		})
		return
	}

	fuente, err := h.service.ActualizarFuente(c.Request.Context(), id, input)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrFuenteNoEncontrada):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Fuente no encontrada",
			})
		case errors.Is(err, services.ErrEstadoFuenteRequerido):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
		default:
			middleware.LogInternalError(c, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error interno al actualizar fuente",
			})
		}
		return
	}

	c.JSON(http.StatusOK, fuente)
}
