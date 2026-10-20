package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr         string
	DatabaseURL        string
	JWTSecret          string
	IdentidadURL       string
	InternalToken      string
	CORSAllowedOrigins []string
	MaxOpenConns       int
	MaxIdleConns       int
	ConnMaxLifetime    time.Duration
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL: os.Getenv("DB_URL"), JWTSecret: os.Getenv("JWT_SECRET"),
		InternalToken: os.Getenv("CATALOGO_INTERNAL_TOKEN"),
		MaxOpenConns:  10, MaxIdleConns: 5, ConnMaxLifetime: 30 * time.Minute,
	}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DB_URL es obligatorio")
	}
	if c.JWTSecret == "" {
		return Config{}, errors.New("JWT_SECRET es obligatorio para verificar permisos administrativos")
	}
	if raw := strings.TrimSpace(os.Getenv("IDENTIDAD_URL")); raw != "" {
		value := strings.TrimRight(raw, "/")
		parsed, err := url.Parse(value)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.Hostname() == "" || parsed.User != nil || parsed.Path != "" ||
			parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" ||
			strings.ContainsAny(value, "*?#\\ \t\r\n") {
			return Config{}, errors.New("IDENTIDAD_URL debe ser un origen HTTP(S) exacto, sin ruta, credenciales ni query")
		}
		c.IdentidadURL = value
	}
	if c.InternalToken != "" && (len(c.InternalToken) < 32 || strings.ContainsAny(c.InternalToken, " \t\r\n") || c.InternalToken == c.JWTSecret) {
		return Config{}, errors.New("CATALOGO_INTERNAL_TOKEN debe tener al menos 32 caracteres sin espacios y ser distinto de JWT_SECRET")
	}
	if raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS")); raw != "" {
		for _, entry := range strings.Split(raw, ",") {
			origin := strings.TrimSpace(entry)
			u, err := url.Parse(origin)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
				u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(origin, "*?#\\ \t\r\n") {
				return Config{}, errors.New("CORS_ALLOWED_ORIGINS debe contener orígenes HTTP(S) exactos, sin rutas ni comodines")
			}
			c.CORSAllowedOrigins = append(c.CORSAllowedOrigins, origin)
		}
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return Config{}, errors.New("PORT debe ser un número entre 1 y 65535")
	}
	c.ListenAddr = ":" + strconv.Itoa(value)
	for name, target := range map[string]*int{
		"DB_MAX_OPEN_CONNS": &c.MaxOpenConns,
		"DB_MAX_IDLE_CONNS": &c.MaxIdleConns,
	} {
		if raw := os.Getenv(name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 {
				return Config{}, errors.New(name + " debe ser un entero positivo")
			}
			*target = value
		}
	}
	if c.MaxIdleConns > c.MaxOpenConns {
		return Config{}, errors.New("DB_MAX_IDLE_CONNS no puede superar DB_MAX_OPEN_CONNS")
	}
	if raw := os.Getenv("DB_CONN_MAX_LIFETIME"); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 {
			return Config{}, errors.New("DB_CONN_MAX_LIFETIME debe ser una duración positiva")
		}
		c.ConnMaxLifetime = value
	}
	return c, nil
}
