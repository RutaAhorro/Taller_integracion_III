package domain

import "context"

// ProductoRepository define el contrato para operaciones sobre el catálogo de productos
type ProductoRepository interface {
	Listar(ctx context.Context, filtro FiltroProductosDTO) ([]ProductoDTO, int64, error)
	ListarParaAdmin(ctx context.Context, filtro FiltroProductosDTO) ([]ProductoAdminDTO, int64, error)
	ObtenerPorID(ctx context.Context, id string) (*ProductoDetalleDTO, error)
}
