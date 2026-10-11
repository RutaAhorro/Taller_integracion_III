package ingesta

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
)

// IngestarLote confirma productos y precios juntos. Un error SQL revierte el lote
// completo; nunca se continúa con una transacción PostgreSQL ya abortada.
func (r *Repository) IngestarLote(ctx context.Context, input Request) (*IngestaResultadoDTO, error) {
	result := &IngestaResultadoDTO{SucursalID: input.SucursalID, TotalRecibidos: len(input.Productos)}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ahora := time.Now()
		for i, p := range input.Productos {
			sku, titulo := strings.TrimSpace(p.SKU), strings.TrimSpace(p.Producto)
			if sku == "" || titulo == "" {
				result.Errores = append(result.Errores, fmt.Sprintf("item %d: SKU o título vacío", i))
				continue
			}
			var imagen *string
			if p.URLImagen != nil && strings.TrimSpace(*p.URLImagen) != "" {
				imagen = p.URLImagen
			} else if p.Imagen != nil && strings.TrimSpace(*p.Imagen) != "" {
				imagen = p.Imagen
			}
			stock := true
			if p.EnStock != nil {
				stock = *p.EnStock
			}
			var product struct {
				ID        string
				Insertado bool
			}
			// La restricción existente uq_sucursal_sku serializa upserts concurrentes.
			// NULL/empty conserva los campos opcionales del producto existente.
			query := tx.Raw(`INSERT INTO scraper.productos_crudos AS pc
    (sucursal_id, sku, ean_gtin, titulo_crudo, marca_cruda, categoria_cruda,
     formato_crudo, url_producto, url_imagen, en_stock, ultima_extraccion_el)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT (sucursal_id, sku) DO UPDATE SET
     titulo_crudo = EXCLUDED.titulo_crudo, en_stock = EXCLUDED.en_stock,
     ultima_extraccion_el = EXCLUDED.ultima_extraccion_el,
     ean_gtin = COALESCE(NULLIF(EXCLUDED.ean_gtin, ''), pc.ean_gtin),
     marca_cruda = COALESCE(NULLIF(EXCLUDED.marca_cruda, ''), pc.marca_cruda),
     categoria_cruda = COALESCE(NULLIF(EXCLUDED.categoria_cruda, ''), pc.categoria_cruda),
     formato_crudo = COALESCE(NULLIF(EXCLUDED.formato_crudo, ''), pc.formato_crudo),
     url_producto = COALESCE(NULLIF(EXCLUDED.url_producto, ''), pc.url_producto),
     url_imagen = COALESCE(NULLIF(EXCLUDED.url_imagen, ''), pc.url_imagen)
    RETURNING id, (xmax = 0) AS insertado`, input.SucursalID, sku, p.EANGTIN,
				titulo, p.Marca, p.Categoria, p.FormatoCrudo, p.URLProducto, imagen, stock, ahora).Scan(&product)
			if query.Error != nil {
				return query.Error
			}
			if product.Insertado {
				result.Insertados++
			} else {
				result.Actualizados++
			}
			oferta := p.PrecioOferta
			if oferta == nil && p.Precio != nil && *p.Precio < p.PrecioNormal {
				oferta = p.Precio
			}
			// INSERT explícito conserva stock=false, sin el default:true de GORM.
			if err := tx.Exec(`INSERT INTO scraper.capturas_precios
    (producto_crudo_id, precio_normal, precio_oferta, precio_tarjeta,
     precio_por_unidad, metrica_unidad, mecanica_promocion, esta_disponible, capturado_el)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, product.ID, p.PrecioNormal, oferta,
				p.PrecioTarjeta, p.PrecioPorUnidad, p.MetricaUnidad, p.MecanicaPromocion, stock, ahora).Error; err != nil {
				return err
			}
			result.PreciosRegistrados++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result.Mensaje = fmt.Sprintf("Lote procesado: %d insertados, %d actualizados, %d precios guardados", result.Insertados, result.Actualizados, result.PreciosRegistrados)
	return result, nil
}
