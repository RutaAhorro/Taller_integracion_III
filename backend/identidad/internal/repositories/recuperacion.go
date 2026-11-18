// Persistencia de tokens de recuperación en api.tokens_recuperacion.

package repositories

import (
	"context"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// gormRecuperacionRepository implementa domain.RecuperacionRepository.
type gormRecuperacionRepository struct {
	baseDatos *gorm.DB
}

// NewRecuperacionRepository crea el repositorio de tokens sobre la conexión dada.
func NewRecuperacionRepository(baseDatos *gorm.DB) domain.RecuperacionRepository {
	return &gormRecuperacionRepository{baseDatos: baseDatos}
}

// SolicitudesRecientes cuenta los tokens emitidos dentro de la ventana con el
// reloj de PostgreSQL, igual que la expiración.
func (repositorio *gormRecuperacionRepository) SolicitudesRecientes(contexto context.Context, usuarioID uuid.UUID, ventana, espera time.Duration) (int, bool, error) {
	var fila struct {
		EnVentana int
		EnEspera  bool
	}
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Raw(`SELECT count(*) AS en_ventana,
            coalesce(bool_or(creado_el > now() - make_interval(secs => ?)), false) AS en_espera
        FROM api.tokens_recuperacion
        WHERE usuario_id = ? AND creado_el > now() - make_interval(secs => ?)`,
		espera.Seconds(), usuarioID, ventana.Seconds()).Scan(&fila).Error
	return fila.EnVentana, fila.EnEspera, errorObtenido
}

// EmitirToken invalida los tokens pendientes de la cuenta y guarda el hash
// nuevo en una sola transacción.
func (repositorio *gormRecuperacionRepository) EmitirToken(contexto context.Context, usuarioID uuid.UUID, hash []byte, vigencia time.Duration) error {
	return repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		// Sólo sirve el enlace más reciente: los anteriores pendientes quedan inutilizados.
		if errorObtenido := transaccion.Exec(`UPDATE api.tokens_recuperacion SET usado_el = now() WHERE usuario_id = ? AND usado_el IS NULL`, usuarioID).Error; errorObtenido != nil {
			return errorObtenido
		}
		return transaccion.Exec(`INSERT INTO api.tokens_recuperacion (usuario_id, token_hash, expira_el) VALUES (?, ?, now() + make_interval(secs => ?))`,
			usuarioID, hash, vigencia.Seconds()).Error
	})
}
