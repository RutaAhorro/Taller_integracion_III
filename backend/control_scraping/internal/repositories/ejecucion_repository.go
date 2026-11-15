package repositories

import (
	"context"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ejecucionRepository struct{ baseDatos *gorm.DB }

func NewEjecucionRepository(baseDatos *gorm.DB) domain.EjecucionRepository {
	return &ejecucionRepository{baseDatos}
}
func (repositorio *ejecucionRepository) RegistrarEjecucion(contexto context.Context, ejecucion domain.Ejecucion) error {
	return repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		var trabajo domain.TrabajoScraper
		if errorObtenido := transaccion.Clauses(clause.Locking{Strength: "UPDATE"}).First(&trabajo, "id=?", ejecucion.TrabajoID).Error; errorObtenido != nil {
			if errors.Is(errorObtenido, gorm.ErrRecordNotFound) {
				return services.ErrTrabajoNoEncontrado
			}
			return errorObtenido
		}
		if trabajo.Estado != "en_progreso" {
			return services.ErrTrabajoYaFinalizado
		}
		if errorObtenido := transaccion.Exec(`INSERT INTO scraper.entregas_trabajos (trabajo_id,payload_hash,payload) VALUES (?,?,?::jsonb) ON CONFLICT (trabajo_id) DO NOTHING`, ejecucion.TrabajoID, ejecucion.PayloadHash, ejecucion.Payload).Error; errorObtenido != nil {
			return errorObtenido
		}
		var huella string
		if errorObtenido := transaccion.Raw(`SELECT payload_hash FROM scraper.entregas_trabajos WHERE trabajo_id=?`, ejecucion.TrabajoID).Scan(&huella).Error; errorObtenido != nil {
			return errorObtenido
		}
		if huella != ejecucion.PayloadHash {
			return services.ErrIdempotencia
		}
		return nil
	})
}
func (repositorio *ejecucionRepository) Pendientes(contexto context.Context) ([]domain.Ejecucion, error) {
	// Retirar también outboxes terminales anteriores a este fix, en lotes acotados.
	if errorObtenido := repositorio.baseDatos.WithContext(contexto).Exec(`DELETE FROM scraper.entregas_trabajos WHERE trabajo_id IN (
 SELECT et.trabajo_id FROM scraper.entregas_trabajos et JOIN scraper.trabajos_scraper t ON t.id=et.trabajo_id
 WHERE t.estado IN ('completado','fallido') LIMIT 100)`).Error; errorObtenido != nil {
		return nil, errorObtenido
	}
	var resultado []domain.Ejecucion
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Raw(`SELECT et.trabajo_id,et.payload_hash,et.payload::text FROM scraper.entregas_trabajos et
 JOIN scraper.trabajos_scraper t ON t.id=et.trabajo_id WHERE t.estado='en_progreso'
 AND et.proxima_publicacion<=now() ORDER BY et.proxima_publicacion LIMIT 20`).Scan(&resultado).Error
	return resultado, errorObtenido
}
func (repositorio *ejecucionRepository) PosponerPublicacion(contexto context.Context, identificador uuid.UUID) error {
	return repositorio.baseDatos.WithContext(contexto).Exec(`UPDATE scraper.entregas_trabajos SET proxima_publicacion=now()+interval '30 seconds' WHERE trabajo_id=?`, identificador).Error
}
