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

// ===== Persistencia del ciclo de vida de trabajos =====

// gormTrabajoRepository implementa la persistencia SQL del ciclo de vida de trabajos.
type gormTrabajoRepository struct{ baseDatos *gorm.DB }

// NewTrabajoRepository crea el repositorio GORM de trabajos de scraping.
func NewTrabajoRepository(baseDatos *gorm.DB) domain.TrabajoRepository {
	return &gormTrabajoRepository{baseDatos: baseDatos}
}

// CrearTrabajo inserta una nueva ejecución en scraper.trabajos_scraper
func (repositorio *gormTrabajoRepository) CrearTrabajo(contexto context.Context, trabajo *domain.TrabajoScraper) error {
	return repositorio.baseDatos.WithContext(contexto).Create(trabajo).Error
}

// ObtenerTrabajoPorID lee únicamente el trabajo y su snapshot de auditoría.
func (repositorio *gormTrabajoRepository) ObtenerTrabajoPorID(contexto context.Context, identificador uuid.UUID) (*domain.TrabajoScraper, error) {
	var trabajo domain.TrabajoScraper
	errorObtenido := repositorio.baseDatos.WithContext(contexto).
		Where("id = ?", identificador).
		First(&trabajo).Error
	if errorObtenido != nil {
		if errors.Is(errorObtenido, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, errorObtenido
	}
	return &trabajo, nil
}

// IncrementarElementos mantiene las métricas bajo propiedad del coordinador.
// Se ejecuta sólo después de confirmar el commit de Catálogo.
func (repositorio *gormTrabajoRepository) IncrementarElementos(contexto context.Context, identificador uuid.UUID, cantidad int) error {
	resultado := repositorio.baseDatos.WithContext(contexto).Model(&domain.TrabajoScraper{}).
		Where("id = ?", identificador).Update("elementos_extraidos", gorm.Expr("elementos_extraidos + ?", cantidad))
	if resultado.Error != nil {
		return resultado.Error
	}
	if resultado.RowsAffected != 1 {
		return errors.New("no se pudo actualizar el contador del trabajo")
	}
	return nil
}

// FinalizarTrabajo actualiza el estado, fecha fin, items extraídos y posibles errores
func (repositorio *gormTrabajoRepository) FinalizarTrabajo(contexto context.Context, identificador uuid.UUID, estado string, elementosExtraidos *int, respuestasBloqueadas *int, registroErrores *string) (*domain.TrabajoScraper, error) {
	var trabajo domain.TrabajoScraper
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		if errorObtenido := transaccion.Clauses(clause.Locking{Strength: "UPDATE"}).First(&trabajo, "id = ?", identificador).Error; errorObtenido != nil {
			return errorObtenido
		}
		if trabajo.Estado == estado {
			return transaccion.Exec(`DELETE FROM scraper.entregas_trabajos WHERE trabajo_id=?`, identificador).Error
		}
		if trabajo.Estado != "en_progreso" {
			return services.ErrTrabajoYaFinalizado
		}
		if estado == "completado" {
			var fallidos, pendientes int64
			if errorObtenido := transaccion.Table("scraper.entregas_ingesta").Where("trabajo_id=? AND estado='fallida'", identificador).Count(&fallidos).Error; errorObtenido != nil {
				return errorObtenido
			}
			if fallidos > 0 {
				return services.ErrEntregaFallida
			}
			if errorObtenido := transaccion.Table("scraper.entregas_ingesta").Where("trabajo_id=? AND estado='pendiente'", identificador).Count(&pendientes).Error; errorObtenido != nil {
				return errorObtenido
			}
			if pendientes > 0 {
				return services.ErrEntregaPendiente
			}
		}
		// Un cierre fallido no espera los backoffs ni cancela entregas ambiguas:
		// su ledger sigue conciliándose, incluso si Catálogo ya hizo commit.
		var gestionados int64
		if errorObtenido := transaccion.Table("scraper.entregas_ingesta").Where("trabajo_id=?", identificador).Count(&gestionados).Error; errorObtenido != nil {
			return errorObtenido
		}
		actualizaciones := map[string]any{"estado": estado, "finalizado_el": time.Now()}
		// Un worker no puede sobrescribir el contador del ledger durable.
		if gestionados == 0 && elementosExtraidos != nil {
			actualizaciones["elementos_extraidos"] = *elementosExtraidos
		}
		if respuestasBloqueadas != nil {
			actualizaciones["respuestas_bloqueadas"] = *respuestasBloqueadas
		}
		if registroErrores != nil {
			actualizaciones["registro_errores"] = *registroErrores
		}
		if errorObtenido := transaccion.Model(&trabajo).Updates(actualizaciones).Error; errorObtenido != nil {
			return errorObtenido
		}
		return transaccion.Exec(`DELETE FROM scraper.entregas_trabajos WHERE trabajo_id=?`, identificador).Error
	})
	if errors.Is(errorObtenido, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	return repositorio.ObtenerTrabajoPorID(contexto, identificador)
}
