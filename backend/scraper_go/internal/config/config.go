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

// DefaultMaxPages es el límite de páginas por categoría cuando no se configura
// SCRAPER_MAX_PAGES.
const DefaultMaxPages = 100

// Config concentra las variables de entorno del worker.
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
	MaxPages        int

	RedisDialTimeout time.Duration
	RedisPingTimeout time.Duration
	QueueBlock       time.Duration
	QueueClosedTTL   time.Duration
	PollInterval     time.Duration
	RetryInterval    time.Duration
	CollectInterval  time.Duration
	CollectLimit     int
	CollectTimeout   time.Duration
	GCTimeout        time.Duration
	CrawlAttempts    int
	RobotsCacheTTL   time.Duration
	RobotsFailureTTL time.Duration
	RobotsMaxBytes   int64
	BackoffMin       time.Duration
	BackoffMax       time.Duration
}

// Default devuelve los valores por defecto documentados en .env.example.
func Default() Config {
	return Config{
		RedisURL:        "redis://localhost:6379/0",
		APIURL:          "http://localhost:8082/api/v1/scraper/productos",
		StateDir:        "/var/lib/scraper",
		Lease:           60 * time.Second,
		MinFreeBytes:    64 * 1024 * 1024,
		FailedRetention: 24 * time.Hour,
		APITimeout:      30 * time.Second,
		BatchSize:       100,
		QueueName:       "scraper:jobs:stream",
		Group:           "scraper-workers",
		CategoryDir:     "/app/research",
		LogLevel:        slog.LevelInfo,
		UserAgent:       "TallerIntegracionIII/1.0 (contacto del proyecto)",
		DownloadDelay:   2 * time.Second,
		DownloadTimeout: 30 * time.Second,
		MaxDownloadSize: 5 * 1024 * 1024,
		MaxRetries:      3,
		MaxPages:        DefaultMaxPages,

		RedisDialTimeout: 5 * time.Second,
		RedisPingTimeout: 5 * time.Second,
		QueueBlock:       time.Second,
		QueueClosedTTL:   24 * time.Hour,
		PollInterval:     200 * time.Millisecond,
		RetryInterval:    5 * time.Second,
		CollectInterval:  10 * time.Second,
		CollectLimit:     20,
		CollectTimeout:   5 * time.Second,
		GCTimeout:        2 * time.Second,
		CrawlAttempts:    3,
		RobotsCacheTTL:   time.Hour,
		RobotsFailureTTL: 10 * time.Minute,
		RobotsMaxBytes:   512 * 1024,
		BackoffMin:       10 * time.Millisecond,
		BackoffMax:       120 * time.Second,
	}
}

// FromEnv construye y valida la configuración del scraper a partir de variables de entorno.
func FromEnv() (Config, error) {
	cfg := Default()
	cfg.RedisURL = env("REDIS_URL", cfg.RedisURL)
	cfg.APIURL = env("SCRAPER_API_URL", cfg.APIURL)
	cfg.Token = os.Getenv("SCRAPING_SERVICE_TOKEN")
	cfg.StateDir = env("SCRAPER_STATE_DIR", cfg.StateDir)
	cfg.Lease = time.Duration(envInt("SCRAPER_LEASE_MS", int(cfg.Lease/time.Millisecond))) * time.Millisecond
	cfg.MinFreeBytes = int64(envInt("SCRAPER_STATE_MIN_FREE_BYTES", int(cfg.MinFreeBytes)))
	cfg.FailedRetention = time.Duration(envInt("SCRAPER_FAILED_RETENTION_SECONDS", int(cfg.FailedRetention/time.Second))) * time.Second
	cfg.APITimeout = time.Duration(envInt("SCRAPER_API_TIMEOUT", int(cfg.APITimeout/time.Second))) * time.Second
	cfg.BatchSize = envInt("SCRAPER_API_BATCH_SIZE", cfg.BatchSize)
	cfg.QueueName = env("SCRAPER_QUEUE_NAME", cfg.QueueName)
	cfg.Group = env("SCRAPER_GROUP", cfg.Group)
	cfg.CategoryDir = env("SCRAPER_CATEGORY_DIR", cfg.CategoryDir)
	cfg.LogLevel = parseLevel(env("LOG_LEVEL", cfg.LogLevel.String()))
	cfg.UserAgent = env("SCRAPER_USER_AGENT", cfg.UserAgent)
	cfg.DownloadDelay = time.Duration(envInt("SCRAPER_DOWNLOAD_DELAY_MS", int(cfg.DownloadDelay/time.Millisecond))) * time.Millisecond
	cfg.DownloadTimeout = time.Duration(envInt("SCRAPER_DOWNLOAD_TIMEOUT", int(cfg.DownloadTimeout/time.Second))) * time.Second
	cfg.MaxDownloadSize = int64(envInt("SCRAPER_DOWNLOAD_MAXSIZE", int(cfg.MaxDownloadSize)))
	cfg.MaxRetries = envInt("SCRAPER_RETRY_TIMES", cfg.MaxRetries)
	cfg.MaxPages = envInt("SCRAPER_MAX_PAGES", cfg.MaxPages)

	cfg.RedisDialTimeout = time.Duration(envInt("SCRAPER_REDIS_TIMEOUT_SECONDS", int(cfg.RedisDialTimeout/time.Second))) * time.Second
	cfg.RedisPingTimeout = time.Duration(envInt("SCRAPER_REDIS_PING_TIMEOUT_SECONDS", int(cfg.RedisPingTimeout/time.Second))) * time.Second
	cfg.QueueBlock = time.Duration(envInt("SCRAPER_QUEUE_BLOCK_MS", int(cfg.QueueBlock/time.Millisecond))) * time.Millisecond
	cfg.QueueClosedTTL = time.Duration(envInt("SCRAPER_JOB_CLOSED_TTL_SECONDS", int(cfg.QueueClosedTTL/time.Second))) * time.Second
	cfg.PollInterval = time.Duration(envInt("SCRAPER_POLL_INTERVAL_MS", int(cfg.PollInterval/time.Millisecond))) * time.Millisecond
	cfg.RetryInterval = time.Duration(envInt("SCRAPER_RETRY_INTERVAL_SECONDS", int(cfg.RetryInterval/time.Second))) * time.Second
	cfg.CollectInterval = time.Duration(envInt("SCRAPER_COLLECT_INTERVAL_SECONDS", int(cfg.CollectInterval/time.Second))) * time.Second
	cfg.CollectLimit = envInt("SCRAPER_COLLECT_LIMIT", cfg.CollectLimit)
	cfg.CollectTimeout = time.Duration(envInt("SCRAPER_COLLECT_TIMEOUT_SECONDS", int(cfg.CollectTimeout/time.Second))) * time.Second
	cfg.GCTimeout = time.Duration(envInt("SCRAPER_GC_TIMEOUT_SECONDS", int(cfg.GCTimeout/time.Second))) * time.Second
	cfg.CrawlAttempts = envInt("SCRAPER_CRAWL_ATTEMPTS", cfg.CrawlAttempts)
	cfg.RobotsCacheTTL = time.Duration(envInt("SCRAPER_ROBOTS_CACHE_TTL_SECONDS", int(cfg.RobotsCacheTTL/time.Second))) * time.Second
	cfg.RobotsFailureTTL = time.Duration(envInt("SCRAPER_ROBOTS_FAILURE_TTL_SECONDS", int(cfg.RobotsFailureTTL/time.Second))) * time.Second
	cfg.RobotsMaxBytes = int64(envInt("SCRAPER_ROBOTS_MAX_BYTES", int(cfg.RobotsMaxBytes)))
	cfg.BackoffMin = time.Duration(envInt("SCRAPER_BACKOFF_MIN_MS", int(cfg.BackoffMin/time.Millisecond))) * time.Millisecond
	cfg.BackoffMax = time.Duration(envInt("SCRAPER_BACKOFF_MAX_SECONDS", int(cfg.BackoffMax/time.Second))) * time.Second

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate comprueba los rangos admitidos por el worker.
func (cfg Config) Validate() error {
	switch {
	case cfg.Lease < 10*time.Second:
		return fmt.Errorf("SCRAPER_LEASE_MS debe ser al menos 10000")
	case cfg.MinFreeBytes < 1024*1024:
		return fmt.Errorf("SCRAPER_STATE_MIN_FREE_BYTES debe ser al menos 1 MiB")
	case cfg.BatchSize < 1 || cfg.BatchSize > 1000:
		return fmt.Errorf("SCRAPER_API_BATCH_SIZE debe estar entre 1 y 1000")
	case cfg.APITimeout <= 0:
		return fmt.Errorf("SCRAPER_API_TIMEOUT debe ser mayor que cero")
	case cfg.DownloadTimeout <= 0:
		return fmt.Errorf("SCRAPER_DOWNLOAD_TIMEOUT debe ser mayor que cero")
	case cfg.MaxDownloadSize <= 0:
		return fmt.Errorf("SCRAPER_DOWNLOAD_MAXSIZE debe ser mayor que cero")
	case cfg.DownloadDelay < 0:
		return fmt.Errorf("SCRAPER_DOWNLOAD_DELAY_MS no puede ser negativo")
	case cfg.MaxRetries < 0:
		return fmt.Errorf("SCRAPER_RETRY_TIMES no puede ser negativo")
	case cfg.FailedRetention < 0:
		return fmt.Errorf("SCRAPER_FAILED_RETENTION_SECONDS no puede ser negativo")
	case cfg.MaxPages < 1:
		return fmt.Errorf("SCRAPER_MAX_PAGES debe ser mayor que cero")
	case cfg.RedisDialTimeout <= 0:
		return fmt.Errorf("SCRAPER_REDIS_TIMEOUT_SECONDS debe ser mayor que cero")
	case cfg.RedisPingTimeout <= 0:
		return fmt.Errorf("SCRAPER_REDIS_PING_TIMEOUT_SECONDS debe ser mayor que cero")
	case cfg.QueueBlock < 0:
		return fmt.Errorf("SCRAPER_QUEUE_BLOCK_MS no puede ser negativo")
	case cfg.QueueClosedTTL <= 0:
		return fmt.Errorf("SCRAPER_JOB_CLOSED_TTL_SECONDS debe ser mayor que cero")
	case cfg.PollInterval <= 0:
		return fmt.Errorf("SCRAPER_POLL_INTERVAL_MS debe ser mayor que cero")
	case cfg.RetryInterval <= 0:
		return fmt.Errorf("SCRAPER_RETRY_INTERVAL_SECONDS debe ser mayor que cero")
	case cfg.CollectInterval <= 0:
		return fmt.Errorf("SCRAPER_COLLECT_INTERVAL_SECONDS debe ser mayor que cero")
	case cfg.CollectLimit < 1:
		return fmt.Errorf("SCRAPER_COLLECT_LIMIT debe ser mayor que cero")
	case cfg.CollectTimeout <= 0:
		return fmt.Errorf("SCRAPER_COLLECT_TIMEOUT_SECONDS debe ser mayor que cero")
	case cfg.GCTimeout <= 0:
		return fmt.Errorf("SCRAPER_GC_TIMEOUT_SECONDS debe ser mayor que cero")
	case cfg.CrawlAttempts < 1:
		return fmt.Errorf("SCRAPER_CRAWL_ATTEMPTS debe ser mayor que cero")
	case cfg.RobotsCacheTTL <= 0:
		return fmt.Errorf("SCRAPER_ROBOTS_CACHE_TTL_SECONDS debe ser mayor que cero")
	case cfg.RobotsFailureTTL <= 0:
		return fmt.Errorf("SCRAPER_ROBOTS_FAILURE_TTL_SECONDS debe ser mayor que cero")
	case cfg.RobotsMaxBytes <= 0:
		return fmt.Errorf("SCRAPER_ROBOTS_MAX_BYTES debe ser mayor que cero")
	case cfg.BackoffMin < 0:
		return fmt.Errorf("SCRAPER_BACKOFF_MIN_MS no puede ser negativo")
	case cfg.BackoffMax <= 0:
		return fmt.Errorf("SCRAPER_BACKOFF_MAX_SECONDS debe ser mayor que cero")
	}
	return nil
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
