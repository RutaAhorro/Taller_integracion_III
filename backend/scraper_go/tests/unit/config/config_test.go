// config_test.go verifica la carga de variables de entorno, valores por defecto
// y validación de rangos en el paquete config.
package config_test

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
)

// ===== Variables de Entorno de Prueba =====

var clavesEntorno = []string{
	"REDIS_URL", "SCRAPER_API_URL", "SCRAPING_SERVICE_TOKEN", "SCRAPER_STATE_DIR",
	"SCRAPER_LEASE_MS", "SCRAPER_STATE_MIN_FREE_BYTES", "SCRAPER_FAILED_RETENTION_SECONDS",
	"SCRAPER_API_TIMEOUT", "SCRAPER_API_BATCH_SIZE", "SCRAPER_QUEUE_NAME", "SCRAPER_GROUP",
	"SCRAPER_CATEGORY_DIR", "SCRAPER_SPIDERS_FILE", "LOG_LEVEL", "SCRAPER_USER_AGENT",
	"SCRAPER_DOWNLOAD_DELAY_MS", "SCRAPER_DOWNLOAD_TIMEOUT", "SCRAPER_DOWNLOAD_MAXSIZE",
	"SCRAPER_RETRY_TIMES", "SCRAPER_MAX_PAGES", "SCRAPER_REDIS_TIMEOUT_SECONDS",
	"SCRAPER_REDIS_PING_TIMEOUT_SECONDS", "SCRAPER_QUEUE_BLOCK_MS",
	"SCRAPER_JOB_CLOSED_TTL_SECONDS", "SCRAPER_POLL_INTERVAL_MS",
	"SCRAPER_RETRY_INTERVAL_SECONDS", "SCRAPER_COLLECT_INTERVAL_SECONDS",
	"SCRAPER_COLLECT_LIMIT", "SCRAPER_COLLECT_TIMEOUT_SECONDS",
	"SCRAPER_GC_TIMEOUT_SECONDS", "SCRAPER_CRAWL_ATTEMPTS",
	"SCRAPER_ROBOTS_CACHE_TTL_SECONDS", "SCRAPER_ROBOTS_FAILURE_TTL_SECONDS",
	"SCRAPER_ROBOTS_MAX_BYTES", "SCRAPER_BACKOFF_MIN_MS", "SCRAPER_BACKOFF_MAX_SECONDS",
	"JUMBO_CATEGORY_URLS", "SANTA_ISABEL_CATEGORY_URLS", "CUGAT_CATEGORY_URLS",
	"ACUENTA_CATEGORY_URLS", "LIDER_CATEGORY_URLS",
}

// limpiarEntorno deja en blanco todas las variables de configuración del scraper.
func limpiarEntorno(prueba *testing.T) {
	prueba.Helper()
	for _, clave := range clavesEntorno {
		prueba.Setenv(clave, "")
	}
}

// ===== Pruebas de Carga y Validación =====

// TestFromEnvUsaLosValoresPorDefecto comprueba que FromEnv aplica Default cuando no hay variables definidas.
func TestFromEnvUsaLosValoresPorDefecto(prueba *testing.T) {
	limpiarEntorno(prueba)

	configuracion, errorObtenido := config.FromEnv()
	if errorObtenido != nil {
		prueba.Fatalf("FromEnv() error = %v", errorObtenido)
	}
	predeterminada := config.Default()
	if configuracion.Lease != predeterminada.Lease {
		prueba.Errorf("Lease = %v, esperado %v", configuracion.Lease, predeterminada.Lease)
	}
	if configuracion.BatchSize != predeterminada.BatchSize {
		prueba.Errorf("BatchSize = %d, esperado %d", configuracion.BatchSize, predeterminada.BatchSize)
	}
	if configuracion.QueueName != predeterminada.QueueName || configuracion.Group != predeterminada.Group {
		prueba.Errorf("cola/grupo = %q/%q", configuracion.QueueName, configuracion.Group)
	}
	if configuracion.CategoryDir != predeterminada.CategoryDir {
		prueba.Errorf("CategoryDir = %q, esperado %q", configuracion.CategoryDir, predeterminada.CategoryDir)
	}
	if configuracion.LogLevel != slog.LevelInfo {
		prueba.Errorf("LogLevel = %v", configuracion.LogLevel)
	}
	if configuracion.MaxPages != config.DefaultMaxPages {
		prueba.Errorf("MaxPages = %d, esperado %d", configuracion.MaxPages, config.DefaultMaxPages)
	}
}

// TestFromEnvLeeValoresConfigurados comprueba la lectura de variables personalizadas, incluyendo catálogo y categorías.
func TestFromEnvLeeValoresConfigurados(prueba *testing.T) {
	limpiarEntorno(prueba)
	prueba.Setenv("SCRAPER_LEASE_MS", "90000")
	prueba.Setenv("SCRAPER_API_BATCH_SIZE", "25")
	prueba.Setenv("SCRAPER_MAX_PAGES", "7")
	prueba.Setenv("LOG_LEVEL", "debug")
	prueba.Setenv("SCRAPER_CATEGORY_DIR", "/app/research")
	prueba.Setenv("SCRAPER_SPIDERS_FILE", "/tmp/spiders.json")
	prueba.Setenv("JUMBO_CATEGORY_URLS", "https://www.jumbo.cl/frutas")

	configuracion, errorObtenido := config.FromEnv()
	if errorObtenido != nil {
		prueba.Fatalf("FromEnv() error = %v", errorObtenido)
	}
	if configuracion.Lease != 90*time.Second {
		prueba.Errorf("Lease = %v", configuracion.Lease)
	}
	if configuracion.BatchSize != 25 {
		prueba.Errorf("BatchSize = %d", configuracion.BatchSize)
	}
	if configuracion.MaxPages != 7 {
		prueba.Errorf("MaxPages = %d", configuracion.MaxPages)
	}
	if configuracion.LogLevel != slog.LevelDebug {
		prueba.Errorf("LogLevel = %v", configuracion.LogLevel)
	}
	if configuracion.CategoryDir != "/app/research" {
		prueba.Errorf("CategoryDir = %q", configuracion.CategoryDir)
	}
	if configuracion.SpidersFile != "/tmp/spiders.json" {
		prueba.Errorf("SpidersFile = %q", configuracion.SpidersFile)
	}
	if urls := configuracion.CategoryURLs["JUMBO_CATEGORY_URLS"]; len(urls) != 1 || urls[0] != "https://www.jumbo.cl/frutas" {
		prueba.Errorf("CategoryURLs = %v", configuracion.CategoryURLs)
	}
}

// TestFromEnvRechazaEnterosMalFormados verifica que un valor no numérico en variables enteras aborte la carga.
func TestFromEnvRechazaEnterosMalFormados(prueba *testing.T) {
	casos := []string{
		"SCRAPER_LEASE_MS",
		"SCRAPER_STATE_MIN_FREE_BYTES",
		"SCRAPER_API_BATCH_SIZE",
		"SCRAPER_MAX_PAGES",
		"SCRAPER_BACKOFF_MIN_MS",
	}
	for _, variable := range casos {
		prueba.Run(variable, func(prueba *testing.T) {
			limpiarEntorno(prueba)
			prueba.Setenv(variable, "no_es_numero")
			_, errorObtenido := config.FromEnv()
			if errorObtenido == nil || !strings.Contains(errorObtenido.Error(), variable) {
				prueba.Fatalf("FromEnv() = %v; se esperaba error mencionando %s", errorObtenido, variable)
			}
		})
	}
}

// TestValidateRechazaValoresFueraDeRango verifica que Validate rechace límites inválidos.
func TestValidateRechazaValoresFueraDeRango(prueba *testing.T) {
	casos := map[string]func(*config.Config){
		"lease corta":         func(configuracion *config.Config) { configuracion.Lease = time.Second },
		"batch cero":          func(configuracion *config.Config) { configuracion.BatchSize = 0 },
		"batch demasiado":     func(configuracion *config.Config) { configuracion.BatchSize = 1001 },
		"max pages cero":      func(configuracion *config.Config) { configuracion.MaxPages = 0 },
		"timeout api":         func(configuracion *config.Config) { configuracion.APITimeout = 0 },
		"intentos crawl":      func(configuracion *config.Config) { configuracion.CrawlAttempts = 0 },
		"backoff minimo cero": func(configuracion *config.Config) { configuracion.BackoffMin = 0 },
		"capacidad negativa":  func(configuracion *config.Config) { configuracion.MinFreeBytes = 0 },
		"backoff maximo cero": func(configuracion *config.Config) { configuracion.BackoffMax = 0 },
	}
	for nombre, mutar := range casos {
		prueba.Run(nombre, func(prueba *testing.T) {
			configuracion := config.Default()
			mutar(&configuracion)
			if errorObtenido := configuracion.Validate(); errorObtenido == nil {
				prueba.Fatal("Validate() debió rechazar la configuración")
			}
		})
	}
}

// TestDefaultEsValido comprueba que la configuración predeterminada supera Validate.
func TestDefaultEsValido(prueba *testing.T) {
	if errorObtenido := config.Default().Validate(); errorObtenido != nil {
		prueba.Fatalf("Default() no es válido: %v", errorObtenido)
	}
}

// TestFromEnvRechazaLimitesInvalidos verifica que FromEnv ejecute Validate sobre los valores leídos.
func TestFromEnvRechazaLimitesInvalidos(prueba *testing.T) {
	limpiarEntorno(prueba)
	prueba.Setenv("SCRAPER_API_BATCH_SIZE", "0")
	if _, errorObtenido := config.FromEnv(); errorObtenido == nil {
		prueba.Fatal("FromEnv() debió rechazar batch size 0")
	}
}

// TestFromEnvRechazaBackoffMinimoCero asegura que el crawler no aplique un respaldo silencioso.
func TestFromEnvRechazaBackoffMinimoCero(prueba *testing.T) {
	limpiarEntorno(prueba)
	prueba.Setenv("SCRAPER_BACKOFF_MIN_MS", "0")
	if _, errorObtenido := config.FromEnv(); errorObtenido == nil {
		prueba.Fatal("FromEnv() debió rechazar SCRAPER_BACKOFF_MIN_MS=0")
	}
}
