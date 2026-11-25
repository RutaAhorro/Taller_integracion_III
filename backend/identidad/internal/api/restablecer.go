// Endpoint público de restablecimiento de contraseña con token de un solo uso.
// No reimplementa el cambio voluntario: ése sigue siendo PUT /auth/me.

package api

import (
	"errors"
	"net/http"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
)

// ===== Restablecimiento =====

// PeticionRestablecer DTO de entrada: token recibido por correo y clave nueva.
type PeticionRestablecer struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// ManejadorRestablecer publica el caso de uso de restablecimiento.
type ManejadorRestablecer struct {
	servicio *services.ServicioRecuperacion
}

// NuevoManejadorRestablecer recibe el servicio ya armado al arrancar.
func NuevoManejadorRestablecer(servicio *services.ServicioRecuperacion) *ManejadorRestablecer {
	return &ManejadorRestablecer{servicio: servicio}
}

// Restablecer valida el token, guarda la clave nueva y responde sin exponer
// datos de la cuenta ni el hash resultante.
func (manejador *ManejadorRestablecer) Restablecer(contexto *gin.Context) {
	var peticion PeticionRestablecer
	if errorObtenido := contexto.ShouldBindJSON(&peticion); errorObtenido != nil {
		ResponderError(contexto, http.StatusBadRequest, "Datos de entrada inválidos para restablecer la contraseña.", errorObtenido)
		return
	}

	errorOperacion := manejador.servicio.Restablecer(contexto.Request.Context(), peticion.Token, peticion.Password)
	if errorOperacion != nil {
		switch {
		case errors.Is(errorOperacion, domain.ErrTokenRecuperacionInvalido):
			ResponderError(contexto, http.StatusBadRequest, "El token de restablecimiento no existe, expiró o ya fue usado.", errorOperacion)
		case errors.Is(errorOperacion, services.ErrContrasenaInvalida):
			ResponderError(contexto, http.StatusBadRequest, errorOperacion.Error(), errorOperacion)
		default:
			ResponderError(contexto, http.StatusInternalServerError, "Error interno al restablecer la contraseña.", errorOperacion)
		}
		return
	}

	contexto.JSON(http.StatusOK, gin.H{"mensaje": "Contraseña restablecida correctamente."})
}
