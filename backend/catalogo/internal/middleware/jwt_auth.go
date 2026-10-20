package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/auth"
	"github.com/gin-gonic/gin"
)

// RequireAuth valida la firma del JWT y, si recibe un validador, revalida la
// cookie contra Identidad para usar el estado y rol vigentes de api.usuarios.
// Sin validador falla cerrado: la administración queda en 503.
func RequireAuth(secret string, validators ...auth.SessionValidator) gin.HandlerFunc {
	var validator auth.SessionValidator
	if len(validators) > 0 {
		validator = validators[0]
	}

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

		if validator == nil {
			ResponderError(
				c,
				http.StatusServiceUnavailable,
				"No se pudo validar la sesión.",
				errors.New("validador de sesión de Identidad no configurado"),
			)
			return
		}

		session, err := validator.Validate(c.Request.Context(), c.Request.Header.Get("Cookie"))
		if err != nil {
			if errors.Is(err, auth.ErrSesionInactiva) {
				ResponderError(c, http.StatusUnauthorized, "La sesión ya no está activa.", nil)
				return
			}
			ResponderError(
				c,
				http.StatusServiceUnavailable,
				"No se pudo validar la sesión.",
				err,
			)
			return
		}
		// Identidad debe devolver el mismo sujeto que el token firmado. Un
		// identificador vacío o distinto se rechaza para no aceptar sesiones
		// ambiguas aunque el rol devuelto sea correcto.
		if strings.TrimSpace(session.UserID) == "" || session.UserID != claims.UserID {
			ResponderError(c, http.StatusUnauthorized, "Token inválido o expirado.", nil)
			return
		}

		// El rol vigente es el de Identidad/api.usuarios, no el claim firmado.
		rol := strings.ToLower(strings.TrimSpace(session.Rol))
		if rol == "administrador" {
			rol = "admin"
		}

		c.Set("user_id", session.UserID)
		c.Set("rol", rol)
		c.Set("provider", claims.Provider)
		c.Next()
	}
}
