package main

import (
	"github.com/google/uuid"
	"time"
)

// productoCrudoSemilla mapea scraper.productos_crudos
type productoCrudoSemilla struct {
	ID                 uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SucursalID         int       `gorm:"not null;index" json:"sucursal_id"`
	SKU                string    `gorm:"type:varchar(100);not null" json:"sku"`
	EANGTIN            *string   `gorm:"column:ean_gtin;type:varchar(50);index" json:"ean_gtin,omitempty"`
	TituloCrudo        string    `gorm:"type:varchar(255);not null" json:"titulo_crudo"`
	MarcaCruda         *string   `gorm:"type:varchar(100)" json:"marca_cruda,omitempty"`
	CategoriaCruda     *string   `gorm:"type:varchar(100)" json:"categoria_cruda,omitempty"`
	FormatoCrudo       *string   `gorm:"type:varchar(100)" json:"formato_crudo,omitempty"`
	URLProducto        *string   `gorm:"type:text" json:"url_producto,omitempty"`
	URLImagen          *string   `gorm:"type:text" json:"url_imagen,omitempty"`
	EnStock            bool      `gorm:"default:true" json:"en_stock"`
	UltimaExtraccionEl time.Time `gorm:"default:now();not null" json:"ultima_extraccion_el"`
}

func (productoCrudoSemilla) TableName() string {
	return "scraper.productos_crudos"
}

// capturaPrecioSemilla mapea scraper.capturas_precios
type capturaPrecioSemilla struct {
	ID                int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ProductoCrudoID   uuid.UUID `gorm:"type:uuid;not null;index" json:"producto_crudo_id"`
	PrecioNormal      float64   `gorm:"type:decimal(12,2);not null" json:"precio_normal"`
	PrecioOferta      *float64  `gorm:"type:decimal(12,2)" json:"precio_oferta,omitempty"`
	PrecioTarjeta     *float64  `gorm:"type:decimal(12,2)" json:"precio_tarjeta,omitempty"`
	PrecioPorUnidad   *float64  `gorm:"type:decimal(12,2)" json:"precio_por_unidad,omitempty"`
	MetricaUnidad     *string   `gorm:"type:varchar(20)" json:"metrica_unidad,omitempty"`
	MecanicaPromocion *string   `gorm:"type:varchar(255)" json:"mecanica_promocion,omitempty"`
	EstaDisponible    bool      `gorm:"default:true" json:"esta_disponible"`
	CapturadoEl       time.Time `gorm:"default:now();not null" json:"capturado_el"`
}

func (capturaPrecioSemilla) TableName() string {
	return "scraper.capturas_precios"
}
