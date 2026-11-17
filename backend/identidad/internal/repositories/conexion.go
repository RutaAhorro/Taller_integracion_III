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
func Open(ctx context.Context, dsn string, ajustes config.Ajustes) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, errors.New("no se pudo configurar PostgreSQL de Identidad")
	}
	pool, err := db.DB()
	if err != nil {
		return nil, errors.New("no se pudo obtener el pool de Identidad")
	}
	pool.SetMaxOpenConns(ajustes.DBMaxOpen)
	pool.SetMaxIdleConns(ajustes.DBMaxIdle)
	pool.SetConnMaxLifetime(ajustes.DBMaxLifetime)
	if pool.PingContext(ctx) != nil {
		pool.Close()
		return nil, errors.New("no se pudo conectar a PostgreSQL de Identidad")
	}
	return db, nil
}
