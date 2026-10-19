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

type productoService struct {
	repo domain.ProductoRepository
}

// NewProductoService instancia el servicio inyectando el repositorio de productos
func NewProductoService(repo domain.ProductoRepository) ProductoService {
	return &productoService{repo: repo}
}

// ObtenerCatalogo valida parámetros, aplica valores por defecto y sanitización, y obtiene los productos paginados
func (s *productoService) ObtenerCatalogo(ctx context.Context, filtro domain.FiltroProductosDTO) (*domain.PaginaProductosDTO, error) {
	// 1. Validación y sanitización del término de búsqueda general
	if strings.TrimSpace(filtro.Query) != "" {
		filtro.Query = utils.SanitizarInputBusqueda(filtro.Query)
		if len(filtro.Query) < 3 {
			return nil, ErrBusquedaCorta
		}
	}

	// 2. Valores por defecto para paginación
	if filtro.Page < 1 {
		filtro.Page = 1
	}
	if filtro.Limit < 1 {
		filtro.Limit = 20
	} else if filtro.Limit > 100 {
		filtro.Limit = 100
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
		filtro.Categoria = utils.SanitizarInputBusqueda(filtro.Categoria)
	}
	if filtro.Supermercado != "" {
		filtro.Supermercado = utils.SanitizarInputBusqueda(filtro.Supermercado)
	}
	if filtro.Marca != "" {
		filtro.Marca = utils.SanitizarInputBusqueda(filtro.Marca)
	}

	// 5. Invocar repositorio
	productos, totalRegistros, err := s.repo.Listar(ctx, filtro)
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
func (s *productoService) ObtenerCatalogoAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) (*domain.PaginaProductosAdminDTO, error) {
	// 1. Validación y sanitización del término de búsqueda general
	if strings.TrimSpace(filtro.Query) != "" {
		filtro.Query = utils.SanitizarInputBusqueda(filtro.Query)
		if len(filtro.Query) < 3 {
			return nil, ErrBusquedaCorta
		}
	}

	// 2. Valores por defecto para paginación
	if filtro.Page < 1 {
		filtro.Page = 1
	}
	if filtro.Limit < 1 {
		filtro.Limit = 20
	} else if filtro.Limit > 100 {
		filtro.Limit = 100
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
		filtro.Categoria = utils.SanitizarInputBusqueda(filtro.Categoria)
	}
	if filtro.Supermercado != "" {
		filtro.Supermercado = utils.SanitizarInputBusqueda(filtro.Supermercado)
	}
	if filtro.Marca != "" {
		filtro.Marca = utils.SanitizarInputBusqueda(filtro.Marca)
	}
	if filtro.SKU != "" {
		filtro.SKU = utils.SanitizarInputBusqueda(filtro.SKU)
	}

	// 5. Invocar repositorio administrativo
	productos, totalRegistros, err := s.repo.ListarParaAdmin(ctx, filtro)
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
func (s *productoService) ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrProductoNoEncontrado
	}

	producto, err := s.repo.ObtenerPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if producto == nil {
		return nil, ErrProductoNoEncontrado
	}

	return producto, nil
}
