// Comando identidad: carga la configuración, abre PostgreSQL y Redis,
// verifica tablas y permisos, sirve HTTP y apaga ordenadamente con SIGINT/SIGTERM.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/repositories"
	"github.com/redis/go-redis/v9"
)

// ===== Entrada =====

// main configura el log JSON y termina con código 1 si el servicio no arranca.
func main() {
	registrador := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(registrador)
	if errorObtenido := ejecutar(registrador); errorObtenido != nil {
		registrador.Error("identidad_stopped", "error", errorObtenido)
		os.Exit(1)
	}
}

// ejecutar arma y sirve Identidad hasta recibir una señal de apagado.
func ejecutar(registrador *slog.Logger) error {
	configuracion, errorObtenido := config.Cargar()
	if errorObtenido != nil {
		return errorObtenido
	}
	opciones, errorObtenido := redis.ParseURL(configuracion.URLRedis)
	if errorObtenido != nil {
		return errors.New("REDIS_URL inválido")
	}
	opciones.ContextTimeoutEnabled = true
	opciones.MaxRetries = -1
	opciones.DialTimeout = configuracion.Ajustes.PlazoRedis
	opciones.ReadTimeout = configuracion.Ajustes.PlazoRedis
	opciones.WriteTimeout = configuracion.Ajustes.PlazoRedis
	redisCliente := redis.NewClient(opciones)
	defer redisCliente.Close()
	contexto, detener := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer detener()
	contextoInicio, cancelar := context.WithTimeout(contexto, configuracion.Ajustes.PlazoInicio)
	baseDatos, errorObtenido := repositories.Abrir(contextoInicio, configuracion.URLBaseDatos, configuracion.Ajustes)
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
	if errorOperacion := repositories.VerificarAlmacenamiento(contextoInicio, baseDatos); errorOperacion != nil {
		cancelar()
		registrador.Error("identidad_storage_invalid", "error", errorOperacion)
		return errors.New("Identidad requiere tablas y permisos antes de iniciar")
	}
	cancelar()
	modulo := api.NuevoModulo(baseDatos, redisCliente, configuracion)
	servidorHTTP := &http.Server{Addr: configuracion.DireccionEscucha, Handler: api.NuevoEnrutador(configuracion, modulo, redisCliente, pool.PingContext, registrador), ReadHeaderTimeout: configuracion.Ajustes.PlazoCabeceras, ReadTimeout: configuracion.Ajustes.PlazoLectura, WriteTimeout: configuracion.Ajustes.PlazoEscritura, IdleTimeout: configuracion.Ajustes.PlazoInactividad, MaxHeaderBytes: configuracion.Ajustes.MaximoBytesCabeceras}
	terminado := make(chan error, 1)
	go func() { terminado <- servidorHTTP.ListenAndServe() }()
	select {
	case errorOperacion := <-terminado:
		if !errors.Is(errorOperacion, http.ErrServerClosed) {
			return fmt.Errorf("no se pudo iniciar HTTP de Identidad: %w", errorOperacion)
		}
	case <-contexto.Done():
		contextoApagado, cancelarOperacion := context.WithTimeout(context.Background(), configuracion.Ajustes.PlazoApagado)
		defer cancelarOperacion()
		if errorOperacion := servidorHTTP.Shutdown(contextoApagado); errorOperacion != nil {
			_ = servidorHTTP.Close()
			return errors.New("Identidad agotó el plazo de apagado")
		}
	}
	return nil
}
