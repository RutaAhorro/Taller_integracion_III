package ingesta

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// VerifyStorage mantiene Catálogo fuera de readiness si faltan tablas o permisos
// propios. El modo lector de los contratos históricos no requiere escrituras.
func VerifyStorage(ctx context.Context, db *gorm.DB, writer bool) error {
	var schemaAllowed bool
	if err := db.WithContext(ctx).Raw("SELECT has_schema_privilege(current_user, 'scraper', 'USAGE')").Scan(&schemaAllowed).Error; err != nil || !schemaAllowed {
		return errors.New("faltan permisos de esquema de Catálogo")
	}
	tables := []string{"scraper.cadenas_supermercado", "scraper.sucursales_supermercado", "scraper.productos_crudos", "scraper.capturas_precios"}
	if writer {
		tables = append(tables, "scraper.lotes_catalogo")
	}
	for _, table := range tables {
		var permitted bool
		if err := db.WithContext(ctx).Raw("SELECT has_table_privilege(current_user, ?, 'SELECT')", table).Scan(&permitted).Error; err != nil || !permitted {
			return errors.New("faltan tablas o permisos de lectura de Catálogo")
		}
		if writer {
			if err := db.WithContext(ctx).Raw("SELECT has_table_privilege(current_user, ?, 'INSERT')", table).Scan(&permitted).Error; err != nil || !permitted {
				return errors.New("faltan permisos de ingesta de Catálogo")
			}
		}
	}
	if writer {
		var permitted bool
		if err := db.WithContext(ctx).Raw("SELECT has_table_privilege(current_user, 'scraper.productos_crudos', 'UPDATE')").Scan(&permitted).Error; err != nil || !permitted {
			return errors.New("faltan permisos de actualización de productos")
		}
		for _, sequence := range []string{"scraper.cadenas_supermercado_id_seq", "scraper.sucursales_supermercado_id_seq", "scraper.capturas_precios_id_seq"} {
			if err := db.WithContext(ctx).Raw("SELECT has_sequence_privilege(current_user, ?, 'USAGE')", sequence).Scan(&permitted).Error; err != nil || !permitted {
				return errors.New("faltan permisos de secuencias de Catálogo")
			}
		}
	}
	return nil
}
