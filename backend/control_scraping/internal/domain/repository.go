package domain

import (
	"context"

	"github.com/google/uuid"
)

// ===== Contratos de persistencia y comunicación =====

// TrabajoRepository es la persistencia propia de coordinación de scraping.
type TrabajoRepository interface {
	CrearTrabajo(contexto context.Context, trabajo *TrabajoScraper) error
	ObtenerTrabajoPorID(contexto context.Context, identificador uuid.UUID) (*TrabajoScraper, error)
	FinalizarTrabajo(contexto context.Context, identificador uuid.UUID, estado string, elementosExtraidos *int, respuestasBloqueadas *int, registroErrores *string) (*TrabajoScraper, error)
	IncrementarElementos(contexto context.Context, identificador uuid.UUID, cantidad int) error
}

// IngestaRepository es el contrato consumidor del servicio Catálogo.
type IngestaRepository interface {
	ObtenerCadena(contexto context.Context, cadenaID int, nombreCadena string) (*CadenaSupermercado, error)
	ObtenerSucursal(contexto context.Context, cadenaID int, sucursalID *int, codigoSucursal *string) (*SucursalSupermercado, error)
	IngestarLote(contexto context.Context, trabajoID *uuid.UUID, sucursalID int, productos []ProductoScrapeadoDTO) (*IngestaResultadoDTO, error)
}
