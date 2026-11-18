// Package config lee y valida una sola vez el entorno de Identidad y fija sus
// plazos y límites. Ningún otro paquete consulta variables de entorno: reciben
// la configuración inyectada al arrancar.
package config

import (
	"errors"
	"net/mail"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// ===== Configuración del servicio =====

// Config es la instantánea validada del entorno que el arranque inyecta en
// cada componente.
type Config struct {
	ListenAddr, DatabaseURL, RedisURL, JWTSecret string
	GoogleClientID                               string
	Ajustes                                      Ajustes
	CookieSecure                                 bool
	AllowedOrigins                               []string
	TrustedProxies                               []string
	Correo                                       Correo
	// URLRestablecer es la página del frontend que recibe ?token= (SUP-184). Vacía apaga la recuperación.
	URLRestablecer string
}

// Load lee y valida el entorno. Falla al arrancar ante cualquier valor
// inválido, sin reproducir secretos en el mensaje de error.
func Load() (Config, error) {
	configuracion := Config{DatabaseURL: os.Getenv("IDENTITY_DB_URL"), RedisURL: os.Getenv("REDIS_URL"), JWTSecret: os.Getenv("JWT_SECRET"), CookieSecure: true}
	dsn, errorOperacion := pgx.ParseConfig(configuracion.DatabaseURL)
	if configuracion.DatabaseURL == "" || errorOperacion != nil || dsn.User != "app_identity" || dsn.Database == "" || dsn.Password == "" {
		return Config{}, errors.New("IDENTITY_DB_URL requiere rol app_identity, base y contraseña explícitos")
	}
	if len(configuracion.JWTSecret) < 32 || strings.TrimSpace(configuracion.JWTSecret) != configuracion.JWTSecret {
		return Config{}, errors.New("JWT_SECRET requiere al menos 32 bytes y no admite espacios externos")
	}
	if configuracion.RedisURL == "" {
		return Config{}, errors.New("REDIS_URL es obligatorio para limitar login")
	}
	if _, errorObtenido := redis.ParseURL(configuracion.RedisURL); errorObtenido != nil {
		return Config{}, errors.New("REDIS_URL inválido")
	}
	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8080"
	}
	cantidad, errorOperacion := strconv.Atoi(puerto)
	if errorOperacion != nil || cantidad < 1 || cantidad > 65535 {
		return Config{}, errors.New("PORT debe estar entre 1 y 65535")
	}
	configuracion.ListenAddr = ":" + strconv.Itoa(cantidad)
	if texto := os.Getenv("IDENTIDAD_COOKIE_SECURE"); texto != "" {
		configuracion.CookieSecure, errorOperacion = strconv.ParseBool(texto)
		if errorOperacion != nil {
			return Config{}, errors.New("IDENTIDAD_COOKIE_SECURE debe ser booleano")
		}
	}
	if texto := os.Getenv("CORS_ALLOWED_ORIGINS"); texto != "" {
		for _, entrada := range strings.Split(texto, ",") {
			origen := strings.TrimSpace(entrada)
			usuario, errorObtenido := url.Parse(origen)
			if errorObtenido != nil || usuario.Hostname() == "" || (usuario.Scheme != "http" && usuario.Scheme != "https") || usuario.User != nil || usuario.Path != "" || usuario.RawQuery != "" || usuario.ForceQuery || usuario.Fragment != "" || strings.ContainsAny(origen, "*?#\\ \t\r\n") {
				return Config{}, errors.New("CORS_ALLOWED_ORIGINS requiere orígenes HTTP(S) exactos")
			}
			configuracion.AllowedOrigins = append(configuracion.AllowedOrigins, origen)
		}
	}
	if texto := os.Getenv("IDENTIDAD_TRUSTED_PROXIES"); texto != "" {
		for _, entrada := range strings.Split(texto, ",") {
			entrada = strings.TrimSpace(entrada)
			if prefijo, errorObtenido := netip.ParsePrefix(entrada); errorObtenido == nil && prefijo.Bits() == 0 {
				return Config{}, errors.New("IDENTIDAD_TRUSTED_PROXIES no puede confiar en todas las IPs")
			}
			if _, errorInterno := netip.ParsePrefix(entrada); errorInterno != nil {
				if _, errorObtenido := netip.ParseAddr(entrada); errorObtenido != nil {
					return Config{}, errors.New("IDENTIDAD_TRUSTED_PROXIES requiere IPs o CIDRs explícitos")
				}
			}
			configuracion.TrustedProxies = append(configuracion.TrustedProxies, entrada)
		}
	}
	if len(configuracion.TrustedProxies) == 0 {
		return Config{}, errors.New("IDENTIDAD_TRUSTED_PROXIES requiere los proxies Gateway reales antes del corte")
	}
	if configuracion.Correo, errorOperacion = LoadCorreo(); errorOperacion != nil {
		return Config{}, errorOperacion
	}
	if texto := strings.TrimSpace(os.Getenv("IDENTIDAD_URL_RESTABLECER")); texto != "" {
		usuario, errorObtenido := url.Parse(texto)
		if errorObtenido != nil || usuario.Hostname() == "" || (usuario.Scheme != "http" && usuario.Scheme != "https") || usuario.User != nil || usuario.RawQuery != "" || usuario.ForceQuery || usuario.Fragment != "" || strings.ContainsAny(texto, "?# \t\r\n") {
			return Config{}, errors.New("IDENTIDAD_URL_RESTABLECER requiere una URL HTTP(S) absoluta sin query ni fragmento")
		}
		configuracion.URLRestablecer = texto
	}
	configuracion.GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	configuracion.Ajustes = AjustesPredeterminados()
	return configuracion, nil
}

// ===== Plazos y límites fijos =====

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

// ===== Constantes =====

// TimeoutSMTPPredeterminado acota cada envío por el transporte SMTP.
const TimeoutSMTPPredeterminado = 30 * time.Second

// Modos de envío de correo de Identidad (SUP-189).
const (
	CorreoDeshabilitado = "deshabilitado"
	CorreoSMTP          = "smtp"
	// CorreoLog imprime los mensajes completos en el log: sólo desarrollo local.
	CorreoLog = "log"
)

// Modos TLS admitidos. No existe modo sin cifrado: las credenciales nunca viajan en claro.
const (
	TLSStartTLS  = "starttls"
	TLSImplicito = "tls"
)

// Correo configura el transporte SMTP. Clave nunca se registra ni aparece en errores.
type Correo struct {
	Modo, Host, TLS, Usuario, Clave, Remitente string
	Puerto                                     int
	Timeout                                    time.Duration
}

// ===== Carga =====

// LoadCorreo lee y valida sólo la configuración de correo; Load la incorpora al arrancar.
// Sin IDENTIDAD_CORREO_MODO, hay SMTP cuando existen credenciales: así el Deployment
// arranca con correo deshabilitado mientras el Secret identidad-smtp no exista.
func LoadCorreo() (Correo, error) {
	configuracion := Correo{
		Timeout:   TimeoutSMTPPredeterminado,
		Modo:      strings.TrimSpace(os.Getenv("IDENTIDAD_CORREO_MODO")),
		Host:      strings.TrimSpace(os.Getenv("SMTP_HOST")),
		Usuario:   os.Getenv("SMTP_USER"),
		Clave:     os.Getenv("SMTP_PASS"),
		Remitente: strings.TrimSpace(os.Getenv("SMTP_FROM")),
	}
	if configuracion.Modo == "" {
		configuracion.Modo = CorreoDeshabilitado
		if configuracion.Usuario != "" || configuracion.Clave != "" {
			configuracion.Modo = CorreoSMTP
		}
	}
	switch configuracion.Modo {
	case CorreoDeshabilitado, CorreoLog:
		return Correo{Modo: configuracion.Modo, Timeout: configuracion.Timeout}, nil
	case CorreoSMTP:
		// Continúa con las validaciones de SMTP más abajo.
	default:
		return Correo{}, errors.New("IDENTIDAD_CORREO_MODO debe ser smtp, log o deshabilitado")
	}
	if configuracion.Host == "" || strings.ContainsAny(configuracion.Host, " \t\r\n/@") {
		return Correo{}, errors.New("SMTP_HOST requiere un nombre de servidor sin esquema ni espacios")
	}
	configuracion.Puerto = 587
	if texto := strings.TrimSpace(os.Getenv("SMTP_PORT")); texto != "" {
		cantidad, errorObtenido := strconv.Atoi(texto)
		if errorObtenido != nil || cantidad < 1 || cantidad > 65535 {
			return Correo{}, errors.New("SMTP_PORT debe estar entre 1 y 65535")
		}
		configuracion.Puerto = cantidad
	}
	configuracion.TLS = strings.TrimSpace(os.Getenv("SMTP_TLS"))
	if configuracion.TLS == "" {
		configuracion.TLS = TLSStartTLS
		if configuracion.Puerto == 465 {
			configuracion.TLS = TLSImplicito
		}
	}
	if configuracion.TLS != TLSStartTLS && configuracion.TLS != TLSImplicito {
		return Correo{}, errors.New("SMTP_TLS debe ser starttls o tls")
	}
	if configuracion.Usuario == "" || configuracion.Clave == "" || strings.ContainsAny(configuracion.Usuario, "\r\n") {
		return Correo{}, errors.New("SMTP_USER y SMTP_PASS son obligatorios en modo smtp")
	}
	if configuracion.Remitente == "" {
		configuracion.Remitente = configuracion.Usuario
	}
	if _, errorObtenido := mail.ParseAddress(configuracion.Remitente); errorObtenido != nil || strings.ContainsAny(configuracion.Remitente, "\r\n") {
		return Correo{}, errors.New("SMTP_FROM requiere una dirección de correo válida")
	}
	return configuracion, nil
}
