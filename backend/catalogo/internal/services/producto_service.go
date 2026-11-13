package services

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/utils"
)

// Errores del servicio de productos
var (
	ErrProductoNoEncontrado = errors.New("producto no encontrado")
	ErrBusquedaCorta        = errors.New("el término de búsqueda debe tener al menos 3 caracteres")
)

// ProductoService define el contrato para la lógica de negocio del catálogo de productos
type ProductoService interface {
	ObtenerCatalogo(ctx context.Context, filtro domain.FiltroProductosDTO) (*domain.PaginaProductosDTO, error)
	ObtenerCatalogoAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) (*domain.PaginaProductosAdminDTO, error)
	ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error)
}

// LimitesConsulta agrupa los límites configurables de las consultas de productos.
// Se leen una sola vez en internal/config y se inyectan al crear el servicio.
type LimitesConsulta struct {
	TamanoPaginaPredeterminado int // limit cuando el cliente no lo envía
	TamanoPaginaMaximo         int // tope de limit
	LargoMaximoBusqueda        int // bytes conservados del texto de búsqueda
}

type productoService struct {
	repo    domain.ProductoRepository
	limites LimitesConsulta
}

// NewProductoService instancia el servicio inyectando el repositorio de productos
// y los límites de consulta configurados.
func NewProductoService(repo domain.ProductoRepository, limites LimitesConsulta) ProductoService {
	return &productoService{repo: repo, limites: limites}
}

// ObtenerCatalogo valida parámetros, aplica valores por defecto y sanitización, y obtiene los productos paginados
func (servicio *productoService) ObtenerCatalogo(ctx context.Context, filtro domain.FiltroProductosDTO) (*domain.PaginaProductosDTO, error) {
	// 1. Validación y sanitización del término de búsqueda general
	if strings.TrimSpace(filtro.Query) != "" {
		filtro.Query = utils.SanitizarInputBusqueda(filtro.Query, servicio.limites.LargoMaximoBusqueda)
		if len(filtro.Query) < 3 {
			return nil, ErrBusquedaCorta
		}
	}

	// 2. Valores por defecto para paginación
	if filtro.Page < 1 {
		filtro.Page = 1
	}
	if filtro.Limit < 1 {
		filtro.Limit = servicio.limites.TamanoPaginaPredeterminado
	} else if filtro.Limit > servicio.limites.TamanoPaginaMaximo {
		filtro.Limit = servicio.limites.TamanoPaginaMaximo
	}

	// 3. Valores por defecto para ordenamiento
	filtro.SortBy = strings.ToLower(strings.TrimSpace(filtro.SortBy))
	if filtro.SortBy != "precio" && filtro.SortBy != "nombre" && filtro.SortBy != "marca" {
		filtro.SortBy = "precio"
	}

	filtro.Order = strings.ToLower(strings.TrimSpace(filtro.Order))
	if filtro.Order != "asc" && filtro.Order != "desc" {
		filtro.Order = "asc"
	}

	// 4. Sanitización de textos para prevenir ataques XSS o inyecciones
	if filtro.Categoria != "" {
		filtro.Categoria = utils.SanitizarInputBusqueda(filtro.Categoria, servicio.limites.LargoMaximoBusqueda)
	}
	if filtro.Supermercado != "" {
		filtro.Supermercado = utils.SanitizarInputBusqueda(filtro.Supermercado, servicio.limites.LargoMaximoBusqueda)
	}
	if filtro.Marca != "" {
		filtro.Marca = utils.SanitizarInputBusqueda(filtro.Marca, servicio.limites.LargoMaximoBusqueda)
	}

	// 5. Invocar repositorio
	productos, totalRegistros, err := servicio.repo.Listar(ctx, filtro)
	if err != nil {
		return nil, err
	}

	// 6. Cálculo de metadatos de paginación
	totalPaginas := 0
	if totalRegistros > 0 {
		totalPaginas = int(math.Ceil(float64(totalRegistros) / float64(filtro.Limit)))
	}

	paginacion := domain.MetadatosPaginacionDTO{
		TotalRegistros: totalRegistros,
		PaginaActual:   filtro.Page,
		TotalPaginas:   totalPaginas,
		Limite:         filtro.Limit,
	}

	return &domain.PaginaProductosDTO{
		Data:           productos,
		Paginacion:     paginacion,
		TotalRegistros: totalRegistros,
		PaginaActual:   filtro.Page,
		TotalPaginas:   totalPaginas,
		Limite:         filtro.Limit,
	}, nil
}

// ObtenerCatalogoAdmin valida parámetros, aplica valores por defecto y sanitización, y obtiene los productos enriquecidos para administración
func (servicio *productoService) ObtenerCatalogoAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) (*domain.PaginaProductosAdminDTO, error) {
	// 1. Validación y sanitización del término de búsqueda general
	if strings.TrimSpace(filtro.Query) != "" {
		filtro.Query = utils.SanitizarInputBusqueda(filtro.Query, servicio.limites.LargoMaximoBusqueda)
		if len(filtro.Query) < 3 {
			return nil, ErrBusquedaCorta
		}
	}

	// 2. Valores por defecto para paginación
	if filtro.Page < 1 {
		filtro.Page = 1
	}
	if filtro.Limit < 1 {
		filtro.Limit = servicio.limites.TamanoPaginaPredeterminado
	} else if filtro.Limit > servicio.limites.TamanoPaginaMaximo {
		filtro.Limit = servicio.limites.TamanoPaginaMaximo
	}

	// 3. Valores por defecto para ordenamiento
	filtro.SortBy = strings.ToLower(strings.TrimSpace(filtro.SortBy))
	if filtro.SortBy != "precio" && filtro.SortBy != "nombre" && filtro.SortBy != "marca" && filtro.SortBy != "sku" && filtro.SortBy != "fecha" && filtro.SortBy != "ultima_extraccion" {
		filtro.SortBy = "precio"
	}

	filtro.Order = strings.ToLower(strings.TrimSpace(filtro.Order))
	if filtro.Order != "asc" && filtro.Order != "desc" {
		filtro.Order = "asc"
	}

	// 4. Sanitización de textos para prevenir ataques XSS o inyecciones
	if filtro.Categoria != "" {
		filtro.Categoria = utils.SanitizarInputBusqueda(filtro.Categoria, servicio.limites.LargoMaximoBusqueda)
	}
	if filtro.Supermercado != "" {
		filtro.Supermercado = utils.SanitizarInputBusqueda(filtro.Supermercado, servicio.limites.LargoMaximoBusqueda)
	}
	if filtro.Marca != "" {
		filtro.Marca = utils.SanitizarInputBusqueda(filtro.Marca, servicio.limites.LargoMaximoBusqueda)
	}
	if filtro.SKU != "" {
		filtro.SKU = utils.SanitizarInputBusqueda(filtro.SKU, servicio.limites.LargoMaximoBusqueda)
	}

	// 5. Invocar repositorio administrativo
	productos, totalRegistros, err := servicio.repo.ListarParaAdmin(ctx, filtro)
	if err != nil {
		return nil, err
	}

	// 6. Cálculo de metadatos de paginación
	totalPaginas := 0
	if totalRegistros > 0 {
		totalPaginas = int(math.Ceil(float64(totalRegistros) / float64(filtro.Limit)))
	}

	paginacion := domain.MetadatosPaginacionDTO{
		TotalRegistros: totalRegistros,
		PaginaActual:   filtro.Page,
		TotalPaginas:   totalPaginas,
		Limite:         filtro.Limit,
	}

	return &domain.PaginaProductosAdminDTO{
		Data:           productos,
		Paginacion:     paginacion,
		TotalRegistros: totalRegistros,
		PaginaActual:   filtro.Page,
		TotalPaginas:   totalPaginas,
		Limite:         filtro.Limit,
	}, nil
}

// ObtenerPorID busca un producto específico por su identificador e incluye su historial de precios
func (servicio *productoService) ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrProductoNoEncontrado
	}

	producto, err := servicio.repo.ObtenerPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if producto == nil {
		return nil, ErrProductoNoEncontrado
	}

	return producto, nil
}
