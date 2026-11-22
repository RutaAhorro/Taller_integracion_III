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

	cola "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/cola"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/worker"
)

// ===== Ejecución =====

// main es el punto de entrada del demonio trabajador del scraper.
func main() {
	if errorObtenido := ejecutar(); errorObtenido != nil {
		slog.Error("worker_exit", "error", errorObtenido)
		os.Exit(1)
	}
}

// ejecutar inicializa las dependencias de red, catálogo y cola, y arranca el worker.
func ejecutar() error {
	configuracion, errorObtenido := config.FromEnv()
	if errorObtenido != nil {
		return errorObtenido
	}
	registrador := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: configuracion.LogLevel}))
	slog.SetDefault(registrador)

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

	clienteHTTP := extraccion.New(extraccion.Options{
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
	registro, errorObtenido := extraccion.NewRegistryFromConfig(configuracion, clienteHTTP, configuracion.CategoryDir, configuracion.MaxPages, registrador)
	if errorObtenido != nil {
		return errorObtenido
	}

	clienteCola, errorObtenido := cola.NewQueue(clienteRedis, cola.Options{
		Stream:    configuracion.QueueName,
		Group:     configuracion.Group,
		Lease:     configuracion.Lease,
		Block:     configuracion.QueueBlock,
		ClosedTTL: configuracion.QueueClosedTTL,
	})
	if errorObtenido != nil {
		return errorObtenido
	}
	clienteAPI := cola.NewControlClient(configuracion.APIURL, configuracion.Token, configuracion.APITimeout)

	trabajadorScraper, errorObtenido := worker.New(configuracion, clienteAPI, clienteCola, registro, registrador)
	if errorObtenido != nil {
		return errorObtenido
	}
	if errorObtenido := trabajadorScraper.Run(contexto); errorObtenido != nil && !errors.Is(errorObtenido, context.Canceled) {
		return errorObtenido
	}
	registrador.Info("worker_stopped")
	return nil
}
