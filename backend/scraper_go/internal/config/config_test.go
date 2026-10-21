package config

import (
	"log/slog"
	"testing"
	"time"
)

func TestFromEnvDefaults(t *testing.T) {
	for _, key := range []string{
		"REDIS_URL", "SCRAPER_API_URL", "SCRAPING_SERVICE_TOKEN", "SCRAPER_STATE_DIR",
		"SCRAPER_LEASE_MS", "SCRAPER_STATE_MIN_FREE_BYTES", "SCRAPER_FAILED_RETENTION_SECONDS",
		"SCRAPER_API_TIMEOUT", "SCRAPER_API_BATCH_SIZE", "SCRAPER_QUEUE_NAME", "SCRAPER_GROUP",
		"SCRAPER_CATEGORY_DIR", "LOG_LEVEL", "SCRAPER_USER_AGENT", "SCRAPER_DOWNLOAD_DELAY_MS",
		"SCRAPER_DOWNLOAD_TIMEOUT", "SCRAPER_DOWNLOAD_MAXSIZE", "SCRAPER_RETRY_TIMES",
	} {
		t.Setenv(key, "")
	}

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if cfg.Lease != 60*time.Second {
		t.Errorf("Lease = %v", cfg.Lease)
	}
	if cfg.BatchSize != 100 {
		t.Errorf("BatchSize = %d", cfg.BatchSize)
	}
	if cfg.QueueName != "scraper:jobs:stream" || cfg.Group != "scraper-workers" {
		t.Errorf("cola/grupo = %q/%q", cfg.QueueName, cfg.Group)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v", cfg.LogLevel)
	}
}

func TestFromEnvRejectsInvalidLimits(t *testing.T) {
	t.Setenv("SCRAPER_API_BATCH_SIZE", "0")
	if _, err := FromEnv(); err == nil {
		t.Fatal("FromEnv() debió rechazar batch size 0")
	}
}
