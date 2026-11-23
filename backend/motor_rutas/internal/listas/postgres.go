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

// columnasArticulo es el SELECT común de un artículo, en el orden que espera
// escanearArticulo. producto_id es NULL sólo en filas previas a la migración 11.
const columnasArticulo = `articulo.id, articulo.producto_id::text, articulo.cantidad,
    COALESCE(articulo.esta_comprado, false)`

// escanearArticulo lee una fila con las columnas de columnasArticulo.
func escanearArticulo(fila pgx.Row) (Articulo, error) {
	var articulo Articulo
	errorObtenido := fila.Scan(&articulo.ID, &articulo.ProductoID, &articulo.Cantidad, &articulo.EstaComprado)
	return articulo, errorObtenido
}

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
        SELECT `+columnasArticulo+`
        FROM rutas.articulos_lista_compras articulo
        WHERE articulo.lista_id = $1
        ORDER BY articulo.id`, listaID)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	defer filas.Close()

	// El estado en Catálogo lo completa el handler: el repositorio sólo lee rutas.*.
	detalle := &ListaDetalle{Lista: lista, Articulos: []ArticuloDetalle{}}
	for filas.Next() {
		articulo, errorObtenido := escanearArticulo(filas)
		if errorObtenido != nil {
			return nil, errorObtenido
		}
		detalle.Articulos = append(detalle.Articulos, ArticuloDetalle{Articulo: articulo})
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
// Sus artículos se eliminan en cascada por la FK interna.
func (repositorio *Postgres) Eliminar(contexto context.Context, usuarioID, listaID string) (bool, error) {
	resultadoComando, errorObtenido := repositorio.pool.Exec(contexto,
		`DELETE FROM rutas.listas_compras WHERE id = $1 AND usuario_id = $2`, listaID, usuarioID)
	return resultadoComando.RowsAffected() == 1, errorObtenido
}

// =============================================================================
// Artículos (SUP-241)
// =============================================================================

// ListaEsDelUsuario comprueba que la lista exista y sea del usuario, sin
// bloquearla: AgregarArticulo lo vuelve a comprobar dentro de su transacción.
func (repositorio *Postgres) ListaEsDelUsuario(contexto context.Context, usuarioID, listaID string) (bool, error) {
	var existe bool
	errorObtenido := repositorio.pool.QueryRow(contexto, `
        SELECT EXISTS (SELECT 1 FROM rutas.listas_compras WHERE id = $1 AND usuario_id = $2)`,
		listaID, usuarioID).Scan(&existe)
	return existe, errorObtenido
}

// AgregarArticulo guarda el producto en la lista o suma la cantidad si ya
// estaba. Bloquea la fila de la lista durante la transacción: así dos altas
// simultáneas no superan juntas el máximo de artículos ni duplican el producto.
func (repositorio *Postgres) AgregarArticulo(contexto context.Context, usuarioID, listaID, productoID string, cantidad int, limites Limites) (*Articulo, bool, error) {
	transaccion, errorObtenido := repositorio.pool.Begin(contexto)
	if errorObtenido != nil {
		return nil, false, errorObtenido
	}
	defer transaccion.Rollback(contexto) // no hace nada si ya se confirmó

	// --- La lista debe ser del usuario ---------------------------------------
	errorObtenido = transaccion.QueryRow(contexto, `
        SELECT 1 FROM rutas.listas_compras
        WHERE id = $1 AND usuario_id = $2
        FOR UPDATE`, listaID, usuarioID).Scan(new(int))
	if errors.Is(errorObtenido, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if errorObtenido != nil {
		return nil, false, errorObtenido
	}

	// --- Sumar si ya existe; si no, insertar respetando el tope ---------------
	var (
		articuloID     int64
		cantidadActual int
	)
	errorObtenido = transaccion.QueryRow(contexto, `
        SELECT id, cantidad FROM rutas.articulos_lista_compras
        WHERE lista_id = $1 AND producto_id = $2`, listaID, productoID).Scan(&articuloID, &cantidadActual)
	creado := errors.Is(errorObtenido, pgx.ErrNoRows)
	if errorObtenido != nil && !creado {
		return nil, false, errorObtenido
	}

	var articulo Articulo
	if creado {
		var totalArticulos int
		if errorObtenido := transaccion.QueryRow(contexto,
			`SELECT count(*) FROM rutas.articulos_lista_compras WHERE lista_id = $1`, listaID).Scan(&totalArticulos); errorObtenido != nil {
			return nil, false, errorObtenido
		}
		if totalArticulos >= limites.MaxArticulos {
			return nil, false, ErrMaxArticulos
		}
		articulo, errorObtenido = escanearArticulo(transaccion.QueryRow(contexto, `
            INSERT INTO rutas.articulos_lista_compras AS articulo (lista_id, producto_id, cantidad)
            VALUES ($1, $2, $3)
            RETURNING `+columnasArticulo, listaID, productoID, cantidad))
	} else {
		if cantidadActual+cantidad > limites.MaxCantidad {
			return nil, false, ErrMaxCantidad
		}
		articulo, errorObtenido = escanearArticulo(transaccion.QueryRow(contexto, `
            UPDATE rutas.articulos_lista_compras articulo
            SET cantidad = articulo.cantidad + $1
            WHERE articulo.id = $2
            RETURNING `+columnasArticulo, cantidad, articuloID))
	}
	if errorObtenido != nil {
		return nil, false, errorObtenido
	}

	// --- Marcar la lista como modificada y confirmar --------------------------
	if _, errorObtenido := transaccion.Exec(contexto,
		`UPDATE rutas.listas_compras SET actualizada_el = now() WHERE id = $1`, listaID); errorObtenido != nil {
		return nil, false, errorObtenido
	}
	if errorObtenido := transaccion.Commit(contexto); errorObtenido != nil {
		return nil, false, errorObtenido
	}
	return &articulo, creado, nil
}

// ActualizarArticulo cambia cantidad y/o estado comprado (COALESCE deja el
// valor si llega nil) y marca la lista como modificada en la misma sentencia.
// Devuelve nil si el artículo no existe en esa lista del usuario.
func (repositorio *Postgres) ActualizarArticulo(contexto context.Context, usuarioID, listaID string, articuloID int64, cantidad *int, estaComprado *bool) (*Articulo, error) {
	articulo, errorObtenido := escanearArticulo(repositorio.pool.QueryRow(contexto, `
        WITH actualizado AS (
            UPDATE rutas.articulos_lista_compras articulo
            SET cantidad = COALESCE($1, articulo.cantidad),
                esta_comprado = COALESCE($2, articulo.esta_comprado)
            FROM rutas.listas_compras lista
            WHERE articulo.id = $3 AND articulo.lista_id = $4
              AND lista.id = articulo.lista_id AND lista.usuario_id = $5
            RETURNING `+columnasArticulo+`
        ), lista_tocada AS (
            UPDATE rutas.listas_compras SET actualizada_el = now()
            WHERE id = $4 AND EXISTS (SELECT 1 FROM actualizado)
        )
        SELECT * FROM actualizado`, cantidad, estaComprado, articuloID, listaID, usuarioID))
	if errors.Is(errorObtenido, pgx.ErrNoRows) {
		return nil, nil
	}
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	return &articulo, nil
}

// EliminarArticulo quita el artículo si pertenece a esa lista del usuario y
// marca la lista como modificada. Devuelve true si se borró.
func (repositorio *Postgres) EliminarArticulo(contexto context.Context, usuarioID, listaID string, articuloID int64) (bool, error) {
	var borrados int
	errorObtenido := repositorio.pool.QueryRow(contexto, `
        WITH borrado AS (
            DELETE FROM rutas.articulos_lista_compras articulo
            USING rutas.listas_compras lista
            WHERE articulo.id = $1 AND articulo.lista_id = $2
              AND lista.id = articulo.lista_id AND lista.usuario_id = $3
            RETURNING articulo.id
        ), lista_tocada AS (
            UPDATE rutas.listas_compras SET actualizada_el = now()
            WHERE id = $2 AND EXISTS (SELECT 1 FROM borrado)
        )
        SELECT count(*) FROM borrado`, articuloID, listaID, usuarioID).Scan(&borrados)
	return borrados == 1, errorObtenido
}
