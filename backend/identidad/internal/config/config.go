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
	CorreosAdmin                                       map[string]struct{}
	Ajustes                                            Ajustes
	CookieSegura                                       bool
	AuthLocalHabilitado                                bool
	OrigenesPermitidos                                 []string
	ProxiesConfiables                                  []string
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
	numeroPuerto, errorOperacion := strconv.Atoi(puerto)
	if errorOperacion != nil || numeroPuerto < 1 || numeroPuerto > 65535 {
		return Configuracion{}, errors.New("PORT debe estar entre 1 y 65535")
	}
	configuracion.DireccionEscucha = ":" + strconv.Itoa(numeroPuerto)
	if texto := os.Getenv("IDENTIDAD_COOKIE_SECURE"); texto != "" {
		configuracion.CookieSegura, errorOperacion = strconv.ParseBool(texto)
		if errorOperacion != nil {
			return Configuracion{}, errors.New("IDENTIDAD_COOKIE_SECURE debe ser booleano")
		}
	}
	if texto := os.Getenv("AUTH_LOCAL_HABILITADO"); texto != "" {
		// Sólo "true"/"false" son válidos: ParseBool también acepta "1" y "TRUE",
		// que facilitarían habilitar el acceso local por una configuración ambigua.
		if texto != "true" && texto != "false" {
			return Configuracion{}, errors.New("AUTH_LOCAL_HABILITADO debe ser true o false")
		}
		configuracion.AuthLocalHabilitado = texto == "true"
	}
	if texto := os.Getenv("CORS_ALLOWED_ORIGINS"); texto != "" {
		for _, entrada := range strings.Split(texto, ",") {
			origen := strings.TrimSpace(entrada)
			origenAnalizado, errorObtenido := url.Parse(origen)
			if errorObtenido != nil || origenAnalizado.Hostname() == "" || (origenAnalizado.Scheme != "http" && origenAnalizado.Scheme != "https") || origenAnalizado.User != nil || origenAnalizado.Path != "" || origenAnalizado.RawQuery != "" || origenAnalizado.ForceQuery || origenAnalizado.Fragment != "" || strings.ContainsAny(origen, "*?#\\ \t\r\n") {
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
	configuracion.ClienteGoogle = os.Getenv("GOOGLE_CLIENT_ID")
	correosAdmin, errorCorreosAdmin := parsearCorreosAdmin(os.Getenv("ADMIN_EMAILS"))
	if errorCorreosAdmin != nil {
		return Configuracion{}, errorCorreosAdmin
	}
	configuracion.CorreosAdmin = correosAdmin
	configuracion.Ajustes = AjustesPredeterminados()
	return configuracion, nil
}

// parsearCorreosAdmin interpreta ADMIN_EMAILS (lista separada por comas),
// normaliza a minúsculas sin espacios, ignora entradas vacías y valida cada
// correo sin reproducir direcciones en el mensaje de error.
func parsearCorreosAdmin(entrada string) (map[string]struct{}, error) {
	correos := make(map[string]struct{})
	if strings.TrimSpace(entrada) == "" {
		return correos, nil
	}
	for _, elemento := range strings.Split(entrada, ",") {
		correo := strings.ToLower(strings.TrimSpace(elemento))
		if correo == "" {
			continue
		}
		direccionAnalizada, errorObtenido := mail.ParseAddress(correo)
		if errorObtenido != nil || direccionAnalizada.Address != correo || !strings.Contains(correo, "@") || !strings.Contains(correo, ".") {
			return nil, errors.New("ADMIN_EMAILS contiene un correo electrónico inválido")
		}
		correos[correo] = struct{}{}
	}
	return correos, nil
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
	MaximoBytesCabeceras, MaximoBytesCuerpo, MaximoConexionesAbiertas, MaximoConexionesInactivas int
	MaximoLogin, MaximoRegistro, MaximoGoogle, MaximoRestablecer                                 int
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

		// Contadores por IP: login y restablecimiento cuentan fallos; registro y Google, todo.
		PlazoContador:     time.Second,
		VentanaContador:   15 * time.Minute,
		MaximoLogin:       5,
		MaximoRestablecer: 10,
		MaximoRegistro:    30,
		MaximoGoogle:      30,
	}
}
