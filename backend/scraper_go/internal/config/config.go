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

// ===== Valores Compartidos =====

const (
	// DefaultQueueName identifica el stream de trabajos del scraper.
	DefaultQueueName = "scraper:jobs:stream"
	// DefaultQueueGroup identifica el grupo de consumidores del scraper.
	DefaultQueueGroup = "scraper-workers"
	// MinLeaseDuration es la duración mínima aceptada para la lease de un mensaje.
	MinLeaseDuration = 10 * time.Second
	// MinFreeBytesLimit es la reserva mínima en disco aceptada para el spool.
	MinFreeBytesLimit = 1024 * 1024
	// SufijoURLsCategoria identifica las variables con categorías adicionales
	// por spider; cada sitio del catálogo declara la suya en `category_env`.
	SufijoURLsCategoria = "_CATEGORY_URLS"
	// MinBatchSize es la cantidad mínima de productos permitida por lote.
	MinBatchSize = 1
	// MaxBatchSize es la cantidad máxima de productos permitida por lote.
	MaxBatchSize = 1000
)

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
	SpidersFile     string
	CategoryURLs    map[string][]string
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
		QueueName:       DefaultQueueName,
		Group:           DefaultQueueGroup,
		CategoryDir:     "/app/research",
		CategoryURLs:    map[string][]string{},
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
	var errorCarga error
	leerEntero := func(clave string, respaldo int) int {
		crudo := strings.TrimSpace(os.Getenv(clave))
		if crudo == "" {
			return respaldo
		}
		valor, errorObtenido := strconv.Atoi(crudo)
		if errorObtenido != nil {
			if errorCarga == nil {
				errorCarga = fmt.Errorf("%s debe ser un número entero: %w", clave, errorObtenido)
			}
			return respaldo
		}
		return valor
	}
	configuracion.RedisURL = leerCadena("REDIS_URL", configuracion.RedisURL)
	configuracion.APIURL = leerCadena("SCRAPER_API_URL", configuracion.APIURL)
	configuracion.Token = os.Getenv("SCRAPING_SERVICE_TOKEN")
	configuracion.StateDir = leerCadena("SCRAPER_STATE_DIR", configuracion.StateDir)
	configuracion.Lease = time.Duration(leerEntero("SCRAPER_LEASE_MS", int(configuracion.Lease/time.Millisecond))) * time.Millisecond
	configuracion.MinFreeBytes = int64(leerEntero("SCRAPER_STATE_MIN_FREE_BYTES", int(configuracion.MinFreeBytes)))
	configuracion.FailedRetention = time.Duration(leerEntero("SCRAPER_FAILED_RETENTION_SECONDS", int(configuracion.FailedRetention/time.Second))) * time.Second
	configuracion.APITimeout = time.Duration(leerEntero("SCRAPER_API_TIMEOUT", int(configuracion.APITimeout/time.Second))) * time.Second
	configuracion.BatchSize = leerEntero("SCRAPER_API_BATCH_SIZE", configuracion.BatchSize)
	configuracion.QueueName = leerCadena("SCRAPER_QUEUE_NAME", configuracion.QueueName)
	configuracion.Group = leerCadena("SCRAPER_GROUP", configuracion.Group)
	configuracion.CategoryDir = leerCadena("SCRAPER_CATEGORY_DIR", configuracion.CategoryDir)
	configuracion.SpidersFile = leerCadena("SCRAPER_SPIDERS_FILE", "")
	configuracion.CategoryURLs = leerURLsCategoriaEntorno()
	configuracion.LogLevel = interpretarNivelLog(leerCadena("LOG_LEVEL", "INFO"))
	configuracion.UserAgent = leerCadena("SCRAPER_USER_AGENT", configuracion.UserAgent)
	configuracion.DownloadDelay = time.Duration(leerEntero("SCRAPER_DOWNLOAD_DELAY_MS", int(configuracion.DownloadDelay/time.Millisecond))) * time.Millisecond
	configuracion.DownloadTimeout = time.Duration(leerEntero("SCRAPER_DOWNLOAD_TIMEOUT", int(configuracion.DownloadTimeout/time.Second))) * time.Second
	configuracion.MaxDownloadSize = int64(leerEntero("SCRAPER_DOWNLOAD_MAXSIZE", int(configuracion.MaxDownloadSize)))
	configuracion.MaxRetries = leerEntero("SCRAPER_RETRY_TIMES", configuracion.MaxRetries)
	configuracion.MaxPages = leerEntero("SCRAPER_MAX_PAGES", configuracion.MaxPages)

	configuracion.RedisDialTimeout = time.Duration(leerEntero("SCRAPER_REDIS_TIMEOUT_SECONDS", int(configuracion.RedisDialTimeout/time.Second))) * time.Second
	configuracion.RedisPingTimeout = time.Duration(leerEntero("SCRAPER_REDIS_PING_TIMEOUT_SECONDS", int(configuracion.RedisPingTimeout/time.Second))) * time.Second
	configuracion.QueueBlock = time.Duration(leerEntero("SCRAPER_QUEUE_BLOCK_MS", int(configuracion.QueueBlock/time.Millisecond))) * time.Millisecond
	configuracion.QueueClosedTTL = time.Duration(leerEntero("SCRAPER_JOB_CLOSED_TTL_SECONDS", int(configuracion.QueueClosedTTL/time.Second))) * time.Second
	configuracion.PollInterval = time.Duration(leerEntero("SCRAPER_POLL_INTERVAL_MS", int(configuracion.PollInterval/time.Millisecond))) * time.Millisecond
	configuracion.RetryInterval = time.Duration(leerEntero("SCRAPER_RETRY_INTERVAL_SECONDS", int(configuracion.RetryInterval/time.Second))) * time.Second
	configuracion.CollectInterval = time.Duration(leerEntero("SCRAPER_COLLECT_INTERVAL_SECONDS", int(configuracion.CollectInterval/time.Second))) * time.Second
	configuracion.CollectLimit = leerEntero("SCRAPER_COLLECT_LIMIT", configuracion.CollectLimit)
	configuracion.CollectTimeout = time.Duration(leerEntero("SCRAPER_COLLECT_TIMEOUT_SECONDS", int(configuracion.CollectTimeout/time.Second))) * time.Second
	configuracion.GCTimeout = time.Duration(leerEntero("SCRAPER_GC_TIMEOUT_SECONDS", int(configuracion.GCTimeout/time.Second))) * time.Second
	configuracion.CrawlAttempts = leerEntero("SCRAPER_CRAWL_ATTEMPTS", configuracion.CrawlAttempts)
	configuracion.RobotsCacheTTL = time.Duration(leerEntero("SCRAPER_ROBOTS_CACHE_TTL_SECONDS", int(configuracion.RobotsCacheTTL/time.Second))) * time.Second
	configuracion.RobotsFailureTTL = time.Duration(leerEntero("SCRAPER_ROBOTS_FAILURE_TTL_SECONDS", int(configuracion.RobotsFailureTTL/time.Second))) * time.Second
	configuracion.RobotsMaxBytes = int64(leerEntero("SCRAPER_ROBOTS_MAX_BYTES", int(configuracion.RobotsMaxBytes)))
	configuracion.BackoffMin = time.Duration(leerEntero("SCRAPER_BACKOFF_MIN_MS", int(configuracion.BackoffMin/time.Millisecond))) * time.Millisecond
	configuracion.BackoffMax = time.Duration(leerEntero("SCRAPER_BACKOFF_MAX_SECONDS", int(configuracion.BackoffMax/time.Second))) * time.Second

	if errorCarga != nil {
		return Config{}, errorCarga
	}
	if errorObtenido := configuracion.Validate(); errorObtenido != nil {
		return Config{}, errorObtenido
	}
	return configuracion, nil
}

// ===== Validación Operativa =====

// Validate comprueba los rangos admitidos por el worker.
func (configuracion Config) Validate() error {
	switch {
	case configuracion.Lease < MinLeaseDuration:
		return fmt.Errorf("SCRAPER_LEASE_MS debe ser al menos %d", MinLeaseDuration.Milliseconds())
	case configuracion.MinFreeBytes < MinFreeBytesLimit:
		return fmt.Errorf("SCRAPER_STATE_MIN_FREE_BYTES debe ser al menos %d bytes", MinFreeBytesLimit)
	case configuracion.BatchSize < MinBatchSize || configuracion.BatchSize > MaxBatchSize:
		return fmt.Errorf("SCRAPER_API_BATCH_SIZE debe estar entre %d y %d", MinBatchSize, MaxBatchSize)
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
	case configuracion.BackoffMin <= 0:
		return fmt.Errorf("SCRAPER_BACKOFF_MIN_MS debe ser mayor que cero")
	case configuracion.BackoffMax <= 0:
		return fmt.Errorf("SCRAPER_BACKOFF_MAX_SECONDS debe ser mayor que cero")
	}
	return nil
}

// ===== Lectura de Entorno =====

// leerCadena obtiene el valor de una variable de entorno o devuelve el respaldo por defecto.
func leerCadena(clave, respaldo string) string {
	if valor := strings.TrimSpace(os.Getenv(clave)); valor != "" {
		return valor
	}
	return respaldo
}

// leerURLsCategoriaEntorno reúne todas las variables `<SPIDER>_CATEGORY_URLS`
// (URLs separadas por coma). Se leen por sufijo y no con una lista fija para
// que un catálogo externo (SCRAPER_SPIDERS_FILE) pueda declarar su propia
// variable en `category_env` sin tocar este paquete.
func leerURLsCategoriaEntorno() map[string][]string {
	urlsPorVariable := map[string][]string{}
	for _, entrada := range os.Environ() {
		clave, valor, tieneValor := strings.Cut(entrada, "=")
		if !tieneValor || !strings.HasSuffix(clave, SufijoURLsCategoria) {
			continue
		}
		for _, urlCruda := range strings.Split(valor, ",") {
			if urlLimpia := strings.TrimSpace(urlCruda); urlLimpia != "" {
				urlsPorVariable[clave] = append(urlsPorVariable[clave], urlLimpia)
			}
		}
	}
	return urlsPorVariable
}

// interpretarNivelLog mapea la cadena de texto de nivel de log a slog.Level.
func interpretarNivelLog(crudo string) slog.Level {
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
