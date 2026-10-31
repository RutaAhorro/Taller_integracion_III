package domain

import "time"

// FuenteDTO representa una cadena de supermercado como fuente de scraping para
// la administración. El flag `habilitada` es el estado de activación que vive en
// scraper.cadenas_supermercado.
type FuenteDTO struct {
	ID               int        `json:"id"`
	Nombre           string     `json:"nombre"`
	URLSitioWeb      *string    `json:"url_sitio_web,omitempty"`
	URLLogo          *string    `json:"url_logo,omitempty"`
	Habilitada       bool       `json:"habilitada"`
	TotalSucursales  int        `json:"sucursales"`
	TotalProductos   int64      `json:"productos"`
	UltimaExtraccion *time.Time `json:"ultima_extraccion,omitempty"`
	CreadoEl         time.Time  `json:"creado_el"`
}

// ActualizarFuenteDTO define el payload para habilitar o deshabilitar una fuente.
type ActualizarFuenteDTO struct {
	Habilitada *bool `json:"habilitada"`
}
