package config

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ListenAddr, DatabaseURL, RedisURL, JWTSecret string
	CookieSecure                                 bool
	AllowedOrigins                               []string
	TrustedProxies                               []string
	Correo                                       Correo
	// URLRestablecer es la página del frontend que recibe ?token= (SUP-184). Vacía apaga la recuperación.
	URLRestablecer string
}

func Load() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("IDENTITY_DB_URL"), RedisURL: os.Getenv("REDIS_URL"), JWTSecret: os.Getenv("JWT_SECRET"), CookieSecure: true}
	dsn, err := pgx.ParseConfig(c.DatabaseURL)
	if c.DatabaseURL == "" || err != nil || dsn.User != "app_identity" || dsn.Database == "" || dsn.Password == "" {
		return Config{}, errors.New("IDENTITY_DB_URL requiere rol app_identity, base y contraseña explícitos")
	}
	if len(c.JWTSecret) < 32 || strings.TrimSpace(c.JWTSecret) != c.JWTSecret {
		return Config{}, errors.New("JWT_SECRET requiere al menos 32 bytes y no admite espacios externos")
	}
	if c.RedisURL == "" {
		return Config{}, errors.New("REDIS_URL es obligatorio para limitar login")
	}
	if _, err := redis.ParseURL(c.RedisURL); err != nil {
		return Config{}, errors.New("REDIS_URL inválido")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, errors.New("PORT debe estar entre 1 y 65535")
	}
	c.ListenAddr = ":" + strconv.Itoa(n)
	if raw := os.Getenv("IDENTIDAD_COOKIE_SECURE"); raw != "" {
		c.CookieSecure, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, errors.New("IDENTIDAD_COOKIE_SECURE debe ser booleano")
		}
	}
	if raw := os.Getenv("CORS_ALLOWED_ORIGINS"); raw != "" {
		for _, entry := range strings.Split(raw, ",") {
			origin := strings.TrimSpace(entry)
			u, err := url.Parse(origin)
			if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(origin, "*?#\\ \t\r\n") {
				return Config{}, errors.New("CORS_ALLOWED_ORIGINS requiere orígenes HTTP(S) exactos")
			}
			c.AllowedOrigins = append(c.AllowedOrigins, origin)
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
			c.TrustedProxies = append(c.TrustedProxies, entry)
		}
	}
	if len(c.TrustedProxies) == 0 {
		return Config{}, errors.New("IDENTIDAD_TRUSTED_PROXIES requiere los proxies Gateway reales antes del corte")
	}
	if c.Correo, err = LoadCorreo(); err != nil {
		return Config{}, err
	}
	if raw := strings.TrimSpace(os.Getenv("IDENTIDAD_URL_RESTABLECER")); raw != "" {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(raw, "?# \t\r\n") {
			return Config{}, errors.New("IDENTIDAD_URL_RESTABLECER requiere una URL HTTP(S) absoluta sin query ni fragmento")
		}
		c.URLRestablecer = raw
	}
	return c, nil
}
