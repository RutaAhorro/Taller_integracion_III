// Ingesta privada: contrato que usa Control de scraping para resolver cadenas
// y sucursales y confirmar lotes de productos con sus precios.

package domain

import (
	"context"
	"errors"
	"time"
)

// ===== Cadenas y sucursales =====

// CadenaSupermercado mapea scraper.cadenas_supermercado.
type CadenaSupermercado struct {
	ID                int       `gorm:"primaryKey" json:"id"`
	Nombre            string    `json:"nombre"`
	URLSitioWeb       *string   `json:"url_sitio_web,omitempty"`
	URLLogo           *string   `json:"url_logo,omitempty"`
	ConfigScraperJSON *string   `gorm:"type:jsonb" json:"config_scraper_json,omitempty"`
	EstaActiva        bool      `json:"esta_activa"`
	CreadoEl          time.Time `json:"creado_el"`
}

// TableName fija el esquema de la tabla para GORM.
func (CadenaSupermercado) TableName() string { return "scraper.cadenas_supermercado" }

// SucursalSupermercado mapea scraper.sucursales_supermercado.
type SucursalSupermercado struct {
	ID             int     `gorm:"primaryKey" json:"id"`
	CadenaID       int     `json:"cadena_id"`
	CodigoSucursal *string `json:"codigo_sucursal,omitempty"`
	Nombre         string  `json:"nombre"`
	Direccion      string  `json:"direccion"`
	Comuna         *string `json:"comuna,omitempty"`
	Ciudad         *string `json:"ciudad,omitempty"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	HoraApertura   *string `json:"hora_apertura,omitempty"`
	HoraCierre     *string `json:"hora_cierre,omitempty"`
	EstaActiva     bool    `json:"esta_activa"`
}

// TableName fija el esquema de la tabla para GORM.
func (SucursalSupermercado) TableName() string { return "scraper.sucursales_supermercado" }

// ===== Lotes =====

// ProductoScrapeadoDTO es un producto del lote en el formato normalizado de
// Control. `precio` e `imagen` son alias heredados de `precio_oferta` y
// `url_imagen` que el contrato aún acepta.
type ProductoScrapeadoDTO struct {
	SKU               string   `json:"sku"`
	EANGTIN           *string  `json:"ean_gtin,omitempty"`
	Producto          string   `json:"producto"`
	Marca             *string  `json:"marca,omitempty"`
	Categoria         *string  `json:"categoria,omitempty"`
	FormatoCrudo      *string  `json:"formato_crudo,omitempty"`
	Precio            *float64 `json:"precio,omitempty"`
	PrecioNormal      float64  `json:"precio_normal"`
	PrecioOferta      *float64 `json:"precio_oferta,omitempty"`
	PrecioTarjeta     *float64 `json:"precio_tarjeta,omitempty"`
	PrecioPorUnidad   *float64 `json:"precio_por_unidad,omitempty"`
	MetricaUnidad     *string  `json:"metrica_unidad,omitempty"`
	MecanicaPromocion *string  `json:"mecanica_promocion,omitempty"`
	EnStock           *bool    `json:"en_stock,omitempty"`
	URLProducto       *string  `json:"url_producto,omitempty"`
	Imagen            *string  `json:"imagen,omitempty"`
	URLImagen         *string  `json:"url_imagen,omitempty"`
}

// LoteIngestaDTO es el cuerpo de POST /internal/v1/ingestas. Sólo referencia
// datos de Catálogo; los trabajos de scraping viven en Control.
type LoteIngestaDTO struct {
	LoteID     string                 `json:"lote_id,omitempty"`
	SucursalID int                    `json:"sucursal_id"`
	Productos  []ProductoScrapeadoDTO `json:"productos"`
}

// ResultadoIngestaDTO resume un lote. Se guarda como recibo idempotente y
// Control lo reenvía a sus alertas.
type ResultadoIngestaDTO struct {
	SucursalID         int      `json:"sucursal_id"`
	TotalRecibidos     int      `json:"total_recibidos"`
	Insertados         int      `json:"insertados"`
	Actualizados       int      `json:"actualizados"`
	PreciosRegistrados int      `json:"precios_registrados"`
	Errores            []string `json:"errores,omitempty"`
	Mensaje            string   `json:"mensaje"`
}

// ErrIdempotencia indica que la clave del lote ya se confirmó con otro
// contenido: repetir la petición no lo arregla.
var ErrIdempotencia = errors.New("la clave del lote ya se utilizó con otro contenido")

// RepositorioIngesta resuelve referencias y confirma lotes en una transacción.
type RepositorioIngesta interface {
	ObtenerCadena(contexto context.Context, cadenaID int, nombre string) (*CadenaSupermercado, error)
	ObtenerSucursal(contexto context.Context, cadenaID int, sucursalID *int, codigoSucursal *string) (*SucursalSupermercado, error)
	IngestarLote(contexto context.Context, lote LoteIngestaDTO) (*ResultadoIngestaDTO, error)
}
