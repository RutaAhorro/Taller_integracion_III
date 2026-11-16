// Package domain define los datos y contratos del catálogo. Este archivo agrupa
// los DTO, estados y la interfaz de la consulta interna de disponibilidad.
package domain

import (
	"context"
	"time"
)

// ===== Estados de la consulta interna de disponibilidad =====

// Estados del producto que consume Listas e IA para decidir si un artículo se
// puede guardar o mostrar. "desconocido" significa que Catálogo no pudo
// verificar el dato porque no hay una captura vigente.
const (
	EstadoProductoDisponible   = "disponible"
	EstadoProductoSinStock     = "sin_stock"
	EstadoProductoNoEncontrado = "no_encontrado"
	EstadoProductoDesconocido  = "desconocido"
)

// Estados de vigencia del precio. Un precio vencido nunca se publica como
// actual: se devuelve nulo para que el consumidor no lo use en cálculos.
const (
	EstadoPrecioVigente    = "vigente"
	EstadoPrecioVencido    = "vencido"
	EstadoPrecioSinCaptura = "sin_captura"
)

// ConsultaDisponibilidadDTO es el cuerpo aceptado por el endpoint interno.
// Recibe sólo identificadores, nunca cantidades ni datos de la lista.
type ConsultaDisponibilidadDTO struct {
	ProductoIDs []string `json:"producto_ids"`
}

// RespuestaDisponibilidadDTO agrupa el resultado de un lote de productos.
type RespuestaDisponibilidadDTO struct {
	Productos []DisponibilidadItemDTO `json:"productos"`
}

// DisponibilidadItemDTO describe un producto solicitado. Los campos punteros
// permiten distinguir "no hay dato" de "el dato es falso o cero".
type DisponibilidadItemDTO struct {
	ProductoID     string                     `json:"producto_id"`
	EstadoProducto string                     `json:"estado_producto"`
	EnStock        *bool                      `json:"en_stock"`
	CapturadoEl    *time.Time                 `json:"capturado_el"`
	EstadoPrecio   string                     `json:"estado_precio"`
	Producto       *ProductoDisponibilidadDTO `json:"producto"`
}

// ProductoDisponibilidadDTO contiene sólo los datos que Listas e IA necesitan
// para mostrar y validar un producto. Los precios se omiten cuando la captura
// no es vigente.
type ProductoDisponibilidadDTO struct {
	Nombre       string   `json:"nombre"`
	Marca        string   `json:"marca"`
	Categoria    string   `json:"categoria"`
	Supermercado string   `json:"supermercado"`
	SucursalID   int      `json:"sucursal_id"`
	Sucursal     string   `json:"sucursal"`
	Unidad       string   `json:"unidad"`
	Precio       *float64 `json:"precio"`
	PrecioNormal *float64 `json:"precio_normal"`
	PrecioOferta *float64 `json:"precio_oferta"`
	URLImagen    string   `json:"url_imagen"`
}

// ProductoDisponibilidadRegistro representa la fila que devuelve la consulta
// por lote. La captura puede faltar si el producto nunca tuvo precios y el
// stock es puntero porque NULL significa que el scraper no lo confirmó.
type ProductoDisponibilidadRegistro struct {
	ProductoID   string
	Nombre       string
	Marca        string
	Categoria    string
	Supermercado string
	SucursalID   int
	Sucursal     string
	Unidad       string
	URLImagen    string
	EnStock      *bool
	PrecioNormal *float64
	PrecioOferta *float64
	CapturadoEl  *time.Time
}

// DisponibilidadRepository define la consulta de lectura que usa el servicio
// de disponibilidad. La implementación vive en repositories y sólo lee tablas
// del dominio Catálogo.
type DisponibilidadRepository interface {
	ConsultarPorIDs(contexto context.Context, productoIDs []string) ([]ProductoDisponibilidadRegistro, error)
}
