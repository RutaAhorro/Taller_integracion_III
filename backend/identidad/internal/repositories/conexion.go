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

// Open conecta a PostgreSQL y aplica los límites del pool validados al arrancar.
func Open(contexto context.Context, dsn string, ajustes config.Ajustes) (*gorm.DB, error) {
	baseDatos, errorObtenido := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if errorObtenido != nil {
		return nil, errors.New("no se pudo configurar PostgreSQL de Identidad")
	}
	pool, errorObtenido := baseDatos.DB()
	if errorObtenido != nil {
		return nil, errors.New("no se pudo obtener el pool de Identidad")
	}
	pool.SetMaxOpenConns(ajustes.DBMaxOpen)
	pool.SetMaxIdleConns(ajustes.DBMaxIdle)
	pool.SetConnMaxLifetime(ajustes.DBMaxLifetime)
	if pool.PingContext(contexto) != nil {
		pool.Close()
		return nil, errors.New("no se pudo conectar a PostgreSQL de Identidad")
	}
	return baseDatos, nil
}

// ===== Verificación de almacenamiento =====

// VerifyStorage comprueba las tablas que utiliza Identidad, incluida la
// auditoría administrativa agregada en SUP-242.
func VerifyStorage(contexto context.Context, baseDatos *gorm.DB) error {
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
              AND column_name IN ('plan', 'ultimo_acceso')) = 2`).Scan(&permitido).Error
	if errorObtenido != nil {
		// Conexión, timeout o credenciales: no es un problema de migraciones.
		return fmt.Errorf("no se pudo verificar el almacenamiento de Identidad: %w", errorObtenido)
	}
	if !permitido {
		return errors.New("aplicar 06_propiedad_permisos.sql, 08_normalizar_rol_admin_y_auditoria.sql y 09_plan_y_ultimo_acceso.sql antes de iniciar Identidad")
	}
	return nil
}

// VerificarRecuperacion comprueba api.tokens_recuperacion. A diferencia de VerifyStorage,
// su ausencia no impide iniciar: el login sigue disponible y sólo se apaga la recuperación.
func VerificarRecuperacion(contexto context.Context, baseDatos *gorm.DB) error {
	var permitido bool
	errorObtenido := baseDatos.WithContext(contexto).Raw(`SELECT to_regclass('api.tokens_recuperacion') IS NOT NULL AND
        has_table_privilege(current_user, 'api.tokens_recuperacion', 'SELECT') AND
        has_table_privilege(current_user, 'api.tokens_recuperacion', 'INSERT') AND
        has_table_privilege(current_user, 'api.tokens_recuperacion', 'UPDATE')`).Scan(&permitido).Error
	if errorObtenido != nil {
		return fmt.Errorf("no se pudo verificar la recuperación: %w", errorObtenido)
	}
	if !permitido {
		return errors.New("aplicar 05_tokens_recuperacion.sql y 06_propiedad_permisos.sql para activar la recuperación")
	}
	return nil
}
