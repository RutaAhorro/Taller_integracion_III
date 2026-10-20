package repositories

import (
	"context"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"gorm.io/gorm"
)

// gormFuenteRepository implementa domain.FuenteRepository sobre
// scraper.cadenas_supermercado, la tabla cuyo flag de activación administra Catálogo.
type gormFuenteRepository struct {
	db *gorm.DB
}

// NewFuenteRepository inicializa el repositorio de fuentes de scraping con GORM.
func NewFuenteRepository(db *gorm.DB) domain.FuenteRepository {
	return &gormFuenteRepository{db: db}
}

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

// fuenteRow es la fila cruda de la consulta de fuentes.
type fuenteRow struct {
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

func (row fuenteRow) aDTO() domain.FuenteDTO {
	return domain.FuenteDTO{
		ID:               row.ID,
		Nombre:           row.Nombre,
		URLSitioWeb:      row.URLSitioWeb,
		URLLogo:          row.URLLogo,
		Habilitada:       row.EstaActiva,
		TotalSucursales:  row.TotalSucursales,
		TotalProductos:   row.TotalProductos,
		UltimaExtraccion: row.UltimaExtraccion,
		CreadoEl:         row.CreadoEl,
	}
}

// ListarFuentes devuelve todas las cadenas ordenadas por id.
func (r *gormFuenteRepository) ListarFuentes(ctx context.Context) ([]domain.FuenteDTO, error) {
	var rows []fuenteRow
	if err := r.db.WithContext(ctx).Raw(consultaFuente + "ORDER BY c.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	fuentes := make([]domain.FuenteDTO, 0, len(rows))
	for _, row := range rows {
		fuentes = append(fuentes, row.aDTO())
	}
	return fuentes, nil
}

// ObtenerFuente devuelve una sola cadena o nil si no existe.
func (r *gormFuenteRepository) ObtenerFuente(ctx context.Context, id int) (*domain.FuenteDTO, error) {
	var rows []fuenteRow
	query := consultaFuente + "WHERE c.id = ?"
	if err := r.db.WithContext(ctx).Raw(query, id).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	fuente := rows[0].aDTO()
	return &fuente, nil
}

// ActualizarEstadoFuente persiste el flag esta_activa y devuelve la fuente
// actualizada en la misma sentencia SQL.
func (r *gormFuenteRepository) ActualizarEstadoFuente(ctx context.Context, id int, habilitada bool) (*domain.FuenteDTO, error) {
	var rows []fuenteRow
	if err := r.db.WithContext(ctx).Raw(actualizarFuente, habilitada, id).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	fuente := rows[0].aDTO()
	return &fuente, nil
}
