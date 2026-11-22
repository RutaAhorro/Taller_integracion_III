// Package repositories implementa los contratos de domain sobre PostgreSQL
// (rol app_scraping), Redis Streams y la API HTTP interna de Catálogo. No
// contiene reglas de negocio ni lee variables de entorno.
package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ===== Límites de conexión =====

const (
	// Un pool pequeño por réplica: Control sólo coordina y no hace lecturas masivas.
	maximoConexionesAbiertas  = 5
	maximoConexionesInactivas = 2
	vidaMaximaConexion        = 30 * time.Minute
	// PlazoRedis acota conexión, lectura y escritura de cada comando Redis.
	PlazoRedis = time.Second
)

// tablasCoordinacion son las tablas que Control lee y escribe en runtime.
var tablasCoordinacion = []string{"scraper.trabajos_scraper", "scraper.entregas_ingesta", "scraper.entregas_trabajos"}

// ===== PostgreSQL =====

// Abrir configura el pool PostgreSQL del servicio y confirma que puede conectarse.
func Abrir(contexto context.Context, cadenaConexion string) (*gorm.DB, error) {
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
	baseDatosSQL.SetMaxOpenConns(maximoConexionesAbiertas)
	baseDatosSQL.SetMaxIdleConns(maximoConexionesInactivas)
	baseDatosSQL.SetConnMaxLifetime(vidaMaximaConexion)
	if errorObtenido := baseDatosSQL.PingContext(contexto); errorObtenido != nil {
		_ = baseDatosSQL.Close()
		return nil, errors.New("no se pudo conectar a PostgreSQL con el rol configurado")
	}
	return baseDatos, nil
}

// VerificarAlmacenamiento comprueba tablas y permisos antes de aceptar tráfico.
// UPDATE y DELETE se verifican porque la coordinación actualiza entregas y
// depura outboxes al cerrar trabajos; sin ellos no completa el flujo durable.
func VerificarAlmacenamiento(contexto context.Context, baseDatos *gorm.DB) error {
	filas, errorObtenido := baseDatos.WithContext(contexto).Raw("SELECT cadena_nombre FROM scraper.trabajos_scraper LIMIT 0").Rows()
	if errorObtenido != nil {
		return errors.New("aplicar 06_propiedad_permisos.sql y permisos de coordinación antes de iniciar la API")
	}
	_ = filas.Close()
	for _, tabla := range tablasCoordinacion {
		filas, errorObtenido := baseDatos.WithContext(contexto).Raw("SELECT * FROM " + tabla + " LIMIT 0").Rows()
		if errorObtenido != nil {
			return errors.New("aplicar 05_recuperacion_scraping.sql y permisos de coordinación antes de iniciar la API")
		}
		_ = filas.Close()
		var permitido bool
		if errorObtenido := baseDatos.WithContext(contexto).Raw(`SELECT
            has_table_privilege(current_user, ?, 'SELECT') AND
            has_table_privilege(current_user, ?, 'INSERT') AND
            has_table_privilege(current_user, ?, 'UPDATE')`, tabla, tabla, tabla).Scan(&permitido).Error; errorObtenido != nil || !permitido {
			return errors.New("faltan permisos runtime de coordinación; revisar 06_propiedad_permisos.sql")
		}
	}
	var puedeEliminar bool
	if errorObtenido := baseDatos.WithContext(contexto).Raw("SELECT has_table_privilege(current_user, 'scraper.entregas_trabajos', 'DELETE')").Scan(&puedeEliminar).Error; errorObtenido != nil || !puedeEliminar {
		return errors.New("conceder DELETE en scraper.entregas_trabajos al rol de coordinación antes de iniciar la API")
	}
	return nil
}

// Cerrar libera el pool SQL asociado a la conexión GORM.
func Cerrar(baseDatos *gorm.DB) {
	if baseDatosSQL, errorObtenido := baseDatos.DB(); errorObtenido == nil {
		_ = baseDatosSQL.Close()
	}
}

// ===== Redis =====

// NuevoClienteRedis crea un cliente sin reintentos y con plazos cortos. La URL
// ya fue validada por config. Con ContextTimeoutEnabled los comandos respetan
// el contexto del probe en vez de los timeouts de socket de go-redis.
func NuevoClienteRedis(direccionRedis string) *redis.Client {
	opciones, _ := redis.ParseURL(direccionRedis)
	return redis.NewClient(limitarOpcionesRedis(opciones))
}

// limitarOpcionesRedis aplica los plazos y la política sin reintentos.
func limitarOpcionesRedis(opciones *redis.Options) *redis.Options {
	opciones.MaxRetries = -1
	opciones.ContextTimeoutEnabled = true
	opciones.DialTimeout = PlazoRedis
	opciones.ReadTimeout = PlazoRedis
	opciones.WriteTimeout = PlazoRedis
	return opciones
}
