// Package services contiene la lógica de negocio del catálogo: normaliza
// filtros y paginación de productos, valida cambios de fuentes y aplica la
// regla de vigencia de la disponibilidad. Sólo depende de domain.
package services

import (
	"context"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
)

// ===== Límites de las consultas =====

// LimitesProductos son los límites configurables del listado (internal/config).
type LimitesProductos struct {
	TamanoPagina        int // limit cuando el cliente no lo envía
	TamanoPaginaMaximo  int // tope de limit
	LargoMaximoBusqueda int // bytes conservados de cada texto de búsqueda
}

// Límites propios del contrato, no configurables.
const (
	largoMinimoBusqueda = 3 // bytes mínimos de q tras sanear
	paginaInicial       = 1
)

// Orden por defecto y valores aceptados de sort_by/order.
const (
	ordenPredeterminado     = "precio"
	direccionPredeterminada = "asc"
)

var (
	ordenesPublicos     = map[string]bool{"precio": true, "nombre": true, "marca": true}
	ordenesAdmin        = map[string]bool{"precio": true, "nombre": true, "marca": true, "sku": true, "fecha": true, "ultima_extraccion": true}
	direccionesValidas  = map[string]bool{"asc": true, "desc": true}
	caracteresNoSeguros = regexp.MustCompile(`[^a-zA-Z0-9\sñÑáéíóúÁÉÍÓÚ\-]`)
)

// Errores del servicio de productos.
var (
	ErrProductoNoEncontrado = errors.New("producto no encontrado")
	ErrBusquedaCorta        = errors.New("el término de búsqueda debe tener al menos " + strconv.Itoa(largoMinimoBusqueda) + " caracteres")
)

// ===== Servicio =====

// ServicioProductos expone el catálogo público, el administrativo y el detalle.
type ServicioProductos struct {
	repositorio domain.RepositorioProductos
	limites     LimitesProductos
}

// NuevoServicioProductos inyecta el repositorio y los límites ya validados.
func NuevoServicioProductos(repositorio domain.RepositorioProductos, limites LimitesProductos) *ServicioProductos {
	return &ServicioProductos{repositorio: repositorio, limites: limites}
}

// ObtenerCatalogo normaliza el filtro público y devuelve la página pedida.
func (servicio *ServicioProductos) ObtenerCatalogo(contexto context.Context, filtro domain.FiltroProductosDTO) (*domain.PaginaProductosDTO, error) {
	filtro, errorFiltro := servicio.normalizarFiltro(filtro, ordenesPublicos)
	if errorFiltro != nil {
		return nil, errorFiltro
	}
	productos, total, errorConsulta := servicio.repositorio.Listar(contexto, filtro)
	if errorConsulta != nil {
		return nil, errorConsulta
	}
	paginacion := metadatos(filtro, total)
	return &domain.PaginaProductosDTO{
		Data: productos, Paginacion: paginacion,
		TotalRegistros: paginacion.TotalRegistros, PaginaActual: paginacion.PaginaActual,
		TotalPaginas: paginacion.TotalPaginas, Limite: paginacion.Limite,
	}, nil
}

// ObtenerCatalogoAdmin normaliza el filtro administrativo (admite SKU y orden
// por fecha) y devuelve la página pedida.
func (servicio *ServicioProductos) ObtenerCatalogoAdmin(contexto context.Context, filtro domain.FiltroProductosDTO) (*domain.PaginaProductosAdminDTO, error) {
	filtro, errorFiltro := servicio.normalizarFiltro(filtro, ordenesAdmin)
	if errorFiltro != nil {
		return nil, errorFiltro
	}
	if filtro.SKU != "" {
		filtro.SKU = servicio.sanearBusqueda(filtro.SKU)
	}
	productos, total, errorConsulta := servicio.repositorio.ListarParaAdmin(contexto, filtro)
	if errorConsulta != nil {
		return nil, errorConsulta
	}
	paginacion := metadatos(filtro, total)
	return &domain.PaginaProductosAdminDTO{
		Data: productos, Paginacion: paginacion,
		TotalRegistros: paginacion.TotalRegistros, PaginaActual: paginacion.PaginaActual,
		TotalPaginas: paginacion.TotalPaginas, Limite: paginacion.Limite,
	}, nil
}

// ObtenerPorID devuelve el producto con su historial o ErrProductoNoEncontrado.
func (servicio *ServicioProductos) ObtenerPorID(contexto context.Context, productoID string) (*domain.ProductoDetalleDTO, error) {
	productoID = strings.TrimSpace(productoID)
	if productoID == "" {
		return nil, ErrProductoNoEncontrado
	}
	producto, errorConsulta := servicio.repositorio.ObtenerPorID(contexto, productoID)
	if errorConsulta != nil {
		return nil, errorConsulta
	}
	if producto == nil {
		return nil, ErrProductoNoEncontrado
	}
	return producto, nil
}

// ===== Normalización del filtro =====

// normalizarFiltro sanea los textos, exige el largo mínimo de q y corrige la
// paginación y el orden a valores permitidos. La corrección silenciosa de
// valores fuera de rango es contrato vigente con Frontend e IA.
func (servicio *ServicioProductos) normalizarFiltro(filtro domain.FiltroProductosDTO, ordenesPermitidos map[string]bool) (domain.FiltroProductosDTO, error) {
	if strings.TrimSpace(filtro.Query) != "" {
		filtro.Query = servicio.sanearBusqueda(filtro.Query)
		if len(filtro.Query) < largoMinimoBusqueda {
			return filtro, ErrBusquedaCorta
		}
	}
	if filtro.Page < paginaInicial {
		filtro.Page = paginaInicial
	}
	if filtro.Limit < 1 {
		filtro.Limit = servicio.limites.TamanoPagina
	} else if filtro.Limit > servicio.limites.TamanoPaginaMaximo {
		filtro.Limit = servicio.limites.TamanoPaginaMaximo
	}
	filtro.SortBy = strings.ToLower(strings.TrimSpace(filtro.SortBy))
	if !ordenesPermitidos[filtro.SortBy] {
		filtro.SortBy = ordenPredeterminado
	}
	filtro.Order = strings.ToLower(strings.TrimSpace(filtro.Order))
	if !direccionesValidas[filtro.Order] {
		filtro.Order = direccionPredeterminada
	}
	for _, campo := range []*string{&filtro.Categoria, &filtro.Supermercado, &filtro.Marca} {
		if *campo != "" {
			*campo = servicio.sanearBusqueda(*campo)
		}
	}
	return filtro, nil
}

// sanearBusqueda recorta el texto a LargoMaximoBusqueda bytes y deja sólo
// letras, dígitos, espacios y guiones. Si el corte parte un carácter multibyte,
// el filtro descarta el byte suelto. Los valores siempre viajan como parámetros
// SQL; el filtro evita patrones ILIKE costosos o inesperados.
func (servicio *ServicioProductos) sanearBusqueda(texto string) string {
	limpio := strings.TrimSpace(texto)
	if len(limpio) > servicio.limites.LargoMaximoBusqueda {
		limpio = limpio[:servicio.limites.LargoMaximoBusqueda]
	}
	return caracteresNoSeguros.ReplaceAllString(limpio, "")
}

// metadatos calcula el total de páginas de la respuesta.
func metadatos(filtro domain.FiltroProductosDTO, total int64) domain.MetadatosPaginacionDTO {
	totalPaginas := 0
	if total > 0 {
		totalPaginas = int(math.Ceil(float64(total) / float64(filtro.Limit)))
	}
	return domain.MetadatosPaginacionDTO{TotalRegistros: total, PaginaActual: filtro.Page, TotalPaginas: totalPaginas, Limite: filtro.Limit}
}
