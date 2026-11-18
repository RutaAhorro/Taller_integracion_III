// Endpoints del panel de administración de cuentas (rol admin).

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

// ===== Construcción =====

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
func NewAdminUsuarioHandler(servicioAdmin services.AdminUsuarioService) *AdminUsuarioHandler {
	return &AdminUsuarioHandler{adminService: servicioAdmin}
}

// ===== Endpoints =====

// ListarUsuarios: listar cuentas de usuario.
// Devuelve las cuentas registradas en api.usuarios con filtros opcionales por texto, rol y estado. Sólo rol admin.
func (manejador *AdminUsuarioHandler) ListarUsuarios(contexto *gin.Context) {
	maximoEntero := int64(^uint(0) >> 1)
	filtro := domain.FiltroUsuariosAdminDTO{
		Query: contexto.Query("q"),
		Rol:   contexto.Query("rol"),
		Page:  1,
		Limit: defaultUsuariosAdminPorPagina,
	}

	if valor := contexto.Query("esta_activo"); valor != "" {
		activo, errorObtenido := strconv.ParseBool(valor)
		if errorObtenido != nil {
			httperrors.ResponderError(contexto, http.StatusBadRequest, "El filtro esta_activo debe ser true o false.", errorObtenido)
			return
		}
		filtro.EstaActivo = &activo
	}

	if valor := contexto.Query("limit"); valor != "" {
		valorNumerico, errorObtenido := strconv.Atoi(valor)
		if errorObtenido != nil || valorNumerico <= 0 {
			httperrors.ResponderError(contexto, http.StatusBadRequest, "El parámetro limit debe ser un entero positivo.", errorObtenido)
			return
		}
		if valorNumerico > maxUsuariosAdminPorPagina {
			httperrors.ResponderError(contexto, http.StatusBadRequest,
				"El parámetro limit no puede superar 200.", nil)
			return
		}
		filtro.Limit = valorNumerico
	}

	if valor := contexto.Query("page"); valor != "" {
		valorNumerico, errorObtenido := strconv.ParseInt(valor, 10, 64)
		if errorObtenido != nil || valorNumerico < 1 {
			httperrors.ResponderError(contexto, http.StatusBadRequest, "El parámetro page debe ser un entero positivo.", errorObtenido)
			return
		}
		if valorNumerico > maximoEntero {
			httperrors.ResponderError(contexto, http.StatusBadRequest, "El parámetro page excede el rango admitido.", nil)
			return
		}
		filtro.Page = int(valorNumerico)
	}

	if filtro.Page > 1 {
		// Se divide antes de multiplicar para no desbordar int64.
		if int64(filtro.Page-1) > maximoEntero/int64(filtro.Limit) {
			httperrors.ResponderError(contexto, http.StatusBadRequest, "El parámetro page excede el rango admitido.", nil)
			return
		}
		filtro.Offset = (filtro.Page - 1) * filtro.Limit
	}

	pagina, errorOperacion := manejador.adminService.Listar(contexto.Request.Context(), filtro)
	if errorOperacion != nil {
		if errors.Is(errorOperacion, services.ErrRolInvalido) {
			httperrors.ResponderError(contexto, http.StatusBadRequest, errorOperacion.Error(), errorOperacion)
			return
		}
		httperrors.ResponderError(contexto, http.StatusInternalServerError, "Error interno al listar las cuentas de usuario.", errorOperacion)
		return
	}

	contexto.JSON(http.StatusOK, pagina)
}

// Metricas: métricas de cuentas de usuario.
// Devuelve el total de cuentas y cuántas están activas. Sólo rol admin.
func (manejador *AdminUsuarioHandler) Metricas(contexto *gin.Context) {
	metricas, errorObtenido := manejador.adminService.Metricas(contexto.Request.Context())
	if errorObtenido != nil {
		httperrors.ResponderError(contexto, http.StatusInternalServerError, "Error interno al calcular las métricas de usuarios.", errorObtenido)
		return
	}
	contexto.JSON(http.StatusOK, metricas)
}

// ActualizarUsuario: activar/desactivar, asignar rol o plan a una cuenta.
// Actualiza esta_activo, el rol RBAC y/o el plan comercial de un usuario. Sólo rol admin.
func (manejador *AdminUsuarioHandler) ActualizarUsuario(contexto *gin.Context) {
	// El cuerpo sólo admite los campos administrables; el identificador viaja
	// siempre en la ruta para evitar dos fuentes de verdad.
	var peticion domain.ActualizarUsuarioAdminDTO
	if errorObtenido := bindJSONStrict(contexto, &peticion); errorObtenido != nil {
		mensaje := "Datos de entrada inválidos para actualizar la cuenta."
		if strings.Contains(errorObtenido.Error(), "unknown field") {
			mensaje = "La actualización contiene campos no permitidos."
		}
		httperrors.ResponderError(contexto, http.StatusBadRequest, mensaje, errorObtenido)
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

	dto, errorOperacion := manejador.adminService.Actualizar(contexto.Request.Context(), id, peticion, actorID)
	if errorOperacion != nil {
		switch {
		case errors.Is(errorOperacion, services.ErrIDUsuarioInvalido), errors.Is(errorOperacion, services.ErrRolInvalido), errors.Is(errorOperacion, services.ErrPlanInvalido), errors.Is(errorOperacion, services.ErrSinCambiosAdmin):
			httperrors.ResponderError(contexto, http.StatusBadRequest, errorOperacion.Error(), errorOperacion)
		case errors.Is(errorOperacion, services.ErrUsuarioNoEncontrado):
			httperrors.ResponderError(contexto, http.StatusNotFound, errorOperacion.Error(), errorOperacion)
		case errors.Is(errorOperacion, services.ErrAutoModificacionAdmin), errors.Is(errorOperacion, services.ErrUltimoAdmin):
			httperrors.ResponderError(contexto, http.StatusConflict, errorOperacion.Error(), errorOperacion)
		default:
			httperrors.ResponderError(contexto, http.StatusInternalServerError, "Error interno al actualizar la cuenta de usuario.", errorOperacion)
		}
		return
	}

	contexto.JSON(http.StatusOK, dto)
}

// ===== Utilidades =====

// bindJSONStrict decodifica un único objeto JSON y rechaza campos desconocidos.
// Así un cliente no recibe 200 cuando intenta enviar, por ejemplo, id o correo
// fuera del contrato administrable.
func bindJSONStrict(contexto *gin.Context, destino any) error {
	decodificador := json.NewDecoder(contexto.Request.Body)
	decodificador.DisallowUnknownFields()
	if errorObtenido := decodificador.Decode(destino); errorObtenido != nil {
		return errorObtenido
	}
	var extra json.RawMessage
	if errorObtenido := decodificador.Decode(&extra); errorObtenido != io.EOF {
		if errorObtenido == nil {
			return errors.New("el cuerpo debe contener un único objeto JSON")
		}
		return errorObtenido
	}
	return nil
}
