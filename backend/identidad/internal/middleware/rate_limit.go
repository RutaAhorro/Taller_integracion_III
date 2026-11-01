package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	httperrors "github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/httperrors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// ventanaLimite es el período en que se cuentan los intentos de cada endpoint.
const ventanaLimite = 15 * time.Minute

// incrementWithExpiry suma un intento y fija el TTL en una sola operación atómica:
// ni la concurrencia ni una caída entre comandos dejan una IP bloqueada para siempre.
// Se preserva el namespace Redis del monolito.
//
//	INCR       suma 1 (crea la clave en 1 si no existía)
//	PEXPIRE    fija la ventana en el primer intento, o si la clave quedó sin TTL
var incrementWithExpiry = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 or redis.call('PTTL', KEYS[1]) < 0 then
 redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return n`)

// refundAttempt devuelve el intento de una respuesta exitosa. Si la clave ya expiró
// no hace nada: un DECR la recrearía en -1 y sin TTL.
var refundAttempt = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 and tonumber(redis.call('GET', KEYS[1])) > 0 then
 return redis.call('DECR', KEYS[1])
end
return 0`)

// RateLimiterIP cuenta todas las peticiones por IP (registro, Google).
func RateLimiterIP(rdb *redis.Client, prefijo string, maxIntentos int64, ventana time.Duration) gin.HandlerFunc {
	return rateLimiter(rdb, prefijo, maxIntentos, ventana, false)
}

// rateLimiter incrementa antes del handler para que la cuenta sea atómica bajo
// concurrencia. Con soloFallidos, una respuesta 2xx devuelve su intento: varios
// usuarios legítimos tras un mismo NAT no se bloquean por iniciar sesión bien.
//
// Si Redis no responde, la petición se rechaza (falla cerrado): sin contador,
// el endpoint quedaría expuesto a fuerza bruta.
func rateLimiter(rdb *redis.Client, prefijo string, maxIntentos int64, ventana time.Duration, soloFallidos bool) gin.HandlerFunc {
	mensajeLimite := fmt.Sprintf("Demasiados intentos de acceso. Por favor, espere %d minutos antes de volver a intentar.", int(ventana.Minutes()))
	return func(c *gin.Context) {
		if rdb == nil {
			httperrors.ResponderError(c, http.StatusInternalServerError, "Servicio temporalmente no disponible", errors.New("rate limiter no configurado"))
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		key := fmt.Sprintf("rate_limit:%s:%s", prefijo, c.ClientIP())
		attempts, err := incrementWithExpiry.Run(ctx, rdb, []string{key}, ventana.Milliseconds()).Int64()
		if err != nil {
			httperrors.ResponderError(c, http.StatusInternalServerError, "Servicio temporalmente no disponible", errors.New("rate limiter no disponible"))
			return
		}
		if attempts > maxIntentos {
			httperrors.ResponderError(c, http.StatusTooManyRequests, mensajeLimite, fmt.Errorf("límite de peticiones excedido (máximo %d cada %s)", maxIntentos, ventana))
			return
		}
		c.Next()
		if soloFallidos && c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			// Contexto propio: el de la petición puede haberse cancelado al responder.
			refundCtx, refundCancel := context.WithTimeout(context.Background(), time.Second)
			defer refundCancel()
			_ = refundAttempt.Run(refundCtx, rdb, []string{key}).Err()
		}
	}
}

// RateLimitLogin: 5 intentos fallidos por IP cada 15 minutos.
func RateLimitLogin(rdb *redis.Client) gin.HandlerFunc {
	return rateLimiter(rdb, "login", 5, ventanaLimite, true)
}

// RateLimitRegistro limita registros por IP: bcrypt cuesta CPU y el 409 permite
// enumerar correos. Holgado para una clase registrándose tras el mismo NAT.
func RateLimitRegistro(rdb *redis.Client) gin.HandlerFunc {
	return RateLimiterIP(rdb, "register", 30, ventanaLimite)
}

// RateLimitGoogle limita la verificación de tokens Google por IP.
func RateLimitGoogle(rdb *redis.Client) gin.HandlerFunc {
	return RateLimiterIP(rdb, "google", 30, ventanaLimite)
}
