// Endpoints de las direcciones guardadas del usuario autenticado. El dueño sale
// siempre de la sesión validada, nunca del cuerpo, de la ruta ni de headers.

package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
)

// ===== Construcción =====

// ManejadorDirecciones gestiona /usuarios/me/direcciones.
type ManejadorDirecciones struct {
	servicio *services.ServicioDirecciones
}

// NuevoManejadorDirecciones construye el handler inyectando el servicio.
func NuevoManejadorDirecciones(servicio *services.ServicioDirecciones) *ManejadorDirecciones {
	return &ManejadorDirecciones{servicio: servicio}
}

// ===== Endpoints =====

// ListarDirecciones devuelve las direcciones del usuario de la sesión,
// principal primero.
func (manejador *ManejadorDirecciones) ListarDirecciones(contexto *gin.Context) {
	usuarioID, autenticado := usuarioDeSesion(contexto)
	if !autenticado {
		return
	}
	lista, errorObtenido := manejador.servicio.Listar(contexto.Request.Context(), usuarioID)
	if errorObtenido != nil {
		responderErrorDireccion(contexto, errorObtenido, "Error interno al listar las direcciones.")
		return
	}
	contexto.JSON(http.StatusOK, lista)
}

// CrearDireccion guarda una dirección nueva y responde 201 con ella.
func (manejador *ManejadorDirecciones) CrearDireccion(contexto *gin.Context) {
	usuarioID, autenticado := usuarioDeSesion(contexto)
	if !autenticado {
		return
	}
	entrada, valida := leerEntradaDireccion(contexto)
	if !valida {
		return
	}
	creada, errorObtenido := manejador.servicio.Crear(contexto.Request.Context(), usuarioID, entrada)
	if errorObtenido != nil {
		responderErrorDireccion(contexto, errorObtenido, "Error interno al crear la dirección.")
		return
	}
	contexto.JSON(http.StatusCreated, creada)
}

// ActualizarDireccion reemplaza los campos editables de una dirección propia.
func (manejador *ManejadorDirecciones) ActualizarDireccion(contexto *gin.Context) {
	usuarioID, autenticado := usuarioDeSesion(contexto)
	if !autenticado {
		return
	}
	entrada, valida := leerEntradaDireccion(contexto)
	if !valida {
		return
	}
	actualizada, errorObtenido := manejador.servicio.Actualizar(contexto.Request.Context(), usuarioID, contexto.Param("id"), entrada)
	if errorObtenido != nil {
		responderErrorDireccion(contexto, errorObtenido, "Error interno al actualizar la dirección.")
		return
	}
	contexto.JSON(http.StatusOK, actualizada)
}

// EliminarDireccion borra una dirección propia y responde 204 sin cuerpo.
func (manejador *ManejadorDirecciones) EliminarDireccion(contexto *gin.Context) {
	usuarioID, autenticado := usuarioDeSesion(contexto)
	if !autenticado {
		return
	}
	if errorObtenido := manejador.servicio.Eliminar(contexto.Request.Context(), usuarioID, contexto.Param("id")); errorObtenido != nil {
		responderErrorDireccion(contexto, errorObtenido, "Error interno al eliminar la dirección.")
		return
	}
	contexto.Status(http.StatusNoContent)
}

// ===== Utilidades =====

// usuarioDeSesion lee el usuario que ExigirSesion dejó en el contexto tras
// revalidarlo en api.usuarios. Si falta, responde 401 y devuelve false.
func usuarioDeSesion(contexto *gin.Context) (string, bool) {
	usuarioID := strings.TrimSpace(contexto.GetString("user_id"))
	if usuarioID == "" {
		ResponderError(contexto, http.StatusUnauthorized, "La sesión no identifica al usuario.", nil)
		return "", false
	}
	return usuarioID, true
}

// leerEntradaDireccion decodifica el cuerpo de forma estricta: un campo
// desconocido (por ejemplo usuario_id o id) se rechaza en vez de ignorarse.
func leerEntradaDireccion(contexto *gin.Context) (domain.EntradaDireccionDTO, bool) {
	var entrada domain.EntradaDireccionDTO
	if errorObtenido := leerJSONEstricto(contexto, &entrada); errorObtenido != nil {
		mensaje := "Datos de entrada inválidos para la dirección."
		if strings.Contains(errorObtenido.Error(), "unknown field") {
			mensaje = "La dirección contiene campos no permitidos."
		}
		ResponderError(contexto, http.StatusBadRequest, mensaje, errorObtenido)
		return entrada, false
	}
	return entrada, true
}

// responderErrorDireccion traduce los errores del servicio a HTTP. Una
// dirección ajena responde 404, igual que una inexistente, para no revelar que
// existe en otra cuenta; los estados que dependen de los datos propios (límite,
// principal) responden 409.
func responderErrorDireccion(contexto *gin.Context, errorObtenido error, mensajeInterno string) {
	switch {
	case errors.Is(errorObtenido, services.ErrDireccionInvalida), errors.Is(errorObtenido, services.ErrIDDireccionInvalido):
		ResponderError(contexto, http.StatusBadRequest, errorObtenido.Error(), errorObtenido)
	case errors.Is(errorObtenido, services.ErrDireccionNoEncontrada):
		ResponderError(contexto, http.StatusNotFound, errorObtenido.Error(), errorObtenido)
	case errors.Is(errorObtenido, services.ErrLimiteDirecciones), errors.Is(errorObtenido, services.ErrPrincipalObligatoria), errors.Is(errorObtenido, services.ErrConflictoPrincipal):
		ResponderError(contexto, http.StatusConflict, errorObtenido.Error(), errorObtenido)
	case errors.Is(errorObtenido, services.ErrIDUsuarioInvalido):
		ResponderError(contexto, http.StatusUnauthorized, "La sesión no identifica al usuario.", errorObtenido)
	default:
		ResponderError(contexto, http.StatusInternalServerError, mensajeInterno, errorObtenido)
	}
}
