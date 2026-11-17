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
	defaultUsuariosAdminPorPagina = domain.UsuariosAdminPorPagina
	// maxUsuariosAdminPorPagina acota el listado cuando el cliente pide límite.
	maxUsuariosAdminPorPagina = domain.MaxUsuariosAdminPorPagina
)

// AdminUsuarioHandler gestiona los endpoints administrativos de api.usuarios.
type AdminUsuarioHandler struct {
	adminService services.AdminUsuarioService
}

// NewAdminUsuarioHandler construye el handler administrativo inyectando el servicio.
func NewAdminUsuarioHandler(adminService services.AdminUsuarioService) *AdminUsuarioHandler {
	return &AdminUsuarioHandler{adminService: adminService}
}

// ListarUsuarios: listar cuentas de usuario.
// Devuelve las cuentas registradas en api.usuarios con filtros opcionales por texto, rol y estado. Sólo rol admin.
func (manejador *AdminUsuarioHandler) ListarUsuarios(contexto *gin.Context) {
	maxInt := int64(^uint(0) >> 1)
	filtro := domain.FiltroUsuariosAdminDTO{
		Query: contexto.Query("q"),
		Rol:   contexto.Query("rol"),
		Page:  1,
		Limit: defaultUsuariosAdminPorPagina,
	}

	if value := contexto.Query("esta_activo"); value != "" {
		activo, err := strconv.ParseBool(value)
		if err != nil {
			httperrors.ResponderError(contexto, http.StatusBadRequest, "El filtro esta_activo debe ser true o false.", err)
			return
		}
		filtro.EstaActivo = &activo
	}

	if value := contexto.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			httperrors.ResponderError(contexto, http.StatusBadRequest, "El parámetro limit debe ser un entero positivo.", err)
			return
		}
		if parsed > maxUsuariosAdminPorPagina {
			httperrors.ResponderError(contexto, http.StatusBadRequest,
				"El parámetro limit no puede superar 200.", nil)
			return
		}
		filtro.Limit = parsed
	}

	if value := contexto.Query("page"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 1 {
			httperrors.ResponderError(contexto, http.StatusBadRequest, "El parámetro page debe ser un entero positivo.", err)
			return
		}
		if parsed > maxInt {
			httperrors.ResponderError(contexto, http.StatusBadRequest, "El parámetro page excede el rango admitido.", nil)
			return
		}
		filtro.Page = int(parsed)
	}

	if filtro.Page > 1 {
		// Se divide antes de multiplicar para no desbordar int64.
		if int64(filtro.Page-1) > maxInt/int64(filtro.Limit) {
			httperrors.ResponderError(contexto, http.StatusBadRequest, "El parámetro page excede el rango admitido.", nil)
			return
		}
		filtro.Offset = (filtro.Page - 1) * filtro.Limit
	}

	pagina, err := manejador.adminService.Listar(contexto.Request.Context(), filtro)
	if err != nil {
		if errors.Is(err, services.ErrRolInvalido) {
			httperrors.ResponderError(contexto, http.StatusBadRequest, err.Error(), err)
			return
		}
		httperrors.ResponderError(contexto, http.StatusInternalServerError, "Error interno al listar las cuentas de usuario.", err)
		return
	}

	contexto.JSON(http.StatusOK, pagina)
}

// Metricas: métricas de cuentas de usuario.
// Devuelve el total de cuentas y cuántas están activas. Sólo rol admin.
func (manejador *AdminUsuarioHandler) Metricas(contexto *gin.Context) {
	metricas, err := manejador.adminService.Metricas(contexto.Request.Context())
	if err != nil {
		httperrors.ResponderError(contexto, http.StatusInternalServerError, "Error interno al calcular las métricas de usuarios.", err)
		return
	}
	contexto.JSON(http.StatusOK, metricas)
}

// ActualizarUsuario: activar/desactivar, asignar rol o plan a una cuenta.
// Actualiza esta_activo, el rol RBAC y/o el plan comercial de un usuario. Sólo rol admin.
func (manejador *AdminUsuarioHandler) ActualizarUsuario(contexto *gin.Context) {
	// El cuerpo sólo admite los campos administrables; el identificador viaja
	// siempre en la ruta para evitar dos fuentes de verdad.
	var req domain.ActualizarUsuarioAdminDTO
	if err := bindJSONStrict(contexto, &req); err != nil {
		mensaje := "Datos de entrada inválidos para actualizar la cuenta."
		if strings.Contains(err.Error(), "unknown field") {
			mensaje = "La actualización contiene campos no permitidos."
		}
		httperrors.ResponderError(contexto, http.StatusBadRequest, mensaje, err)
		return
	}

	id := strings.TrimSpace(contexto.Param("id"))
	if id == "" {
		httperrors.ResponderError(contexto, http.StatusBadRequest, "El identificador de usuario es obligatorio en la ruta.", nil)
		return
	}
	actorID := strings.TrimSpace(contexto.GetString("user_id"))
	if actorID == "" {
		httperrors.ResponderError(contexto, http.StatusUnauthorized, "La sesión no identifica al administrador.", nil)
		return
	}

	dto, err := manejador.adminService.Actualizar(contexto.Request.Context(), id, req, actorID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrIDUsuarioInvalido), errors.Is(err, services.ErrRolInvalido), errors.Is(err, services.ErrPlanInvalido), errors.Is(err, services.ErrSinCambiosAdmin):
			httperrors.ResponderError(contexto, http.StatusBadRequest, err.Error(), err)
		case errors.Is(err, services.ErrUsuarioNoEncontrado):
			httperrors.ResponderError(contexto, http.StatusNotFound, err.Error(), err)
		case errors.Is(err, services.ErrAutoModificacionAdmin), errors.Is(err, services.ErrUltimoAdmin):
			httperrors.ResponderError(contexto, http.StatusConflict, err.Error(), err)
		default:
			httperrors.ResponderError(contexto, http.StatusInternalServerError, "Error interno al actualizar la cuenta de usuario.", err)
		}
		return
	}

	contexto.JSON(http.StatusOK, dto)
}

// bindJSONStrict decodifica un único objeto JSON y rechaza campos desconocidos.
// Así un cliente no recibe 200 cuando intenta enviar, por ejemplo, id o correo
// fuera del contrato administrable.
func bindJSONStrict(contexto *gin.Context, destino any) error {
	decoder := json.NewDecoder(contexto.Request.Body)
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
