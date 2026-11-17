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

func Load() (Config, error) {
	configuracion := Config{DatabaseURL: os.Getenv("IDENTITY_DB_URL"), RedisURL: os.Getenv("REDIS_URL"), JWTSecret: os.Getenv("JWT_SECRET"), CookieSecure: true}
	dsn, err := pgx.ParseConfig(configuracion.DatabaseURL)
	if configuracion.DatabaseURL == "" || err != nil || dsn.User != "app_identity" || dsn.Database == "" || dsn.Password == "" {
		return Config{}, errors.New("IDENTITY_DB_URL requiere rol app_identity, base y contraseña explícitos")
	}
	if len(configuracion.JWTSecret) < 32 || strings.TrimSpace(configuracion.JWTSecret) != configuracion.JWTSecret {
		return Config{}, errors.New("JWT_SECRET requiere al menos 32 bytes y no admite espacios externos")
	}
	if configuracion.RedisURL == "" {
		return Config{}, errors.New("REDIS_URL es obligatorio para limitar login")
	}
	if _, err := redis.ParseURL(configuracion.RedisURL); err != nil {
		return Config{}, errors.New("REDIS_URL inválido")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	cantidad, err := strconv.Atoi(port)
	if err != nil || cantidad < 1 || cantidad > 65535 {
		return Config{}, errors.New("PORT debe estar entre 1 y 65535")
	}
	configuracion.ListenAddr = ":" + strconv.Itoa(cantidad)
	if raw := os.Getenv("IDENTIDAD_COOKIE_SECURE"); raw != "" {
		configuracion.CookieSecure, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, errors.New("IDENTIDAD_COOKIE_SECURE debe ser booleano")
		}
	}
	if raw := os.Getenv("CORS_ALLOWED_ORIGINS"); raw != "" {
		for _, entry := range strings.Split(raw, ",") {
			origin := strings.TrimSpace(entry)
			usuario, err := url.Parse(origin)
			if err != nil || usuario.Hostname() == "" || (usuario.Scheme != "http" && usuario.Scheme != "https") || usuario.User != nil || usuario.Path != "" || usuario.RawQuery != "" || usuario.ForceQuery || usuario.Fragment != "" || strings.ContainsAny(origin, "*?#\\ \t\r\n") {
				return Config{}, errors.New("CORS_ALLOWED_ORIGINS requiere orígenes HTTP(S) exactos")
			}
			configuracion.AllowedOrigins = append(configuracion.AllowedOrigins, origin)
		}
	}
	if raw := os.Getenv("IDENTIDAD_TRUSTED_PROXIES"); raw != "" {
		for _, entry := range strings.Split(raw, ",") {
			entry = strings.TrimSpace(entry)
			if prefix, err := netip.ParsePrefix(entry); err == nil && prefix.Bits() == 0 {
				return Config{}, errors.New("IDENTIDAD_TRUSTED_PROXIES no puede confiar en todas las IPs")
			}
			if _, err := netip.ParsePrefix(entry); err != nil {
				if _, err := netip.ParseAddr(entry); err != nil {
					return Config{}, errors.New("IDENTIDAD_TRUSTED_PROXIES requiere IPs o CIDRs explícitos")
				}
			}
			configuracion.TrustedProxies = append(configuracion.TrustedProxies, entry)
		}
	}
	if len(configuracion.TrustedProxies) == 0 {
		return Config{}, errors.New("IDENTIDAD_TRUSTED_PROXIES requiere los proxies Gateway reales antes del corte")
	}
	if configuracion.Correo, err = LoadCorreo(); err != nil {
		return Config{}, err
	}
	if raw := strings.TrimSpace(os.Getenv("IDENTIDAD_URL_RESTABLECER")); raw != "" {
		usuario, err := url.Parse(raw)
		if err != nil || usuario.Hostname() == "" || (usuario.Scheme != "http" && usuario.Scheme != "https") || usuario.User != nil || usuario.RawQuery != "" || usuario.ForceQuery || usuario.Fragment != "" || strings.ContainsAny(raw, "?# \t\r\n") {
			return Config{}, errors.New("IDENTIDAD_URL_RESTABLECER requiere una URL HTTP(S) absoluta sin query ni fragmento")
		}
		configuracion.URLRestablecer = raw
	}
	configuracion.GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	configuracion.Ajustes = AjustesPredeterminados()
	return configuracion, nil
}
