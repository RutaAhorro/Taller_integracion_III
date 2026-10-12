package middleware

import (
	"context"
	"errors"
	"fmt"
	httperrors "github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/httperrors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"net/http"
	"time"
)

// Incremento y TTL son atómicos: ni concurrencia ni caída entre comandos dejan
// una IP bloqueada indefinidamente. Se preserva el namespace Redis del monolito.
var incrementWithExpiry = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 or redis.call('PTTL', KEYS[1]) < 0 then
 redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return n`)

func RateLimiterIP(rdb *redis.Client, prefijo string, maxIntentos int64, ventana time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil {
			httperrors.ResponderError(c, 500, "Servicio temporalmente no disponible", errors.New("rate limiter no configurado"))
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		key := fmt.Sprintf("rate_limit:%s:%s", prefijo, c.ClientIP())
		attempts, err := incrementWithExpiry.Run(ctx, rdb, []string{key}, ventana.Milliseconds()).Int64()
		if err != nil {
			httperrors.ResponderError(c, 500, "Servicio temporalmente no disponible", errors.New("rate limiter no disponible"))
			return
		}
		if attempts > maxIntentos {
			httperrors.ResponderError(c, http.StatusTooManyRequests, "Demasiados intentos de acceso. Por favor, espere 15 minutos antes de volver a intentar.", errors.New("límite de peticiones excedido (máximo 5 intentos cada 15 minutos)"))
			return
		}
		c.Next()
	}
}
func RateLimitLogin(rdb *redis.Client) gin.HandlerFunc {
	return RateLimiterIP(rdb, "login", 5, 15*time.Minute)
}
