package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

// ValidarTokenServicio exige un secreto propio para worker y bot: debe ser largo,
// sin espacios y distinto del token interno que Control usa ante Catálogo.
func ValidarTokenServicio(token, catalogoToken string) error {
	if len(token) < 32 || strings.ContainsAny(token, " \t\r\n") {
		return errors.New("SCRAPING_SERVICE_TOKEN debe tener al menos 32 caracteres y no contener espacios")
	}
	if token == catalogoToken {
		return errors.New("SCRAPING_SERVICE_TOKEN debe ser distinto de CATALOGO_INTERNAL_TOKEN")
	}
	return nil
}

// RequireServiceToken protege /api/v1/scraper: el Gateway es público, así que
// sólo los clientes de servicio (worker y bot) con el Bearer correcto pasan.
// Se comparan hashes de largo fijo para no filtrar el largo ni el contenido por tiempo.
func RequireServiceToken(token string) gin.HandlerFunc {
	expected := sha256.Sum256([]byte("Bearer " + token))
	return func(c *gin.Context) {
		got := sha256.Sum256([]byte(c.GetHeader("Authorization")))
		if subtle.ConstantTimeCompare(got[:], expected[:]) != 1 {
			c.Header("WWW-Authenticate", `Bearer realm="control-scraping"`)
			ResponderError(c, 401, "Token de servicio inválido o ausente.", nil)
			return
		}
		c.Next()
	}
}
