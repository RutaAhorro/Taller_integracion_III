package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
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
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&trabajo).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	ahora := time.Now()
	updates := map[string]interface{}{
		"estado":        estado,
		"finalizado_el": ahora,
	}
	if elementosExtraidos != nil {
		updates["elementos_extraidos"] = *elementosExtraidos
	}
	if registroErrores != nil && *registroErrores != "" {
		updates["registro_errores"] = *registroErrores
	}

	if err := r.db.WithContext(ctx).Model(&trabajo).Updates(updates).Error; err != nil {
		return nil, err
	}

	return r.ObtenerTrabajoPorID(ctx, id)
}
