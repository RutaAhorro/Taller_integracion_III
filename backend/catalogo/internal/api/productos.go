// Productos: catálogo público, búsqueda, detalle y listado de administración.
// Lee la query string y delega la normalización en services.

package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
)

// ===== Handlers =====

// manejadorProductos agrupa los handlers de productos.
type manejadorProductos struct {
	servicio *services.ServicioProductos
}

// listar atiende GET /api/v1/productos con todos los filtros públicos.
func (manejador manejadorProductos) listar(contexto *gin.Context) {
	filtro := leerFiltro(contexto)
	filtro.SKU = ""
	pagina, errorCatalogo := manejador.servicio.ObtenerCatalogo(contexto.Request.Context(), filtro)
	responderPagina(contexto, pagina, errorCatalogo, "Error interno al obtener los productos del catálogo")
}

// buscar atiende GET /api/v1/productos/buscar: exige q y sólo pagina.
func (manejador manejadorProductos) buscar(contexto *gin.Context) {
	termino := strings.TrimSpace(contexto.Query("q"))
	if termino == "" {
		contexto.JSON(http.StatusBadRequest, gin.H{"error": "El parámetro de búsqueda 'q' es obligatorio"})
		return
	}
	filtro := domain.FiltroProductosDTO{Query: termino, Page: leerEntero(contexto, "page"), Limit: leerEntero(contexto, "limit")}
	pagina, errorCatalogo := manejador.servicio.ObtenerCatalogo(contexto.Request.Context(), filtro)
	responderPagina(contexto, pagina, errorCatalogo, "Error interno al buscar productos")
}

// detalle atiende GET /api/v1/productos/:id con el historial de precios.
func (manejador manejadorProductos) detalle(contexto *gin.Context) {
	producto, errorDetalle := manejador.servicio.ObtenerPorID(contexto.Request.Context(), contexto.Param("id"))
	if errorDetalle != nil {
		if errors.Is(errorDetalle, services.ErrProductoNoEncontrado) {
			contexto.JSON(http.StatusNotFound, gin.H{"error": "Producto no encontrado"})
			return
		}
		registrarErrorInterno(contexto, errorDetalle)
		contexto.JSON(http.StatusInternalServerError, gin.H{"error": "Error interno al obtener el detalle del producto"})
		return
	}
	contexto.JSON(http.StatusOK, producto)
}

// listarAdmin atiende GET /api/v1/admin/productos; admite además sku. Sus
// errores usan el formato RespuestaError de las rutas protegidas.
func (manejador manejadorProductos) listarAdmin(contexto *gin.Context) {
	pagina, errorCatalogo := manejador.servicio.ObtenerCatalogoAdmin(contexto.Request.Context(), leerFiltro(contexto))
	if errorCatalogo != nil {
		if errors.Is(errorCatalogo, services.ErrBusquedaCorta) {
			responderError(contexto, http.StatusBadRequest, services.ErrBusquedaCorta.Error(), services.ErrBusquedaCorta.Error())
			return
		}
		registrarErrorInterno(contexto, errorCatalogo)
		responderError(contexto, http.StatusInternalServerError, mensajeErrorInterno, "")
		return
	}
	contexto.JSON(http.StatusOK, pagina)
}

// responderPagina traduce el resultado de los listados públicos.
func responderPagina(contexto *gin.Context, pagina *domain.PaginaProductosDTO, errorCatalogo error, mensajeInterno string) {
	if errorCatalogo == nil {
		contexto.JSON(http.StatusOK, pagina)
		return
	}
	if errors.Is(errorCatalogo, services.ErrBusquedaCorta) {
		contexto.JSON(http.StatusBadRequest, gin.H{"error": errorCatalogo.Error()})
		return
	}
	registrarErrorInterno(contexto, errorCatalogo)
	contexto.JSON(http.StatusInternalServerError, gin.H{"error": mensajeInterno})
}

// ===== Lectura de parámetros =====

// leerFiltro lee todos los filtros de la query string. Un número o booleano
// inválido se ignora como si no se hubiera enviado: es el contrato vigente con
// Frontend e IA (ver pendientes SUP-394 en el README).
func leerFiltro(contexto *gin.Context) domain.FiltroProductosDTO {
	return domain.FiltroProductosDTO{
		Query:             contexto.Query("q"),
		Page:              leerEntero(contexto, "page"),
		Limit:             leerEntero(contexto, "limit"),
		Categoria:         contexto.Query("categoria"),
		Supermercado:      contexto.Query("supermercado"),
		Marca:             contexto.Query("marca"),
		SKU:               contexto.Query("sku"),
		EnStock:           leerBooleano(contexto, "en_stock"),
		PrecioMin:         leerDecimal(contexto, "precio_min"),
		PrecioMax:         leerDecimal(contexto, "precio_max"),
		EnOferta:          leerBooleano(contexto, "en_oferta"),
		SoloMarcasBlancas: leerBooleano(contexto, "solo_marcas_blancas"),
		SortBy:            contexto.Query("sort_by"),
		Order:             contexto.Query("order"),
	}
}

// leerEntero devuelve 0 si el parámetro falta o no es entero; el servicio lo
// reemplaza por el valor por defecto.
func leerEntero(contexto *gin.Context, nombre string) int {
	valor, errorConversion := strconv.Atoi(contexto.Query(nombre))
	if errorConversion != nil {
		return 0
	}
	return valor
}

// leerBooleano devuelve nil si el parámetro falta o no es booleano.
func leerBooleano(contexto *gin.Context, nombre string) *bool {
	valor, errorConversion := strconv.ParseBool(contexto.Query(nombre))
	if errorConversion != nil {
		return nil
	}
	return &valor
}

// leerDecimal devuelve nil si el parámetro falta o no es numérico.
func leerDecimal(contexto *gin.Context, nombre string) *float64 {
	valor, errorConversion := strconv.ParseFloat(contexto.Query(nombre), 64)
	if errorConversion != nil {
		return nil
	}
	return &valor
}
