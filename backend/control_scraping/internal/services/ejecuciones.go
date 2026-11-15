package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/google/uuid"
	"log/slog"
)

type QueueService struct {
	jobs      domain.EjecucionRepository
	publisher domain.JobPublisher
}

func NewQueueService(jobs domain.EjecucionRepository, publisher domain.JobPublisher) *QueueService {
	return &QueueService{jobs, publisher}
}
func (servicioCola *QueueService) Encolar(ctx context.Context, id string, payload []byte) error {
	jobID, err := uuid.Parse(id)
	if err != nil {
		return ErrUUIDInvalido
	}
	hash := sha256.Sum256(payload)
	ejecucion := domain.Ejecucion{TrabajoID: jobID, PayloadHash: hex.EncodeToString(hash[:]), Payload: string(payload)}
	if err := servicioCola.jobs.RegistrarEjecucion(ctx, ejecucion); err != nil {
		return err
	}
	// La aceptación depende sólo de SQL; Redis nunca bloquea el HTTP público.
	slog.Info("scraper_execution_accepted", "trabajo_id", id)
	return nil
}
func (servicioCola *QueueService) Recuperar(ctx context.Context) {
	entries, err := servicioCola.jobs.Pendientes(ctx)
	if err != nil {
		slog.Error("scraper_outbox_error", "error", err)
		return
	}
	for _, ejecucion := range entries {
		if err := servicioCola.publisher.Publicar(ctx, ejecucion); err != nil {
			slog.Warn("scraper_publicacion_pendiente", "trabajo_id", ejecucion.TrabajoID)
			continue
		}
		if err := servicioCola.jobs.PosponerPublicacion(ctx, ejecucion.TrabajoID); err != nil {
			slog.Error("scraper_outbox_error", "trabajo_id", ejecucion.TrabajoID, "error", err)
		}
	}
}
