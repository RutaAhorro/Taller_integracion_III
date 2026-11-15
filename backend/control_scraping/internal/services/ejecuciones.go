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
	trabajos   domain.EjecucionRepository
	publicador domain.JobPublisher
}

func NewQueueService(trabajos domain.EjecucionRepository, publicador domain.JobPublisher) *QueueService {
	return &QueueService{trabajos: trabajos, publicador: publicador}
}
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
