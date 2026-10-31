package domain

import "time"

// FiltroProductosDTO define los criterios de filtrado, paginación y ordenamiento para el catálogo de productos.
type FiltroProductosDTO struct {
	Query        string   `json:"q" form:"q"`
	Page         int      `json:"page" form:"page"`
	Limit        int      `json:"limit" form:"limit"`
	Categoria    string   `json:"categoria" form:"categoria"`
	Supermercado string   `json:"supermercado" form:"supermercado"`
	Marca        string   `json:"marca" form:"marca"`
	PrecioMin    *float64 `json:"precio_min" form:"precio_min"`
	PrecioMax    *float64 `json:"precio_max" form:"precio_max"`
	EnOferta     *bool    `json:"en_oferta" form:"en_oferta"`
	SKU          string   `json:"sku" form:"sku"`
	EnStock      *bool    `json:"en_stock" form:"en_stock"`
	SortBy       string   `json:"sort_by" form:"sort_by"`
	Order        string   `json:"order" form:"order"`
}

// ProductoDTO representa el objeto de producto enriquecido para el catálogo de cara a la UI y clientes de la API.
type ProductoDTO struct {
	ID           string   `json:"id"`
	Nombre       string   `json:"nombre"`
	Marca        string   `json:"marca"`
	Categoria    string   `json:"categoria"`
	Supermercado string   `json:"supermercado"`
	Precio       float64  `json:"precio"`
	PrecioNormal float64  `json:"precio_normal"`
	PrecioOferta *float64 `json:"precio_oferta,omitempty"`
	EnOferta     bool     `json:"en_oferta"`
	URLImagen    string   `json:"url_imagen"`
	Unidad       string   `json:"unidad"`
	EnStock      bool     `json:"en_stock"`
}

// ProductoAdminDTO incluye la información técnica y sensible del scraper para el dashboard de administración.
type ProductoAdminDTO struct {
	ID                 string    `json:"id"`
	SKU                string    `json:"sku"`
	Nombre             string    `json:"nombre"`
	Marca              string    `json:"marca"`
	Categoria          string    `json:"categoria"`
	Supermercado       string    `json:"supermercado"`
	PrecioNormal       int       `json:"precio_normal"`
	PrecioOferta       *int      `json:"precio_oferta,omitempty"`
	EnOferta           bool      `json:"en_oferta"`
	URLImagen          string    `json:"url_imagen"`
	Unidad             string    `json:"unidad"`
	EnStock            bool      `json:"en_stock"`
	UltimaExtraccionEl time.Time `json:"ultima_extraccion_el"`
	SucursalID         int       `json:"sucursal_id"`
}

// MetadatosPaginacionDTO describe los metadatos de paginación devueltos en la respuesta.
type MetadatosPaginacionDTO struct {
	TotalRegistros int64 `json:"total_registros"`
	PaginaActual   int   `json:"pagina_actual"`
	TotalPaginas   int   `json:"total_paginas"`
	Limite         int   `json:"limite"`
}

// PaginaProductosDTO estructura la respuesta completa del catálogo con la lista de productos y metadatos de paginación.
type PaginaProductosDTO struct {
	Data           []ProductoDTO          `json:"data"`
	Paginacion     MetadatosPaginacionDTO `json:"paginacion"`
	TotalRegistros int64                  `json:"total_registros"`
	PaginaActual   int                    `json:"pagina_actual"`
	TotalPaginas   int                    `json:"total_paginas"`
	Limite         int                    `json:"limite"`
}

// PaginaProductosAdminDTO estructura la respuesta del catálogo administrativo con metadatos de paginación.
type PaginaProductosAdminDTO struct {
	Data           []ProductoAdminDTO     `json:"data"`
	Paginacion     MetadatosPaginacionDTO `json:"paginacion"`
	TotalRegistros int64                  `json:"total_registros"`
	PaginaActual   int                    `json:"pagina_actual"`
	TotalPaginas   int                    `json:"total_paginas"`
	Limite         int                    `json:"limite"`
}

// HistorialPrecioDTO representa una captura histórica de precio para visualización y gráficas.
type HistorialPrecioDTO struct {
	PrecioNormal float64   `json:"precio_normal"`
	PrecioOferta *float64  `json:"precio_oferta,omitempty"`
	CapturadoEl  time.Time `json:"capturado_el"`
}

// ProductoDetalleDTO contiene la información base del producto junto con su historial de capturas de precio.
type ProductoDetalleDTO struct {
	ProductoDTO
	Historial []HistorialPrecioDTO `json:"historial"`
}
