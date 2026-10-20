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
)

// ActualizarUsuarioAdminRequest contiene los campos administrables de la cuenta.
// El identificador viaja siempre en la ruta para evitar dos fuentes de verdad.
type ActualizarUsuarioAdminRequest struct {
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
// @Param        esta_activo  query     bool    false  "Filtrar por cuentas activas/inactivas (true/false)"
// @Param        page         query     int     false  "Número de página, base 1; requiere limit si es mayor a 1"
// @Param        limit        query     int     false  "Cantidad de cuentas por página (máx. 200; sin límite si se omite)"
// @Param        offset       query     int     false  "Desplazamiento explícito, base 0; excluyente con page"
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
		activo, err := strconv.ParseBool(value)
		if err != nil {
			httperrors.ResponderError(c, http.StatusBadRequest, "El filtro esta_activo debe ser true o false.", err)
			return
		}
		filtro.EstaActivo = &activo
	}

	// Sin `limit` se devuelve el listado completo, que es lo que espera el
	// panel mientras no tenga paginación propia. Un límite explícito sigue
	// acotado para evitar respuestas desmedidas.
	limit, offset := 0, 0
	limitDefinido := false
	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			httperrors.ResponderError(c, http.StatusBadRequest, "El parámetro limit debe ser un entero positivo.", err)
			return
		}
		limit = parsed
		if limit > maxUsuariosAdminPorPagina {
			limit = maxUsuariosAdminPorPagina
		}
		limitDefinido = true
	}

	offsetDefinido := false
	if value := c.Query("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			httperrors.ResponderError(c, http.StatusBadRequest, "El parámetro offset debe ser un entero no negativo.", err)
			return
		}
		offset = parsed
		offsetDefinido = true
	}

	if value := c.Query("page"); value != "" {
		if offsetDefinido {
			httperrors.ResponderError(c, http.StatusBadRequest, "Los parámetros page y offset son mutuamente excluyentes.", nil)
			return
		}
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 1 {
			httperrors.ResponderError(c, http.StatusBadRequest, "El parámetro page debe ser un entero positivo.", err)
			return
		}
		if parsed > 1 && !limitDefinido {
			httperrors.ResponderError(c, http.StatusBadRequest, "El parámetro page requiere limit cuando es mayor a 1.", nil)
			return
		}
		if parsed > 1 {
			maxInt := int64(^uint(0) >> 1)
			// Se divide antes de multiplicar para no desbordar int64.
			if parsed-1 > maxInt/int64(limit) {
				httperrors.ResponderError(c, http.StatusBadRequest, "El parámetro page excede el rango admitido.", nil)
				return
			}
			offset = int((parsed - 1) * int64(limit))
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
// @Failure      409  {object}  httperrors.RespuestaError
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
		httperrors.ResponderError(c, http.StatusBadRequest, "El identificador de usuario es obligatorio en la ruta.", nil)
		return
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
		case errors.Is(err, services.ErrUltimoAdmin):
			httperrors.ResponderError(c, http.StatusConflict, err.Error(), err)
		default:
			httperrors.ResponderError(c, http.StatusInternalServerError, "Error interno al actualizar la cuenta de usuario.", err)
		}
		return
	}

	c.JSON(http.StatusOK, dto)
}
