package domain

import "context"

// FuenteRepository es el contrato de persistencia de las fuentes de scraping
// (scraper.cadenas_supermercado), tabla propiedad de Catálogo.
type FuenteRepository interface {
	ListarFuentes(ctx context.Context) ([]FuenteDTO, error)
	ObtenerFuente(ctx context.Context, id int) (*FuenteDTO, error)
	ActualizarEstadoFuente(ctx context.Context, id int, habilitada bool) (*FuenteDTO, error)
}
