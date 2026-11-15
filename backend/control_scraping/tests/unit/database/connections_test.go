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
	if _, errorObtenido := database.Load(); errorObtenido == nil {
		prueba.Fatal("se aceptó el fallback al administrador")
	}
	prueba.Setenv("IDENTITY_DB_URL", "postgres://app_identity:identity-secret@db/test")
	prueba.Setenv("SCRAPING_DB_URL", "postgres://app_scraping:scraping-secret@db/test")
	if _, errorObtenido := database.Load(); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	for _, cadenaConexion := range []string{
		"postgres://postgres:do-not-leak@db/test", "postgres://app_identity:do-not-leak@db/test",
		"postgres://app_scraping@db/test", "postgres://app_scraping:do-not-leak@db/",
		"postgres://app_scraping:do-not-leak@bad host/test",
	} {
		prueba.Run(cadenaConexion, func(prueba *testing.T) {
			prueba.Setenv("SCRAPING_DB_URL", cadenaConexion)
			_, errorObtenido := database.Load()
			if errorObtenido == nil || strings.Contains(errorObtenido.Error(), "do-not-leak") || strings.Contains(errorObtenido.Error(), "old-admin-secret") {
				prueba.Fatal("credencial inválida aceptada o expuesta", errorObtenido)
			}
		})
	}
}
