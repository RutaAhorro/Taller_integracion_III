// Pruebas de la lectura y validación del entorno: todo valor inválido impide
// arrancar y ningún mensaje de error reproduce secretos.
package config_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/config"
)

// ===== Entorno base =====

const (
	tokenServicioPrueba = "ssssssssssssssssssssssssssssssss"
	tokenCatalogoPrueba = "tttttttttttttttttttttttttttttttt"
)

// prepararEntorno define un entorno válido mínimo para cada prueba.
func prepararEntorno(prueba *testing.T) {
	prueba.Helper()
	prueba.Setenv("SCRAPING_DB_URL", "postgres://app_scraping:secreto@db/prueba")
	prueba.Setenv("REDIS_URL", "redis://redis:6379/0")
	prueba.Setenv("CATALOGO_INTERNAL_URL", "http://catalogo:8080")
	prueba.Setenv("CATALOGO_INTERNAL_TOKEN", tokenCatalogoPrueba)
	prueba.Setenv("SCRAPING_SERVICE_TOKEN", tokenServicioPrueba)
	prueba.Setenv("CATALOGO_INGEST_TIMEOUT", "")
	prueba.Setenv("SCRAPING_CORS_ALLOWED_ORIGINS", "")
	prueba.Setenv("SCRAPING_SWAGGER_ENABLED", "")
	prueba.Setenv("PORT", "")
}

// ===== Valores válidos y por defecto =====

// TestCargarValoresPredeterminados comprueba los valores por defecto documentados.
func TestCargarValoresPredeterminados(prueba *testing.T) {
	prepararEntorno(prueba)
	configuracion, errorObtenido := config.Cargar()
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if configuracion.DireccionEscucha != ":8080" || configuracion.PlazoCatalogo != 10*time.Second || configuracion.SwaggerHabilitado || len(configuracion.OrigenesPermitidos) != 0 {
		prueba.Fatalf("valores por defecto inesperados: %+v", configuracion)
	}
}

// TestCargarNormalizaOrigenCatalogo quita la barra final del origen de Catálogo.
func TestCargarNormalizaOrigenCatalogo(prueba *testing.T) {
	prepararEntorno(prueba)
	prueba.Setenv("CATALOGO_INTERNAL_URL", "https://catalogo:8443/")
	configuracion, errorObtenido := config.Cargar()
	if errorObtenido != nil || configuracion.URLCatalogo != "https://catalogo:8443" {
		prueba.Fatalf("origen=%q error=%v", configuracion.URLCatalogo, errorObtenido)
	}
}

// TestCargarOrigenesCORS lee la variable propia y, si no existe, el fallback compartido.
func TestCargarOrigenesCORS(prueba *testing.T) {
	prepararEntorno(prueba)
	prueba.Setenv("SCRAPING_CORS_ALLOWED_ORIGINS", " https://a.example , ,http://b.example:3000")
	configuracion, errorObtenido := config.Cargar()
	if errorObtenido != nil || strings.Join(configuracion.OrigenesPermitidos, "|") != "https://a.example|http://b.example:3000" {
		prueba.Fatalf("orígenes=%v error=%v", configuracion.OrigenesPermitidos, errorObtenido)
	}
	prueba.Setenv("CORS_ALLOWED_ORIGINS", "https://comun.example")
	configuracion, _ = config.Cargar()
	if strings.Join(configuracion.OrigenesPermitidos, "|") != "https://a.example|http://b.example:3000" {
		prueba.Fatal("la variable propia debe prevalecer sobre el fallback")
	}
	// Setenv registra la restauración; Unsetenv deja la variable sin definir.
	prueba.Setenv("SCRAPING_CORS_ALLOWED_ORIGINS", "")
	if errorObtenido := os.Unsetenv("SCRAPING_CORS_ALLOWED_ORIGINS"); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	configuracion, errorObtenido = config.Cargar()
	if errorObtenido != nil || strings.Join(configuracion.OrigenesPermitidos, "|") != "https://comun.example" {
		prueba.Fatalf("fallback no aplicado: %v %v", configuracion.OrigenesPermitidos, errorObtenido)
	}
}

// ===== Valores rechazados =====

// TestCargarRechazaConfiguracionInsegura recorre cada variable con valores inválidos.
func TestCargarRechazaConfiguracionInsegura(prueba *testing.T) {
	casos := []struct{ variable, valor string }{
		{"SCRAPING_DB_URL", ""},
		{"SCRAPING_DB_URL", "postgres://postgres:no-filtrar@db/prueba"},
		{"SCRAPING_DB_URL", "postgres://app_identity:no-filtrar@db/prueba"},
		{"SCRAPING_DB_URL", "postgres://app_scraping@db/prueba"},
		{"SCRAPING_DB_URL", "postgres://app_scraping:no-filtrar@db/"},
		{"SCRAPING_DB_URL", "postgres://app_scraping:no-filtrar@host invalido/prueba"},
		{"REDIS_URL", ""},
		{"REDIS_URL", "no-es-url"},
		{"REDIS_URL", "ftp://redis:6379"},
		{"SCRAPING_SERVICE_TOKEN", ""},
		{"SCRAPING_SERVICE_TOKEN", strings.Repeat("s", 31)},
		{"SCRAPING_SERVICE_TOKEN", strings.Repeat("s", 20) + " " + strings.Repeat("s", 20)},
		{"SCRAPING_SERVICE_TOKEN", tokenCatalogoPrueba},
		{"CATALOGO_INTERNAL_TOKEN", "corto"},
		{"CATALOGO_INTERNAL_URL", ""},
		{"CATALOGO_INTERNAL_URL", "ftp://catalogo"},
		{"CATALOGO_INTERNAL_URL", "http://usuario:no-filtrar@catalogo"},
		{"CATALOGO_INTERNAL_URL", "http://catalogo/ruta"},
		{"CATALOGO_INTERNAL_URL", "http://catalogo?token=no-filtrar"},
		{"CATALOGO_INTERNAL_URL", "http://catalogo#fragmento"},
		{"CATALOGO_INGEST_TIMEOUT", "diez"},
		{"CATALOGO_INGEST_TIMEOUT", "0s"},
		{"CATALOGO_INGEST_TIMEOUT", "30s"},
		{"SCRAPING_CORS_ALLOWED_ORIGINS", "*"},
		{"SCRAPING_CORS_ALLOWED_ORIGINS", "null"},
		{"SCRAPING_CORS_ALLOWED_ORIGINS", "https://a.example/ruta"},
		{"SCRAPING_SWAGGER_ENABLED", "yes"},
		{"PORT", "0"},
		{"PORT", "70000"},
		{"PORT", "ocho"},
	}
	for _, caso := range casos {
		prueba.Run(caso.variable+"="+caso.valor, func(prueba *testing.T) {
			prepararEntorno(prueba)
			prueba.Setenv(caso.variable, caso.valor)
			_, errorObtenido := config.Cargar()
			if errorObtenido == nil {
				prueba.Fatal("la configuración inválida fue aceptada")
			}
			if strings.Contains(errorObtenido.Error(), "no-filtrar") {
				prueba.Fatalf("el error expone un secreto: %v", errorObtenido)
			}
		})
	}
}
