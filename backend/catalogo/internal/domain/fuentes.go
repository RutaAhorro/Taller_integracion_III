// Fuentes de scraping: cadenas de supermercado cuyo flag de activación
// administra Catálogo en scraper.cadenas_supermercado.

package domain

import (
	"context"
	"time"
)

// ===== Fuentes =====

// FuenteDTO es una cadena con sus métricas para el panel de administración.
// `habilitada` refleja la columna esta_activa.
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

// ActualizarFuenteDTO es el cuerpo de PUT /fuentes/:id. El puntero distingue
// un campo ausente de `false`.
type ActualizarFuenteDTO struct {
	Habilitada *bool `json:"habilitada"`
}

// RepositorioFuentes lista las fuentes y cambia sólo su esta_activa.
type RepositorioFuentes interface {
	ListarFuentes(contexto context.Context) ([]FuenteDTO, error)
	ActualizarEstadoFuente(contexto context.Context, fuenteID int, habilitada bool) (*FuenteDTO, error)
}
