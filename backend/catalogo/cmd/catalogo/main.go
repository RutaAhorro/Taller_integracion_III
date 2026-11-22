// Command catalogo arranca el servicio de Catálogo: lee la configuración,
// verifica tablas y permisos una sola vez, compone repositorios, servicios y
// router, y atiende HTTP hasta recibir SIGINT o SIGTERM.
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

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
)

// ===== Arranque =====

// main registra en JSON y termina con código 1 si el arranque falla.
func main() {
	registrador := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "catalogo")
	if errorEjecucion := ejecutar(registrador); errorEjecucion != nil {
		registrador.Error("catalogo_stopped", "error", errorEjecucion)
		os.Exit(1)
	}
}

// ejecutar compone el servicio y bloquea hasta el apagado ordenado.
func ejecutar(registrador *slog.Logger) error {
	configuracion, errorConfiguracion := config.Cargar()
	if errorConfiguracion != nil {
		return errorConfiguracion
	}
	baseDatos, errorBase := repositories.AbrirBaseDatos(configuracion.URLBaseDatos)
	if errorBase != nil {
		return errorBase
	}
	conexiones, errorConexiones := baseDatos.DB()
	if errorConexiones != nil {
		return errors.New("no se pudo obtener la conexión PostgreSQL")
	}
	defer conexiones.Close()
	conexiones.SetMaxOpenConns(configuracion.MaximoConexionesAbiertas)
	conexiones.SetMaxIdleConns(configuracion.MaximoConexionesInactivas)
	conexiones.SetConnMaxLifetime(configuracion.VidaMaximaConexion)

	contextoInicio, cancelarInicio := context.WithTimeout(context.Background(), configuracion.PlazoInicio)
	errorAlmacenamiento := repositories.VerificarAlmacenamiento(contextoInicio, baseDatos, configuracion.TokenIngesta != "")
	cancelarInicio()
	if errorAlmacenamiento != nil {
		registrador.Error("catalogo_storage_invalid", "error", errorAlmacenamiento)
		return errors.New("Catálogo requiere tablas y permisos runtime antes de iniciar")
	}

	enrutador := api.NuevoEnrutador(api.Dependencias{
		Productos: services.NuevoServicioProductos(repositories.NuevoRepositorioProductos(baseDatos), services.LimitesProductos{
			TamanoPagina:        configuracion.TamanoPagina,
			TamanoPaginaMaximo:  configuracion.TamanoPaginaMaximo,
			LargoMaximoBusqueda: configuracion.LargoMaximoBusqueda,
		}),
		Fuentes: services.NuevoServicioFuentes(repositories.NuevoRepositorioFuentes(baseDatos)),
		Disponibilidad: services.NuevoServicioDisponibilidad(repositories.NuevoRepositorioDisponibilidad(baseDatos), services.LimitesDisponibilidad{
			MaximoProductos:  configuracion.MaximoProductosConsulta,
			AntiguedadMaxima: configuracion.AntiguedadMaximaDisponibilidad,
		}, time.Now),
		Ingesta:                repositories.NuevoRepositorioIngesta(baseDatos),
		ClaveJWT:               configuracion.ClaveJWT,
		TokenIngesta:           configuracion.TokenIngesta,
		TokenConsulta:          configuracion.TokenConsulta,
		OrigenesPermitidos:     configuracion.OrigenesPermitidos,
		SondaBase:              conexiones.PingContext,
		Registrador:            registrador,
		PlazoReadiness:         configuracion.PlazoReadiness,
		MaximoProductosPorLote: configuracion.MaximoProductosPorLote,
		MaximoBytesCuerpo:      configuracion.MaximoBytesCuerpo,
	})
	return servir(registrador, &http.Server{
		Addr: configuracion.DireccionEscucha, Handler: enrutador,
		ReadHeaderTimeout: configuracion.PlazoCabeceras, ReadTimeout: configuracion.PlazoLectura,
		WriteTimeout: configuracion.PlazoEscritura, IdleTimeout: configuracion.PlazoInactividad,
	}, configuracion.PlazoApagado)
}

// ===== Ciclo de vida HTTP =====

// servir atiende hasta una señal y luego espera las peticiones activas durante
// plazoApagado (CATALOGO_SHUTDOWN_TIMEOUT) antes de cerrar a la fuerza.
func servir(registrador *slog.Logger, servidor *http.Server, plazoApagado time.Duration) error {
	contextoSenal, detener := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer detener()
	terminado := make(chan error, 1)
	go func() { terminado <- servidor.ListenAndServe() }()
	registrador.Info("catalogo_started", "listen_addr", servidor.Addr)
	select {
	case errorServidor := <-terminado:
		if errors.Is(errorServidor, http.ErrServerClosed) {
			return nil
		}
		return errorServidor
	case <-contextoSenal.Done():
		contextoApagado, cancelar := context.WithTimeout(context.Background(), plazoApagado)
		defer cancelar()
		if errorApagado := servidor.Shutdown(contextoApagado); errorApagado != nil {
			_ = servidor.Close()
			return errorApagado
		}
		return nil
	}
}
