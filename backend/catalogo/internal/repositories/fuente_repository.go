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

// consultaFuente es la consulta base: la cadena junto con el conteo de sucursales
// y productos y la última captura de precios.
const consultaFuente = `
	SELECT
		c.id,
		c.nombre,
		c.url_sitio_web,
		c.url_logo,
		c.esta_activa,
		c.creado_el,
		COALESCE(COUNT(DISTINCT s.id), 0) AS total_sucursales,
		COALESCE(COUNT(DISTINCT p.id), 0) AS total_productos,
		MAX(cp.capturado_el) AS ultima_extraccion
	FROM scraper.cadenas_supermercado c
	LEFT JOIN scraper.sucursales_supermercado s ON s.cadena_id = c.id
	LEFT JOIN scraper.productos_crudos p ON p.sucursal_id = s.id
	LEFT JOIN scraper.capturas_precios cp ON cp.producto_crudo_id = p.id
`

const agrupacionFuente = `
	GROUP BY c.id, c.nombre, c.url_sitio_web, c.url_logo, c.esta_activa, c.creado_el
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
	if err := r.db.WithContext(ctx).Raw(consultaFuente + agrupacionFuente + "ORDER BY c.id ASC").Scan(&rows).Error; err != nil {
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
	query := consultaFuente + "WHERE c.id = ?" + agrupacionFuente
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
// actualizada con una única consulta posterior.
func (r *gormFuenteRepository) ActualizarEstadoFuente(ctx context.Context, id int, habilitada bool) (*domain.FuenteDTO, error) {
	res := r.db.WithContext(ctx).
		Table("scraper.cadenas_supermercado").
		Where("id = ?", id).
		Update("esta_activa", habilitada)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	return r.ObtenerFuente(ctx, id)
}
