// Ejecución durable de trabajos: la solicitud se acepta al quedar guardada en
// el outbox SQL y la recuperación la publica en Redis Streams. Redis nunca
// bloquea la respuesta HTTP ni el cierre de un trabajo.
package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/google/uuid"
)

// ===== Servicio de ejecuciones =====

// ServicioEjecuciones guarda ejecuciones y las publica de forma recuperable.
type ServicioEjecuciones struct {
	ejecuciones domain.EjecucionRepository
	publicador  domain.PublicadorTrabajos
}

// NuevoServicioEjecuciones compone el outbox SQL con su publicador.
func NuevoServicioEjecuciones(ejecuciones domain.EjecucionRepository, publicador domain.PublicadorTrabajos) *ServicioEjecuciones {
	return &ServicioEjecuciones{ejecuciones: ejecuciones, publicador: publicador}
}

// Encolar registra la ejecución en SQL antes de responder a quien la solicita.
func (servicio *ServicioEjecuciones) Encolar(contexto context.Context, trabajoID string, carga []byte) error {
	identificador, errorObtenido := uuid.Parse(trabajoID)
	if errorObtenido != nil {
		return domain.ErrUUIDInvalido
	}
	huella := sha256.Sum256(carga)
	ejecucion := domain.Ejecucion{TrabajoID: identificador, PayloadHash: hex.EncodeToString(huella[:]), Payload: string(carga)}
	if errorObtenido := servicio.ejecuciones.RegistrarEjecucion(contexto, ejecucion); errorObtenido != nil {
		return errorObtenido
	}
	slog.Info("scraper_execution_accepted", "trabajo_id", trabajoID)
	return nil
}

// RecuperarEjecuciones publica las ejecuciones vencidas y pospone su siguiente
// publicación; una falla de Redis deja la ejecución para el próximo ciclo.
func (servicio *ServicioEjecuciones) RecuperarEjecuciones(contexto context.Context) {
	pendientes, errorObtenido := servicio.ejecuciones.Pendientes(contexto)
	if errorObtenido != nil {
		slog.Error("scraper_outbox_error", "error", errorObtenido)
		return
	}
	for _, ejecucion := range pendientes {
		if errorObtenido := servicio.publicador.Publicar(contexto, ejecucion); errorObtenido != nil {
			slog.Warn("scraper_publicacion_pendiente", "trabajo_id", ejecucion.TrabajoID)
			continue
		}
		if errorObtenido := servicio.ejecuciones.PosponerPublicacion(contexto, ejecucion.TrabajoID); errorObtenido != nil {
			slog.Error("scraper_outbox_error", "trabajo_id", ejecucion.TrabajoID, "error", errorObtenido)
		}
	}
}

// ===== Recuperación en segundo plano =====

// Intervalos y plazos de los dos ciclos de recuperación. El plazo de una
// entrega cubre las tres llamadas HTTP a Catálogo.
const (
	intervaloEntregas       = time.Second
	plazoCicloEntregas      = 65 * time.Second
	intervaloEjecuciones    = 5 * time.Second
	plazoCicloEjecuciones   = 10 * time.Second
	ciclosRecuperacionTotal = 2
)

// Recuperacion ejecuta los ciclos que reintentan entregas y publican
// ejecuciones. Sólo main los inicia; construir el router no crea goroutines.
type Recuperacion struct {
	entregas         *ServicioEntregas
	ejecuciones      *ServicioEjecuciones
	cerrarPublicador func() error
	ciclosActivos    sync.WaitGroup
}

// NuevaRecuperacion compone los ciclos; cerrarPublicador se llama al terminar
// el ciclo de ejecuciones para liberar su cliente Redis.
func NuevaRecuperacion(entregas *ServicioEntregas, ejecuciones *ServicioEjecuciones, cerrarPublicador func() error) *Recuperacion {
	return &Recuperacion{entregas: entregas, ejecuciones: ejecuciones, cerrarPublicador: cerrarPublicador}
}

// Iniciar lanza los dos ciclos hasta que se cancele el contexto.
func (recuperacion *Recuperacion) Iniciar(contexto context.Context) {
	recuperacion.ciclosActivos.Add(ciclosRecuperacionTotal)
	go func() {
		defer recuperacion.ciclosActivos.Done()
		repetir(contexto, intervaloEntregas, plazoCicloEntregas, recuperacion.entregas.RecuperarEntregas)
	}()
	go func() {
		defer recuperacion.ciclosActivos.Done()
		defer func() { _ = recuperacion.cerrarPublicador() }()
		repetir(contexto, intervaloEjecuciones, plazoCicloEjecuciones, recuperacion.ejecuciones.RecuperarEjecuciones)
	}()
}

// Esperar aguarda el fin de ambos ciclos o el vencimiento del contexto.
func (recuperacion *Recuperacion) Esperar(contexto context.Context) error {
	terminado := make(chan struct{})
	go func() { recuperacion.ciclosActivos.Wait(); close(terminado) }()
	select {
	case <-terminado:
		return nil
	case <-contexto.Done():
		return contexto.Err()
	}
}

// repetir ejecuta la tarea en cada intervalo con un plazo propio por ciclo.
func repetir(contexto context.Context, intervalo, plazo time.Duration, tarea func(context.Context)) {
	temporizador := time.NewTicker(intervalo)
	defer temporizador.Stop()
	for {
		select {
		case <-contexto.Done():
			return
		case <-temporizador.C:
			contextoCiclo, cancelar := context.WithTimeout(contexto, plazo)
			tarea(contextoCiclo)
			cancelar()
		}
	}
}
