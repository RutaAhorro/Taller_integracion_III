// Contadores de intentos por IP en Redis para login, registro, Google y
// restablecimiento de contraseña.

package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// ===== Scripts Redis =====

// sumarIntento suma un intento y fija el TTL en una sola operación atómica:
// ni la concurrencia ni una caída entre comandos dejan una IP bloqueada para siempre.
// Se preserva el namespace Redis del monolito.
//
//	INCR       suma 1 (crea la clave en 1 si no existía)
//	PEXPIRE    fija la ventana en el primer intento, o si la clave quedó sin TTL
var sumarIntento = redis.NewScript(`
local intentos = redis.call('INCR', KEYS[1])
if intentos == 1 or redis.call('PTTL', KEYS[1]) < 0 then
 redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return intentos`)

// devolverIntento devuelve el intento de una respuesta exitosa. Si la clave ya expiró
// no hace nada: un DECR la recrearía en -1 y sin TTL.
var devolverIntento = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 and tonumber(redis.call('GET', KEYS[1])) > 0 then
 return redis.call('DECR', KEYS[1])
end
return 0`)

// ===== Contador por IP =====

// limitarPorIP cuenta por IP; sin soloFallidos cuentan todas las peticiones
// (registro, Google). Incrementa antes del handler para que la cuenta sea atómica bajo
// concurrencia. Con soloFallidos, una respuesta 2xx devuelve su intento: varios
// usuarios legítimos tras un mismo NAT no se bloquean por iniciar sesión bien.
//
// Si Redis no responde, la petición se rechaza (falla cerrado): sin contador,
// el endpoint quedaría expuesto a fuerza bruta.
func limitarPorIP(redisCliente *redis.Client, prefijo string, maxIntentos int64, ventana time.Duration, soloFallidos bool, ajustes config.Ajustes) gin.HandlerFunc {
	mensajeLimite := fmt.Sprintf("Demasiados intentos de acceso. Por favor, espere %d minutos antes de volver a intentar.", int(ventana.Minutes()))
	return func(contexto *gin.Context) {
		if redisCliente == nil {
			ResponderError(contexto, http.StatusInternalServerError, "Servicio temporalmente no disponible", errors.New("rate limiter no configurado"))
			return
		}
		contextoOperacion, cancelar := context.WithTimeout(contexto.Request.Context(), ajustes.PlazoContador)
		defer cancelar()
		clave := fmt.Sprintf("rate_limit:%s:%s", prefijo, contexto.ClientIP())
		intentos, errorObtenido := sumarIntento.Run(contextoOperacion, redisCliente, []string{clave}, ventana.Milliseconds()).Int64()
		if errorObtenido != nil {
			ResponderError(contexto, http.StatusInternalServerError, "Servicio temporalmente no disponible", errors.New("rate limiter no disponible"))
			return
		}
		if intentos > maxIntentos {
			ResponderError(contexto, http.StatusTooManyRequests, mensajeLimite, fmt.Errorf("límite de peticiones excedido (máximo %d cada %s)", maxIntentos, ventana))
			return
		}
		contexto.Next()
		if soloFallidos && contexto.Writer.Status() >= 200 && contexto.Writer.Status() < 300 {
			// Contexto propio: el de la petición puede haberse cancelado al responder.
			contextoDevolucion, cancelarDevolucion := context.WithTimeout(context.Background(), ajustes.PlazoContador)
			defer cancelarDevolucion()
			_ = devolverIntento.Run(contextoDevolucion, redisCliente, []string{clave}).Err()
		}
	}
}

// LimitarLogin: 5 intentos fallidos por IP cada 15 minutos.
func LimitarLogin(redisCliente *redis.Client, ajustes config.Ajustes) gin.HandlerFunc {
	return limitarPorIP(redisCliente, "login", int64(ajustes.MaximoLogin), ajustes.VentanaContador, true, ajustes)
}

// LimitarRegistro limita registros por IP: bcrypt cuesta CPU y el 409 permite
// enumerar correos. Holgado para una clase registrándose tras el mismo NAT.
func LimitarRegistro(redisCliente *redis.Client, ajustes config.Ajustes) gin.HandlerFunc {
	return limitarPorIP(redisCliente, "register", int64(ajustes.MaximoRegistro), ajustes.VentanaContador, false, ajustes)
}

// LimitarGoogle limita la verificación de tokens Google por IP.
func LimitarGoogle(redisCliente *redis.Client, ajustes config.Ajustes) gin.HandlerFunc {
	return limitarPorIP(redisCliente, "google", int64(ajustes.MaximoGoogle), ajustes.VentanaContador, false, ajustes)
}

// LimitarRestablecer limita los intentos fallidos de restablecimiento por IP.
// El token tiene 256 bits de azar, pero el endpoint queda expuesto a quien
// intente adivinar claves o consumir enlaces ajenos.
func LimitarRestablecer(redisCliente *redis.Client, ajustes config.Ajustes) gin.HandlerFunc {
	return limitarPorIP(redisCliente, "restablecer", int64(ajustes.MaximoRestablecer), ajustes.VentanaContador, true, ajustes)
}
