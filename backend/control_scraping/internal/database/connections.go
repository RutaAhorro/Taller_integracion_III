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

// Config reúne la configuración de conexión propia de Control de scraping.
type Config struct {
	ScrapingURL string
}

// ===== Carga y validación de configuración =====

// No hay fallback a DB_URL ni al administrador de PostgreSQL.
func Load() (Config, error) {
	configuracion := Config{ScrapingURL: os.Getenv("SCRAPING_DB_URL")}
	for _, entrada := range []struct{ nombre, cadenaConexion, rol string }{
		{"SCRAPING_DB_URL", configuracion.ScrapingURL, "app_scraping"},
	} {
		if entrada.cadenaConexion == "" {
			return Config{}, errors.New(entrada.nombre + " es obligatorio")
		}
		analizado, errorObtenido := pgx.ParseConfig(entrada.cadenaConexion)
		if errorObtenido != nil || analizado.User != entrada.rol || analizado.Database == "" || analizado.Password == "" {
			// No devolver el DSN ni el error del parser: pueden contener secretos.
			return Config{}, errors.New(entrada.nombre + " debe usar el rol " + entrada.rol + ", una base y una contraseña explícitos")
		}
	}
	return configuracion, nil
}

// ===== Apertura y cierre de conexiones =====

// Open configura el pool PostgreSQL del servicio y confirma que puede conectarse.
func Open(contexto context.Context, cadenaConexion string) (*gorm.DB, error) {
	baseDatos, errorObtenido := gorm.Open(postgres.Open(cadenaConexion), &gorm.Config{
		DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent),
	})
	if errorObtenido != nil {
		return nil, errors.New("no se pudo configurar PostgreSQL")
	}
	baseDatosSQL, errorObtenido := baseDatos.DB()
	if errorObtenido != nil {
		return nil, errors.New("no se pudo obtener la conexión PostgreSQL")
	}
	// Un pool por API de coordinación; Identidad mantiene su pool independiente.
	baseDatosSQL.SetMaxOpenConns(5)
	baseDatosSQL.SetMaxIdleConns(2)
	baseDatosSQL.SetConnMaxLifetime(30 * time.Minute)
	if errorObtenido := baseDatosSQL.PingContext(contexto); errorObtenido != nil {
		baseDatosSQL.Close()
		return nil, errors.New("no se pudo conectar a PostgreSQL con el rol configurado")
	}
	return baseDatos, nil
}

// Close cierra el pool SQL asociado a la conexión GORM cuando existe.
func Close(baseDatos *gorm.DB) {
	if baseDatos != nil {
		if baseDatosSQL, errorObtenido := baseDatos.DB(); errorObtenido == nil {
			baseDatosSQL.Close()
		}
	}
}
