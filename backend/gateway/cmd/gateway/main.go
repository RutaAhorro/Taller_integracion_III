// Command gateway arranca el Gateway HTTP de RutaAhorro: carga la
// configuración, sirve el enrutador y se apaga ordenadamente con SIGTERM.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/config"
)

// ===== Arranque =====

// main sale con código 1 si la configuración es inválida o el servidor falla.
func main() {
	registro := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "gateway")
	configuracion, errorConfiguracion := config.Cargar(os.Getenv)
	if errorConfiguracion != nil {
		registro.Error("invalid_configuration", "error", errorConfiguracion.Error())
		os.Exit(1)
	}
	contexto, detener := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer detener()
	if errorServidor := servir(contexto, configuracion, registro); errorServidor != nil {
		registro.Error("server_stopped", "error", errorServidor.Error())
		os.Exit(1)
	}
}

// ===== Servidor =====

// servir atiende hasta que el contexto se cancele y luego espera a que las
// peticiones en curso terminen, como máximo GATEWAY_SHUTDOWN_TIMEOUT.
func servir(contexto context.Context, configuracion config.Configuracion, registro *slog.Logger) error {
	pasarela := api.Nueva(configuracion, registro)
	defer pasarela.CerrarConexionesInactivas()
	servidor := &http.Server{
		Addr:              configuracion.DireccionEscucha,
		Handler:           pasarela,
		ReadHeaderTimeout: configuracion.LimiteLecturaCabeceras,
		ReadTimeout:       configuracion.LimitePeticion,
		// La escritura incluye el tiempo de leer cabeceras más el de la petición.
		WriteTimeout: configuracion.LimitePeticion + configuracion.LimiteLecturaCabeceras,
		IdleTimeout:  configuracion.LimiteInactividad,
		ErrorLog:     slog.NewLogLogger(registro.Handler(), slog.LevelError),
	}
	detenido := make(chan error, 1)
	go func() { detenido <- servidor.ListenAndServe() }()
	registro.Info("server_starting", "listen_addr", configuracion.DireccionEscucha)
	select {
	case errorServidor := <-detenido:
		return errorServidor
	case <-contexto.Done():
		contextoApagado, cancelar := context.WithTimeout(context.Background(), configuracion.LimiteApagado)
		defer cancelar()
		if errorApagado := servidor.Shutdown(contextoApagado); errorApagado != nil {
			_ = servidor.Close()
			return errorApagado
		}
		if errorServidor := <-detenido; !errors.Is(errorServidor, http.ErrServerClosed) {
			return errorServidor
		}
		registro.Info("server_stopped")
		return nil
	}
}
