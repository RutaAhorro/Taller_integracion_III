package repositories

import (
	"context"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type gormRecuperacionRepository struct {
	db *gorm.DB
}

func NewRecuperacionRepository(db *gorm.DB) domain.RecuperacionRepository {
	return &gormRecuperacionRepository{db: db}
}

func (r *gormRecuperacionRepository) SolicitudesRecientes(ctx context.Context, usuarioID uuid.UUID, ventana, espera time.Duration) (int, bool, error) {
	var fila struct {
		EnVentana int
		EnEspera  bool
	}
	err := r.db.WithContext(ctx).Raw(`SELECT count(*) AS en_ventana,
            coalesce(bool_or(creado_el > now() - make_interval(secs => ?)), false) AS en_espera
        FROM api.tokens_recuperacion
        WHERE usuario_id = ? AND creado_el > now() - make_interval(secs => ?)`,
		espera.Seconds(), usuarioID, ventana.Seconds()).Scan(&fila).Error
	return fila.EnVentana, fila.EnEspera, err
}

func (r *gormRecuperacionRepository) EmitirToken(ctx context.Context, usuarioID uuid.UUID, hash []byte, vigencia time.Duration) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Sólo sirve el enlace más reciente: los anteriores pendientes quedan inutilizados.
		if err := tx.Exec(`UPDATE api.tokens_recuperacion SET usado_el = now() WHERE usuario_id = ? AND usado_el IS NULL`, usuarioID).Error; err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO api.tokens_recuperacion (usuario_id, token_hash, expira_el) VALUES (?, ?, now() + make_interval(secs => ?))`,
			usuarioID, hash, vigencia.Seconds()).Error
	})
}
