package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/auth"
	"github.com/gin-gonic/gin"
)

// RequireAuth valida la firma HS256 y la expiración del JWT local. Catálogo es
// autónomo: no consulta Identidad en cada petición. La revocación inmediata de
// cuentas desactivadas depende de la vigencia del access token; si se requiere
// invalidación instantánea, debe resolverse con una lista de revocación
// desacoplada o refresh tokens (ver GUIA_CATALOGO_INDEPENDIENTE.md).
func RequireAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := c.Cookie("jwt")
		if err != nil || strings.TrimSpace(tokenString) == "" {
			ResponderError(
				c,
				http.StatusUnauthorized,
				"Cookie de autenticación requerida.",
				errors.New("cookie 'jwt' ausente o no autorizada"),
			)
			return
		}

		token, err := auth.ValidarToken(strings.TrimSpace(tokenString), secret)
		if err != nil || token == nil || !token.Valid {
			ResponderError(
				c,
				http.StatusUnauthorized,
				"Token inválido o expirado.",
				err,
			)
			return
		}

		claims, ok := token.Claims.(*auth.JWTClaims)
		if !ok {
			ResponderError(
				c,
				http.StatusUnauthorized,
				"Claims del token inválidos.",
				errors.New("no fue posible procesar la información de identidad del token"),
			)
			return
		}

		userID := strings.TrimSpace(claims.UserID)
		rol := strings.ToLower(strings.TrimSpace(claims.Rol))
		if rol == "administrador" {
			rol = "admin"
		}
		if userID == "" || rol == "" {
			ResponderError(
				c,
				http.StatusUnauthorized,
				"Token inválido o expirado.",
				errors.New("el token no identifica usuario y rol"),
			)
			return
		}

		c.Set("user_id", userID)
		c.Set("rol", rol)
		c.Set("provider", claims.Provider)
		c.Next()
	}
}
