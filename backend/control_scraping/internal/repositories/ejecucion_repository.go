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

type ejecucionRepository struct{ db *gorm.DB }

func NewEjecucionRepository(db *gorm.DB) domain.EjecucionRepository { return &ejecucionRepository{db} }
func (r *ejecucionRepository) RegistrarEjecucion(ctx context.Context, e domain.Ejecucion) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job domain.TrabajoScraper
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, "id=?", e.TrabajoID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return services.ErrTrabajoNoEncontrado
			}
			return err
		}
		if job.Estado != "en_progreso" {
			return services.ErrTrabajoYaFinalizado
		}
		if err := tx.Exec(`INSERT INTO scraper.entregas_trabajos (trabajo_id,payload_hash,payload) VALUES (?,?,?::jsonb) ON CONFLICT (trabajo_id) DO NOTHING`, e.TrabajoID, e.PayloadHash, e.Payload).Error; err != nil {
			return err
		}
		var hash string
		if err := tx.Raw(`SELECT payload_hash FROM scraper.entregas_trabajos WHERE trabajo_id=?`, e.TrabajoID).Scan(&hash).Error; err != nil {
			return err
		}
		if hash != e.PayloadHash {
			return services.ErrIdempotencia
		}
		return nil
	})
}
func (r *ejecucionRepository) Pendientes(ctx context.Context) ([]domain.Ejecucion, error) {
	// Retirar también outboxes terminales anteriores a este fix, en lotes acotados.
	if err := r.db.WithContext(ctx).Exec(`DELETE FROM scraper.entregas_trabajos WHERE trabajo_id IN (
 SELECT et.trabajo_id FROM scraper.entregas_trabajos et JOIN scraper.trabajos_scraper t ON t.id=et.trabajo_id
 WHERE t.estado IN ('completado','fallido') LIMIT 100)`).Error; err != nil {
		return nil, err
	}
	var result []domain.Ejecucion
	err := r.db.WithContext(ctx).Raw(`SELECT et.trabajo_id,et.payload_hash,et.payload::text FROM scraper.entregas_trabajos et
 JOIN scraper.trabajos_scraper t ON t.id=et.trabajo_id WHERE t.estado='en_progreso'
 AND et.proxima_publicacion<=now() ORDER BY et.proxima_publicacion LIMIT 20`).Scan(&result).Error
	return result, err
}
func (r *ejecucionRepository) PosponerPublicacion(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Exec(`UPDATE scraper.entregas_trabajos SET proxima_publicacion=now()+interval '30 seconds' WHERE trabajo_id=?`, id).Error
}
