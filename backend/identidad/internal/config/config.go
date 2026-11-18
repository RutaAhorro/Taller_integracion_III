// Package config lee y valida una sola vez el entorno de Identidad. Ningún
// otro paquete consulta variables de entorno: reciben la configuración inyectada.
package config

import (
	"errors"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"

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
