package database

import (
	"strings"
	"testing"
)

func TestRuntimeCredentials(t *testing.T) {
	t.Setenv("DB_URL", "postgres://postgres:old-admin-secret@db/test")
	t.Setenv("IDENTITY_DB_URL", "")
	t.Setenv("SCRAPING_DB_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("se aceptó el fallback al administrador")
	}
	t.Setenv("IDENTITY_DB_URL", "postgres://app_identity:identity-secret@db/test")
	t.Setenv("SCRAPING_DB_URL", "postgres://app_scraping:scraping-secret@db/test")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	for _, dsn := range []string{
		"postgres://postgres:do-not-leak@db/test", "postgres://app_identity:do-not-leak@db/test",
		"postgres://app_scraping@db/test", "postgres://app_scraping:do-not-leak@db/",
		"postgres://app_scraping:do-not-leak@bad host/test",
	} {
		t.Run(dsn, func(t *testing.T) {
			t.Setenv("SCRAPING_DB_URL", dsn)
			_, err := Load()
			if err == nil || strings.Contains(err.Error(), "do-not-leak") || strings.Contains(err.Error(), "old-admin-secret") {
				t.Fatal("credencial inválida aceptada o expuesta", err)
			}
		})
	}
}
