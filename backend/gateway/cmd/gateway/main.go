package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/proxy"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "gateway")
	c, err := config.Load(os.Getenv)
	if err != nil {
		logger.Error("invalid_configuration", "error", err.Error())
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, c, logger); err != nil {
		logger.Error("server_stopped", "error", err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, c config.Config, logger *slog.Logger) error {
	gateway := proxy.New(c, logger)
	defer gateway.CloseIdleConnections()
	server := &http.Server{
		Addr: c.ListenAddr, Handler: gateway,
		ReadHeaderTimeout: c.ReadHeaderTimeout,
		ReadTimeout:       c.RequestTimeout,
		WriteTimeout:      c.RequestTimeout + c.ReadHeaderTimeout,
		IdleTimeout:       c.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	stopped := make(chan error, 1)
	go func() { stopped <- server.ListenAndServe() }()
	logger.Info("server_starting", "listen_addr", c.ListenAddr)
	select {
	case err := <-stopped:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), c.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return err
		}
		err := <-stopped
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		logger.Info("server_stopped")
		return nil
	}
}
