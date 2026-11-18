// Comando identidad: carga la configuración, abre PostgreSQL y Redis,
// verifica tablas y permisos, sirve HTTP y apaga ordenadamente con SIGINT/SIGTERM.
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

// ===== Entrada =====

// main configura el log JSON y termina con código 1 si el servicio no arranca.
func main() {
	registrador := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(registrador)
	if errorObtenido := run(registrador); errorObtenido != nil {
		registrador.Error("identidad_stopped", "error", errorObtenido)
		os.Exit(1)
	}
}

// run arma y sirve Identidad hasta recibir una señal de apagado.
func run(registrador *slog.Logger) error {
	configuracion, errorObtenido := config.Load()
	if errorObtenido != nil {
		return errorObtenido
	}
	registrador.Info("identidad_correo", "modo", configuracion.Correo.Modo, "host", configuracion.Correo.Host, "puerto", configuracion.Correo.Puerto)
	if configuracion.Correo.Modo == config.CorreoLog {
		registrador.Warn("identidad_correo_modo_log", "detalle", "los correos se imprimen completos en el log; sólo para desarrollo local")
	}
	opciones, errorObtenido := redis.ParseURL(configuracion.RedisURL)
	if errorObtenido != nil {
		return errors.New("REDIS_URL inválido")
	}
	opciones.ContextTimeoutEnabled = true
	opciones.MaxRetries = -1
	opciones.DialTimeout = configuracion.Ajustes.RedisTimeout
	opciones.ReadTimeout = configuracion.Ajustes.RedisTimeout
	opciones.WriteTimeout = configuracion.Ajustes.RedisTimeout
	redisCliente := redis.NewClient(opciones)
	defer redisCliente.Close()
	contexto, detener := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer detener()
	contextoInicio, cancelar := context.WithTimeout(contexto, configuracion.Ajustes.InicioTimeout)
	baseDatos, errorObtenido := repositories.Open(contextoInicio, configuracion.DatabaseURL, configuracion.Ajustes)
	if errorObtenido != nil {
		cancelar()
		return errorObtenido
	}
	pool, errorObtenido := baseDatos.DB()
	if errorObtenido != nil {
		cancelar()
		return errors.New("pool de Identidad inválido")
	}
	defer pool.Close()
	if errorOperacion := server.VerifyStorage(contextoInicio, baseDatos); errorOperacion != nil {
		cancelar()
		registrador.Error("identidad_storage_invalid", "error", errorOperacion)
		return errors.New("Identidad requiere tablas y permisos antes de iniciar")
	}
	recuperacion, errorObtenido := recuperacionDesde(contextoInicio, configuracion, baseDatos, registrador)
	cancelar()
	if errorObtenido != nil {
		return errorObtenido
	}
	modulo := server.NewModule(baseDatos, redisCliente, recuperacion, configuracion)
	servidorHTTP := &http.Server{Addr: configuracion.ListenAddr, Handler: server.NewRouter(configuracion, modulo, redisCliente, pool.PingContext, registrador), ReadHeaderTimeout: configuracion.Ajustes.ReadHeaderTimeout, ReadTimeout: configuracion.Ajustes.ReadTimeout, WriteTimeout: configuracion.Ajustes.WriteTimeout, IdleTimeout: configuracion.Ajustes.IdleTimeout, MaxHeaderBytes: configuracion.Ajustes.MaxHeaderBytes}
	terminado := make(chan error, 1)
	go func() { terminado <- servidorHTTP.ListenAndServe() }()
	select {
	case errorOperacion := <-terminado:
		if !errors.Is(errorOperacion, http.ErrServerClosed) {
			return fmt.Errorf("no se pudo iniciar HTTP de Identidad: %w", errorOperacion)
		}
	case <-contexto.Done():
		contextoApagado, cancelarOperacion := context.WithTimeout(context.Background(), configuracion.Ajustes.ApagadoTimeout)
		defer cancelarOperacion()
		if errorOperacion := servidorHTTP.Shutdown(contextoApagado); errorOperacion != nil {
			_ = servidorHTTP.Close()
			return errors.New("Identidad agotó el plazo de apagado")
		}
		if errorOperacion := modulo.Cerrar(contextoApagado); errorOperacion != nil {
			registrador.Warn("identidad_recuperacion_pendiente", "error", errorOperacion)
		}
	}
	return nil
}

// ===== Recuperación de contraseña =====

// recuperacionDesde decide si la recuperación de contraseña (SUP-184) queda activa.
// Cualquier falta la apaga con un log explícito, sin impedir el login.
func recuperacionDesde(contexto context.Context, configuracion config.Config, baseDatos *gorm.DB, registrador *slog.Logger) (server.Recuperacion, error) {
	if configuracion.Correo.Modo == config.CorreoDeshabilitado || configuracion.URLRestablecer == "" {
		registrador.Warn("identidad_recuperacion_deshabilitada", "detalle", "requiere correo (smtp o log) e IDENTIDAD_URL_RESTABLECER")
		return server.Recuperacion{}, nil
	}
	if errorOperacion := server.VerificarRecuperacion(contexto, baseDatos); errorOperacion != nil {
		registrador.Warn("identidad_recuperacion_deshabilitada", "detalle", errorOperacion.Error())
		return server.Recuperacion{}, nil
	}
	correo, errorObtenido := mailer.Nuevo(configuracion.Correo, registrador)
	if errorObtenido != nil {
		return server.Recuperacion{}, errorObtenido
	}
	registrador.Info("identidad_recuperacion_activa", "url_restablecer", configuracion.URLRestablecer)
	return server.Recuperacion{Correo: correo, URLRestablecer: configuracion.URLRestablecer, Log: registrador}, nil
}
