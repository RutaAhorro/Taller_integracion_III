package database

import (
	"context"
	"errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"time"
)

func Open(ctx context.Context, dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, errors.New("no se pudo configurar PostgreSQL de Identidad")
	}
	pool, err := db.DB()
	if err != nil {
		return nil, errors.New("no se pudo obtener el pool de Identidad")
	}
	pool.SetMaxOpenConns(5)
	pool.SetMaxIdleConns(2)
	pool.SetConnMaxLifetime(30 * time.Minute)
	if pool.PingContext(ctx) != nil {
		pool.Close()
		return nil, errors.New("no se pudo conectar a PostgreSQL de Identidad")
	}
	return db, nil
}
