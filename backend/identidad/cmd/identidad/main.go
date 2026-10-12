package main

import (
	"context"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/database"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/identity"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/server"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if err := run(log); err != nil {
		log.Error("identidad_stopped", "error", err)
		os.Exit(1)
	}
}
func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return errors.New("REDIS_URL inválido")
	}
	opt.ContextTimeoutEnabled = true
	opt.MaxRetries = -1
	opt.DialTimeout = time.Second
	opt.ReadTimeout = time.Second
	opt.WriteTimeout = time.Second
	rdb := redis.NewClient(opt)
	defer rdb.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 5*time.Second)
	db, err := database.Open(startup, cfg.DatabaseURL)
	if err != nil {
		cancel()
		return err
	}
	pool, err := db.DB()
	if err != nil {
		cancel()
		return errors.New("pool de Identidad inválido")
	}
	defer pool.Close()
	err = identity.VerifyStorage(startup, db)
	cancel()
	if err != nil {
		log.Error("identidad_storage_invalid", "error", err)
		return errors.New("Identidad requiere tablas y permisos antes de iniciar")
	}
	httpServer := &http.Server{Addr: cfg.ListenAddr, Handler: server.NewRouter(cfg, db, rdb, pool.PingContext, log), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 64 * 1024}
	done := make(chan error, 1)
	go func() { done <- httpServer.ListenAndServe() }()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return errors.New("no se pudo iniciar HTTP de Identidad")
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdown); err != nil {
			_ = httpServer.Close()
			return errors.New("Identidad agotó el plazo de apagado")
		}
	}
	return nil
}
