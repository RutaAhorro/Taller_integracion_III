// Package middleware agrupa los middlewares HTTP de Catálogo. Este archivo
// valida el token de servicio de las rutas internas.
package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ===== Token de servicio =====

// RequireInternalToken protege las rutas /internal/v1/* con el token de
// servicio que reciba. No usa JWT ni headers de identidad: la comparación se
// hace en tiempo constante y cualquier petición de navegador (Origin) se
// rechaza antes de llegar al handler.
func RequireInternalToken(token string) gin.HandlerFunc {
	esperado := sha256.Sum256([]byte("Bearer " + token))
	return func(contexto *gin.Context) {
		if contexto.GetHeader("Origin") != "" {
			contexto.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "internal_origin_forbidden"})
			return
		}
		recibido := sha256.Sum256([]byte(contexto.GetHeader("Authorization")))
		if subtle.ConstantTimeCompare(recibido[:], esperado[:]) != 1 {
			contexto.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "internal_unauthorized"})
			return
		}
		contexto.Next()
	}
}
