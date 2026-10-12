package domain

import (
	"context"
	"github.com/google/uuid"
)

// Payload y resultado son documentos opacos al adaptador SQL de coordinación.
type Entrega struct {
	ID          uuid.UUID
	TrabajoID   *uuid.UUID
	CadenaID    int
	SucursalID  *int
	PayloadHash string
	Payload     string
	Estado      string
	Intentos    int
	LeaseToken  *uuid.UUID
	Resultado   *string
}

func (Entrega) TableName() string { return "scraper.entregas_ingesta" }

type EntregaRepository interface {
	Registrar(context.Context, *Entrega) (*Entrega, error)
	Obtener(context.Context, uuid.UUID) (*Entrega, error)
	Adquirir(context.Context, *uuid.UUID) (*Entrega, error)
	FijarSucursal(context.Context, uuid.UUID, int) (int, error)
	Confirmar(context.Context, *Entrega, IngestaResultadoDTO) error
	Reprogramar(context.Context, *Entrega, string, bool) error
}

type loteIDKey struct{}

func ConLoteID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, loteIDKey{}, id)
}
func LoteID(ctx context.Context) string { id, _ := ctx.Value(loteIDKey{}).(string); return id }

type Ejecucion struct {
	TrabajoID   uuid.UUID
	PayloadHash string
	Payload     string
}
type EjecucionRepository interface {
	RegistrarEjecucion(context.Context, Ejecucion) error
	Pendientes(context.Context) ([]Ejecucion, error)
	PosponerPublicacion(context.Context, uuid.UUID) error
}
type JobPublisher interface {
	Publicar(context.Context, Ejecucion) error
}
