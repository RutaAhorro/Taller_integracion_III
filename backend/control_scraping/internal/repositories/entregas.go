// Persistencia SQL de las entregas durables de lotes en scraper.entregas_ingesta:
// registro idempotente, leases, confirmación del contador y reintentos.
package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ===== Leases y reintentos =====

const (
	// duracionLease supera las tres llamadas HTTP internas (máximo 20 s cada una).
	duracionLease = 90 * time.Second
	// maximoIntentosEntrega marca la entrega como fallida para revisión manual.
	maximoIntentosEntrega = 10
	// exponenteMaximoBackoff acota el retraso exponencial en 2^8 = 256 s.
	exponenteMaximoBackoff = 8
)

// ===== Repositorio de entregas =====

// repositorioEntregas implementa domain.EntregaRepository con GORM.
type repositorioEntregas struct{ baseDatos *gorm.DB }

// NuevoRepositorioEntregas crea el repositorio SQL de lotes recibidos.
func NuevoRepositorioEntregas(baseDatos *gorm.DB) domain.EntregaRepository {
	return &repositorioEntregas{baseDatos: baseDatos}
}

// Registrar inserta una entrega una sola vez y valida la huella de su contenido.
// Si el lote pertenece a un trabajo, bloquea primero el trabajo: orden global
// trabajo → entrega, igual que FinalizarTrabajo y Confirmar.
func (repositorio *repositorioEntregas) Registrar(contexto context.Context, entrega *domain.Entrega) (*domain.Entrega, error) {
	var resultado domain.Entrega
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		if entrega.TrabajoID != nil {
			var trabajo domain.TrabajoScraper
			if errorObtenido := transaccion.Clauses(clause.Locking{Strength: "UPDATE"}).First(&trabajo, "id = ?", entrega.TrabajoID).Error; errorObtenido != nil {
				if errors.Is(errorObtenido, gorm.ErrRecordNotFound) {
					return domain.ErrTrabajoNoEncontrado
				}
				return errorObtenido
			}
			entrega.CadenaID = trabajo.CadenaID
			// Un trabajo cerrado sólo admite reenviar un lote ya registrado.
			if trabajo.Estado != domain.EstadoEnProgreso {
				if errorObtenido := transaccion.First(&resultado, "id = ?", entrega.ID).Error; errorObtenido != nil {
					return domain.ErrTrabajoYaFinalizado
				}
				if resultado.PayloadHash != entrega.PayloadHash {
					return domain.ErrIdempotencia
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
			return domain.ErrIdempotencia
		}
		return nil
	})
	return &resultado, errorObtenido
}

// Adquirir toma una lease sobre una entrega pendiente (la indicada o la más
// antigua). SKIP LOCKED permite varias réplicas; nil indica que no hay ninguna.
func (repositorio *repositorioEntregas) Adquirir(contexto context.Context, identificador *uuid.UUID) (*domain.Entrega, error) {
	var entrega domain.Entrega
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		consulta := transaccion.Where("estado = ? AND proximo_intento <= now() AND (lease_hasta IS NULL OR lease_hasta < now())", domain.EntregaPendiente)
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
		return transaccion.Model(&domain.Entrega{}).Where("id = ?", entrega.ID).
			Updates(map[string]any{"lease_token": token, "lease_hasta": time.Now().Add(duracionLease), "intentos": entrega.Intentos}).Error
	})
	if errors.Is(errorObtenido, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &entrega, errorObtenido
}

// FijarSucursal persiste la primera sucursal resuelta: un reintento nunca
// cambia el destino de un lote que Catálogo pudo haber confirmado.
func (repositorio *repositorioEntregas) FijarSucursal(contexto context.Context, identificador uuid.UUID, sucursalID int) (int, error) {
	var fila struct{ SucursalID int }
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Raw(`UPDATE scraper.entregas_ingesta SET sucursal_id = COALESCE(sucursal_id,?) WHERE id = ? RETURNING sucursal_id`, sucursalID, identificador).Scan(&fila).Error
	return fila.SucursalID, errorObtenido
}

// Confirmar guarda el resultado y suma el contador del trabajo una sola vez,
// aunque otro proceso haya tomado la lease después de una caída.
func (repositorio *repositorioEntregas) Confirmar(contexto context.Context, entrega *domain.Entrega, resultado domain.IngestaResultadoDTO) error {
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
		if actual.Estado == domain.EntregaConfirmada {
			return nil
		}
		if entrega.TrabajoID != nil {
			if errorObtenido := transaccion.Exec(`UPDATE scraper.trabajos_scraper SET elementos_extraidos = elementos_extraidos + ? WHERE id = ?`, resultado.Insertados+resultado.Actualizados, entrega.TrabajoID).Error; errorObtenido != nil {
				return errorObtenido
			}
		}
		return transaccion.Exec(`UPDATE scraper.entregas_ingesta SET estado=?, resultado=?::jsonb,
 confirmado_el=now(), lease_hasta=NULL, lease_token=NULL, ultimo_error=NULL WHERE id=?`, domain.EntregaConfirmada, string(codificado), entrega.ID).Error
	})
}

// Reprogramar libera la lease y fija el siguiente intento con backoff
// exponencial, o el estado terminal si el fallo es permanente o se agotaron
// los intentos. Sólo actúa si la lease sigue siendo de este proceso.
func (repositorio *repositorioEntregas) Reprogramar(contexto context.Context, entrega *domain.Entrega, codigo string, permanente bool) error {
	estado := domain.EntregaPendiente
	if permanente || entrega.Intentos >= maximoIntentosEntrega {
		estado = domain.EntregaFallida
	}
	retraso := time.Duration(1<<min(entrega.Intentos, exponenteMaximoBackoff)) * time.Second
	return repositorio.baseDatos.WithContext(contexto).Model(&domain.Entrega{}).
		Where("id = ? AND estado = ? AND lease_token = ?", entrega.ID, domain.EntregaPendiente, entrega.LeaseToken).
		Updates(map[string]any{"estado": estado, "ultimo_error": codigo, "proximo_intento": time.Now().Add(retraso), "lease_hasta": nil, "lease_token": nil}).Error
}
