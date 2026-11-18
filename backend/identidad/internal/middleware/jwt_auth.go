// Package middleware contiene los controles previos a los handlers: sesión
// JWT revalidada en base de datos, rol requerido y límite de intentos por IP.
package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	httperrors "github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/httperrors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ===== Sesión =====

// RequireAuth intercepta la petición HTTP validando el token en la cookie
// segura HttpOnly 'jwt' y revalida en api.usuarios que la cuenta siga activa,
// usando el rol actual de la base de datos: así una desactivación o degradación
// surte efecto sin esperar a que expire el JWT.
//
// El repositorio es obligatorio (fail-closed): una ruta protegida no puede
// omitir por accidente la revalidación de sesión. Sin él sólo se validaría la
// firma del JWT, que sigue vigente hasta 24 h después de desactivar la cuenta.
func RequireAuth(repositorio domain.UsuarioRepository, clave string) gin.HandlerFunc {
	if repositorio == nil {
		panic("middleware.RequireAuth: el repositorio de usuarios es obligatorio")
	}

	return func(contexto *gin.Context) {
		tokenTexto, errorOperacion := contexto.Cookie("jwt")
		if errorOperacion != nil || strings.TrimSpace(tokenTexto) == "" {
			httperrors.ResponderError(
				contexto,
				http.StatusUnauthorized,
				"Cookie de autenticación requerida.",
				errors.New("cookie 'jwt' ausente o no autorizada"),
			)
			return
		}

		token, errorOperacion := security.ValidarToken(strings.TrimSpace(tokenTexto), clave)
		if errorOperacion != nil || token == nil || !token.Valid {
			httperrors.ResponderError(
				contexto,
				http.StatusUnauthorized,
				"Token inválido o expirado.",
				errorOperacion,
			)
			return
		}

		datosToken, existe := token.Claims.(*security.JWTClaims)
		if !existe {
			httperrors.ResponderError(
				contexto,
				http.StatusUnauthorized,
				"Claims del token inválidos.",
				errors.New("no fue posible procesar la información de identidad del token"),
			)
			return
		}

		usuarioID := strings.TrimSpace(datosToken.UserID)
		rol := datosToken.Rol

		if _, errorObtenido := uuid.Parse(usuarioID); errorObtenido != nil {
			httperrors.ResponderError(contexto, http.StatusUnauthorized, "Token inválido o expirado.", errorObtenido)
			return
		}

		usuario, errorOperacion := repositorio.FindByID(contexto.Request.Context(), usuarioID)
		if errorOperacion != nil {
			httperrors.ResponderError(contexto, http.StatusInternalServerError, "No se pudo validar la sesión.", errorOperacion)
			return
		}
		if usuario == nil || !usuario.EstaActivo {
			httperrors.ResponderError(contexto, http.StatusUnauthorized, "La sesión ya no está activa.", nil)
			return
		}

		// El rol vigente es el de la base de datos, no el que quedó en el
		// JWT. Los alias históricos se normalizan a "admin".
		if canonico, valido := domain.NormalizarRol(usuario.Rol); valido {
			rol = canonico
		} else {
			rol = strings.ToLower(strings.TrimSpace(usuario.Rol))
		}
		usuarioID = usuario.ID.String()

		// Inyectar identidad vigente en el contexto de Gin para las rutas protegidas.
		contexto.Set("user_id", usuarioID)
		contexto.Set("rol", rol)
		contexto.Set("provider", datosToken.Provider)

		contexto.Next()
	}
}
