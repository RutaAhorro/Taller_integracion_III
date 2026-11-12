package integration_test

import (
	"context"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
)

type mockProductoRepository struct {
	items      []domain.ProductoDTO
	adminItems []domain.ProductoAdminDTO
	total      int64
	err        error
	lastFilter domain.FiltroProductosDTO
	detalle    *domain.ProductoDetalleDTO
}

func (repositorio *mockProductoRepository) Listar(contexto context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoDTO, int64, error) {
	repositorio.lastFilter = filtro
	if repositorio.err != nil {
		return nil, 0, repositorio.err
	}
	return repositorio.items, repositorio.total, nil
}

func (repositorio *mockProductoRepository) ListarParaAdmin(contexto context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoAdminDTO, int64, error) {
	repositorio.lastFilter = filtro
	if repositorio.err != nil {
		return nil, 0, repositorio.err
	}
	return repositorio.adminItems, repositorio.total, nil
}

func (repositorio *mockProductoRepository) ObtenerPorID(contexto context.Context, identificador string) (*domain.ProductoDetalleDTO, error) {
	if repositorio.err != nil {
		return nil, repositorio.err
	}
	if repositorio.detalle != nil && repositorio.detalle.ID == identificador {
		return repositorio.detalle, nil
	}
	return nil, nil
}

type mockFuenteRepository struct {
	fuentes []domain.FuenteDTO
	err     error
}

func (repositorio *mockFuenteRepository) ListarFuentes(contexto context.Context) ([]domain.FuenteDTO, error) {
	if repositorio.err != nil {
		return nil, repositorio.err
	}
	return repositorio.fuentes, nil
}

func (repositorio *mockFuenteRepository) ObtenerFuente(contexto context.Context, identificador int) (*domain.FuenteDTO, error) {
	if repositorio.err != nil {
		return nil, repositorio.err
	}
	for indice := range repositorio.fuentes {
		if repositorio.fuentes[indice].ID == identificador {
			return &repositorio.fuentes[indice], nil
		}
	}
	return nil, nil
}

func (repositorio *mockFuenteRepository) ActualizarEstadoFuente(contexto context.Context, identificador int, habilitada bool) (*domain.FuenteDTO, error) {
	if repositorio.err != nil {
		return nil, repositorio.err
	}
	for indice := range repositorio.fuentes {
		if repositorio.fuentes[indice].ID == identificador {
			repositorio.fuentes[indice].Habilitada = habilitada
			return &repositorio.fuentes[indice], nil
		}
	}
	return nil, nil
}

// limitesDePrueba replica los valores por defecto de internal/config para las pruebas.
var limitesDePrueba = services.LimitesConsulta{TamanoPaginaPredeterminado: 20, TamanoPaginaMaximo: 100, LargoMaximoBusqueda: 100}

// plazoReadinessDePrueba replica CATALOGO_READINESS_TIMEOUT por defecto.
const plazoReadinessDePrueba = 2 * time.Second

// productosPorLoteDePrueba replica CATALOGO_MAX_BATCH_PRODUCTOS por defecto.
const productosPorLoteDePrueba = 1000
