package ingesta

import "time"

// CadenaSupermercado mapea scraper.cadenas_supermercado
type CadenaSupermercado struct {
	ID                int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Nombre            string    `gorm:"type:varchar(100);unique;not null" json:"nombre"`
	URLSitioWeb       *string   `gorm:"type:varchar(255)" json:"url_sitio_web,omitempty"`
	URLLogo           *string   `gorm:"type:varchar(255)" json:"url_logo,omitempty"`
	ConfigScraperJSON *string   `gorm:"type:jsonb" json:"config_scraper_json,omitempty"`
	EstaActiva        bool      `gorm:"default:true" json:"esta_activa"`
	CreadoEl          time.Time `gorm:"default:now();not null" json:"creado_el"`
}

func (CadenaSupermercado) TableName() string {
	return "scraper.cadenas_supermercado"
}

// SucursalSupermercado mapea scraper.sucursales_supermercado
type SucursalSupermercado struct {
	ID             int     `gorm:"primaryKey;autoIncrement" json:"id"`
	CadenaID       int     `gorm:"not null;index" json:"cadena_id"`
	CodigoSucursal *string `gorm:"type:varchar(50)" json:"codigo_sucursal,omitempty"`
	Nombre         string  `gorm:"type:varchar(150);not null" json:"nombre"`
	Direccion      string  `gorm:"type:varchar(255);not null" json:"direccion"`
	Comuna         *string `gorm:"type:varchar(100)" json:"comuna,omitempty"`
	Ciudad         *string `gorm:"type:varchar(100)" json:"ciudad,omitempty"`
	Lat            float64 `gorm:"type:decimal(10,8);not null" json:"lat"`
	Lon            float64 `gorm:"type:decimal(11,8);not null" json:"lon"`
	HoraApertura   *string `gorm:"type:time" json:"hora_apertura,omitempty"`
	HoraCierre     *string `gorm:"type:time" json:"hora_cierre,omitempty"`
	EstaActiva     bool    `gorm:"default:true" json:"esta_activa"`

	Cadena *CadenaSupermercado `gorm:"foreignKey:CadenaID;constraint:OnDelete:CASCADE" json:"-"`
}

func (SucursalSupermercado) TableName() string {
	return "scraper.sucursales_supermercado"
}

// ProductoScrapeadoDTO representa cada item extraído por los spiders (Jumbo, Santa Isabel, etc.)
// compatible con el formato normalizado del microservicio de scraping.
type ProductoScrapeadoDTO struct {
	SKU               string   `json:"sku" binding:"required"`
	EANGTIN           *string  `json:"ean_gtin,omitempty"`
	Producto          string   `json:"producto" binding:"required"`
	Marca             *string  `json:"marca,omitempty"`
	Categoria         *string  `json:"categoria,omitempty"`
	FormatoCrudo      *string  `json:"formato_crudo,omitempty"`
	Precio            *float64 `json:"precio,omitempty"`
	PrecioNormal      float64  `json:"precio_normal" binding:"required"`
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

// IngestaResultadoDTO resume el resultado de procesar un lote de productos,
// diseñado con métricas claras para respuestas de API y alertas del bot de Discord.
type IngestaResultadoDTO struct {
	SucursalID         int      `json:"sucursal_id"`
	TotalRecibidos     int      `json:"total_recibidos"`
	Insertados         int      `json:"insertados"`
	Actualizados       int      `json:"actualizados"`
	PreciosRegistrados int      `json:"precios_registrados"`
	Errores            []string `json:"errores,omitempty"`
	Mensaje            string   `json:"mensaje"`
}

// Request contiene únicamente referencias de Catálogo; ningún trabajo se escribe aquí.
type Request struct {
	SucursalID int                    `json:"sucursal_id"`
	Productos  []ProductoScrapeadoDTO `json:"productos"`
}
