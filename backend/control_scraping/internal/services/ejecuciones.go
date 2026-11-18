package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/google/uuid"
)

// ===== Encolado y recuperación del outbox =====

// QueueService coordina el registro SQL y la publicación recuperable de ejecuciones.
type QueueService struct {
	trabajos   domain.EjecucionRepository
	publicador domain.JobPublisher
}

// NewQueueService compone el repositorio durable y su publicador de trabajos.
func NewQueueService(trabajos domain.EjecucionRepository, publicador domain.JobPublisher) *QueueService {
	return &QueueService{trabajos: trabajos, publicador: publicador}
}

// Encolar registra la ejecución en SQL antes de responder a quien la solicita.
func (servicioCola *QueueService) Encolar(contexto context.Context, identificador string, carga []byte) error {
	trabajoID, errorObtenido := uuid.Parse(identificador)
	if errorObtenido != nil {
		return ErrUUIDInvalido
	}
	huella := sha256.Sum256(carga)
	ejecucion := domain.Ejecucion{TrabajoID: trabajoID, PayloadHash: hex.EncodeToString(huella[:]), Payload: string(carga)}
	if errorObtenido := servicioCola.trabajos.RegistrarEjecucion(contexto, ejecucion); errorObtenido != nil {
		return errorObtenido
	}
	// La aceptación depende sólo de SQL; Redis nunca bloquea el HTTP público.
	slog.Info("scraper_execution_accepted", "trabajo_id", identificador)
	return nil
}

// Recuperar publica las ejecuciones pendientes y pospone el siguiente intento.
func (servicioCola *QueueService) Recuperar(contexto context.Context) {
	entradas, errorObtenido := servicioCola.trabajos.Pendientes(contexto)
	if errorObtenido != nil {
		slog.Error("scraper_outbox_error", "error", errorObtenido)
		return
	}
	for _, ejecucion := range entradas {
		if errorObtenido := servicioCola.publicador.Publicar(contexto, ejecucion); errorObtenido != nil {
			slog.Warn("scraper_publicacion_pendiente", "trabajo_id", ejecucion.TrabajoID)
			continue
		}
		if errorObtenido := servicioCola.trabajos.PosponerPublicacion(contexto, ejecucion.TrabajoID); errorObtenido != nil {
			slog.Error("scraper_outbox_error", "trabajo_id", ejecucion.TrabajoID, "error", errorObtenido)
		}
	}
}
