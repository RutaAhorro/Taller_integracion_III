package domain

import (
	"time"

	"github.com/google/uuid"
)

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

// TrabajoScraper mapea scraper.trabajos_scraper
type TrabajoScraper struct {
	ID                    uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CadenaID              int        `gorm:"not null;index" json:"cadena_id"`
	CadenaNombre          string     `gorm:"column:cadena_nombre;not null" json:"-"`
	DisparadoPorUsuarioID *uuid.UUID `gorm:"type:uuid" json:"disparado_por_usuario_id,omitempty"`
	Estado                string     `gorm:"type:varchar(30);default:'pendiente';not null" json:"estado"`
	IniciadoEl            time.Time  `gorm:"default:now();not null" json:"iniciado_el"`
	FinalizadoEl          *time.Time `json:"finalizado_el,omitempty"`
	ElementosExtraidos    int        `gorm:"default:0" json:"elementos_extraidos"`
	RegistroErrores       *string    `gorm:"type:text" json:"registro_errores,omitempty"`
}

func (TrabajoScraper) TableName() string {
	return "scraper.trabajos_scraper"
}
