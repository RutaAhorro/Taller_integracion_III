// Package config concentra y valida las variables de entorno del worker.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config concentra las variables de entorno del worker. Los valores por
// defecto replican scraper_core/settings.py y worker.py.
type Config struct {
	RedisURL        string
	APIURL          string
	Token           string
	StateDir        string
	Lease           time.Duration
	MinFreeBytes    int64
	FailedRetention time.Duration
	APITimeout      time.Duration
	BatchSize       int
	QueueName       string
	Group           string
	CategoryDir     string
	LogLevel        slog.Level
	UserAgent       string
	DownloadDelay   time.Duration
	DownloadTimeout time.Duration
	MaxDownloadSize int64
	MaxRetries      int
}

// FromEnv construye la configuración y valida los límites que también valida
// el worker de Python.
func FromEnv() (Config, error) {
	cfg := Config{
		RedisURL:        env("REDIS_URL", "redis://localhost:6379/0"),
		APIURL:          env("SCRAPER_API_URL", "http://localhost:8082/api/v1/scraper/productos"),
		Token:           os.Getenv("SCRAPING_SERVICE_TOKEN"),
		StateDir:        env("SCRAPER_STATE_DIR", "/var/lib/scraper"),
		Lease:           time.Duration(envInt("SCRAPER_LEASE_MS", 60000)) * time.Millisecond,
		MinFreeBytes:    int64(envInt("SCRAPER_STATE_MIN_FREE_BYTES", 64*1024*1024)),
		FailedRetention: time.Duration(envInt("SCRAPER_FAILED_RETENTION_SECONDS", 86400)) * time.Second,
		APITimeout:      time.Duration(envInt("SCRAPER_API_TIMEOUT", 30)) * time.Second,
		BatchSize:       envInt("SCRAPER_API_BATCH_SIZE", 100),
		QueueName:       env("SCRAPER_QUEUE_NAME", "scraper:jobs:stream"),
		Group:           env("SCRAPER_GROUP", "scraper-workers"),
		CategoryDir:     env("SCRAPER_CATEGORY_DIR", "/app/research"),
		LogLevel:        parseLevel(env("LOG_LEVEL", "INFO")),
		UserAgent:       env("SCRAPER_USER_AGENT", "TallerIntegracionIII/1.0 (contacto del proyecto)"),
		DownloadDelay:   time.Duration(envInt("SCRAPER_DOWNLOAD_DELAY_MS", 2000)) * time.Millisecond,
		DownloadTimeout: time.Duration(envInt("SCRAPER_DOWNLOAD_TIMEOUT", 30)) * time.Second,
		MaxDownloadSize: int64(envInt("SCRAPER_DOWNLOAD_MAXSIZE", 5*1024*1024)),
		MaxRetries:      envInt("SCRAPER_RETRY_TIMES", 3),
	}

	switch {
	case cfg.Lease < 10*time.Second:
		return Config{}, fmt.Errorf("SCRAPER_LEASE_MS debe ser al menos 10000")
	case cfg.MinFreeBytes < 1024*1024:
		return Config{}, fmt.Errorf("SCRAPER_STATE_MIN_FREE_BYTES debe ser al menos 1 MiB")
	case cfg.BatchSize < 1 || cfg.BatchSize > 1000:
		return Config{}, fmt.Errorf("SCRAPER_API_BATCH_SIZE debe estar entre 1 y 1000")
	case cfg.APITimeout <= 0:
		return Config{}, fmt.Errorf("SCRAPER_API_TIMEOUT debe ser mayor que cero")
	case cfg.DownloadTimeout <= 0:
		return Config{}, fmt.Errorf("SCRAPER_DOWNLOAD_TIMEOUT debe ser mayor que cero")
	case cfg.MaxDownloadSize <= 0:
		return Config{}, fmt.Errorf("SCRAPER_DOWNLOAD_MAXSIZE debe ser mayor que cero")
	case cfg.DownloadDelay < 0:
		return Config{}, fmt.Errorf("SCRAPER_DOWNLOAD_DELAY_MS no puede ser negativo")
	case cfg.MaxRetries < 0:
		return Config{}, fmt.Errorf("SCRAPER_RETRY_TIMES no puede ser negativo")
	case cfg.FailedRetention < 0:
		return Config{}, fmt.Errorf("SCRAPER_FAILED_RETENTION_SECONDS no puede ser negativo")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func parseLevel(raw string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARNING", "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
