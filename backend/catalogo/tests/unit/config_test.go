package unit_test

import (
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/config"
)

func TestLoad(prueba *testing.T) {
	for _, name := range []string{"DB_URL", "JWT_SECRET", "PORT", "DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME", "CORS_ALLOWED_ORIGINS"} {
		prueba.Setenv(name, "")
	}
	if _, err := config.Load(); err == nil {
		prueba.Fatal("se aceptó la configuración sin PostgreSQL")
	}
	prueba.Setenv("DB_URL", "postgres://test-only")
	if _, err := config.Load(); err == nil {
		prueba.Fatal("se aceptó una clave JWT implícita")
	}
	prueba.Setenv("JWT_SECRET", "test-only")
	cfg, err := config.Load()
	if err != nil || cfg.ListenAddr != ":8080" || cfg.MaxOpenConns != 10 || cfg.MaxIdleConns != 5 || cfg.ConnMaxLifetime != 30*time.Minute {
		prueba.Fatalf("configuración predeterminada incorrecta: %v", err)
	}
	for _, tc := range []struct{ name, value string }{
		{"PORT", "0"}, {"PORT", "70000"}, {"PORT", "not-a-port"},
		{"DB_MAX_OPEN_CONNS", "0"}, {"DB_MAX_IDLE_CONNS", "11"},
		{"DB_CONN_MAX_LIFETIME", "-1m"},
	} {
		prueba.Run(tc.name+"_"+tc.value, func(subprueba *testing.T) {
			subprueba.Setenv(tc.name, tc.value)
			if _, err := config.Load(); err == nil {
				subprueba.Fatal("se aceptó una configuración inválida")
			}
		})
	}
}

func TestCORSOriginsConfiguration(prueba *testing.T) {
	prueba.Setenv("DB_URL", "postgres://test-only")
	prueba.Setenv("JWT_SECRET", "test-only")
	for _, origin := range []string{"*", "null", "https://*.example.com", "https://example.com/path", "https://user@example.com", "https://example.com?", "https://example.com#", "https://example.com,", "example.com", "ftp://example.com"} {
		prueba.Run(origin, func(subprueba *testing.T) {
			subprueba.Setenv("CORS_ALLOWED_ORIGINS", origin)
			if _, err := config.Load(); err == nil {
				subprueba.Fatal("se aceptó un origen inválido")
			}
		})
	}
	prueba.Setenv("CORS_ALLOWED_ORIGINS", " https://frontend.example, http://localhost:5173 ")
	cfg, err := config.Load()
	if err != nil || len(cfg.CORSAllowedOrigins) != 2 || cfg.CORSAllowedOrigins[1] != "http://localhost:5173" {
		prueba.Fatalf("lista explícita inválida: %v", err)
	}
	prueba.Setenv("CORS_ALLOWED_ORIGINS", "")
	cfg, err = config.Load()
	if err != nil || len(cfg.CORSAllowedOrigins) != 0 {
		prueba.Fatal("CORS no queda cerrado por defecto")
	}
}

func TestInternalTokenConfiguration(prueba *testing.T) {
	prueba.Setenv("DB_URL", "postgres://test-only")
	prueba.Setenv("JWT_SECRET", "test-only")
	for _, token := range []string{"short", "test-only", "012345678901234567890123456789012\n"} {
		prueba.Setenv("CATALOGO_INTERNAL_TOKEN", token)
		if _, err := config.Load(); err == nil {
			prueba.Error("se aceptó token inválido")
		}
	}
	prueba.Setenv("CATALOGO_INTERNAL_TOKEN", "0123456789012345678901234567890123")
	if _, err := config.Load(); err != nil {
		prueba.Fatal(err)
	}
	prueba.Setenv("CATALOGO_INTERNAL_TOKEN", "")
	if cfg, err := config.Load(); err != nil || cfg.InternalToken != "" {
		prueba.Fatal("modo lector no disponible", err)
	}
}
