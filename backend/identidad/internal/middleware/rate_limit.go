// Contadores de intentos por IP en Redis para login, registro y Google.

package middleware

import (
	"context"
	"errors"
	"fmt"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"net/http"
	"time"

	httperrors "github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/httperrors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// ===== Scripts Redis =====

// incrementWithExpiry suma un intento y fija el TTL en una sola operación atómica:
// ni la concurrencia ni una caída entre comandos dejan una IP bloqueada para siempre.
// Se preserva el namespace Redis del monolito.
//
//	INCR       suma 1 (crea la clave en 1 si no existía)
//	PEXPIRE    fija la ventana en el primer intento, o si la clave quedó sin TTL
var incrementWithExpiry = redis.NewScript(`
local intentos = redis.call('INCR', KEYS[1])
if intentos == 1 or redis.call('PTTL', KEYS[1]) < 0 then
 redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return intentos`)

// refundAttempt devuelve el intento de una respuesta exitosa. Si la clave ya expiró
// no hace nada: un DECR la recrearía en -1 y sin TTL.
var refundAttempt = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 and tonumber(redis.call('GET', KEYS[1])) > 0 then
 return redis.call('DECR', KEYS[1])
end
return 0`)

// ===== Contador por IP =====

// rateLimiter cuenta por IP; sin soloFallidos cuentan todas las peticiones
// (registro, Google). Incrementa antes del handler para que la cuenta sea atómica bajo
// concurrencia. Con soloFallidos, una respuesta 2xx devuelve su intento: varios
// usuarios legítimos tras un mismo NAT no se bloquean por iniciar sesión bien.
//
// Si Redis no responde, la petición se rechaza (falla cerrado): sin contador,
// el endpoint quedaría expuesto a fuerza bruta.
func rateLimiter(redisCliente *redis.Client, prefijo string, maxIntentos int64, ventana time.Duration, soloFallidos bool, ajustes config.Ajustes) gin.HandlerFunc {
	mensajeLimite := fmt.Sprintf("Demasiados intentos de acceso. Por favor, espere %d minutos antes de volver a intentar.", int(ventana.Minutes()))
	return func(contexto *gin.Context) {
		if redisCliente == nil {
			httperrors.ResponderError(contexto, http.StatusInternalServerError, "Servicio temporalmente no disponible", errors.New("rate limiter no configurado"))
			return
		}
		contextoOperacion, cancelar := context.WithTimeout(contexto.Request.Context(), ajustes.RateTimeout)
		defer cancelar()
		clave := fmt.Sprintf("rate_limit:%s:%s", prefijo, contexto.ClientIP())
		intentos, errorObtenido := incrementWithExpiry.Run(contextoOperacion, redisCliente, []string{clave}, ventana.Milliseconds()).Int64()
		if errorObtenido != nil {
			httperrors.ResponderError(contexto, http.StatusInternalServerError, "Servicio temporalmente no disponible", errors.New("rate limiter no disponible"))
			return
		}
		if intentos > maxIntentos {
			httperrors.ResponderError(contexto, http.StatusTooManyRequests, mensajeLimite, fmt.Errorf("límite de peticiones excedido (máximo %d cada %s)", maxIntentos, ventana))
			return
		}
		contexto.Next()
		if soloFallidos && contexto.Writer.Status() >= 200 && contexto.Writer.Status() < 300 {
			// Contexto propio: el de la petición puede haberse cancelado al responder.
			contextoDevolucion, cancelarDevolucion := context.WithTimeout(context.Background(), ajustes.RateTimeout)
			defer cancelarDevolucion()
			_ = refundAttempt.Run(contextoDevolucion, redisCliente, []string{clave}).Err()
		}
	}
}

// RateLimitLogin: 5 intentos fallidos por IP cada 15 minutos.
func RateLimitLogin(redisCliente *redis.Client, ajustes config.Ajustes) gin.HandlerFunc {
	return rateLimiter(redisCliente, "login", int64(ajustes.LoginMax), ajustes.RateWindow, true, ajustes)
}

// RateLimitRegistro limita registros por IP: bcrypt cuesta CPU y el 409 permite
// enumerar correos. Holgado para una clase registrándose tras el mismo NAT.
func RateLimitRegistro(redisCliente *redis.Client, ajustes config.Ajustes) gin.HandlerFunc {
	return rateLimiter(redisCliente, "register", int64(ajustes.RegistroMax), ajustes.RateWindow, false, ajustes)
}

// RateLimitGoogle limita la verificación de tokens Google por IP.
func RateLimitGoogle(redisCliente *redis.Client, ajustes config.Ajustes) gin.HandlerFunc {
	return rateLimiter(redisCliente, "google", int64(ajustes.GoogleMax), ajustes.RateWindow, false, ajustes)
}
