// Command worker arranca el microservicio de scraping en Go.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/controlscraping"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/queue"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/worker"
)

// ===== Ejecución =====

// main es el punto de entrada del demonio trabajador del scraper.
func main() {
	if errorObtenido := run(); errorObtenido != nil {
		slog.Error("worker_exit", "error", errorObtenido)
		os.Exit(1)
	}
}

// run ejecuta el bucle de procesamiento continuo o la tarea solicitada.
func run() error {
	configuracion, errorObtenido := config.FromEnv()
	if errorObtenido != nil {
		return errorObtenido
	}
	registrador := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: configuracion.LogLevel}))
	slog.SetDefault(registrador)

	if configuracion.Token == "" {
		return fmt.Errorf("SCRAPING_SERVICE_TOKEN no configurado: Control de scraping rechazaría al worker")
	}
	if errorObtenido := os.MkdirAll(configuracion.StateDir, 0o700); errorObtenido != nil {
		return errorObtenido
	}

	contexto, parada := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parada()

	opcionesRedis, errorObtenido := redis.ParseURL(configuracion.RedisURL)
	if errorObtenido != nil {
		return fmt.Errorf("REDIS_URL inválida: %w", errorObtenido)
	}
	opcionesRedis.DialTimeout = configuracion.RedisDialTimeout
	opcionesRedis.ReadTimeout = configuracion.RedisDialTimeout
	opcionesRedis.WriteTimeout = configuracion.RedisDialTimeout
	clienteRedis := redis.NewClient(opcionesRedis)
	defer clienteRedis.Close()

	contextoPing, cancelar := context.WithTimeout(contexto, configuracion.RedisPingTimeout)
	defer cancelar()
	if errorObtenido := clienteRedis.Ping(contextoPing).Err(); errorObtenido != nil {
		return fmt.Errorf("no se pudo conectar a Redis: %w", errorObtenido)
	}

	clienteHTTP := crawler.New(crawler.Options{
		UserAgent:        configuracion.UserAgent,
		Delay:            configuracion.DownloadDelay,
		Timeout:          configuracion.DownloadTimeout,
		MaxSize:          configuracion.MaxDownloadSize,
		MaxRetries:       configuracion.MaxRetries,
		RobotsCacheTTL:   configuracion.RobotsCacheTTL,
		RobotsFailureTTL: configuracion.RobotsFailureTTL,
		RobotsMaxBytes:   configuracion.RobotsMaxBytes,
		BackoffMin:       configuracion.BackoffMin,
		BackoffMax:       configuracion.BackoffMax,
		Logger:           registrador,
	})
	registro, errorObtenido := spiders.NewRegistryFromEnv(clienteHTTP, configuracion.CategoryDir, configuracion.MaxPages, registrador)
	if errorObtenido != nil {
		return errorObtenido
	}

	clienteCola, errorObtenido := queue.New(clienteRedis, queue.Options{
		Stream:    configuracion.QueueName,
		Group:     configuracion.Group,
		Lease:     configuracion.Lease,
		Block:     configuracion.QueueBlock,
		ClosedTTL: configuracion.QueueClosedTTL,
	})
	if errorObtenido != nil {
		return errorObtenido
	}
	clienteAPI := controlscraping.New(configuracion.APIURL, configuracion.Token, configuracion.APITimeout)

	trabajadorScraper := worker.New(configuracion, clienteAPI, clienteCola, registro, registrador)
	if errorObtenido := trabajadorScraper.Run(contexto); errorObtenido != nil && !errors.Is(errorObtenido, context.Canceled) {
		return errorObtenido
	}
	registrador.Info("worker_stopped")
	return nil
}
