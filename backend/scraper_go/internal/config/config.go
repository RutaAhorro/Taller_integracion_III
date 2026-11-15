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

// ===== Estructura de Configuración =====

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

// ===== Valores por Defecto =====

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

// ===== Carga desde Entorno =====

// FromEnv construye y valida la configuración del scraper a partir de variables de entorno.
func FromEnv() (Config, error) {
	configuracion := Default()
	configuracion.RedisURL = env("REDIS_URL", configuracion.RedisURL)
	configuracion.APIURL = env("SCRAPER_API_URL", configuracion.APIURL)
	configuracion.Token = os.Getenv("SCRAPING_SERVICE_TOKEN")
	configuracion.StateDir = env("SCRAPER_STATE_DIR", configuracion.StateDir)
	configuracion.Lease = time.Duration(envInt("SCRAPER_LEASE_MS", int(configuracion.Lease/time.Millisecond))) * time.Millisecond
	configuracion.MinFreeBytes = int64(envInt("SCRAPER_STATE_MIN_FREE_BYTES", int(configuracion.MinFreeBytes)))
	configuracion.FailedRetention = time.Duration(envInt("SCRAPER_FAILED_RETENTION_SECONDS", int(configuracion.FailedRetention/time.Second))) * time.Second
	configuracion.APITimeout = time.Duration(envInt("SCRAPER_API_TIMEOUT", int(configuracion.APITimeout/time.Second))) * time.Second
	configuracion.BatchSize = envInt("SCRAPER_API_BATCH_SIZE", configuracion.BatchSize)
	configuracion.QueueName = env("SCRAPER_QUEUE_NAME", configuracion.QueueName)
	configuracion.Group = env("SCRAPER_GROUP", configuracion.Group)
	configuracion.CategoryDir = env("SCRAPER_CATEGORY_DIR", configuracion.CategoryDir)
	configuracion.LogLevel = parseLevel(env("LOG_LEVEL", configuracion.LogLevel.String()))
	configuracion.UserAgent = env("SCRAPER_USER_AGENT", configuracion.UserAgent)
	configuracion.DownloadDelay = time.Duration(envInt("SCRAPER_DOWNLOAD_DELAY_MS", int(configuracion.DownloadDelay/time.Millisecond))) * time.Millisecond
	configuracion.DownloadTimeout = time.Duration(envInt("SCRAPER_DOWNLOAD_TIMEOUT", int(configuracion.DownloadTimeout/time.Second))) * time.Second
	configuracion.MaxDownloadSize = int64(envInt("SCRAPER_DOWNLOAD_MAXSIZE", int(configuracion.MaxDownloadSize)))
	configuracion.MaxRetries = envInt("SCRAPER_RETRY_TIMES", configuracion.MaxRetries)
	configuracion.MaxPages = envInt("SCRAPER_MAX_PAGES", configuracion.MaxPages)

	configuracion.RedisDialTimeout = time.Duration(envInt("SCRAPER_REDIS_TIMEOUT_SECONDS", int(configuracion.RedisDialTimeout/time.Second))) * time.Second
	configuracion.RedisPingTimeout = time.Duration(envInt("SCRAPER_REDIS_PING_TIMEOUT_SECONDS", int(configuracion.RedisPingTimeout/time.Second))) * time.Second
	configuracion.QueueBlock = time.Duration(envInt("SCRAPER_QUEUE_BLOCK_MS", int(configuracion.QueueBlock/time.Millisecond))) * time.Millisecond
	configuracion.QueueClosedTTL = time.Duration(envInt("SCRAPER_JOB_CLOSED_TTL_SECONDS", int(configuracion.QueueClosedTTL/time.Second))) * time.Second
	configuracion.PollInterval = time.Duration(envInt("SCRAPER_POLL_INTERVAL_MS", int(configuracion.PollInterval/time.Millisecond))) * time.Millisecond
	configuracion.RetryInterval = time.Duration(envInt("SCRAPER_RETRY_INTERVAL_SECONDS", int(configuracion.RetryInterval/time.Second))) * time.Second
	configuracion.CollectInterval = time.Duration(envInt("SCRAPER_COLLECT_INTERVAL_SECONDS", int(configuracion.CollectInterval/time.Second))) * time.Second
	configuracion.CollectLimit = envInt("SCRAPER_COLLECT_LIMIT", configuracion.CollectLimit)
	configuracion.CollectTimeout = time.Duration(envInt("SCRAPER_COLLECT_TIMEOUT_SECONDS", int(configuracion.CollectTimeout/time.Second))) * time.Second
	configuracion.GCTimeout = time.Duration(envInt("SCRAPER_GC_TIMEOUT_SECONDS", int(configuracion.GCTimeout/time.Second))) * time.Second
	configuracion.CrawlAttempts = envInt("SCRAPER_CRAWL_ATTEMPTS", configuracion.CrawlAttempts)
	configuracion.RobotsCacheTTL = time.Duration(envInt("SCRAPER_ROBOTS_CACHE_TTL_SECONDS", int(configuracion.RobotsCacheTTL/time.Second))) * time.Second
	configuracion.RobotsFailureTTL = time.Duration(envInt("SCRAPER_ROBOTS_FAILURE_TTL_SECONDS", int(configuracion.RobotsFailureTTL/time.Second))) * time.Second
	configuracion.RobotsMaxBytes = int64(envInt("SCRAPER_ROBOTS_MAX_BYTES", int(configuracion.RobotsMaxBytes)))
	configuracion.BackoffMin = time.Duration(envInt("SCRAPER_BACKOFF_MIN_MS", int(configuracion.BackoffMin/time.Millisecond))) * time.Millisecond
	configuracion.BackoffMax = time.Duration(envInt("SCRAPER_BACKOFF_MAX_SECONDS", int(configuracion.BackoffMax/time.Second))) * time.Second

	if errorObtenido := configuracion.Validate(); errorObtenido != nil {
		return Config{}, errorObtenido
	}
	return configuracion, nil
}

// ===== Validación Operativa =====

// Validate comprueba los rangos admitidos por el worker.
func (configuracion Config) Validate() error {
	switch {
	case configuracion.Lease < 10*time.Second:
		return fmt.Errorf("SCRAPER_LEASE_MS debe ser al menos 10000")
	case configuracion.MinFreeBytes < 1024*1024:
		return fmt.Errorf("SCRAPER_STATE_MIN_FREE_BYTES debe ser al menos 1 MiB")
	case configuracion.BatchSize < 1 || configuracion.BatchSize > 1000:
		return fmt.Errorf("SCRAPER_API_BATCH_SIZE debe estar entre 1 y 1000")
	case configuracion.APITimeout <= 0:
		return fmt.Errorf("SCRAPER_API_TIMEOUT debe ser mayor que cero")
	case configuracion.DownloadTimeout <= 0:
		return fmt.Errorf("SCRAPER_DOWNLOAD_TIMEOUT debe ser mayor que cero")
	case configuracion.MaxDownloadSize <= 0:
		return fmt.Errorf("SCRAPER_DOWNLOAD_MAXSIZE debe ser mayor que cero")
	case configuracion.DownloadDelay < 0:
		return fmt.Errorf("SCRAPER_DOWNLOAD_DELAY_MS no puede ser negativo")
	case configuracion.MaxRetries < 0:
		return fmt.Errorf("SCRAPER_RETRY_TIMES no puede ser negativo")
	case configuracion.FailedRetention < 0:
		return fmt.Errorf("SCRAPER_FAILED_RETENTION_SECONDS no puede ser negativo")
	case configuracion.MaxPages < 1:
		return fmt.Errorf("SCRAPER_MAX_PAGES debe ser mayor que cero")
	case configuracion.RedisDialTimeout <= 0:
		return fmt.Errorf("SCRAPER_REDIS_TIMEOUT_SECONDS debe ser mayor que cero")
	case configuracion.RedisPingTimeout <= 0:
		return fmt.Errorf("SCRAPER_REDIS_PING_TIMEOUT_SECONDS debe ser mayor que cero")
	case configuracion.QueueBlock < 0:
		return fmt.Errorf("SCRAPER_QUEUE_BLOCK_MS no puede ser negativo")
	case configuracion.QueueClosedTTL <= 0:
		return fmt.Errorf("SCRAPER_JOB_CLOSED_TTL_SECONDS debe ser mayor que cero")
	case configuracion.PollInterval <= 0:
		return fmt.Errorf("SCRAPER_POLL_INTERVAL_MS debe ser mayor que cero")
	case configuracion.RetryInterval <= 0:
		return fmt.Errorf("SCRAPER_RETRY_INTERVAL_SECONDS debe ser mayor que cero")
	case configuracion.CollectInterval <= 0:
		return fmt.Errorf("SCRAPER_COLLECT_INTERVAL_SECONDS debe ser mayor que cero")
	case configuracion.CollectLimit < 1:
		return fmt.Errorf("SCRAPER_COLLECT_LIMIT debe ser mayor que cero")
	case configuracion.CollectTimeout <= 0:
		return fmt.Errorf("SCRAPER_COLLECT_TIMEOUT_SECONDS debe ser mayor que cero")
	case configuracion.GCTimeout <= 0:
		return fmt.Errorf("SCRAPER_GC_TIMEOUT_SECONDS debe ser mayor que cero")
	case configuracion.CrawlAttempts < 1:
		return fmt.Errorf("SCRAPER_CRAWL_ATTEMPTS debe ser mayor que cero")
	case configuracion.RobotsCacheTTL <= 0:
		return fmt.Errorf("SCRAPER_ROBOTS_CACHE_TTL_SECONDS debe ser mayor que cero")
	case configuracion.RobotsFailureTTL <= 0:
		return fmt.Errorf("SCRAPER_ROBOTS_FAILURE_TTL_SECONDS debe ser mayor que cero")
	case configuracion.RobotsMaxBytes <= 0:
		return fmt.Errorf("SCRAPER_ROBOTS_MAX_BYTES debe ser mayor que cero")
	case configuracion.BackoffMin < 0:
		return fmt.Errorf("SCRAPER_BACKOFF_MIN_MS no puede ser negativo")
	case configuracion.BackoffMax <= 0:
		return fmt.Errorf("SCRAPER_BACKOFF_MAX_SECONDS debe ser mayor que cero")
	}
	return nil
}

// ===== Lectura de Entorno =====

// env obtiene el valor de una variable de entorno o devuelve el respaldo por defecto.
func env(clave, respaldo string) string {
	if valor := strings.TrimSpace(os.Getenv(clave)); valor != "" {
		return valor
	}
	return respaldo
}

// envInt lee una variable de entorno numérica y valida su conversión.
func envInt(clave string, respaldo int) int {
	crudo := strings.TrimSpace(os.Getenv(clave))
	if crudo == "" {
		return respaldo
	}
	valor, errorObtenido := strconv.Atoi(crudo)
	if errorObtenido != nil {
		return respaldo
	}
	return valor
}

// parseLevel mapea la cadena de texto de nivel de log a slog.Level.
func parseLevel(crudo string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(crudo)) {
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
