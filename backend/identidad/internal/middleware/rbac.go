package middleware

import (
	"errors"
	"net/http"
	"strings"

	httperrors "github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/httperrors"
	"github.com/gin-gonic/gin"
)

// RequireRole valida que el usuario autenticado posea al menos uno de los roles permitidos en la petición.
// Si no se encuentra el rol o no coincide con los autorizados, interrumpe el flujo retornando HTTP 403 Forbidden.
func RequireRole(rolesPermitidos ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rolVal, exists := c.Get("rol")
		if !exists {
			httperrors.ResponderError(
				c,
				http.StatusForbidden,
				"Acceso denegado: rol de usuario no identificado en la sesión.",
				errors.New("rol ausente en el contexto de autenticación"),
			)
			return
		}

		rolUsuario, ok := rolVal.(string)
		if !ok || strings.TrimSpace(rolUsuario) == "" {
			httperrors.ResponderError(
				c,
				http.StatusForbidden,
				"Acceso denegado: rol de usuario inválido.",
				errors.New("formato de rol erróneo o vacío"),
			)
			return
		}

		rolUsuarioLimpio := strings.ToLower(strings.TrimSpace(rolUsuario))
		autorizado := false

		for _, rol := range rolesPermitidos {
			if rolUsuarioLimpio == strings.ToLower(strings.TrimSpace(rol)) {
				autorizado = true
				break
			}
		}

		if !autorizado {
			httperrors.ResponderError(
				c,
				http.StatusForbidden,
				"Acceso denegado: permisos insuficientes para acceder a este recurso.",
				errors.New("el rol del usuario no tiene autorización para esta ruta"),
			)
			return
		}

		c.Next()
	}
}
