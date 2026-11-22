// Fuentes: lectura de cadenas con métricas y cambio de esta_activa, la única
// columna de scraper.cadenas_supermercado que Catálogo puede actualizar.

package repositories

import (
	"context"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"gorm.io/gorm"
)

// ===== Repositorio =====

// RepositorioFuentes implementa domain.RepositorioFuentes.
type RepositorioFuentes struct {
	baseDatos *gorm.DB
}

// NuevoRepositorioFuentes crea el repositorio sobre la conexión compartida.
func NuevoRepositorioFuentes(baseDatos *gorm.DB) *RepositorioFuentes {
	return &RepositorioFuentes{baseDatos: baseDatos}
}

// ===== Consultas =====

// camposFuente calcula las métricas del panel con subconsultas escalares.
// Evita unir productos y capturas en una sola consulta: esa combinación genera
// un producto cartesiano que crece con el histórico de precios.
const camposFuente = `
	c.id,
	c.nombre,
	c.url_sitio_web,
	c.url_logo,
	c.esta_activa,
	c.creado_el,
	(SELECT COUNT(*)
	 FROM scraper.sucursales_supermercado s
	 WHERE s.cadena_id = c.id) AS total_sucursales,
	(SELECT COUNT(*)
	 FROM scraper.productos_crudos p
	 JOIN scraper.sucursales_supermercado s2 ON s2.id = p.sucursal_id
	 WHERE s2.cadena_id = c.id) AS total_productos,
	(SELECT MAX(p.ultima_extraccion_el)
	 FROM scraper.productos_crudos p
	 JOIN scraper.sucursales_supermercado s3 ON s3.id = p.sucursal_id
	 WHERE s3.cadena_id = c.id) AS ultima_extraccion
`

const consultaFuente = "SELECT " + camposFuente + `
	FROM scraper.cadenas_supermercado c
`

// actualizarFuente actualiza el flag y devuelve la fuente en una sola consulta.
// El RETURNING alimenta las mismas métricas que usa el listado, sin reconsultar
// la fuente después del UPDATE.
const actualizarFuente = `
	WITH actualizada AS (
		UPDATE scraper.cadenas_supermercado
		SET esta_activa = ?
		WHERE id = ?
		RETURNING id, nombre, url_sitio_web, url_logo, esta_activa, creado_el
	)
	SELECT ` + camposFuente + `
	FROM actualizada c
`

// filaFuente es la fila cruda de la consulta de fuentes.
type filaFuente struct {
	ID               int        `gorm:"column:id"`
	Nombre           string     `gorm:"column:nombre"`
	URLSitioWeb      *string    `gorm:"column:url_sitio_web"`
	URLLogo          *string    `gorm:"column:url_logo"`
	EstaActiva       bool       `gorm:"column:esta_activa"`
	TotalSucursales  int        `gorm:"column:total_sucursales"`
	TotalProductos   int64      `gorm:"column:total_productos"`
	UltimaExtraccion *time.Time `gorm:"column:ultima_extraccion"`
	CreadoEl         time.Time  `gorm:"column:creado_el"`
}

// aDTO traduce la fila al contrato del panel.
func (fila filaFuente) aDTO() domain.FuenteDTO {
	return domain.FuenteDTO{
		ID:               fila.ID,
		Nombre:           fila.Nombre,
		URLSitioWeb:      fila.URLSitioWeb,
		URLLogo:          fila.URLLogo,
		Habilitada:       fila.EstaActiva,
		TotalSucursales:  fila.TotalSucursales,
		TotalProductos:   fila.TotalProductos,
		UltimaExtraccion: fila.UltimaExtraccion,
		CreadoEl:         fila.CreadoEl,
	}
}

// ListarFuentes devuelve todas las cadenas ordenadas por id.
func (repositorio *RepositorioFuentes) ListarFuentes(contexto context.Context) ([]domain.FuenteDTO, error) {
	var filas []filaFuente
	if errorConsulta := repositorio.baseDatos.WithContext(contexto).Raw(consultaFuente + "ORDER BY c.id ASC").Scan(&filas).Error; errorConsulta != nil {
		return nil, errorConsulta
	}
	fuentes := make([]domain.FuenteDTO, 0, len(filas))
	for _, fila := range filas {
		fuentes = append(fuentes, fila.aDTO())
	}
	return fuentes, nil
}

// ActualizarEstadoFuente persiste el flag esta_activa y devuelve la fuente
// actualizada en la misma sentencia SQL.
func (repositorio *RepositorioFuentes) ActualizarEstadoFuente(contexto context.Context, fuenteID int, habilitada bool) (*domain.FuenteDTO, error) {
	var filas []filaFuente
	if errorConsulta := repositorio.baseDatos.WithContext(contexto).Raw(actualizarFuente, habilitada, fuenteID).Scan(&filas).Error; errorConsulta != nil {
		return nil, errorConsulta
	}
	if len(filas) == 0 {
		return nil, nil
	}
	fuente := filas[0].aDTO()
	return &fuente, nil
}
