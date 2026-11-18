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

// Configuracion es la instantánea validada del entorno que el arranque inyecta en
// cada componente.
type Configuracion struct {
	DireccionEscucha, URLBaseDatos, URLRedis, ClaveJWT string
	ClienteGoogle                                      string
	Ajustes                                            Ajustes
	CookieSegura                                       bool
	OrigenesPermitidos                                 []string
	ProxiesConfiables                                  []string
	Correo                                             Correo
	// URLRestablecer es la página del frontend que recibe ?token= (SUP-184). Vacía apaga la recuperación.
	URLRestablecer string
}

// Cargar lee y valida el entorno. Falla al arrancar ante cualquier valor
// inválido, sin reproducir secretos en el mensaje de error.
func Cargar() (Configuracion, error) {
	configuracion := Configuracion{URLBaseDatos: os.Getenv("IDENTITY_DB_URL"), URLRedis: os.Getenv("REDIS_URL"), ClaveJWT: os.Getenv("JWT_SECRET"), CookieSegura: true}
	dsn, errorOperacion := pgx.ParseConfig(configuracion.URLBaseDatos)
	if configuracion.URLBaseDatos == "" || errorOperacion != nil || dsn.User != "app_identity" || dsn.Database == "" || dsn.Password == "" {
		return Configuracion{}, errors.New("IDENTITY_DB_URL requiere rol app_identity, base y contraseña explícitos")
	}
	if len(configuracion.ClaveJWT) < 32 || strings.TrimSpace(configuracion.ClaveJWT) != configuracion.ClaveJWT {
		return Configuracion{}, errors.New("JWT_SECRET requiere al menos 32 bytes y no admite espacios externos")
	}
	if configuracion.URLRedis == "" {
		return Configuracion{}, errors.New("REDIS_URL es obligatorio para limitar login")
	}
	if _, errorObtenido := redis.ParseURL(configuracion.URLRedis); errorObtenido != nil {
		return Configuracion{}, errors.New("REDIS_URL inválido")
	}
	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8080"
	}
	cantidad, errorOperacion := strconv.Atoi(puerto)
	if errorOperacion != nil || cantidad < 1 || cantidad > 65535 {
		return Configuracion{}, errors.New("PORT debe estar entre 1 y 65535")
	}
	configuracion.DireccionEscucha = ":" + strconv.Itoa(cantidad)
	if texto := os.Getenv("IDENTIDAD_COOKIE_SECURE"); texto != "" {
		configuracion.CookieSegura, errorOperacion = strconv.ParseBool(texto)
		if errorOperacion != nil {
			return Configuracion{}, errors.New("IDENTIDAD_COOKIE_SECURE debe ser booleano")
		}
	}
	if texto := os.Getenv("CORS_ALLOWED_ORIGINS"); texto != "" {
		for _, entrada := range strings.Split(texto, ",") {
			origen := strings.TrimSpace(entrada)
			usuario, errorObtenido := url.Parse(origen)
			if errorObtenido != nil || usuario.Hostname() == "" || (usuario.Scheme != "http" && usuario.Scheme != "https") || usuario.User != nil || usuario.Path != "" || usuario.RawQuery != "" || usuario.ForceQuery || usuario.Fragment != "" || strings.ContainsAny(origen, "*?#\\ \t\r\n") {
				return Configuracion{}, errors.New("CORS_ALLOWED_ORIGINS requiere orígenes HTTP(S) exactos")
			}
			configuracion.OrigenesPermitidos = append(configuracion.OrigenesPermitidos, origen)
		}
	}
	if texto := os.Getenv("IDENTIDAD_TRUSTED_PROXIES"); texto != "" {
		for _, entrada := range strings.Split(texto, ",") {
			entrada = strings.TrimSpace(entrada)
			if prefijo, errorObtenido := netip.ParsePrefix(entrada); errorObtenido == nil && prefijo.Bits() == 0 {
				return Configuracion{}, errors.New("IDENTIDAD_TRUSTED_PROXIES no puede confiar en todas las IPs")
			}
			if _, errorInterno := netip.ParsePrefix(entrada); errorInterno != nil {
				if _, errorObtenido := netip.ParseAddr(entrada); errorObtenido != nil {
					return Configuracion{}, errors.New("IDENTIDAD_TRUSTED_PROXIES requiere IPs o CIDRs explícitos")
				}
			}
			configuracion.ProxiesConfiables = append(configuracion.ProxiesConfiables, entrada)
		}
	}
	if len(configuracion.ProxiesConfiables) == 0 {
		return Configuracion{}, errors.New("IDENTIDAD_TRUSTED_PROXIES requiere los proxies Gateway reales antes del corte")
	}
	if configuracion.Correo, errorOperacion = CargarCorreo(); errorOperacion != nil {
		return Configuracion{}, errorOperacion
	}
	if texto := strings.TrimSpace(os.Getenv("IDENTIDAD_URL_RESTABLECER")); texto != "" {
		usuario, errorObtenido := url.Parse(texto)
		if errorObtenido != nil || usuario.Hostname() == "" || (usuario.Scheme != "http" && usuario.Scheme != "https") || usuario.User != nil || usuario.RawQuery != "" || usuario.ForceQuery || usuario.Fragment != "" || strings.ContainsAny(texto, "?# \t\r\n") {
			return Configuracion{}, errors.New("IDENTIDAD_URL_RESTABLECER requiere una URL HTTP(S) absoluta sin query ni fragmento")
		}
		configuracion.URLRestablecer = texto
	}
	configuracion.ClienteGoogle = os.Getenv("GOOGLE_CLIENT_ID")
	configuracion.Ajustes = AjustesPredeterminados()
	return configuracion, nil
}

// ===== Plazos y límites fijos =====

// DuracionSesion es parte del contrato compartido: JWT y cookie duran lo mismo.
const DuracionSesion = 24 * time.Hour

// Ajustes reúne límites y plazos locales; no contiene credenciales.
type Ajustes struct {
	PlazoInicio, PlazoApagado, PlazoRedis                                                        time.Duration
	PlazoCabeceras, PlazoLectura, PlazoEscritura, PlazoInactividad                               time.Duration
	PlazoPeticion, PlazoReadiness, VidaMaximaConexion, PlazoGoogle                               time.Duration
	PlazoContador, VentanaContador                                                               time.Duration
	RecuperacionVigencia, RecuperacionEspera, RecuperacionVentana                                time.Duration
	RecuperacionPlazo                                                                            time.Duration
	MaximoBytesCabeceras, MaximoBytesCuerpo, MaximoConexionesAbiertas, MaximoConexionesInactivas int
	MaximoLogin, MaximoRegistro, MaximoGoogle, RecuperacionMaximo                                int
}

// AjustesPredeterminados son los valores con que corre el servicio.
func AjustesPredeterminados() Ajustes {
	return Ajustes{
		// Arranque y apagado del proceso.
		PlazoInicio:  5 * time.Second,
		PlazoApagado: 15 * time.Second,

		// Servidor HTTP y presupuesto de cada petición.
		PlazoCabeceras:       5 * time.Second,
		PlazoLectura:         15 * time.Second,
		PlazoEscritura:       20 * time.Second,
		PlazoInactividad:     time.Minute,
		PlazoPeticion:        10 * time.Second,
		MaximoBytesCabeceras: 64 * 1024,
		MaximoBytesCuerpo:    64 * 1024,

		// Dependencias: Redis, sonda de readiness y validación de Google.
		PlazoRedis:     time.Second,
		PlazoReadiness: 2 * time.Second,
		PlazoGoogle:    5 * time.Second,

		// Pool de PostgreSQL por réplica.
		MaximoConexionesAbiertas:  5,
		MaximoConexionesInactivas: 2,
		VidaMaximaConexion:        30 * time.Minute,

		// Contadores por IP: login cuenta fallos; registro y Google, todo.
		PlazoContador:   time.Second,
		VentanaContador: 15 * time.Minute,
		MaximoLogin:     5,
		MaximoRegistro:  30,
		MaximoGoogle:    30,

		// Recuperación de contraseña (SUP-184): vigencia del token y límite por cuenta.
		RecuperacionVigencia: 30 * time.Minute,
		RecuperacionEspera:   time.Minute,
		RecuperacionVentana:  time.Hour,
		RecuperacionMaximo:   5,
		RecuperacionPlazo:    45 * time.Second,
	}
}

// ===== Constantes =====

// PlazoSMTPPredeterminado acota cada envío por el transporte SMTP.
const PlazoSMTPPredeterminado = 30 * time.Second

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
	Plazo                                      time.Duration
}

// ===== Carga =====

// CargarCorreo lee y valida sólo la configuración de correo; Load la incorpora al arrancar.
// Sin IDENTIDAD_CORREO_MODO, hay SMTP cuando existen credenciales: así el Deployment
// arranca con correo deshabilitado mientras el Secret identidad-smtp no exista.
func CargarCorreo() (Correo, error) {
	configuracion := Correo{
		Plazo:     PlazoSMTPPredeterminado,
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
		return Correo{Modo: configuracion.Modo, Plazo: configuracion.Plazo}, nil
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
