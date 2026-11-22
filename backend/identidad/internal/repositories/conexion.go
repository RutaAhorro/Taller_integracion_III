// Apertura de la conexión PostgreSQL de Identidad (rol app_identity) y
// verificación de tablas y permisos antes de abrir HTTP.

package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ===== Conexión =====

// Abrir conecta a PostgreSQL y aplica los límites del pool validados al arrancar.
func Abrir(contexto context.Context, dsn string, ajustes config.Ajustes) (*gorm.DB, error) {
	baseDatos, errorObtenido := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if errorObtenido != nil {
		return nil, errors.New("no se pudo configurar PostgreSQL de Identidad")
	}
	pool, errorObtenido := baseDatos.DB()
	if errorObtenido != nil {
		return nil, errors.New("no se pudo obtener el pool de Identidad")
	}
	pool.SetMaxOpenConns(ajustes.MaximoConexionesAbiertas)
	pool.SetMaxIdleConns(ajustes.MaximoConexionesInactivas)
	pool.SetConnMaxLifetime(ajustes.VidaMaximaConexion)
	if pool.PingContext(contexto) != nil {
		pool.Close()
		return nil, errors.New("no se pudo conectar a PostgreSQL de Identidad")
	}
	return baseDatos, nil
}

// ===== Verificación de almacenamiento =====

// VerificarAlmacenamiento comprueba las tablas que utiliza Identidad, incluida la
// auditoría administrativa agregada en SUP-242, y que la base sólo admita los
// roles vigentes (10_restringir_rol_usuarios.sql). Sin esa restricción una fila
// con un rol desconocido dejaría a su cuenta sin acceso.
func VerificarAlmacenamiento(contexto context.Context, baseDatos *gorm.DB) error {
	var permitido bool
	errorObtenido := baseDatos.WithContext(contexto).Raw(`SELECT
        has_schema_privilege(current_user, 'api', 'USAGE') AND
        has_table_privilege(current_user, 'api.usuarios', 'SELECT') AND
        has_table_privilege(current_user, 'api.usuarios', 'INSERT') AND
        has_table_privilege(current_user, 'api.usuarios', 'UPDATE') AND
        to_regclass('api.auditoria_admin_usuarios') IS NOT NULL AND
        has_table_privilege(current_user, 'api.auditoria_admin_usuarios', 'SELECT') AND
        has_table_privilege(current_user, 'api.auditoria_admin_usuarios', 'INSERT') AND
        has_sequence_privilege(current_user, 'api.auditoria_admin_usuarios_id_seq', 'USAGE') AND
        (SELECT COUNT(*) FROM information_schema.columns
            WHERE table_schema = 'api' AND table_name = 'usuarios'
              AND column_name IN ('plan', 'ultimo_acceso')) = 2 AND
        EXISTS (SELECT 1 FROM pg_constraint
            WHERE conrelid = 'api.usuarios'::regclass AND conname = 'usuarios_rol_check' AND convalidated)`).Scan(&permitido).Error
	if errorObtenido != nil {
		// Conexión, timeout o credenciales: no es un problema de migraciones.
		return fmt.Errorf("no se pudo verificar el almacenamiento de Identidad: %w", errorObtenido)
	}
	if !permitido {
		return errors.New("aplicar 06_propiedad_permisos.sql, 08_normalizar_rol_admin_y_auditoria.sql, 09_plan_y_ultimo_acceso.sql y 10_restringir_rol_usuarios.sql antes de iniciar Identidad")
	}
	return nil
}
