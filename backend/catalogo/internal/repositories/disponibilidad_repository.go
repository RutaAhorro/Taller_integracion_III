// Package repositories implementa las consultas SQL del catálogo. Este archivo
// lee en lote la disponibilidad y la captura más reciente de cada producto.
package repositories

import (
	"context"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// gormDisponibilidadRepository implementa la consulta de disponibilidad sobre
// las tablas de Catálogo usando GORM.
type gormDisponibilidadRepository struct {
	db *gorm.DB
}

// NewDisponibilidadRepository crea el repositorio de lectura de disponibilidad.
func NewDisponibilidadRepository(db *gorm.DB) domain.DisponibilidadRepository {
	return &gormDisponibilidadRepository{db: db}
}

// ===== Consulta por lote =====

// ConsultarPorIDs obtiene en una sola consulta los productos solicitados y su
// captura de precio más reciente. La lectura usa la PK de productos_crudos y el
// índice (producto_crudo_id, capturado_el DESC) de capturas_precios.
func (repositorio *gormDisponibilidadRepository) ConsultarPorIDs(contexto context.Context, productoIDs []string) ([]domain.ProductoDisponibilidadRegistro, error) {
	registros := []domain.ProductoDisponibilidadRegistro{}
	if len(productoIDs) == 0 {
		return registros, nil
	}

	consulta := repositorio.db.WithContext(contexto).
		Table("scraper.productos_crudos pc").
		Joins("JOIN scraper.sucursales_supermercado ss ON pc.sucursal_id = ss.id").
		Joins("JOIN scraper.cadenas_supermercado cs ON ss.cadena_id = cs.id").
		Joins(`LEFT JOIN LATERAL (
			SELECT precio_normal, precio_oferta, capturado_el
			FROM scraper.capturas_precios
			WHERE producto_crudo_id = pc.id
			ORDER BY capturado_el DESC
			LIMIT 1
		) cp ON true`).
		Where("pc.id = ANY(?::uuid[])", pq.Array(productoIDs)).
		Select(`
			pc.id::text AS producto_id,
			pc.titulo_crudo AS nombre,
			COALESCE(pc.marca_cruda, '') AS marca,
			COALESCE(pc.categoria_cruda, '') AS categoria,
			cs.nombre AS supermercado,
			pc.sucursal_id AS sucursal_id,
			ss.nombre AS sucursal,
			COALESCE(pc.formato_crudo, '') AS unidad,
			COALESCE(pc.url_imagen, '') AS url_imagen,
			pc.en_stock,
			cp.precio_normal,
			cp.precio_oferta,
			cp.capturado_el
		`)

	if errorConsulta := consulta.Scan(&registros).Error; errorConsulta != nil {
		return nil, errorConsulta
	}
	return registros, nil
}
