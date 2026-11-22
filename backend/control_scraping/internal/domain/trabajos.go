// Modelos y contratos del ciclo de vida de un trabajo de scraping y de su
// ejecución durable (outbox SQL publicado en Redis Streams).
package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ===== Estados de un trabajo =====

// Estados persistidos en scraper.trabajos_scraper.estado. Control crea siempre
// los trabajos en progreso; el worker los cierra como completados o fallidos.
const (
	EstadoEnProgreso = "en_progreso"
	EstadoCompletado = "completado"
	EstadoFallido    = "fallido"
)

// TrabajoTerminado indica si un estado ya no admite lotes ni ejecuciones.
func TrabajoTerminado(estado string) bool {
	return estado == EstadoCompletado || estado == EstadoFallido
}

// ===== Modelo persistido =====

// TrabajoScraper mapea scraper.trabajos_scraper. CadenaNombre es la instantánea
// tomada de Catálogo al crear el trabajo, para auditar sin consultas cruzadas.
type TrabajoScraper struct {
	ID                    uuid.UUID `gorm:"type:uuid;primaryKey"`
	CadenaID              int
	CadenaNombre          string
	DisparadoPorUsuarioID *uuid.UUID `gorm:"type:uuid"`
	Estado                string
	IniciadoEl            time.Time
	FinalizadoEl          *time.Time
	ElementosExtraidos    int
	RespuestasBloqueadas  int
	RegistroErrores       *string
}

// TableName vincula el trabajo con su tabla de coordinación.
func (TrabajoScraper) TableName() string { return "scraper.trabajos_scraper" }

// ===== Contratos HTTP de trabajos =====

// IniciarTrabajoDTO es el cuerpo de POST /api/v1/scraper/trabajos.
type IniciarTrabajoDTO struct {
	CadenaID              int     `json:"cadena_id" binding:"required"`
	Supermercado          string  `json:"supermercado,omitempty"`
	DisparadoPorUsuarioID *string `json:"disparado_por_usuario_id,omitempty"`
}

// TrabajoScraperDTO es la respuesta de los endpoints de trabajos; la consumen
// el bot de Discord y Scraper Go.
type TrabajoScraperDTO struct {
	ID                   string     `json:"id"`
	CadenaID             int        `json:"cadena_id"`
	CadenaNombre         string     `json:"cadena_nombre,omitempty"`
	Estado               string     `json:"estado"`
	IniciadoEl           time.Time  `json:"iniciado_el"`
	FinalizadoEl         *time.Time `json:"finalizado_el,omitempty"`
	DuracionSegundos     *float64   `json:"duracion_segundos,omitempty"`
	ElementosExtraidos   int        `json:"elementos_extraidos"`
	RespuestasBloqueadas int        `json:"respuestas_bloqueadas"`
	RegistroErrores      *string    `json:"registro_errores,omitempty"`
}

// FinalizarTrabajoDTO es el cuerpo de PUT /api/v1/scraper/trabajos/:id/finalizar.
type FinalizarTrabajoDTO struct {
	Estado               string  `json:"estado" binding:"required"`
	ElementosExtraidos   *int    `json:"elementos_extraidos,omitempty"`
	RespuestasBloqueadas *int    `json:"respuestas_bloqueadas,omitempty"`
	RegistroErrores      *string `json:"registro_errores,omitempty"`
}

// ===== Contratos de persistencia de trabajos =====

// TrabajoRepository persiste el ciclo de vida propio de coordinación.
type TrabajoRepository interface {
	CrearTrabajo(contexto context.Context, trabajo *TrabajoScraper) error
	ObtenerTrabajoPorID(contexto context.Context, identificador uuid.UUID) (*TrabajoScraper, error)
	FinalizarTrabajo(contexto context.Context, identificador uuid.UUID, estado string, elementosExtraidos *int, respuestasBloqueadas *int, registroErrores *string) (*TrabajoScraper, error)
}

// ===== Ejecución durable =====

// Ejecucion es el payload guardado en el outbox SQL que se publicará en Redis.
type Ejecucion struct {
	TrabajoID   uuid.UUID
	PayloadHash string
	Payload     string
}

// EjecucionRepository administra el outbox SQL de ejecuciones.
type EjecucionRepository interface {
	RegistrarEjecucion(contexto context.Context, ejecucion Ejecucion) error
	Pendientes(contexto context.Context) ([]Ejecucion, error)
	PosponerPublicacion(contexto context.Context, trabajoID uuid.UUID) error
}

// PublicadorTrabajos entrega al broker una ejecución ya persistida. Se separa
// del outbox porque Redis nunca debe bloquear una operación SQL.
type PublicadorTrabajos interface {
	Publicar(contexto context.Context, ejecucion Ejecucion) error
}
