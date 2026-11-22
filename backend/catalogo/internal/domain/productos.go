// Package domain define los datos y contratos del catálogo: DTO públicos y
// administrativos, fuentes, disponibilidad e ingesta, y las interfaces de
// persistencia que implementa repositories. No depende de infraestructura.
package domain

import (
	"context"
	"time"
)

// ===== Filtro de productos =====

// FiltroProductosDTO reúne los criterios de búsqueda, paginación y orden que el
// handler lee de la query string y el servicio normaliza.
type FiltroProductosDTO struct {
	Query             string
	Page              int
	Limit             int
	Categoria         string
	Supermercado      string
	Marca             string
	PrecioMin         *float64
	PrecioMax         *float64
	EnOferta          *bool
	SKU               string
	EnStock           *bool
	SoloMarcasBlancas *bool
	SortBy            string
	Order             string
}

// ===== Respuestas públicas =====

// ProductoDTO es el producto del catálogo público que consumen Frontend e IA.
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

// HistorialPrecioDTO es una captura histórica de precio del detalle.
type HistorialPrecioDTO struct {
	PrecioNormal float64   `json:"precio_normal"`
	PrecioOferta *float64  `json:"precio_oferta,omitempty"`
	CapturadoEl  time.Time `json:"capturado_el"`
}

// ProductoDetalleDTO es el producto con su historial de capturas.
type ProductoDetalleDTO struct {
	ProductoDTO
	Historial []HistorialPrecioDTO `json:"historial"`
}

// ===== Respuestas de administración =====

// ProductoAdminDTO añade los datos técnicos del scraper para el panel admin.
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

// ===== Paginación =====

// MetadatosPaginacionDTO es el bloque `paginacion` que lee IA.
type MetadatosPaginacionDTO struct {
	TotalRegistros int64 `json:"total_registros"`
	PaginaActual   int   `json:"pagina_actual"`
	TotalPaginas   int   `json:"total_paginas"`
	Limite         int   `json:"limite"`
}

// PaginaProductosDTO repite los metadatos en la raíz porque Frontend los lee
// ahí e IA los lee en `paginacion`; ambos contratos siguen vigentes.
type PaginaProductosDTO struct {
	Data           []ProductoDTO          `json:"data"`
	Paginacion     MetadatosPaginacionDTO `json:"paginacion"`
	TotalRegistros int64                  `json:"total_registros"`
	PaginaActual   int                    `json:"pagina_actual"`
	TotalPaginas   int                    `json:"total_paginas"`
	Limite         int                    `json:"limite"`
}

// PaginaProductosAdminDTO es la página del listado administrativo.
type PaginaProductosAdminDTO struct {
	Data           []ProductoAdminDTO     `json:"data"`
	Paginacion     MetadatosPaginacionDTO `json:"paginacion"`
	TotalRegistros int64                  `json:"total_registros"`
	PaginaActual   int                    `json:"pagina_actual"`
	TotalPaginas   int                    `json:"total_paginas"`
	Limite         int                    `json:"limite"`
}

// ===== Persistencia =====

// RepositorioProductos lee productos y capturas de scraper.*.
type RepositorioProductos interface {
	Listar(contexto context.Context, filtro FiltroProductosDTO) ([]ProductoDTO, int64, error)
	ListarParaAdmin(contexto context.Context, filtro FiltroProductosDTO) ([]ProductoAdminDTO, int64, error)
	ObtenerPorID(contexto context.Context, productoID string) (*ProductoDetalleDTO, error)
}
