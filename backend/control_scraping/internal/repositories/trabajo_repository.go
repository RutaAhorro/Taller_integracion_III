package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gormTrabajoRepository struct{ db *gorm.DB }

func NewTrabajoRepository(db *gorm.DB) domain.TrabajoRepository {
	return &gormTrabajoRepository{db: db}
}

// CrearTrabajo inserta una nueva ejecución en scraper.trabajos_scraper
func (r *gormTrabajoRepository) CrearTrabajo(ctx context.Context, trabajo *domain.TrabajoScraper) error {
	return r.db.WithContext(ctx).Create(trabajo).Error
}

// ObtenerTrabajoPorID lee únicamente el trabajo y su snapshot de auditoría.
func (r *gormTrabajoRepository) ObtenerTrabajoPorID(ctx context.Context, id uuid.UUID) (*domain.TrabajoScraper, error) {
	var trabajo domain.TrabajoScraper
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&trabajo).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &trabajo, nil
}

// IncrementarElementos mantiene las métricas bajo propiedad del coordinador.
// Se ejecuta sólo después de confirmar el commit de Catálogo.
func (r *gormTrabajoRepository) IncrementarElementos(ctx context.Context, id uuid.UUID, cantidad int) error {
	result := r.db.WithContext(ctx).Model(&domain.TrabajoScraper{}).
		Where("id = ?", id).Update("elementos_extraidos", gorm.Expr("elementos_extraidos + ?", cantidad))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("no se pudo actualizar el contador del trabajo")
	}
	return nil
}

// FinalizarTrabajo actualiza el estado, fecha fin, items extraídos y posibles errores
func (r *gormTrabajoRepository) FinalizarTrabajo(ctx context.Context, id uuid.UUID, estado string, elementosExtraidos *int, respuestasBloqueadas *int, registroErrores *string) (*domain.TrabajoScraper, error) {
	var trabajo domain.TrabajoScraper
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&trabajo, "id = ?", id).Error; err != nil {
			return err
		}
		if trabajo.Estado == estado {
			return tx.Exec(`DELETE FROM scraper.entregas_trabajos WHERE trabajo_id=?`, id).Error
		}
		if trabajo.Estado != "en_progreso" {
			return services.ErrTrabajoYaFinalizado
		}
		if estado == "completado" {
			var failed, pending int64
			if err := tx.Table("scraper.entregas_ingesta").Where("trabajo_id=? AND estado='fallida'", id).Count(&failed).Error; err != nil {
				return err
			}
			if failed > 0 {
				return services.ErrEntregaFallida
			}
			if err := tx.Table("scraper.entregas_ingesta").Where("trabajo_id=? AND estado='pendiente'", id).Count(&pending).Error; err != nil {
				return err
			}
			if pending > 0 {
				return services.ErrEntregaPendiente
			}
		}
		// Un cierre fallido no espera los backoffs ni cancela entregas ambiguas:
		// su ledger sigue conciliándose, incluso si Catálogo ya hizo commit.
		var managed int64
		if err := tx.Table("scraper.entregas_ingesta").Where("trabajo_id=?", id).Count(&managed).Error; err != nil {
			return err
		}
		updates := map[string]any{"estado": estado, "finalizado_el": time.Now()}
		// Un worker no puede sobrescribir el contador del ledger durable.
		if managed == 0 && elementosExtraidos != nil {
			updates["elementos_extraidos"] = *elementosExtraidos
		}
		if respuestasBloqueadas != nil {
			updates["respuestas_bloqueadas"] = *respuestasBloqueadas
		}
		if registroErrores != nil {
			updates["registro_errores"] = *registroErrores
		}
		if err := tx.Model(&trabajo).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Exec(`DELETE FROM scraper.entregas_trabajos WHERE trabajo_id=?`, id).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.ObtenerTrabajoPorID(ctx, id)
}
