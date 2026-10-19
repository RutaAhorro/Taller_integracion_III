package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/database"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/identity"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/mailer"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/server"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
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
	if len(os.Args) > 1 && os.Args[1] == "correo-prueba" {
		if err := correoPrueba(os.Args[2:], log); err != nil {
			log.Error("correo_prueba_fallida", "error", err)
			os.Exit(1)
		}
		return
	}
	if err := run(log); err != nil {
		log.Error("identidad_stopped", "error", err)
		os.Exit(1)
	}
}

// correoPrueba envía un correo real desde el mismo pod o contenedor del servicio:
// comprueba salida al puerto SMTP, TLS y App Password sin exigir BD ni Redis.
func correoPrueba(args []string, log *slog.Logger) error {
	if len(args) != 1 {
		return errors.New("uso: identidad correo-prueba <destinatario>")
	}
	cfg, err := config.LoadCorreo()
	if err != nil {
		return err
	}
	if cfg.Modo != config.CorreoSMTP {
		return errors.New("correo-prueba requiere modo smtp: configurar SMTP_USER y SMTP_PASS")
	}
	smtp, err := mailer.NuevoSMTP(cfg)
	if err != nil {
		return err
	}
	origen, _ := os.Hostname()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := smtp.Enviar(ctx, mailer.MensajePrueba(args[0], origen, time.Now())); err != nil {
		return err
	}
	log.Info("correo_prueba_enviada", "host", cfg.Host, "puerto", cfg.Puerto, "tls", cfg.TLS)
	return nil
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
	if err := identity.VerifyStorage(startup, db); err != nil {
		cancel()
		log.Error("identidad_storage_invalid", "error", err)
		return errors.New("Identidad requiere tablas y permisos antes de iniciar")
	}
	recuperacion, err := recuperacionDesde(startup, cfg, db, log)
	cancel()
	if err != nil {
		return err
	}
	modulo := identity.NewConRecuperacion(db, rdb, recuperacion, cfg.CookieSecure)
	httpServer := &http.Server{Addr: cfg.ListenAddr, Handler: server.NewRouterConModulo(cfg, modulo, rdb, pool.PingContext, log), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 64 * 1024}
	done := make(chan error, 1)
	go func() { done <- httpServer.ListenAndServe() }()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("no se pudo iniciar HTTP de Identidad: %w", err)
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
func recuperacionDesde(ctx context.Context, cfg config.Config, db *gorm.DB, log *slog.Logger) (identity.Recuperacion, error) {
	if cfg.Correo.Modo == config.CorreoDeshabilitado || cfg.URLRestablecer == "" {
		log.Warn("identidad_recuperacion_deshabilitada", "detalle", "requiere correo (smtp o log) e IDENTIDAD_URL_RESTABLECER")
		return identity.Recuperacion{}, nil
	}
	if err := identity.VerificarRecuperacion(ctx, db); err != nil {
		log.Warn("identidad_recuperacion_deshabilitada", "detalle", err.Error())
		return identity.Recuperacion{}, nil
	}
	correo, err := mailer.Nuevo(cfg.Correo, log)
	if err != nil {
		return identity.Recuperacion{}, err
	}
	log.Info("identidad_recuperacion_activa", "url_restablecer", cfg.URLRestablecer)
	return identity.Recuperacion{Correo: correo, URLRestablecer: cfg.URLRestablecer, Log: log}, nil
}
