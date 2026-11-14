package config_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
)

var envKeys = []string{
	"REDIS_URL", "SCRAPER_API_URL", "SCRAPING_SERVICE_TOKEN", "SCRAPER_STATE_DIR",
	"SCRAPER_LEASE_MS", "SCRAPER_STATE_MIN_FREE_BYTES", "SCRAPER_FAILED_RETENTION_SECONDS",
	"SCRAPER_API_TIMEOUT", "SCRAPER_API_BATCH_SIZE", "SCRAPER_QUEUE_NAME", "SCRAPER_GROUP",
	"SCRAPER_CATEGORY_DIR", "LOG_LEVEL", "SCRAPER_USER_AGENT", "SCRAPER_DOWNLOAD_DELAY_MS",
	"SCRAPER_DOWNLOAD_TIMEOUT", "SCRAPER_DOWNLOAD_MAXSIZE", "SCRAPER_RETRY_TIMES",
	"SCRAPER_MAX_PAGES", "SCRAPER_REDIS_TIMEOUT_SECONDS", "SCRAPER_REDIS_PING_TIMEOUT_SECONDS",
	"SCRAPER_QUEUE_BLOCK_MS", "SCRAPER_JOB_CLOSED_TTL_SECONDS", "SCRAPER_POLL_INTERVAL_MS",
	"SCRAPER_RETRY_INTERVAL_SECONDS", "SCRAPER_COLLECT_INTERVAL_SECONDS", "SCRAPER_COLLECT_LIMIT",
	"SCRAPER_COLLECT_TIMEOUT_SECONDS", "SCRAPER_GC_TIMEOUT_SECONDS", "SCRAPER_CRAWL_ATTEMPTS",
	"SCRAPER_ROBOTS_CACHE_TTL_SECONDS", "SCRAPER_ROBOTS_FAILURE_TTL_SECONDS",
	"SCRAPER_ROBOTS_MAX_BYTES", "SCRAPER_BACKOFF_MIN_MS", "SCRAPER_BACKOFF_MAX_SECONDS",
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range envKeys {
		t.Setenv(key, "")
	}
}

func TestFromEnvUsaLosValoresPorDefecto(t *testing.T) {
	clearEnv(t)

	cfg, err := config.FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	defaults := config.Default()
	if cfg.Lease != defaults.Lease {
		t.Errorf("Lease = %v, want %v", cfg.Lease, defaults.Lease)
	}
	if cfg.BatchSize != defaults.BatchSize {
		t.Errorf("BatchSize = %d, want %d", cfg.BatchSize, defaults.BatchSize)
	}
	if cfg.QueueName != defaults.QueueName || cfg.Group != defaults.Group {
		t.Errorf("cola/grupo = %q/%q", cfg.QueueName, cfg.Group)
	}
	if cfg.CategoryDir != defaults.CategoryDir {
		t.Errorf("CategoryDir = %q, want %q", cfg.CategoryDir, defaults.CategoryDir)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v", cfg.LogLevel)
	}
	if cfg.MaxPages != config.DefaultMaxPages {
		t.Errorf("MaxPages = %d, want %d", cfg.MaxPages, config.DefaultMaxPages)
	}
}

func TestFromEnvLeeValoresConfigurados(t *testing.T) {
	clearEnv(t)
	t.Setenv("SCRAPER_LEASE_MS", "90000")
	t.Setenv("SCRAPER_API_BATCH_SIZE", "25")
	t.Setenv("SCRAPER_MAX_PAGES", "7")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("SCRAPER_CATEGORY_DIR", "/app/research")

	cfg, err := config.FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if cfg.Lease != 90*time.Second {
		t.Errorf("Lease = %v", cfg.Lease)
	}
	if cfg.BatchSize != 25 {
		t.Errorf("BatchSize = %d", cfg.BatchSize)
	}
	if cfg.MaxPages != 7 {
		t.Errorf("MaxPages = %d", cfg.MaxPages)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v", cfg.LogLevel)
	}
	if cfg.CategoryDir != "/app/research" {
		t.Errorf("CategoryDir = %q", cfg.CategoryDir)
	}
}

func TestValidateRechazaValoresFueraDeRango(t *testing.T) {
	cases := map[string]func(*config.Config){
		"lease corta":         func(cfg *config.Config) { cfg.Lease = time.Second },
		"batch cero":          func(cfg *config.Config) { cfg.BatchSize = 0 },
		"batch demasiado":     func(cfg *config.Config) { cfg.BatchSize = 1001 },
		"max pages cero":      func(cfg *config.Config) { cfg.MaxPages = 0 },
		"timeout api":         func(cfg *config.Config) { cfg.APITimeout = 0 },
		"intentos crawl":      func(cfg *config.Config) { cfg.CrawlAttempts = 0 },
		"capacidad negativa":  func(cfg *config.Config) { cfg.MinFreeBytes = 0 },
		"backoff maximo cero": func(cfg *config.Config) { cfg.BackoffMax = 0 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := config.Default()
			mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() debió rechazar la configuración")
			}
		})
	}
}

func TestDefaultEsValido(t *testing.T) {
	if err := config.Default().Validate(); err != nil {
		t.Fatalf("Default() no es válido: %v", err)
	}
}

func TestFromEnvRechazaLimitesInvalidos(t *testing.T) {
	clearEnv(t)
	t.Setenv("SCRAPER_API_BATCH_SIZE", "0")
	if _, err := config.FromEnv(); err == nil {
		t.Fatal("FromEnv() debió rechazar batch size 0")
	}
}
