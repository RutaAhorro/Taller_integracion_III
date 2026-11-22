// Comando control_scraping: carga la configuración, abre PostgreSQL y Redis,
// verifica tablas y permisos, inicia la recuperación en segundo plano, sirve
// HTTP y apaga ordenadamente con SIGINT/SIGTERM.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/repositories"
	"github.com/joho/godotenv"
)

// ===== Plazos del proceso =====

const (
	plazoInicio          = 5 * time.Second
	plazoPingRedisInicio = 2 * time.Second
	plazoApagado         = 15 * time.Second
	plazoEsperaCiclos    = 2 * time.Second
	plazoCabeceras       = 5 * time.Second
	plazoLectura         = 30 * time.Second
	// plazoEscritura supera el plazo de petición (65 s) para responder su timeout.
	plazoEscritura       = 70 * time.Second
	plazoInactividad     = 60 * time.Second
	maximoBytesCabeceras = 64 << 10
)

// ===== Entrada =====

// main configura el log JSON y termina con código 1 si el servicio no arranca.
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if errorObtenido := ejecutar(); errorObtenido != nil {
		slog.Error("scraping_startup_failed", "error", errorObtenido)
		os.Exit(1)
	}
}

// ejecutar arma y sirve Control hasta recibir una señal de apagado.
func ejecutar() error {
	// .env sólo existe en desarrollo local; Compose y K8s inyectan el entorno.
	if errorObtenido := godotenv.Load(); errorObtenido != nil {
		slog.Info("scraping_env_file_absent")
	}
	configuracion, errorObtenido := config.Cargar()
	if errorObtenido != nil {
		return errorObtenido
	}

	contextoInicio, cancelarInicio := context.WithTimeout(context.Background(), plazoInicio)
	baseDatos, errorObtenido := repositories.Abrir(contextoInicio, configuracion.URLBaseDatos)
	cancelarInicio()
	if errorObtenido != nil {
		return errorObtenido
	}
	defer repositories.Cerrar(baseDatos)

	// Redis caído no impide arrancar: el outbox SQL conserva las ejecuciones.
	clienteRedis := repositories.NuevoClienteRedis(configuracion.URLRedis)
	defer clienteRedis.Close()
	contextoPing, cancelarPing := context.WithTimeout(context.Background(), plazoPingRedisInicio)
	if clienteRedis.Ping(contextoPing).Err() != nil {
		slog.Warn("scraping_redis_unavailable_at_start")
	}
	cancelarPing()

	enrutador, recuperacion := api.Componer(configuracion, baseDatos, clienteRedis)
	contexto, detener := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer detener()
	contextoVerificacion, cancelarVerificacion := context.WithTimeout(contexto, plazoInicio)
	errorObtenido = repositories.VerificarAlmacenamiento(contextoVerificacion, baseDatos)
	cancelarVerificacion()
	if errorObtenido != nil {
		return errorObtenido
	}

	contextoCiclos, cancelarCiclos := context.WithCancel(context.Background())
	defer func() {
		cancelarCiclos()
		contextoEspera, cancelarEspera := context.WithTimeout(context.Background(), plazoEsperaCiclos)
		defer cancelarEspera()
		_ = recuperacion.Esperar(contextoEspera)
	}()
	recuperacion.Iniciar(contextoCiclos)

	servidorHTTP := &http.Server{
		Addr: configuracion.DireccionEscucha, Handler: enrutador,
		ReadHeaderTimeout: plazoCabeceras, ReadTimeout: plazoLectura, WriteTimeout: plazoEscritura,
		IdleTimeout: plazoInactividad, MaxHeaderBytes: maximoBytesCabeceras,
	}
	slog.Info("scraping_listening", "address", configuracion.DireccionEscucha)
	resultadoServidor := make(chan error, 1)
	go func() { resultadoServidor <- servidorHTTP.ListenAndServe() }()
	select {
	case errorServidor := <-resultadoServidor:
		if !errors.Is(errorServidor, http.ErrServerClosed) {
			return errors.New("no se pudo iniciar el listener HTTP")
		}
	case <-contexto.Done():
		contextoApagado, cancelarApagado := context.WithTimeout(context.Background(), plazoApagado)
		defer cancelarApagado()
		if errorApagado := servidorHTTP.Shutdown(contextoApagado); errorApagado != nil {
			_ = servidorHTTP.Close()
		}
		cancelarCiclos()
		if errorEspera := recuperacion.Esperar(contextoApagado); errorEspera != nil {
			slog.Error("scraping_shutdown_timeout")
		}
	}
	return nil
}
