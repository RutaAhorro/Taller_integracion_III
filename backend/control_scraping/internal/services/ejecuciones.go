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
func (s *QueueService) Encolar(ctx context.Context, id string, payload []byte) error {
	jobID, err := uuid.Parse(id)
	if err != nil {
		return ErrUUIDInvalido
	}
	hash := sha256.Sum256(payload)
	e := domain.Ejecucion{TrabajoID: jobID, PayloadHash: hex.EncodeToString(hash[:]), Payload: string(payload)}
	if err := s.jobs.RegistrarEjecucion(ctx, e); err != nil {
		return err
	}
	// La aceptación depende sólo de SQL; Redis nunca bloquea el HTTP público.
	slog.Info("scraper_execution_accepted", "trabajo_id", id)
	return nil
}
func (s *QueueService) Recuperar(ctx context.Context) {
	entries, err := s.jobs.Pendientes(ctx)
	if err != nil {
		slog.Error("scraper_outbox_error", "error", err)
		return
	}
	for _, e := range entries {
		if err := s.publisher.Publicar(ctx, e); err != nil {
			slog.Warn("scraper_publicacion_pendiente", "trabajo_id", e.TrabajoID)
			continue
		}
		if err := s.jobs.PosponerPublicacion(ctx, e.TrabajoID); err != nil {
			slog.Error("scraper_outbox_error", "trabajo_id", e.TrabajoID, "error", err)
		}
	}
}
