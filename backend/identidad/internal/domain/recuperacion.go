package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// RecuperacionRepository persiste tokens en api.tokens_recuperacion (SUP-184).
// Las comparaciones de tiempo usan el reloj de PostgreSQL, igual que el consumo (SUP-185).
type RecuperacionRepository interface {
	// SolicitudesRecientes cuenta los tokens emitidos dentro de ventana e indica si
	// alguno tiene menos de espera.
	SolicitudesRecientes(ctx context.Context, usuarioID uuid.UUID, ventana, espera time.Duration) (enVentana int, enEspera bool, err error)
	// EmitirToken invalida los tokens pendientes del usuario y guarda el hash nuevo.
	EmitirToken(ctx context.Context, usuarioID uuid.UUID, hash []byte, vigencia time.Duration) error
}
