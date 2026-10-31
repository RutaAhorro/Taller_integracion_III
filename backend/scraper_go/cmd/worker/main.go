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
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/controlscraping"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/queue"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker_exit", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	if cfg.Token == "" {
		return fmt.Errorf("SCRAPING_SERVICE_TOKEN no configurado: Control de scraping rechazaría al worker")
	}
	if err := os.MkdirAll(cfg.StateDir, 0o700); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("REDIS_URL inválida: %w", err)
	}
	redisOptions.DialTimeout = 5 * time.Second
	redisOptions.ReadTimeout = 5 * time.Second
	redisOptions.WriteTimeout = 5 * time.Second
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("no se pudo conectar a Redis: %w", err)
	}

	httpClient := crawler.New(crawler.Options{
		UserAgent:  cfg.UserAgent,
		Delay:      cfg.DownloadDelay,
		Timeout:    cfg.DownloadTimeout,
		MaxSize:    cfg.MaxDownloadSize,
		MaxRetries: cfg.MaxRetries,
		Logger:     logger,
	})
	registry := spiders.NewRegistry(httpClient, cfg.CategoryDir, spiders.DefaultMaxPages, logger)

	queueClient, err := queue.New(redisClient, queue.Options{
		Stream: cfg.QueueName,
		Group:  cfg.Group,
		Lease:  cfg.Lease,
	})
	if err != nil {
		return err
	}
	apiClient := controlscraping.New(cfg.APIURL, cfg.Token, cfg.APITimeout)

	scraperWorker := worker.New(cfg, apiClient, queueClient, registry, logger)
	if err := scraperWorker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	logger.Info("worker_stopped")
	return nil
}
