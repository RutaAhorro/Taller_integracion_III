// Equivalencia interna por EAN: DTO, estados e interfaz de lectura que usa el
// motor de rutas para comparar el mismo producto entre sucursales.

package domain

import (
	"context"
	"time"
)

// ===== Estados de la consulta de equivalencia =====

// Estados de un producto solicitado. Sólo "comparable" permite unir ofertas de
// otras sucursales; los demás devuelven como máximo la oferta del propio
// producto y nunca inventan una equivalencia.
const (
	EstadoEquivalenciaComparable            = "comparable"
	EstadoEquivalenciaSinEAN                = "sin_ean"
	EstadoEquivalenciaEANInvalido           = "ean_invalido"
	EstadoEquivalenciaConflictoPresentacion = "conflicto_presentacion"
	EstadoEquivalenciaNoEncontrado          = "no_encontrado"
)

// ===== Contrato de la consulta =====

// ConsultaEquivalenciaDTO es el cuerpo aceptado por la ruta interna. Recibe
// sólo identificadores de producto, nunca cantidades ni datos de la lista.
type ConsultaEquivalenciaDTO struct {
	ProductoIDs []string `json:"producto_ids"`
}

// RespuestaEquivalenciaDTO agrupa el resultado del lote y la cobertura medida
// en esa misma consulta.
type RespuestaEquivalenciaDTO struct {
	Productos []EquivalenciaProductoDTO `json:"productos"`
	Resumen   ResumenEquivalenciaDTO    `json:"resumen"`
}

// ResumenEquivalenciaDTO cuantifica cuántos productos del lote tienen EAN
// utilizable y cuántos quedan fuera por ausencia, formato inválido, conflicto
// de presentación o inexistencia. Sirve para informar cobertura real en vez de
// declarar resuelto un caso con datos ficticios.
type ResumenEquivalenciaDTO struct {
	Solicitados            int `json:"solicitados"`
	Comparables            int `json:"comparables"`
	SinEAN                 int `json:"sin_ean"`
	EANInvalidos           int `json:"ean_invalidos"`
	ConflictosPresentacion int `json:"conflictos_presentacion"`
	NoEncontrados          int `json:"no_encontrados"`
}

// EquivalenciaProductoDTO describe un producto solicitado y las ofertas que
// pueden compararse con él. EAN es el GTIN canónico de 14 dígitos; es nulo
// cuando el producto no tiene un código válido.
type EquivalenciaProductoDTO struct {
	ProductoID   string                 `json:"producto_id"`
	Estado       string                 `json:"estado"`
	EAN          *string                `json:"ean"`
	Presentacion string                 `json:"presentacion"`
	Ofertas      []OfertaEquivalenteDTO `json:"ofertas"`
}

// OfertaEquivalenteDTO es una oferta concreta dentro del grupo de equivalencia.
// ProductoIDOferta conserva el id real de Catálogo que se compra en esa
// sucursal, igual que el contrato del motor de rutas. Los precios se omiten
// cuando la captura no es vigente.
type OfertaEquivalenteDTO struct {
	ProductoIDOferta string     `json:"producto_id_oferta"`
	SucursalID       int        `json:"sucursal_id"`
	Cadena           string     `json:"cadena"`
	Sucursal         string     `json:"sucursal"`
	Nombre           string     `json:"nombre"`
	Unidad           string     `json:"unidad"`
	Precio           *float64   `json:"precio"`
	PrecioNormal     *float64   `json:"precio_normal"`
	PrecioOferta     *float64   `json:"precio_oferta"`
	EnStock          *bool      `json:"en_stock"`
	CapturadoEl      *time.Time `json:"capturado_el"`
	EstadoPrecio     string     `json:"estado_precio"`
}

// ===== Persistencia =====

// ProductoEquivalenciaRegistro es la fila que devuelve la consulta: el
// producto, su sucursal y su última captura. CodigoEAN se conserva crudo para
// que el servicio aplique la normalización y validación del GTIN.
type ProductoEquivalenciaRegistro struct {
	ProductoID   string
	CodigoEAN    *string
	Nombre       string
	Marca        string
	Formato      string
	SucursalID   int
	Cadena       string
	Sucursal     string
	EnStock      *bool
	PrecioNormal *float64
	PrecioOferta *float64
	CapturadoEl  *time.Time
}

// RepositorioEquivalencias define la lectura que usa el servicio de
// equivalencias. La implementación vive en repositories y sólo lee tablas del
// dominio Catálogo.
type RepositorioEquivalencias interface {
	ConsultarPorIDs(contexto context.Context, productoIDs []string) ([]ProductoEquivalenciaRegistro, error)
}
