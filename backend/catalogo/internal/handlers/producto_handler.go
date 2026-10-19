package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/middleware"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
)

// ProductoHandler maneja las peticiones HTTP relacionadas con el catálogo de productos
type ProductoHandler struct {
	service services.ProductoService
}

// NewProductoHandler crea una nueva instancia de ProductoHandler
func NewProductoHandler(service services.ProductoService) *ProductoHandler {
	return &ProductoHandler{service: service}
}

// ObtenerProductos godoc
// @Summary      Catálogo y filtrado de productos
// @Description  Obtiene una lista paginada de productos con soporte para múltiples filtros (categoría, supermercado, marca, rango de precio, en oferta) y ordenamiento.
// @Tags         productos
// @Produce      json
// @Param        q             query     string  false  "Búsqueda por texto libre"
// @Param        page          query     int     false  "Número de página (default: 1)"
// @Param        limit         query     int     false  "Cantidad de productos por página (default: 20)"
// @Param        categoria     query     string  false  "Filtrar por categoría"
// @Param        supermercado  query     string  false  "Filtrar por supermercado o cadena"
// @Param        marca         query     string  false  "Filtrar por marca"
// @Param        en_stock      query     bool    false  "Filtrar por disponibilidad de stock"
// @Param        precio_min    query     number  false  "Precio mínimo"
// @Param        precio_max    query     number  false  "Precio máximo"
// @Param        en_oferta     query     bool    false  "Filtrar solo productos en oferta"
// @Param        sort_by       query     string  false  "Criterio de ordenamiento (precio, nombre, marca)"
// @Param        order         query     string  false  "Dirección del orden (asc, desc)"
// @Success      200           {object}  domain.PaginaProductosDTO
// @Failure      400           {object}  map[string]interface{}
// @Failure      500           {object}  map[string]interface{}
// @Router       /api/v1/productos [get]
func (h *ProductoHandler) ObtenerProductos(c *gin.Context) {
	var filtro domain.FiltroProductosDTO

	filtro.Query = c.Query("q")

	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil {
			filtro.Page = page
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filtro.Limit = limit
		}
	}

	filtro.Categoria = c.Query("categoria")
	filtro.Supermercado = c.Query("supermercado")
	filtro.Marca = c.Query("marca")

	if enStockStr := c.Query("en_stock"); enStockStr != "" {
		if enStock, err := strconv.ParseBool(enStockStr); err == nil {
			filtro.EnStock = &enStock
		}
	}

	if pMinStr := c.Query("precio_min"); pMinStr != "" {
		if pMin, err := strconv.ParseFloat(pMinStr, 64); err == nil {
			filtro.PrecioMin = &pMin
		}
	}

	if pMaxStr := c.Query("precio_max"); pMaxStr != "" {
		if pMax, err := strconv.ParseFloat(pMaxStr, 64); err == nil {
			filtro.PrecioMax = &pMax
		}
	}

	if enOfertaStr := c.Query("en_oferta"); enOfertaStr != "" {
		if enOferta, err := strconv.ParseBool(enOfertaStr); err == nil {
			filtro.EnOferta = &enOferta
		}
	}

	filtro.SortBy = c.Query("sort_by")
	filtro.Order = c.Query("order")

	resultado, err := h.service.ObtenerCatalogo(c.Request.Context(), filtro)
	if err != nil {
		if errors.Is(err, services.ErrBusquedaCorta) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		middleware.LogInternalError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error interno al obtener los productos del catálogo",
		})
		return
	}

	c.JSON(http.StatusOK, resultado)
}

// ObtenerProductosAdmin godoc
// @Summary      Catálogo de productos para administración
// @Description  Obtiene listado paginado y filtrado de productos con datos técnicos de scraping para Superadmin
// @Tags         admin
// @Produce      json
// @Param        q             query     string  false  "Búsqueda por texto libre"
// @Param        page          query     int     false  "Número de página (default: 1)"
// @Param        limit         query     int     false  "Cantidad de productos por página (default: 20)"
// @Param        categoria     query     string  false  "Filtrar por categoría"
// @Param        supermercado  query     string  false  "Filtrar por supermercado o cadena"
// @Param        marca         query     string  false  "Filtrar por marca"
// @Param        sku           query     string  false  "Filtrar por SKU"
// @Param        en_stock      query     bool    false  "Filtrar por disponibilidad de stock"
// @Param        precio_min    query     number  false  "Precio mínimo"
// @Param        precio_max    query     number  false  "Precio máximo"
// @Param        en_oferta     query     bool    false  "Filtrar solo productos en oferta"
// @Param        sort_by       query     string  false  "Criterio de ordenamiento (precio, nombre, marca, sku, fecha)"
// @Param        order         query     string  false  "Dirección del orden (asc, desc)"
// @Success      200           {object}  domain.PaginaProductosAdminDTO
// @Failure      400           {object}  middleware.RespuestaError
// @Failure      401           {object}  middleware.RespuestaError
// @Failure      403           {object}  middleware.RespuestaError
// @Failure      500           {object}  middleware.RespuestaError
// @Router       /api/v1/admin/productos [get]
func (h *ProductoHandler) ObtenerProductosAdmin(c *gin.Context) {
	var filtro domain.FiltroProductosDTO

	filtro.Query = c.Query("q")

	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil {
			filtro.Page = page
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filtro.Limit = limit
		}
	}

	filtro.Categoria = c.Query("categoria")
	filtro.Supermercado = c.Query("supermercado")
	filtro.Marca = c.Query("marca")
	filtro.SKU = c.Query("sku")

	if enStockStr := c.Query("en_stock"); enStockStr != "" {
		if enStock, err := strconv.ParseBool(enStockStr); err == nil {
			filtro.EnStock = &enStock
		}
	}

	if pMinStr := c.Query("precio_min"); pMinStr != "" {
		if pMin, err := strconv.ParseFloat(pMinStr, 64); err == nil {
			filtro.PrecioMin = &pMin
		}
	}

	if pMaxStr := c.Query("precio_max"); pMaxStr != "" {
		if pMax, err := strconv.ParseFloat(pMaxStr, 64); err == nil {
			filtro.PrecioMax = &pMax
		}
	}

	if enOfertaStr := c.Query("en_oferta"); enOfertaStr != "" {
		if enOferta, err := strconv.ParseBool(enOfertaStr); err == nil {
			filtro.EnOferta = &enOferta
		}
	}

	filtro.SortBy = c.Query("sort_by")
	filtro.Order = c.Query("order")

	resultado, err := h.service.ObtenerCatalogoAdmin(c.Request.Context(), filtro)
	if err != nil {
		if errors.Is(err, services.ErrBusquedaCorta) {
			middleware.ResponderError(c, http.StatusBadRequest, services.ErrBusquedaCorta.Error(), middleware.ErrorValidacion{Mensaje: services.ErrBusquedaCorta.Error()})
			return
		}
		middleware.ResponderError(c, http.StatusInternalServerError, "Error interno al obtener los productos para administración", err)
		return
	}

	c.JSON(http.StatusOK, resultado)
}

// BuscarProductos godoc
// @Summary      Búsqueda de productos por término
// @Description  Busca productos que coincidan parcialmente en título, marca o categoría con el parámetro 'q' (mínimo 3 caracteres obligatorios).
// @Tags         productos
// @Produce      json
// @Param        q      query     string  true   "Término de búsqueda (mínimo 3 caracteres)"
// @Param        page   query     int     false  "Número de página (default: 1)"
// @Param        limit  query     int     false  "Cantidad de productos por página (default: 20)"
// @Success      200    {object}  domain.PaginaProductosDTO
// @Failure      400    {object}  map[string]interface{}
// @Failure      500    {object}  map[string]interface{}
// @Router       /api/v1/productos/buscar [get]
func (h *ProductoHandler) BuscarProductos(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "El parámetro de búsqueda 'q' es obligatorio",
		})
		return
	}

	var filtro domain.FiltroProductosDTO
	filtro.Query = q

	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil {
			filtro.Page = page
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filtro.Limit = limit
		}
	}

	resultado, err := h.service.ObtenerCatalogo(c.Request.Context(), filtro)
	if err != nil {
		if errors.Is(err, services.ErrBusquedaCorta) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		middleware.LogInternalError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error interno al buscar productos",
		})
		return
	}

	c.JSON(http.StatusOK, resultado)
}

// ObtenerDetalleProducto godoc
// @Summary      Consulta detalle de producto y su historial de precios
// @Description  Obtiene la información completa de un producto por su ID junto con el historial de precios capturados
// @Tags         productos
// @Produce      json
// @Param        id   path      string  true  "ID del producto (UUID)"
// @Success      200  {object}  domain.ProductoDetalleDTO
// @Failure      404  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /api/v1/productos/{id} [get]
func (h *ProductoHandler) ObtenerDetalleProducto(c *gin.Context) {
	id := c.Param("id")

	producto, err := h.service.ObtenerPorID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrProductoNoEncontrado) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Producto no encontrado",
			})
			return
		}
		middleware.LogInternalError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error interno al obtener el detalle del producto",
		})
		return
	}

	c.JSON(http.StatusOK, producto)
}
