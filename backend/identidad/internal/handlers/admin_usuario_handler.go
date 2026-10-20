package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	httperrors "github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/httperrors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
)

const (
	// defaultUsuariosAdminPorPagina mantiene acotado el listado por defecto.
	defaultUsuariosAdminPorPagina = 20
	// maxUsuariosAdminPorPagina acota el listado cuando el cliente pide límite.
	maxUsuariosAdminPorPagina = 200
)

// ActualizarUsuarioAdminRequest contiene los campos administrables de la cuenta.
// El identificador viaja siempre en la ruta para evitar dos fuentes de verdad.
type ActualizarUsuarioAdminRequest struct {
	EstaActivo *bool   `json:"esta_activo" example:"true"`
	Rol        *string `json:"rol" example:"colaborador"`
	Plan       *string `json:"plan" example:"premium"`
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
// @Param        page         query     int     false  "Número de página, base 1 (por defecto 1)"
// @Param        limit        query     int     false  "Cantidad de cuentas por página (por defecto 20; máx. 200)"
// @Param        offset       query     int     false  "Desplazamiento explícito, base 0; excluyente con page"
// @Success      200  {object}  domain.PaginaUsuariosAdminDTO
// @Failure      400  {object}  httperrors.RespuestaError
// @Failure      401  {object}  httperrors.RespuestaError
// @Failure      403  {object}  httperrors.RespuestaError
// @Failure      500  {object}  httperrors.RespuestaError
// @Router       /api/v1/admin/usuarios [get]
func (h *AdminUsuarioHandler) ListarUsuarios(c *gin.Context) {
	maxInt := int64(^uint(0) >> 1)
	filtro := domain.FiltroUsuariosAdminDTO{
		Query: c.Query("q"),
		Rol:   c.Query("rol"),
		Page:  1,
		Limit: defaultUsuariosAdminPorPagina,
	}

	if value := c.Query("esta_activo"); value != "" {
		activo, err := strconv.ParseBool(value)
		if err != nil {
			httperrors.ResponderError(c, http.StatusBadRequest, "El filtro esta_activo debe ser true o false.", err)
			return
		}
		filtro.EstaActivo = &activo
	}

	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			httperrors.ResponderError(c, http.StatusBadRequest, "El parámetro limit debe ser un entero positivo.", err)
			return
		}
		if parsed > maxUsuariosAdminPorPagina {
			httperrors.ResponderError(c, http.StatusBadRequest,
				"El parámetro limit no puede superar 200.", nil)
			return
		}
		filtro.Limit = parsed
	}

	offsetDefinido := false
	if value := c.Query("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			httperrors.ResponderError(c, http.StatusBadRequest, "El parámetro offset debe ser un entero no negativo.", err)
			return
		}
		filtro.Offset = parsed
		offsetDefinido = true
	}

	pageDefinida := false
	if value := c.Query("page"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 1 {
			httperrors.ResponderError(c, http.StatusBadRequest, "El parámetro page debe ser un entero positivo.", err)
			return
		}
		if parsed > maxInt {
			httperrors.ResponderError(c, http.StatusBadRequest, "El parámetro page excede el rango admitido.", nil)
			return
		}
		filtro.Page = int(parsed)
		pageDefinida = true
	}

	if pageDefinida && offsetDefinido {
		httperrors.ResponderError(c, http.StatusBadRequest, "Los parámetros page y offset son mutuamente excluyentes.", nil)
		return
	}

	if offsetDefinido {
		// Con offset explícito se informa la página equivalente para que el
		// cliente pueda seguir usando los metadatos de paginación. Se calcula
		// en int64 para no desbordar int en el límite superior.
		pagina := int64(filtro.Offset) / int64(filtro.Limit)
		if pagina >= maxInt {
			httperrors.ResponderError(c, http.StatusBadRequest, "El parámetro offset excede el rango admitido.", nil)
			return
		}
		filtro.Page = int(pagina + 1)
	} else if filtro.Page > 1 {
		// Se divide antes de multiplicar para no desbordar int64.
		if int64(filtro.Page-1) > maxInt/int64(filtro.Limit) {
			httperrors.ResponderError(c, http.StatusBadRequest, "El parámetro page excede el rango admitido.", nil)
			return
		}
		filtro.Offset = (filtro.Page - 1) * filtro.Limit
	}

	pagina, err := h.adminService.Listar(c.Request.Context(), filtro)
	if err != nil {
		if errors.Is(err, services.ErrRolInvalido) {
			httperrors.ResponderError(c, http.StatusBadRequest, err.Error(), err)
			return
		}
		httperrors.ResponderError(c, http.StatusInternalServerError, "Error interno al listar las cuentas de usuario.", err)
		return
	}

	c.JSON(http.StatusOK, pagina)
}

// ActualizarUsuario godoc
// @Summary      Activar/desactivar, asignar rol o plan a una cuenta
// @Description  Actualiza esta_activo, el rol RBAC y/o el plan comercial de un usuario. Sólo rol admin.
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
	if err := bindJSONStrict(c, &req); err != nil {
		mensaje := "Datos de entrada inválidos para actualizar la cuenta."
		if strings.Contains(err.Error(), "unknown field") {
			mensaje = "La actualización contiene campos no permitidos."
		}
		httperrors.ResponderError(c, http.StatusBadRequest, mensaje, err)
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		httperrors.ResponderError(c, http.StatusBadRequest, "El identificador de usuario es obligatorio en la ruta.", nil)
		return
	}
	actorID := strings.TrimSpace(c.GetString("user_id"))
	if actorID == "" {
		httperrors.ResponderError(c, http.StatusUnauthorized, "La sesión no identifica al administrador.", nil)
		return
	}

	dto, err := h.adminService.Actualizar(c.Request.Context(), id, domain.ActualizarUsuarioAdminDTO{
		EstaActivo: req.EstaActivo,
		Rol:        req.Rol,
		Plan:       req.Plan,
	}, actorID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrIDUsuarioInvalido), errors.Is(err, services.ErrRolInvalido), errors.Is(err, services.ErrPlanInvalido), errors.Is(err, services.ErrSinCambiosAdmin):
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

// bindJSONStrict decodifica un único objeto JSON y rechaza campos desconocidos.
// Así un cliente no recibe 200 cuando intenta enviar, por ejemplo, id o correo
// fuera del contrato administrable.
func bindJSONStrict(c *gin.Context, destino any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destino); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("el cuerpo debe contener un único objeto JSON")
		}
		return err
	}
	return nil
}
