package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	for _, name := range []string{"DB_URL", "JWT_SECRET", "PORT", "DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME", "CORS_ALLOWED_ORIGINS"} {
		t.Setenv(name, "")
	}
	if _, err := Load(); err == nil {
		t.Fatal("se aceptó la configuración sin PostgreSQL")
	}
	t.Setenv("DB_URL", "postgres://test-only")
	if _, err := Load(); err == nil {
		t.Fatal("se aceptó una clave JWT implícita")
	}
	t.Setenv("JWT_SECRET", "test-only")
	cfg, err := Load()
	if err != nil || cfg.ListenAddr != ":8080" || cfg.MaxOpenConns != 10 || cfg.MaxIdleConns != 5 || cfg.ConnMaxLifetime != 30*time.Minute {
		t.Fatalf("configuración predeterminada incorrecta: %v", err)
	}
	for _, tc := range []struct{ name, value string }{
		{"PORT", "0"}, {"PORT", "70000"}, {"PORT", "not-a-port"},
		{"DB_MAX_OPEN_CONNS", "0"}, {"DB_MAX_IDLE_CONNS", "11"},
		{"DB_CONN_MAX_LIFETIME", "-1m"},
	} {
		t.Run(tc.name+"_"+tc.value, func(t *testing.T) {
			t.Setenv(tc.name, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("se aceptó una configuración inválida")
			}
		})
	}
}

func TestCORSOriginsConfiguration(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test-only")
	t.Setenv("JWT_SECRET", "test-only")
	for _, origin := range []string{"*", "null", "https://*.example.com", "https://example.com/path", "https://user@example.com", "https://example.com?", "https://example.com#", "https://example.com,", "example.com", "ftp://example.com"} {
		t.Run(origin, func(t *testing.T) {
			t.Setenv("CORS_ALLOWED_ORIGINS", origin)
			if _, err := Load(); err == nil {
				t.Fatal("se aceptó un origen inválido")
			}
		})
	}
	t.Setenv("CORS_ALLOWED_ORIGINS", " https://frontend.example, http://localhost:5173 ")
	cfg, err := Load()
	if err != nil || len(cfg.CORSAllowedOrigins) != 2 || cfg.CORSAllowedOrigins[1] != "http://localhost:5173" {
		t.Fatalf("lista explícita inválida: %v", err)
	}
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	cfg, err = Load()
	if err != nil || len(cfg.CORSAllowedOrigins) != 0 {
		t.Fatal("CORS no queda cerrado por defecto")
	}
}

func TestInternalTokenConfiguration(t *testing.T) {
	t.Setenv("DB_URL", "postgres://test-only")
	t.Setenv("JWT_SECRET", "test-only")
	for _, token := range []string{"short", "test-only", "012345678901234567890123456789012\n"} {
		t.Setenv("CATALOGO_INTERNAL_TOKEN", token)
		if _, err := Load(); err == nil {
			t.Error("se aceptó token inválido")
		}
	}
	t.Setenv("CATALOGO_INTERNAL_TOKEN", "0123456789012345678901234567890123")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CATALOGO_INTERNAL_TOKEN", "")
	if cfg, err := Load(); err != nil || cfg.InternalToken != "" {
		t.Fatal("modo lector no disponible", err)
	}
}
