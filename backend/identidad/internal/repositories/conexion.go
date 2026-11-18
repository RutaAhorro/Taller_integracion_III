// Apertura de la conexión PostgreSQL de Identidad (rol app_identity).

package repositories

import (
	"context"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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
