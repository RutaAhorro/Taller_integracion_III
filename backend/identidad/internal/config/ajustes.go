// Parámetros operativos de Identidad: plazos, límites y tamaños de pool.
// Son valores fijos del servicio y no variables de entorno, porque ningún
// ambiente los cambia. El arranque los inyecta en cada componente y las pruebas
// pueden acortarlos sin tocar el entorno.

package config

import "time"

// DuracionSesion es parte del contrato compartido: JWT y cookie duran lo mismo.
const DuracionSesion = 24 * time.Hour

// Ajustes reúne límites y plazos locales; no contiene credenciales.
type Ajustes struct {
	InicioTimeout, ApagadoTimeout, RedisTimeout                    time.Duration
	ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout      time.Duration
	RequestTimeout, ReadinessTimeout, DBMaxLifetime, GoogleTimeout time.Duration
	RateTimeout, RateWindow                                        time.Duration
	RecuperacionVigencia, RecuperacionEspera, RecuperacionVentana  time.Duration
	RecuperacionTimeout                                            time.Duration
	MaxHeaderBytes, MaxBodyBytes, DBMaxOpen, DBMaxIdle             int
	LoginMax, RegistroMax, GoogleMax, RecuperacionMax              int
}

// AjustesPredeterminados son los valores con que corre el servicio.
func AjustesPredeterminados() Ajustes {
	return Ajustes{
		// Arranque y apagado del proceso.
		InicioTimeout:  5 * time.Second,
		ApagadoTimeout: 15 * time.Second,

		// Servidor HTTP y presupuesto de cada petición.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       time.Minute,
		RequestTimeout:    10 * time.Second,
		MaxHeaderBytes:    64 * 1024,
		MaxBodyBytes:      64 * 1024,

		// Dependencias: Redis, sonda de readiness y validación de Google.
		RedisTimeout:     time.Second,
		ReadinessTimeout: 2 * time.Second,
		GoogleTimeout:    5 * time.Second,

		// Pool de PostgreSQL por réplica.
		DBMaxOpen:     5,
		DBMaxIdle:     2,
		DBMaxLifetime: 30 * time.Minute,

		// Contadores por IP: login cuenta fallos; registro y Google, todo.
		RateTimeout: time.Second,
		RateWindow:  15 * time.Minute,
		LoginMax:    5,
		RegistroMax: 30,
		GoogleMax:   30,

		// Recuperación de contraseña (SUP-184): vigencia del token y límite por cuenta.
		RecuperacionVigencia: 30 * time.Minute,
		RecuperacionEspera:   time.Minute,
		RecuperacionVentana:  time.Hour,
		RecuperacionMax:      5,
		RecuperacionTimeout:  45 * time.Second,
	}
}
