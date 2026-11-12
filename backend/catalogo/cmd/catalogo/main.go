package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/ingesta"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/server"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "catalogo")
	if err := run(log); err != nil {
		log.Error("catalogo_stopped", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return errors.New("no se pudo configurar la conexión PostgreSQL")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return errors.New("no se pudo obtener la conexión PostgreSQL")
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	// Validar una sola vez antes de exponer HTTP; no migrar ni servir con grants
	// incompletos. Durante la ejecución readiness sólo comprueba conectividad.
	storageCtx, storageCancel := context.WithTimeout(context.Background(), cfg.StartupTimeout)
	storageErr := ingesta.VerifyStorage(storageCtx, db, cfg.InternalToken != "")
	storageCancel()
	if storageErr != nil {
		log.Error("catalogo_storage_invalid", "error", storageErr)
		return errors.New("Catálogo requiere tablas y permisos runtime antes de iniciar")
	}
	service := services.NewProductoService(repositories.NewProductoRepository(db), services.LimitesConsulta{
		TamanoPaginaPredeterminado: cfg.DefaultPageSize,
		TamanoPaginaMaximo:         cfg.MaxPageSize,
		LargoMaximoBusqueda:        cfg.MaxSearchLength,
	})
	fuenteService := services.NewFuenteService(repositories.NewFuenteRepository(db))
	ready := sqlDB.PingContext
	httpServer := &http.Server{
		Addr: cfg.ListenAddr, Handler: server.NewRouter(service, fuenteService, cfg.JWTSecret, ready, cfg.ReadinessTimeout, log, cfg.CORSAllowedOrigins,
			ingesta.Config{Repository: ingesta.NewRepository(db), Token: cfg.InternalToken, MaxProductos: cfg.MaxBatchProductos}),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout, ReadTimeout: cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- httpServer.ListenAndServe() }()
	log.Info("catalogo_started", "listen_addr", cfg.ListenAddr)
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdown); err != nil {
			_ = httpServer.Close()
			return err
		}
		return nil
	}
}
