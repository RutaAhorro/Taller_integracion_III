// Equivalencias por EAN: lectura de los productos solicitados y de todos los
// que comparten su GTIN normalizado, con la captura de precio más reciente de
// cada uno. No crea tablas ni una plataforma canónica: agrupa sobre los datos
// que ya persiste Catálogo.

package repositories

import (
	"context"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// ===== Repositorio =====

// RepositorioEquivalencias implementa domain.RepositorioEquivalencias.
type RepositorioEquivalencias struct {
	baseDatos *gorm.DB
}

// NuevoRepositorioEquivalencias crea el repositorio sobre la conexión compartida.
func NuevoRepositorioEquivalencias(baseDatos *gorm.DB) *RepositorioEquivalencias {
	return &RepositorioEquivalencias{baseDatos: baseDatos}
}

// ===== Consulta por lote =====

// eanNormalizadoProducto deja sólo dígitos y conserva los ceros iniciales: el
// código se compara como texto, nunca como número. La misma expresión se usa
// para los productos solicitados y para sus candidatos.
const eanNormalizadoProducto = `regexp_replace(COALESCE(pc.ean_gtin, ''), '[^0-9]', '', 'g')`

// ConsultarPorIDs devuelve los productos solicitados y, en la misma consulta,
// todos los productos cuyo EAN normalizado coincide con alguno de ellos. La
// agrupación y el descarte de códigos inválidos quedan en el servicio; aquí
// sólo se leen filas del dominio Catálogo con su última captura.
func (repositorio *RepositorioEquivalencias) ConsultarPorIDs(contexto context.Context, productoIDs []string) ([]domain.ProductoEquivalenciaRegistro, error) {
	registros := []domain.ProductoEquivalenciaRegistro{}

	consulta := repositorio.baseDatos.WithContext(contexto).
		Table("scraper.productos_crudos pc").
		Joins("JOIN scraper.sucursales_supermercado ss ON pc.sucursal_id = ss.id").
		Joins("JOIN scraper.cadenas_supermercado cs ON ss.cadena_id = cs.id").
		Joins(`LEFT JOIN LATERAL (
			SELECT precio_normal, precio_oferta, esta_disponible, capturado_el
			FROM scraper.capturas_precios
			WHERE producto_crudo_id = pc.id
			ORDER BY capturado_el DESC
			LIMIT 1
		) cp ON true`).
		Where(`pc.id = ANY(?::uuid[]) OR (`+eanNormalizadoProducto+` <> '' AND `+eanNormalizadoProducto+` IN (
			SELECT regexp_replace(COALESCE(solicitado.ean_gtin, ''), '[^0-9]', '', 'g')
			FROM scraper.productos_crudos solicitado
			WHERE solicitado.id = ANY(?::uuid[])
		))`, pq.Array(productoIDs), pq.Array(productoIDs)).
		Select(`
			pc.id::text AS producto_id,
			pc.ean_gtin AS codigo_ean,
			pc.titulo_crudo AS nombre,
			COALESCE(pc.marca_cruda, '') AS marca,
			COALESCE(pc.formato_crudo, '') AS formato,
			pc.sucursal_id AS sucursal_id,
			cs.nombre AS cadena,
			ss.nombre AS sucursal,
			cp.esta_disponible AS en_stock,
			cp.precio_normal,
			cp.precio_oferta,
			cp.capturado_el
		`)

	if errorConsulta := consulta.Scan(&registros).Error; errorConsulta != nil {
		return nil, errorConsulta
	}
	return registros, nil
}
