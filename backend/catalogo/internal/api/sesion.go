// Sesión: validación local del JWT que emite Identidad en la cookie `jwt` y
// autorización de las rutas de administración. No consulta Identidad ni confía
// en headers de identidad.

package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ===== Contrato del token =====

// Contrato con Identidad: cookie, algoritmo y rol administrador. Identidad sólo
// emite roles de usuarios_rol_check (migración 10), así que el rol se compara
// sin normalizar.
const (
	nombreCookieSesion = "jwt"
	algoritmoJWT       = "HS256"
	rolAdministrador   = "admin"
)

// datosSesion son los claims que Catálogo lee del token.
type datosSesion struct {
	UserID string `json:"user_id"`
	Rol    string `json:"rol"`
	jwt.RegisteredClaims
}

// ===== Autorización =====

// exigirAdministrador valida firma HS256 y expiración del JWT y exige el rol
// admin. La revocación de una cuenta desactivada depende de la vigencia del
// token (ver README).
func exigirAdministrador(claveJWT string) gin.HandlerFunc {
	clave := []byte(claveJWT)
	return func(contexto *gin.Context) {
		tokenTexto, errorCookie := contexto.Cookie(nombreCookieSesion)
		tokenTexto = strings.TrimSpace(tokenTexto)
		if errorCookie != nil || tokenTexto == "" {
			responderError(contexto, http.StatusUnauthorized, "Cookie de autenticación requerida.", "")
			return
		}
		var datos datosSesion
		token, errorToken := jwt.ParseWithClaims(tokenTexto, &datos, func(*jwt.Token) (any, error) { return clave, nil },
			jwt.WithValidMethods([]string{algoritmoJWT}), jwt.WithExpirationRequired())
		if errorToken != nil || !token.Valid || strings.TrimSpace(datos.UserID) == "" || datos.Rol == "" {
			responderError(contexto, http.StatusUnauthorized, "Token inválido o expirado.", "")
			return
		}
		if datos.Rol != rolAdministrador {
			responderError(contexto, http.StatusForbidden, "Acceso denegado: permisos insuficientes para acceder a este recurso.", "")
			return
		}
		contexto.Next()
	}
}
