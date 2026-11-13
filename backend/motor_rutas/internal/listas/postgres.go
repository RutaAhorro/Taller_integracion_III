package listas

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// =============================================================================
// Repositorio PostgreSQL
// =============================================================================

// Postgres implementa Repositorio sobre las tablas rutas.listas_compras y
// rutas.articulos_lista_compras.
//
// Las columnas de fecha son TIMESTAMP sin zona; el pool fija TimeZone=UTC,
// por eso pgx las entrega en UTC.
type Postgres struct{ pool *pgxpool.Pool }

// NuevoPostgres crea el repositorio sobre un pool ya abierto (ver internal/db).
func NuevoPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

// columnasLista es el SELECT común de una lista, en el orden que espera escanearLista.
// El total de artículos se calcula con una subconsulta.
const columnasLista = `lista.id::text, lista.nombre, COALESCE(lista.estado, 'activa'),
    lista.creada_el, lista.actualizada_el,
    (SELECT count(*) FROM rutas.articulos_lista_compras articulo WHERE articulo.lista_id = lista.id)`

// escanearLista lee una fila con las columnas de columnasLista.
func escanearLista(fila pgx.Row) (Lista, error) {
	var lista Lista
	errorObtenido := fila.Scan(&lista.ID, &lista.Nombre, &lista.Estado,
		&lista.CreadaEl, &lista.ActualizadaEl, &lista.TotalArticulos)
	lista.CreadaEl, lista.ActualizadaEl = lista.CreadaEl.UTC(), lista.ActualizadaEl.UTC()
	return lista, errorObtenido
}

// =============================================================================
// Lectura
// =============================================================================

// Contar devuelve cuántas listas tiene el usuario (para el tope por usuario).
func (repositorio *Postgres) Contar(contexto context.Context, usuarioID string) (int, error) {
	var cantidad int
	errorObtenido := repositorio.pool.QueryRow(contexto,
		`SELECT count(*) FROM rutas.listas_compras WHERE usuario_id = $1`, usuarioID).Scan(&cantidad)
	return cantidad, errorObtenido
}

// Listar devuelve hasta `limite` listas del usuario, la más reciente primero.
func (repositorio *Postgres) Listar(contexto context.Context, usuarioID string, limite int) ([]Lista, error) {
	filas, errorObtenido := repositorio.pool.Query(contexto, `
        SELECT `+columnasLista+`
        FROM rutas.listas_compras lista
        WHERE lista.usuario_id = $1
        ORDER BY lista.actualizada_el DESC, lista.id
        LIMIT $2`, usuarioID, limite)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	defer filas.Close()

	resultado := []Lista{} // vacío y no nil: el JSON debe ser [] y no null
	for filas.Next() {
		lista, errorObtenido := escanearLista(filas)
		if errorObtenido != nil {
			return nil, errorObtenido
		}
		resultado = append(resultado, lista)
	}
	return resultado, filas.Err()
}

// Obtener devuelve la lista con sus artículos, o nil si no es del usuario.
func (repositorio *Postgres) Obtener(contexto context.Context, usuarioID, listaID string) (*ListaDetalle, error) {
	// --- Lista (filtrada por dueño) ------------------------------------------
	lista, errorObtenido := escanearLista(repositorio.pool.QueryRow(contexto, `
        SELECT `+columnasLista+`
        FROM rutas.listas_compras lista
        WHERE lista.id = $1 AND lista.usuario_id = $2`, listaID, usuarioID))
	if errors.Is(errorObtenido, pgx.ErrNoRows) {
		return nil, nil
	}
	if errorObtenido != nil {
		return nil, errorObtenido
	}

	// --- Artículos (solo se leen si la lista ya resultó ser del usuario) -----
	filas, errorObtenido := repositorio.pool.Query(contexto, `
        SELECT id, producto_normalizado_id::text, cantidad, COALESCE(esta_comprado, false)
        FROM rutas.articulos_lista_compras
        WHERE lista_id = $1
        ORDER BY id`, listaID)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	defer filas.Close()

	detalle := &ListaDetalle{Lista: lista, Articulos: []Articulo{}}
	for filas.Next() {
		var articulo Articulo
		errorObtenido := filas.Scan(&articulo.ID, &articulo.ProductoNormalizadoID, &articulo.Cantidad, &articulo.EstaComprado)
		if errorObtenido != nil {
			return nil, errorObtenido
		}
		detalle.Articulos = append(detalle.Articulos, articulo)
	}
	return detalle, filas.Err()
}

// =============================================================================
// Escritura
// =============================================================================

// Crear inserta una lista nueva (estado "activa" por defecto en la base).
func (repositorio *Postgres) Crear(contexto context.Context, usuarioID, nombre string) (Lista, error) {
	return escanearLista(repositorio.pool.QueryRow(contexto, `
        INSERT INTO rutas.listas_compras (usuario_id, nombre)
        VALUES ($1, $2)
        RETURNING id::text, nombre, COALESCE(estado, 'activa'), creada_el, actualizada_el, 0`,
		usuarioID, nombre))
}

// Actualizar cambia nombre y/o estado. Un valor nil deja la columna como estaba
// (COALESCE). Devuelve nil si la lista no es del usuario.
func (repositorio *Postgres) Actualizar(contexto context.Context, usuarioID, listaID string, nombre, estado *string) (*Lista, error) {
	lista, errorObtenido := escanearLista(repositorio.pool.QueryRow(contexto, `
        UPDATE rutas.listas_compras lista
        SET nombre = COALESCE($1, lista.nombre),
            estado = COALESCE($2, lista.estado),
            actualizada_el = now()
        WHERE lista.id = $3 AND lista.usuario_id = $4
        RETURNING `+columnasLista, nombre, estado, listaID, usuarioID))
	if errors.Is(errorObtenido, pgx.ErrNoRows) {
		return nil, nil
	}
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	return &lista, nil
}

// Eliminar borra la lista si es del usuario y devuelve true si se borró.
// Sus artículos se eliminan en cascada por la FK interna, que PostgreSQL
// ejecuta como dueño de la tabla (app_rutas no necesita DELETE sobre artículos).
func (repositorio *Postgres) Eliminar(contexto context.Context, usuarioID, listaID string) (bool, error) {
	resultadoComando, errorObtenido := repositorio.pool.Exec(contexto,
		`DELETE FROM rutas.listas_compras WHERE id = $1 AND usuario_id = $2`, listaID, usuarioID)
	return resultadoComando.RowsAffected() == 1, errorObtenido
}
