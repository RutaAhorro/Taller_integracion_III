package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type entregaRepository struct{ db *gorm.DB }

func NewEntregaRepository(db *gorm.DB) domain.EntregaRepository { return &entregaRepository{db} }

func (r *entregaRepository) Obtener(ctx context.Context, id uuid.UUID) (*domain.Entrega, error) {
	var e domain.Entrega
	err := r.db.WithContext(ctx).First(&e, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &e, err
}
func (r *entregaRepository) Registrar(ctx context.Context, e *domain.Entrega) (*domain.Entrega, error) {
	var result domain.Entrega
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Orden global: trabajo antes de entrega, igual que FinalizarTrabajo.
		if e.TrabajoID != nil {
			var job domain.TrabajoScraper
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, "id = ?", e.TrabajoID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return services.ErrTrabajoNoEncontrado
				}
				return err
			}
			e.CadenaID = job.CadenaID
			if job.Estado != "en_progreso" {
				if err := tx.First(&result, "id = ?", e.ID).Error; err != nil {
					return services.ErrTrabajoYaFinalizado
				}
				if result.PayloadHash != e.PayloadHash {
					return services.ErrIdempotencia
				}
				return nil
			}
		}
		if err := tx.Exec(`INSERT INTO scraper.entregas_ingesta (id,trabajo_id,cadena_id,payload_hash,payload)
 VALUES (?,?,?,?,?::jsonb) ON CONFLICT (id) DO NOTHING`, e.ID, e.TrabajoID, e.CadenaID, e.PayloadHash, e.Payload).Error; err != nil {
			return err
		}
		if err := tx.First(&result, "id = ?", e.ID).Error; err != nil {
			return err
		}
		if result.PayloadHash != e.PayloadHash {
			return services.ErrIdempotencia
		}
		return nil
	})
	return &result, err
}

// La lease dura más que los tres HTTP internos (máximo 20s cada uno).
// Confirmar es idempotente incluso si un proceso muere y otro toma la lease.
func (r *entregaRepository) Adquirir(ctx context.Context, id *uuid.UUID) (*domain.Entrega, error) {
	var e domain.Entrega
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("estado = 'pendiente' AND proximo_intento <= now() AND (lease_hasta IS NULL OR lease_hasta < now())")
		if id != nil {
			query = query.Where("id = ?", id)
		}
		found := query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Order("proximo_intento").Limit(1).Find(&e)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		token := uuid.New()
		e.LeaseToken = &token
		e.Intentos++
		return tx.Model(&domain.Entrega{}).Where("id = ?", e.ID).Updates(map[string]any{"lease_token": token, "lease_hasta": time.Now().Add(90 * time.Second), "intentos": e.Intentos}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &e, err
}
func (r *entregaRepository) FijarSucursal(ctx context.Context, id uuid.UUID, branch int) (int, error) {
	var row struct{ SucursalID int }
	err := r.db.WithContext(ctx).Raw(`UPDATE scraper.entregas_ingesta SET sucursal_id = COALESCE(sucursal_id,?) WHERE id = ? RETURNING sucursal_id`, branch, id).Scan(&row).Error
	return row.SucursalID, err
}
func (r *entregaRepository) Confirmar(ctx context.Context, e *domain.Entrega, result domain.IngestaResultadoDTO) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e.TrabajoID != nil {
			if err := tx.Exec(`SELECT id FROM scraper.trabajos_scraper WHERE id = ? FOR UPDATE`, e.TrabajoID).Error; err != nil {
				return err
			}
		}
		var current domain.Entrega
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", e.ID).Error; err != nil {
			return err
		}
		if current.Estado == "confirmada" {
			return nil
		}
		if e.TrabajoID != nil {
			if err := tx.Exec(`UPDATE scraper.trabajos_scraper SET elementos_extraidos = elementos_extraidos + ? WHERE id = ?`, result.Insertados+result.Actualizados, e.TrabajoID).Error; err != nil {
				return err
			}
		}
		return tx.Exec(`UPDATE scraper.entregas_ingesta SET estado='confirmada', resultado=?::jsonb,
 confirmado_el=now(), lease_hasta=NULL, lease_token=NULL, ultimo_error=NULL WHERE id=?`, string(encoded), e.ID).Error
	})
}
func (r *entregaRepository) Reprogramar(ctx context.Context, e *domain.Entrega, code string, permanent bool) error {
	state := "pendiente"
	if permanent || e.Intentos >= 10 {
		state = "fallida"
	}
	delay := time.Duration(1<<min(e.Intentos, 8)) * time.Second
	return r.db.WithContext(ctx).Model(&domain.Entrega{}).Where("id = ? AND estado='pendiente' AND lease_token = ?", e.ID, e.LeaseToken).
		Updates(map[string]any{"estado": state, "ultimo_error": code, "proximo_intento": time.Now().Add(delay), "lease_hasta": nil, "lease_token": nil}).Error
}
