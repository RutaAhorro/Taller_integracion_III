package database_test

import (
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/database"
)

func TestRuntimeCredentials(prueba *testing.T) {
	prueba.Setenv("DB_URL", "postgres://postgres:old-admin-secret@db/test")
	prueba.Setenv("IDENTITY_DB_URL", "")
	prueba.Setenv("SCRAPING_DB_URL", "")
	if _, err := database.Load(); err == nil {
		prueba.Fatal("se aceptó el fallback al administrador")
	}
	prueba.Setenv("IDENTITY_DB_URL", "postgres://app_identity:identity-secret@db/test")
	prueba.Setenv("SCRAPING_DB_URL", "postgres://app_scraping:scraping-secret@db/test")
	if _, err := database.Load(); err != nil {
		prueba.Fatal(err)
	}
	for _, dsn := range []string{
		"postgres://postgres:do-not-leak@db/test", "postgres://app_identity:do-not-leak@db/test",
		"postgres://app_scraping@db/test", "postgres://app_scraping:do-not-leak@db/",
		"postgres://app_scraping:do-not-leak@bad host/test",
	} {
		prueba.Run(dsn, func(prueba *testing.T) {
			prueba.Setenv("SCRAPING_DB_URL", dsn)
			_, err := database.Load()
			if err == nil || strings.Contains(err.Error(), "do-not-leak") || strings.Contains(err.Error(), "old-admin-secret") {
				prueba.Fatal("credencial inválida aceptada o expuesta", err)
			}
		})
	}
}
