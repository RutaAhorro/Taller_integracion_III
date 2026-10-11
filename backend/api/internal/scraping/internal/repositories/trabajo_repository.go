package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping/internal/services"
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

// ObtenerTrabajoPorID busca un trabajo por su UUID e incluye los datos de la cadena
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
func (r *gormTrabajoRepository) FinalizarTrabajo(ctx context.Context, id uuid.UUID, estado string, elementosExtraidos *int, registroErrores *string) (*domain.TrabajoScraper, error) {
	var trabajo domain.TrabajoScraper
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&trabajo, "id = ?", id).Error; err != nil {
			return err
		}
		if trabajo.Estado == estado {
			return nil
		}
		if trabajo.Estado != "en_progreso" {
			return services.ErrTrabajoYaFinalizado
		}
		var blocked int64
		q := tx.Table("scraper.entregas_ingesta").Where("trabajo_id=? AND estado='pendiente'", id)
		if estado == "completado" {
			q = tx.Table("scraper.entregas_ingesta").Where("trabajo_id=? AND estado!='confirmada'", id)
		}
		if err := q.Count(&blocked).Error; err != nil {
			return err
		}
		if blocked > 0 {
			return services.ErrEntregaPendiente
		}
		var managed int64
		if err := tx.Table("scraper.entregas_ingesta").Where("trabajo_id=?", id).Count(&managed).Error; err != nil {
			return err
		}
		updates := map[string]any{"estado": estado, "finalizado_el": time.Now()}
		// Un worker no puede sobrescribir el contador del ledger durable.
		if managed == 0 && elementosExtraidos != nil {
			updates["elementos_extraidos"] = *elementosExtraidos
		}
		if registroErrores != nil {
			updates["registro_errores"] = *registroErrores
		}
		return tx.Model(&trabajo).Updates(updates).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.ObtenerTrabajoPorID(ctx, id)
}
