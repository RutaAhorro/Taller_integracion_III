// Conexión: apertura a PostgreSQL y verificación única, al arrancar, de
// las tablas y permisos que necesita el rol de runtime.

package repositories

import (
	"context"
	"errors"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ===== Conexión =====

// AbrirBaseDatos configura GORM sin ping automático ni logs de SQL: la
// conectividad se comprueba en la verificación de permisos y en readiness.
func AbrirBaseDatos(urlBaseDatos string) (*gorm.DB, error) {
	baseDatos, errorApertura := gorm.Open(postgres.Open(urlBaseDatos), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if errorApertura != nil {
		return nil, errors.New("no se pudo configurar la conexión PostgreSQL")
	}
	return baseDatos, nil
}

// ===== Permisos =====

// Tablas y secuencias que Catálogo lee siempre o escribe con la ingesta.
var (
	tablasLectura       = []string{"scraper.cadenas_supermercado", "scraper.sucursales_supermercado", "scraper.productos_crudos", "scraper.capturas_precios"}
	tablaRecibos        = "scraper.lotes_catalogo"
	secuenciasEscritura = []string{"scraper.cadenas_supermercado_id_seq", "scraper.sucursales_supermercado_id_seq", "scraper.capturas_precios_id_seq"}
)

// VerificarAlmacenamiento valida tablas y permisos una vez, antes de abrir
// HTTP, para no servir con grants incompletos. Sin ingesta (escritor=false)
// basta la lectura: así arranca el lector aislado de los contratos históricos.
func VerificarAlmacenamiento(contexto context.Context, baseDatos *gorm.DB, escritor bool) error {
	consultar := func(sentencia string, argumentos ...any) bool {
		var permitido bool
		return baseDatos.WithContext(contexto).Raw(sentencia, argumentos...).Scan(&permitido).Error == nil && permitido
	}
	if !consultar("SELECT has_schema_privilege(current_user, 'scraper', 'USAGE')") {
		return errors.New("faltan permisos de esquema de Catálogo")
	}
	tablas := tablasLectura
	if escritor {
		tablas = append(append([]string{}, tablasLectura...), tablaRecibos)
	}
	for _, tabla := range tablas {
		if !consultar("SELECT has_table_privilege(current_user, ?, 'SELECT')", tabla) {
			return errors.New("faltan tablas o permisos de lectura de Catálogo")
		}
		if escritor && !consultar("SELECT has_table_privilege(current_user, ?, 'INSERT')", tabla) {
			return errors.New("faltan permisos de ingesta de Catálogo")
		}
	}
	if !escritor {
		return nil
	}
	if !consultar("SELECT has_table_privilege(current_user, 'scraper.productos_crudos', 'UPDATE')") {
		return errors.New("faltan permisos de actualización de scraper.productos_crudos")
	}
	// Catálogo sólo modifica esta_activa: no se concede UPDATE de tabla completa
	// sobre cadenas para no exponer nombre, URLs u otros campos.
	if !consultar("SELECT has_column_privilege(current_user, 'scraper.cadenas_supermercado', 'esta_activa', 'UPDATE')") {
		return errors.New("faltan permisos de actualización de scraper.cadenas_supermercado.esta_activa")
	}
	for _, secuencia := range secuenciasEscritura {
		if !consultar("SELECT has_sequence_privilege(current_user, ?, 'USAGE')", secuencia) {
			return errors.New("faltan permisos de secuencias de Catálogo")
		}
	}
	return nil
}
