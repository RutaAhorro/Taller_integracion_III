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
// segura HttpOnly 'jwt' y revalida en api.usuarios que la cuenta siga activa,
// usando el rol actual de la base de datos: así una desactivación o degradación
// surte efecto sin esperar a que expire el JWT.
//
// El repositorio es obligatorio (fail-closed): una ruta protegida no puede
// omitir por accidente la revalidación de sesión. Sin él sólo se validaría la
// firma del JWT, que sigue vigente hasta 24 h después de desactivar la cuenta.
func RequireAuth(repo domain.UsuarioRepository, clave string) gin.HandlerFunc {
	if repo == nil {
		panic("middleware.RequireAuth: el repositorio de usuarios es obligatorio")
	}

	return func(contexto *gin.Context) {
		tokenString, err := contexto.Cookie("jwt")
		if err != nil || strings.TrimSpace(tokenString) == "" {
			httperrors.ResponderError(
				contexto,
				http.StatusUnauthorized,
				"Cookie de autenticación requerida.",
				errors.New("cookie 'jwt' ausente o no autorizada"),
			)
			return
		}

		token, err := security.ValidarToken(strings.TrimSpace(tokenString), clave)
		if err != nil || token == nil || !token.Valid {
			httperrors.ResponderError(
				contexto,
				http.StatusUnauthorized,
				"Token inválido o expirado.",
				err,
			)
			return
		}

		claims, ok := token.Claims.(*security.JWTClaims)
		if !ok {
			httperrors.ResponderError(
				contexto,
				http.StatusUnauthorized,
				"Claims del token inválidos.",
				errors.New("no fue posible procesar la información de identidad del token"),
			)
			return
		}

		usuarioID := strings.TrimSpace(claims.UserID)
		rol := claims.Rol

		if _, err := uuid.Parse(usuarioID); err != nil {
			httperrors.ResponderError(contexto, http.StatusUnauthorized, "Token inválido o expirado.", err)
			return
		}

		usuario, err := repo.FindByID(contexto.Request.Context(), usuarioID)
		if err != nil {
			httperrors.ResponderError(contexto, http.StatusInternalServerError, "No se pudo validar la sesión.", err)
			return
		}
		if usuario == nil || !usuario.EstaActivo {
			httperrors.ResponderError(contexto, http.StatusUnauthorized, "La sesión ya no está activa.", nil)
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

		// Inyectar identidad vigente en el contexto de Gin para las rutas protegidas.
		contexto.Set("user_id", usuarioID)
		contexto.Set("rol", rol)
		contexto.Set("provider", claims.Provider)

		contexto.Next()
	}
}
