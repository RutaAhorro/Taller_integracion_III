// Package database configura conexiones; no contiene entidades ni tablas de dominios.
package database

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	IdentityURL string
	ScrapingURL string
}

// No hay fallback a DB_URL ni al administrador de PostgreSQL.
func Load() (Config, error) {
	cfg := Config{IdentityURL: os.Getenv("IDENTITY_DB_URL"), ScrapingURL: os.Getenv("SCRAPING_DB_URL")}
	for _, entry := range []struct{ name, dsn, role string }{
		{"IDENTITY_DB_URL", cfg.IdentityURL, "app_identity"},
		{"SCRAPING_DB_URL", cfg.ScrapingURL, "app_scraping"},
	} {
		if entry.dsn == "" {
			return Config{}, errors.New(entry.name + " es obligatorio")
		}
		parsed, err := pgx.ParseConfig(entry.dsn)
		if err != nil || parsed.User != entry.role || parsed.Database == "" || parsed.Password == "" {
			// No devolver el DSN ni el error del parser: pueden contener secretos.
			return Config{}, errors.New(entry.name + " debe usar el rol " + entry.role + ", una base y una contraseña explícitos")
		}
	}
	return cfg, nil
}

func Open(ctx context.Context, dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, errors.New("no se pudo configurar PostgreSQL")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.New("no se pudo obtener la conexión PostgreSQL")
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, errors.New("no se pudo conectar a PostgreSQL con el rol configurado")
	}
	return db, nil
}

func Close(db *gorm.DB) {
	if db != nil {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}
}
