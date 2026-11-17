package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/mailer"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/server"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	log.Info("identidad_correo", "modo", cfg.Correo.Modo, "host", cfg.Correo.Host, "puerto", cfg.Correo.Puerto)
	if cfg.Correo.Modo == config.CorreoLog {
		log.Warn("identidad_correo_modo_log", "detalle", "los correos se imprimen completos en el log; sólo para desarrollo local")
	}
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return errors.New("REDIS_URL inválido")
	}
	opt.ContextTimeoutEnabled = true
	opt.MaxRetries = -1
	opt.DialTimeout = cfg.Ajustes.RedisTimeout
	opt.ReadTimeout = cfg.Ajustes.RedisTimeout
	opt.WriteTimeout = cfg.Ajustes.RedisTimeout
	rdb := redis.NewClient(opt)
	defer rdb.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, cfg.Ajustes.InicioTimeout)
	db, err := repositories.Open(startup, cfg.DatabaseURL, cfg.Ajustes)
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
	if err := server.VerifyStorage(startup, db); err != nil {
		cancel()
		log.Error("identidad_storage_invalid", "error", err)
		return errors.New("Identidad requiere tablas y permisos antes de iniciar")
	}
	recuperacion, err := recuperacionDesde(startup, cfg, db, log)
	cancel()
	if err != nil {
		return err
	}
	modulo := server.NewModule(db, rdb, recuperacion, cfg)
	httpServer := &http.Server{Addr: cfg.ListenAddr, Handler: server.NewRouter(cfg, modulo, rdb, pool.PingContext, log), ReadHeaderTimeout: cfg.Ajustes.ReadHeaderTimeout, ReadTimeout: cfg.Ajustes.ReadTimeout, WriteTimeout: cfg.Ajustes.WriteTimeout, IdleTimeout: cfg.Ajustes.IdleTimeout, MaxHeaderBytes: cfg.Ajustes.MaxHeaderBytes}
	done := make(chan error, 1)
	go func() { done <- httpServer.ListenAndServe() }()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("no se pudo iniciar HTTP de Identidad: %w", err)
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), cfg.Ajustes.ApagadoTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdown); err != nil {
			_ = httpServer.Close()
			return errors.New("Identidad agotó el plazo de apagado")
		}
		if err := modulo.Cerrar(shutdown); err != nil {
			log.Warn("identidad_recuperacion_pendiente", "error", err)
		}
	}
	return nil
}

// recuperacionDesde decide si la recuperación de contraseña (SUP-184) queda activa.
// Cualquier falta la apaga con un log explícito, sin impedir el login.
func recuperacionDesde(ctx context.Context, cfg config.Config, db *gorm.DB, log *slog.Logger) (server.Recuperacion, error) {
	if cfg.Correo.Modo == config.CorreoDeshabilitado || cfg.URLRestablecer == "" {
		log.Warn("identidad_recuperacion_deshabilitada", "detalle", "requiere correo (smtp o log) e IDENTIDAD_URL_RESTABLECER")
		return server.Recuperacion{}, nil
	}
	if err := server.VerificarRecuperacion(ctx, db); err != nil {
		log.Warn("identidad_recuperacion_deshabilitada", "detalle", err.Error())
		return server.Recuperacion{}, nil
	}
	correo, err := mailer.Nuevo(cfg.Correo, log)
	if err != nil {
		return server.Recuperacion{}, err
	}
	log.Info("identidad_recuperacion_activa", "url_restablecer", cfg.URLRestablecer)
	return server.Recuperacion{Correo: correo, URLRestablecer: cfg.URLRestablecer, Log: log}, nil
}
