// Package db abre el pool de conexiones de PostgreSQL con el rol app_rutas.
//
// No hace migraciones: el esquema y los permisos (grants) los crean los
// scripts de infrastructure/db (en particular 06_propiedad_permisos.sql).
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// =============================================================================
// Parámetros de conexión
// =============================================================================

const (
	plazoConexion      = 5 * time.Second  // máximo para abrir el pool y verificar permisos
	vidaMaximaConexion = 30 * time.Minute // recicla conexiones viejas
	plazoConsultaMs    = "5000"           // statement_timeout de PostgreSQL, en milisegundos
)

// consultaPermisos confirma que app_rutas tiene exactamente lo que el servicio usa.
// app_rutas tiene search_path = pg_catalog, por eso todo se califica con "rutas.".
const consultaPermisos = `
SELECT has_schema_privilege(current_user, 'rutas', 'USAGE')
   AND has_table_privilege(current_user, 'rutas.listas_compras', 'SELECT')
   AND has_table_privilege(current_user, 'rutas.listas_compras', 'INSERT')
   AND has_table_privilege(current_user, 'rutas.listas_compras', 'UPDATE')
   AND has_table_privilege(current_user, 'rutas.listas_compras', 'DELETE')
   AND has_table_privilege(current_user, 'rutas.articulos_lista_compras', 'SELECT')`

// =============================================================================
// Apertura del pool
// =============================================================================

// Abrir conecta a la base y comprueba los permisos una sola vez al arrancar.
// Los errores informan solo el tipo de fallo, nunca el DSN (contiene la contraseña).
func Abrir(contexto context.Context, dsn string, maximoConexiones int32) (*pgxpool.Pool, error) {
	// --- Configuración del pool ----------------------------------------------
	configuracionPool, errorObtenido := pgxpool.ParseConfig(dsn)
	if errorObtenido != nil {
		return nil, fmt.Errorf("RUTAS_DB_URL no es un DSN válido")
	}
	configuracionPool.MaxConns = maximoConexiones
	configuracionPool.MinConns = 1
	configuracionPool.MaxConnLifetime = vidaMaximaConexion
	configuracionPool.ConnConfig.ConnectTimeout = plazoConexion

	// Parámetros de sesión: nombre visible en pg_stat_activity, plazo por
	// consulta y zona horaria UTC para que las fechas no dependan del servidor.
	parametros := configuracionPool.ConnConfig.RuntimeParams
	parametros["application_name"] = "motor_rutas"
	parametros["statement_timeout"] = plazoConsultaMs
	parametros["TimeZone"] = "UTC"

	// --- Conexión ------------------------------------------------------------
	contexto, cancelar := context.WithTimeout(contexto, plazoConexion)
	defer cancelar()
	pool, errorObtenido := pgxpool.NewWithConfig(contexto, configuracionPool)
	if errorObtenido != nil {
		return nil, fmt.Errorf("no se pudo conectar a PostgreSQL con RUTAS_DB_URL (%T)", errorObtenido)
	}

	// --- Verificación de permisos --------------------------------------------
	var tienePermisos bool
	if errorObtenido := pool.QueryRow(contexto, consultaPermisos).Scan(&tienePermisos); errorObtenido != nil {
		pool.Close()
		return nil, fmt.Errorf("no se pudo verificar los permisos de rutas.* (%T)", errorObtenido)
	}
	if !tienePermisos {
		pool.Close()
		return nil, fmt.Errorf("aplicar 06_propiedad_permisos.sql antes de iniciar el motor de rutas")
	}
	return pool, nil
}

// =============================================================================
// Salud
// =============================================================================

// Ping responde true si la base contesta un "SELECT 1" dentro del plazo.
// Lo usa la sonda /_rutas/ready.
func Ping(contexto context.Context, pool *pgxpool.Pool, plazo time.Duration) bool {
	contexto, cancelar := context.WithTimeout(contexto, plazo)
	defer cancelar()
	_, errorObtenido := pool.Exec(contexto, "SELECT 1")
	return errorObtenido == nil
}
