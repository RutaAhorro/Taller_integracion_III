package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	httperrors "github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/httperrors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
)

const (
	// maxUsuariosAdminPorPagina acota el listado cuando el cliente pide límite.
	maxUsuariosAdminPorPagina = 200
	// usuariosAdminPorPaginaPredeterminado evita volcar la tabla completa
	// cuando el cliente no envía `limit`.
	usuariosAdminPorPaginaPredeterminado = 50
)

// ActualizarUsuarioAdminRequest permite recibir el ID tanto en la ruta como en el cuerpo.
type ActualizarUsuarioAdminRequest struct {
	ID         string  `json:"id" example:"a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d"`
	EstaActivo *bool   `json:"esta_activo" example:"true"`
	Rol        *string `json:"rol" example:"colaborador"`
}

// AdminUsuarioHandler gestiona los endpoints administrativos de api.usuarios.
type AdminUsuarioHandler struct {
	adminService services.AdminUsuarioService
}

// NewAdminUsuarioHandler construye el handler administrativo inyectando el servicio.
func NewAdminUsuarioHandler(adminService services.AdminUsuarioService) *AdminUsuarioHandler {
	return &AdminUsuarioHandler{adminService: adminService}
}

// ListarUsuarios godoc
// @Summary      Listar cuentas de usuario
// @Description  Devuelve las cuentas registradas en api.usuarios con filtros opcionales por texto, rol y estado. Sólo rol admin.
// @Tags         admin
// @Produce      json
// @Param        q            query     string  false  "Búsqueda por correo o nombre"
// @Param        rol          query     string  false  "Filtrar por rol (admin, colaborador, registrado)"
// @Param        esta_activo  query     bool    false  "Filtrar por cuentas activas/inactivas"
// @Param        page         query     int     false  "Número de página (requiere limit)"
// @Param        limit        query     int     false  "Cantidad de cuentas por página (máx. 200, por defecto 50)"
// @Param        offset       query     int     false  "Desplazamiento explícito"
// @Success      200  {array}   domain.UsuarioAdminDTO
// @Failure      400  {object}  httperrors.RespuestaError
// @Failure      401  {object}  httperrors.RespuestaError
// @Failure      403  {object}  httperrors.RespuestaError
// @Failure      500  {object}  httperrors.RespuestaError
// @Router       /api/v1/admin/usuarios [get]
func (h *AdminUsuarioHandler) ListarUsuarios(c *gin.Context) {
	filtro := domain.FiltroUsuariosAdminDTO{
		Query: c.Query("q"),
		Rol:   c.Query("rol"),
	}

	if value := c.Query("esta_activo"); value != "" {
		if activo, err := strconv.ParseBool(value); err == nil {
			filtro.EstaActivo = &activo
		}
	}

	limit, offset := usuariosAdminPorPaginaPredeterminado, 0
	if value := c.Query("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			limit = parsed
			if limit > maxUsuariosAdminPorPagina {
				limit = maxUsuariosAdminPorPagina
			}
		}
	}
	if value := c.Query("offset"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			offset = parsed
		}
	}
	if value := c.Query("page"); value != "" && limit > 0 {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 1 {
			offset = (parsed - 1) * limit
		}
	}
	filtro.Limit = limit
	filtro.Offset = offset

	usuarios, err := h.adminService.Listar(c.Request.Context(), filtro)
	if err != nil {
		if errors.Is(err, services.ErrRolInvalido) {
			httperrors.ResponderError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		httperrors.ResponderError(c, http.StatusInternalServerError, "Error interno al listar las cuentas de usuario.", err)
		return
	}

	c.JSON(http.StatusOK, usuarios)
}

// ActualizarUsuario godoc
// @Summary      Activar/desactivar o asignar rol a una cuenta
// @Description  Actualiza esta_activo y/o el rol de un usuario registrado, incluido el rol colaborador. Sólo rol admin.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id       path      string                        true   "UUID del usuario"
// @Param        payload  body      ActualizarUsuarioAdminRequest true   "Campos a actualizar"
// @Success      200  {object}  domain.UsuarioAdminDTO
// @Failure      400  {object}  httperrors.RespuestaError
// @Failure      401  {object}  httperrors.RespuestaError
// @Failure      403  {object}  httperrors.RespuestaError
// @Failure      404  {object}  httperrors.RespuestaError
// @Failure      500  {object}  httperrors.RespuestaError
// @Router       /api/v1/admin/usuarios/{id} [put]
func (h *AdminUsuarioHandler) ActualizarUsuario(c *gin.Context) {
	var req ActualizarUsuarioAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.ResponderError(c, http.StatusBadRequest, "Datos de entrada inválidos para actualizar la cuenta.", err)
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		id = strings.TrimSpace(req.ID)
	}

	dto, err := h.adminService.Actualizar(c.Request.Context(), id, domain.ActualizarUsuarioAdminDTO{
		EstaActivo: req.EstaActivo,
		Rol:        req.Rol,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrIDUsuarioInvalido), errors.Is(err, services.ErrRolInvalido), errors.Is(err, services.ErrSinCambiosAdmin):
			httperrors.ResponderError(c, http.StatusBadRequest, err.Error(), err)
		case errors.Is(err, services.ErrUsuarioNoEncontrado):
			httperrors.ResponderError(c, http.StatusNotFound, err.Error(), err)
		default:
			httperrors.ResponderError(c, http.StatusInternalServerError, "Error interno al actualizar la cuenta de usuario.", err)
		}
		return
	}

	c.JSON(http.StatusOK, dto)
}
