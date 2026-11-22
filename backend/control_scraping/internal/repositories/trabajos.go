// Persistencia SQL del ciclo de vida de los trabajos en scraper.trabajos_scraper.
package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ===== Repositorio de trabajos =====

// repositorioTrabajos implementa domain.TrabajoRepository con GORM.
type repositorioTrabajos struct{ baseDatos *gorm.DB }

// NuevoRepositorioTrabajos crea el repositorio SQL de trabajos.
func NuevoRepositorioTrabajos(baseDatos *gorm.DB) domain.TrabajoRepository {
	return &repositorioTrabajos{baseDatos: baseDatos}
}

// CrearTrabajo inserta una nueva ejecución en scraper.trabajos_scraper.
func (repositorio *repositorioTrabajos) CrearTrabajo(contexto context.Context, trabajo *domain.TrabajoScraper) error {
	return repositorio.baseDatos.WithContext(contexto).Create(trabajo).Error
}

// ObtenerTrabajoPorID lee el trabajo y su instantánea de auditoría; nil si no existe.
func (repositorio *repositorioTrabajos) ObtenerTrabajoPorID(contexto context.Context, identificador uuid.UUID) (*domain.TrabajoScraper, error) {
	var trabajo domain.TrabajoScraper
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Where("id = ?", identificador).First(&trabajo).Error
	if errors.Is(errorObtenido, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	return &trabajo, nil
}

// FinalizarTrabajo cierra el trabajo y retira su outbox en una transacción.
// Bloquea el trabajo antes de leer entregas (mismo orden que Registrar y
// Confirmar) para no cerrar con lotes en curso.
func (repositorio *repositorioTrabajos) FinalizarTrabajo(contexto context.Context, identificador uuid.UUID, estado string, elementosExtraidos *int, respuestasBloqueadas *int, registroErrores *string) (*domain.TrabajoScraper, error) {
	var trabajo domain.TrabajoScraper
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		if errorObtenido := transaccion.Clauses(clause.Locking{Strength: "UPDATE"}).First(&trabajo, "id = ?", identificador).Error; errorObtenido != nil {
			return errorObtenido
		}
		if trabajo.Estado == estado {
			return transaccion.Exec(`DELETE FROM scraper.entregas_trabajos WHERE trabajo_id=?`, identificador).Error
		}
		if trabajo.Estado != domain.EstadoEnProgreso {
			return domain.ErrTrabajoYaFinalizado
		}
		if estado == domain.EstadoCompletado {
			var fallidas, pendientes int64
			if errorObtenido := transaccion.Table("scraper.entregas_ingesta").Where("trabajo_id=? AND estado=?", identificador, domain.EntregaFallida).Count(&fallidas).Error; errorObtenido != nil {
				return errorObtenido
			}
			if fallidas > 0 {
				return domain.ErrEntregaFallida
			}
			if errorObtenido := transaccion.Table("scraper.entregas_ingesta").Where("trabajo_id=? AND estado=?", identificador, domain.EntregaPendiente).Count(&pendientes).Error; errorObtenido != nil {
				return errorObtenido
			}
			if pendientes > 0 {
				return domain.ErrEntregaPendiente
			}
		}
		// Un cierre fallido no espera los backoffs ni cancela entregas ambiguas:
		// su ledger sigue conciliándose, incluso si Catálogo ya hizo commit.
		var gestionadas int64
		if errorObtenido := transaccion.Table("scraper.entregas_ingesta").Where("trabajo_id=?", identificador).Count(&gestionadas).Error; errorObtenido != nil {
			return errorObtenido
		}
		actualizaciones := map[string]any{"estado": estado, "finalizado_el": time.Now()}
		// Con entregas durables el contador lo lleva el ledger; el worker no lo sobrescribe.
		if gestionadas == 0 && elementosExtraidos != nil {
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
