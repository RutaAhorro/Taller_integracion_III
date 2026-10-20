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

// RequireAuth intercepta la petición HTTP validando el token en la cookie
// segura HttpOnly 'jwt'. Si se entrega un repositorio, revalida en api.usuarios
// que la cuenta siga activa y usa el rol actual de la base de datos: así una
// desactivación o degradación surte efecto sin esperar a que expire el JWT.
func RequireAuth(repos ...domain.UsuarioRepository) gin.HandlerFunc {
	var repo domain.UsuarioRepository
	if len(repos) > 0 {
		repo = repos[0]
	}

	return func(c *gin.Context) {
		tokenString, err := c.Cookie("jwt")
		if err != nil || strings.TrimSpace(tokenString) == "" {
			httperrors.ResponderError(
				c,
				http.StatusUnauthorized,
				"Cookie de autenticación requerida.",
				errors.New("cookie 'jwt' ausente o no autorizada"),
			)
			return
		}

		token, err := security.ValidarToken(strings.TrimSpace(tokenString))
		if err != nil || token == nil || !token.Valid {
			httperrors.ResponderError(
				c,
				http.StatusUnauthorized,
				"Token inválido o expirado.",
				err,
			)
			return
		}

		claims, ok := token.Claims.(*security.JWTClaims)
		if !ok {
			httperrors.ResponderError(
				c,
				http.StatusUnauthorized,
				"Claims del token inválidos.",
				errors.New("no fue posible procesar la información de identidad del token"),
			)
			return
		}

		usuarioID := strings.TrimSpace(claims.UserID)
		rol := claims.Rol
		if repo != nil {
			if _, err := uuid.Parse(usuarioID); err != nil {
				httperrors.ResponderError(c, http.StatusUnauthorized, "Token inválido o expirado.", err)
				return
			}

			usuario, err := repo.FindByID(c.Request.Context(), usuarioID)
			if err != nil {
				httperrors.ResponderError(c, http.StatusInternalServerError, "No se pudo validar la sesión.", err)
				return
			}
			if usuario == nil || !usuario.EstaActivo {
				httperrors.ResponderError(c, http.StatusUnauthorized, "La sesión ya no está activa.", nil)
				return
			}

			// El rol vigente es el de la base de datos, no el que quedó en el
			// JWT. Los alias históricos se normalizan a "admin".
			if canonico, ok := domain.NormalizarRol(usuario.Rol); ok {
				rol = canonico
			} else {
				rol = strings.ToLower(strings.TrimSpace(usuario.Rol))
			}
			usuarioID = usuario.ID.String()
		}

		// Inyectar identidad vigente en el contexto de Gin para las rutas protegidas.
		c.Set("user_id", usuarioID)
		c.Set("rol", rol)
		c.Set("provider", claims.Provider)

		c.Next()
	}
}
