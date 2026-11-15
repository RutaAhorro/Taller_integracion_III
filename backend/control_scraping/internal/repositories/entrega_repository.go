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

type entregaRepository struct{ baseDatos *gorm.DB }

func NewEntregaRepository(baseDatos *gorm.DB) domain.EntregaRepository {
	return &entregaRepository{baseDatos}
}

func (repositorio *entregaRepository) Obtener(contexto context.Context, identificador uuid.UUID) (*domain.Entrega, error) {
	var entrega domain.Entrega
	errorObtenido := repositorio.baseDatos.WithContext(contexto).First(&entrega, "id = ?", identificador).Error
	if errors.Is(errorObtenido, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &entrega, errorObtenido
}
func (repositorio *entregaRepository) Registrar(contexto context.Context, entrega *domain.Entrega) (*domain.Entrega, error) {
	var resultado domain.Entrega
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		// Orden global: trabajo antes de entrega, igual que FinalizarTrabajo.
		if entrega.TrabajoID != nil {
			var trabajo domain.TrabajoScraper
			if errorObtenido := transaccion.Clauses(clause.Locking{Strength: "UPDATE"}).First(&trabajo, "id = ?", entrega.TrabajoID).Error; errorObtenido != nil {
				if errors.Is(errorObtenido, gorm.ErrRecordNotFound) {
					return services.ErrTrabajoNoEncontrado
				}
				return errorObtenido
			}
			entrega.CadenaID = trabajo.CadenaID
			if trabajo.Estado != "en_progreso" {
				if errorObtenido := transaccion.First(&resultado, "id = ?", entrega.ID).Error; errorObtenido != nil {
					return services.ErrTrabajoYaFinalizado
				}
				if resultado.PayloadHash != entrega.PayloadHash {
					return services.ErrIdempotencia
				}
				return nil
			}
		}
		if errorObtenido := transaccion.Exec(`INSERT INTO scraper.entregas_ingesta (id,trabajo_id,cadena_id,payload_hash,payload)
 VALUES (?,?,?,?,?::jsonb) ON CONFLICT (id) DO NOTHING`, entrega.ID, entrega.TrabajoID, entrega.CadenaID, entrega.PayloadHash, entrega.Payload).Error; errorObtenido != nil {
			return errorObtenido
		}
		if errorObtenido := transaccion.First(&resultado, "id = ?", entrega.ID).Error; errorObtenido != nil {
			return errorObtenido
		}
		if resultado.PayloadHash != entrega.PayloadHash {
			return services.ErrIdempotencia
		}
		return nil
	})
	return &resultado, errorObtenido
}

// La lease dura más que los tres HTTP internos (máximo 20s cada uno).
// Confirmar es idempotente incluso si un proceso muere y otro toma la lease.
func (repositorio *entregaRepository) Adquirir(contexto context.Context, identificador *uuid.UUID) (*domain.Entrega, error) {
	var entrega domain.Entrega
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		consulta := transaccion.Where("estado = 'pendiente' AND proximo_intento <= now() AND (lease_hasta IS NULL OR lease_hasta < now())")
		if identificador != nil {
			consulta = consulta.Where("id = ?", identificador)
		}
		encontrado := consulta.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Order("proximo_intento").Limit(1).Find(&entrega)
		if encontrado.Error != nil {
			return encontrado.Error
		}
		if encontrado.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		token := uuid.New()
		entrega.LeaseToken = &token
		entrega.Intentos++
		return transaccion.Model(&domain.Entrega{}).Where("id = ?", entrega.ID).Updates(map[string]any{"lease_token": token, "lease_hasta": time.Now().Add(90 * time.Second), "intentos": entrega.Intentos}).Error
	})
	if errors.Is(errorObtenido, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &entrega, errorObtenido
}
func (repositorio *entregaRepository) FijarSucursal(contexto context.Context, identificador uuid.UUID, sucursal int) (int, error) {
	var fila struct{ SucursalID int }
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Raw(`UPDATE scraper.entregas_ingesta SET sucursal_id = COALESCE(sucursal_id,?) WHERE id = ? RETURNING sucursal_id`, sucursal, identificador).Scan(&fila).Error
	return fila.SucursalID, errorObtenido
}
func (repositorio *entregaRepository) Confirmar(contexto context.Context, entrega *domain.Entrega, resultado domain.IngestaResultadoDTO) error {
	codificado, errorObtenido := json.Marshal(resultado)
	if errorObtenido != nil {
		return errorObtenido
	}
	return repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		if entrega.TrabajoID != nil {
			if errorObtenido := transaccion.Exec(`SELECT id FROM scraper.trabajos_scraper WHERE id = ? FOR UPDATE`, entrega.TrabajoID).Error; errorObtenido != nil {
				return errorObtenido
			}
		}
		var actual domain.Entrega
		if errorObtenido := transaccion.Clauses(clause.Locking{Strength: "UPDATE"}).First(&actual, "id = ?", entrega.ID).Error; errorObtenido != nil {
			return errorObtenido
		}
		if actual.Estado == "confirmada" {
			return nil
		}
		if entrega.TrabajoID != nil {
			if errorObtenido := transaccion.Exec(`UPDATE scraper.trabajos_scraper SET elementos_extraidos = elementos_extraidos + ? WHERE id = ?`, resultado.Insertados+resultado.Actualizados, entrega.TrabajoID).Error; errorObtenido != nil {
				return errorObtenido
			}
		}
		return transaccion.Exec(`UPDATE scraper.entregas_ingesta SET estado='confirmada', resultado=?::jsonb,
 confirmado_el=now(), lease_hasta=NULL, lease_token=NULL, ultimo_error=NULL WHERE id=?`, string(codificado), entrega.ID).Error
	})
}
func (repositorio *entregaRepository) Reprogramar(contexto context.Context, entrega *domain.Entrega, codigo string, permanente bool) error {
	estado := "pendiente"
	if permanente || entrega.Intentos >= 10 {
		estado = "fallida"
	}
	retraso := time.Duration(1<<min(entrega.Intentos, 8)) * time.Second
	return repositorio.baseDatos.WithContext(contexto).Model(&domain.Entrega{}).Where("id = ? AND estado='pendiente' AND lease_token = ?", entrega.ID, entrega.LeaseToken).
		Updates(map[string]any{"estado": estado, "ultimo_error": codigo, "proximo_intento": time.Now().Add(retraso), "lease_hasta": nil, "lease_token": nil}).Error
}
