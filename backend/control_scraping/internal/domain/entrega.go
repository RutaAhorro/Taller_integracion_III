package domain

import (
	"context"

	"github.com/google/uuid"
)

// ===== Entregas durables =====

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

// TableName fija la tabla de Catálogo donde Control guarda entregas de ingesta.
func (Entrega) TableName() string { return "scraper.entregas_ingesta" }

// ===== Contratos de persistencia y publicación =====

// EntregaRepository define la persistencia y el ciclo de vida de lotes durables.
type EntregaRepository interface {
	Registrar(context.Context, *Entrega) (*Entrega, error)
	Obtener(context.Context, uuid.UUID) (*Entrega, error)
	Adquirir(context.Context, *uuid.UUID) (*Entrega, error)
	FijarSucursal(context.Context, uuid.UUID, int) (int, error)
	Confirmar(context.Context, *Entrega, IngestaResultadoDTO) error
	Reprogramar(context.Context, *Entrega, string, bool) error
}

// loteIDKey evita colisiones con otras claves guardadas en contextos.
type loteIDKey struct{}

// ConLoteID agrega al contexto el identificador idempotente del lote actual.
func ConLoteID(contexto context.Context, identificador string) context.Context {
	return context.WithValue(contexto, loteIDKey{}, identificador)
}

// LoteID recupera el identificador idempotente adjunto al contexto.
func LoteID(contexto context.Context) string {
	identificador, _ := contexto.Value(loteIDKey{}).(string)
	return identificador
}

// Ejecucion contiene el payload durable que se publicará en la cola.
type Ejecucion struct {
	TrabajoID   uuid.UUID
	PayloadHash string
	Payload     string
}

// EjecucionRepository administra el outbox SQL de trabajos.
type EjecucionRepository interface {
	RegistrarEjecucion(context.Context, Ejecucion) error
	Pendientes(context.Context) ([]Ejecucion, error)
	PosponerPublicacion(context.Context, uuid.UUID) error
}

// JobPublisher publica ejecuciones persistidas en el broker.
type JobPublisher interface {
	Publicar(context.Context, Ejecucion) error
}
