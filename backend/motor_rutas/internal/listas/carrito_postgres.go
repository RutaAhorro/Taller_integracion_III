// Persistencia del borrador y confirmación atómica: lista, recibo y carrito comparten transacción.
package listas

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"
)

// ===== Lectura y control de concurrencia =====

// ObtenerCarrito no crea filas al consultar; una cuenta nueva comienza en versión cero.
func (repositorio *Postgres) ObtenerCarrito(contexto context.Context, usuarioID string) (Carrito, error) {
	return leerCarrito(repositorio.pool.QueryRow(contexto, `
		SELECT version, articulos, envio, ultimo_envio FROM rutas.carritos WHERE usuario_id = $1`, usuarioID))
}

// leerCarrito decodifica sólo datos propios; el usuario jamás procede del documento JSON.
func leerCarrito(fila pgx.Row) (Carrito, error) {
	carrito := Carrito{Articulos: []ArticuloCarrito{}}
	var articulos, envio, ultimo []byte
	errorObtenido := fila.Scan(&carrito.Version, &articulos, &envio, &ultimo)
	if errors.Is(errorObtenido, pgx.ErrNoRows) {
		return carrito, nil
	}
	if errorObtenido != nil {
		return carrito, errorObtenido
	}
	if errorObtenido = json.Unmarshal(articulos, &carrito.Articulos); errorObtenido != nil {
		return carrito, errorObtenido
	}
	if envio != nil {
		if errorObtenido = json.Unmarshal(envio, &carrito.Envio); errorObtenido != nil {
			return carrito, errorObtenido
		}
	}
	if ultimo != nil {
		errorObtenido = json.Unmarshal(ultimo, &carrito.UltimoEnvio)
	}
	return carrito, errorObtenido
}

// cambioCarrito opera con el bloqueo de la cuenta; false evita incrementar una versión en reintentos.
type cambioCarrito func(carrito *Carrito, transaccion pgx.Tx) (bool, error)

// modificarCarrito serializa pestañas y dispositivos sin necesitar una segunda conexión del pool.
func (repositorio *Postgres) modificarCarrito(contexto context.Context, usuarioID string, cambiar cambioCarrito) (Carrito, error) {
	transaccion, errorObtenido := repositorio.pool.Begin(contexto)
	if errorObtenido != nil {
		return Carrito{}, errorObtenido
	}
	defer transaccion.Rollback(contexto)
	if _, errorObtenido = transaccion.Exec(contexto, `
		INSERT INTO rutas.carritos (usuario_id) VALUES ($1) ON CONFLICT DO NOTHING`, usuarioID); errorObtenido != nil {
		return Carrito{}, errorObtenido
	}
	carrito, errorObtenido := leerCarrito(transaccion.QueryRow(contexto, `
		SELECT version, articulos, envio, ultimo_envio FROM rutas.carritos
		WHERE usuario_id = $1 FOR UPDATE`, usuarioID))
	if errorObtenido != nil {
		return carrito, errorObtenido
	}
	modificado, errorObtenido := cambiar(&carrito, transaccion)
	if errorObtenido != nil {
		return carrito, errorObtenido
	}
	if modificado {
		carrito.Version++
		articulos, errorJSON := json.Marshal(carrito.Articulos)
		if errorJSON != nil {
			return carrito, errorJSON
		}
		var envio, ultimo any
		if carrito.Envio != nil {
			datos, errorJSON := json.Marshal(carrito.Envio)
			if errorJSON != nil {
				return carrito, errorJSON
			}
			envio = datos
		}
		if carrito.UltimoEnvio != nil {
			datos, errorJSON := json.Marshal(carrito.UltimoEnvio)
			if errorJSON != nil {
				return carrito, errorJSON
			}
			ultimo = datos
		}
		if _, errorObtenido = transaccion.Exec(contexto, `
			UPDATE rutas.carritos SET version = $2, articulos = $3, envio = $4,
				ultimo_envio = $5, actualizado_el = now() WHERE usuario_id = $1`,
			usuarioID, carrito.Version, articulos, envio, ultimo); errorObtenido != nil {
			return carrito, errorObtenido
		}
	}
	return carrito, transaccion.Commit(contexto)
}

// ===== Borrador y preparación durable =====

// GuardarCarrito sustituye el borrador sólo si la versión sigue vigente; un envío pendiente se congela.
func (repositorio *Postgres) GuardarCarrito(contexto context.Context, usuarioID string, version int64, articulos []ArticuloCarrito) (Carrito, error) {
	return repositorio.modificarCarrito(contexto, usuarioID, func(carrito *Carrito, _ pgx.Tx) (bool, error) {
		if carrito.Envio != nil {
			return false, ErrVersionCarrito
		}
		if slices.Equal(carrito.Articulos, articulos) {
			return false, nil
		}
		if carrito.Version != version {
			return false, ErrVersionCarrito
		}
		carrito.Articulos = articulos
		return true, nil
	})
}

// PrepararCarrito registra destino y clave antes de enviar; repetir la preparación es inocuo.
func (repositorio *Postgres) PrepararCarrito(contexto context.Context, usuarioID string, version int64, envio EnvioCarrito) (Carrito, error) {
	return repositorio.modificarCarrito(contexto, usuarioID, func(carrito *Carrito, transaccion pgx.Tx) (bool, error) {
		if carrito.Envio != nil && *carrito.Envio == envio {
			return false, nil
		}
		if carrito.UltimoEnvio != nil && carrito.UltimoEnvio.EnvioCarrito == envio {
			return false, nil
		}
		if carrito.Version != version || carrito.Envio != nil {
			return false, ErrVersionCarrito
		}
		if len(carrito.Articulos) == 0 {
			return false, ErrCarritoVacio
		}
		var existe bool
		if errorObtenido := transaccion.QueryRow(contexto, `
			SELECT EXISTS(SELECT 1 FROM rutas.listas_compras WHERE id = $1 AND usuario_id = $2)`,
			envio.ListaID, usuarioID).Scan(&existe); errorObtenido != nil {
			return false, errorObtenido
		}
		if !existe {
			return false, ErrDestinoCarrito
		}
		carrito.Envio = &envio
		return true, nil
	})
}

// ===== Confirmación atómica e idempotente =====

// ConfirmarCarrito aplica el lote y vacía el borrador en un único COMMIT.
// Fallos de red/SQL/Catálogo conservan la preparación; rechazos de negocio conservan los productos
// pero liberan la edición. El recibo permite recuperar una respuesta perdida desde otro dispositivo.
func (repositorio *Postgres) ConfirmarCarrito(contexto context.Context, usuarioID, clave string, limites Limites, catalogo Catalogo) (Carrito, error) {
	return repositorio.modificarCarrito(contexto, usuarioID, func(carrito *Carrito, transaccion pgx.Tx) (bool, error) {
		if carrito.Envio == nil {
			if carrito.UltimoEnvio != nil && carrito.UltimoEnvio.Clave == clave {
				return false, nil
			}
			return false, ErrVersionCarrito
		}
		if carrito.Envio.Clave != clave {
			return false, ErrVersionCarrito
		}
		envio := *carrito.Envio
		// El helper comprueba dueño y ledger antes de esta consulta HTTP.
		validar := func(contexto context.Context, articulos []ArticuloLote) ([]Rechazo, error) {
			productoIDs := make([]string, 0, len(articulos))
			for _, articulo := range articulos {
				productoIDs = append(productoIDs, articulo.ProductoID)
			}
			estados, errorObtenido := catalogo.Consultar(contexto, productoIDs)
			if errorObtenido != nil {
				return nil, ErrCatalogoNoDisponible
			}
			rechazados := []Rechazo{}
			for _, productoID := range productoIDs {
				if estados[productoID].Estado == EstadoNoEncontrado {
					rechazados = append(rechazados, Rechazo{ProductoID: productoID, Motivo: MotivoNoEncontrado})
				}
			}
			return rechazados, nil
		}
		resultado, errorObtenido := agregarLoteTransaccion(contexto, transaccion, usuarioID, envio.ListaID, loteCarrito(*carrito), limites, validar)
		recibo := &ReciboCarrito{EnvioCarrito: envio, Aplicado: true}
		switch {
		case errors.Is(errorObtenido, ErrMaxArticulos):
			recibo.Motivo = "max_articulos"
		case errors.Is(errorObtenido, ErrClaveReutilizada):
			recibo.Motivo = "idempotency_conflict"
		case errorObtenido != nil:
			return false, errorObtenido
		case !resultado.ListaExiste:
			recibo.Motivo = "lista_no_existe"
		case len(resultado.Rechazados) > 0:
			recibo.Motivo, recibo.Rechazados = "productos_rechazados", resultado.Rechazados
		}
		recibo.Aplicado = recibo.Motivo == ""
		if recibo.Aplicado {
			carrito.Articulos = []ArticuloCarrito{}
		}
		carrito.Envio, carrito.UltimoEnvio = nil, recibo
		return true, nil
	})
}
