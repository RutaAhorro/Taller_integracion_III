// Motor de rutas: dueño del esquema rutas.* (listas de compra y, más adelante,
// optimización de rutas).
//
// Este archivo solo arma las piezas y arranca el servidor HTTP:
//
//  1. Lee y valida la configuración del entorno (internal/config).
//  2. Abre el pool de PostgreSQL con el rol app_rutas (internal/db).
//  3. Construye el handler HTTP con sus middlewares (internal/httpapi).
//  4. Atiende peticiones hasta recibir SIGINT/SIGTERM y se apaga ordenadamente.
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

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/db"
	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/httpapi"
	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
)

// =============================================================================
// Tiempos del servidor
// =============================================================================

const (
	plazoPing         = 2 * time.Second  // máximo para responder la sonda ready
	plazoCabeceras    = 5 * time.Second  // evita clientes que envían cabeceras muy lento
	plazoLectura      = 10 * time.Second // lectura completa de la petición
	plazoEscritura    = 15 * time.Second // escritura completa de la respuesta
	plazoInactividad  = 60 * time.Second // conexiones keep-alive sin uso
	plazoApagadoSuave = 10 * time.Second // espera a peticiones en curso al apagar
)

// =============================================================================
// Punto de entrada
// =============================================================================

func main() {
	// Logs en JSON por stdout: Kubernetes y Docker los recogen sin configuración extra.
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if errorObtenido := ejecutar(log); errorObtenido != nil {
		log.Error("motor_rutas_detenido", "error", errorObtenido.Error())
		os.Exit(1)
	}
}

// ejecutar contiene todo el ciclo de vida. Separarlo de main permite usar
// defer (cerrar el pool, cancelar contextos) antes de terminar el proceso.
func ejecutar(log *slog.Logger) error {
	// --- 1. Configuración --------------------------------------------------------
	configuracion, errorObtenido := config.Cargar(os.LookupEnv)
	if errorObtenido != nil {
		return errorObtenido
	}

	// El contexto se cancela al recibir Ctrl+C (SIGINT) o la señal de Kubernetes (SIGTERM).
	contexto, detenerSenales := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer detenerSenales()

	// --- 2. Base de datos --------------------------------------------------------
	pool, errorObtenido := db.Abrir(contexto, configuracion.DBURL, configuracion.DBPoolMax)
	if errorObtenido != nil {
		return errorObtenido
	}
	defer pool.Close()

	// --- 3. Servidor HTTP --------------------------------------------------------
	manejadorHTTP := httpapi.Nuevo(httpapi.Opciones{
		Repo:         listas.NuevoPostgres(pool),
		JWTSecret:    configuracion.JWTSecret,
		OrigenesCORS: configuracion.OrigenesCORS,
		MaxListas:    configuracion.MaxListas,
		Ping: func(contextoPing context.Context) bool {
			return db.Ping(contextoPing, pool, plazoPing)
		},
		Log: log,
	})

	servidor := &http.Server{
		Addr:              ":" + configuracion.Puerto,
		Handler:           manejadorHTTP,
		ReadHeaderTimeout: plazoCabeceras,
		ReadTimeout:       plazoLectura,
		WriteTimeout:      plazoEscritura,
		IdleTimeout:       plazoInactividad,
	}

	// ListenAndServe bloquea, por eso corre en otra goroutine y avisa por el canal si falla.
	errorServidor := make(chan error, 1)
	go func() { errorServidor <- servidor.ListenAndServe() }()
	log.Info("motor_rutas_iniciado", "puerto", configuracion.Puerto)

	// --- 4. Esperar fallo del servidor o señal de apagado --------------------------
	select {
	case errorObtenido := <-errorServidor:
		if !errors.Is(errorObtenido, http.ErrServerClosed) {
			return errorObtenido
		}
		return nil
	case <-contexto.Done():
	}

	// Apagado suave: deja terminar las peticiones en curso, con un plazo máximo.
	contextoApagado, cancelarApagado := context.WithTimeout(context.Background(), plazoApagadoSuave)
	defer cancelarApagado()
	return servidor.Shutdown(contextoApagado)
}
