// Modelo del borrador por cuenta y del envío durable; los precios siempre proceden de Catálogo.
package listas

import (
	"context"
	"errors"
)

// ===== Contrato del carrito =====

// ErrVersionCarrito exige reconciliar cambios concurrentes en lugar de sobrescribir otra compra.
var ErrVersionCarrito = errors.New("el carrito cambió; recuperar la versión del servidor")

// ErrCarritoVacio impide preparar un envío sin productos.
var ErrCarritoVacio = errors.New("el carrito está vacío")

// ErrDestinoCarrito no distingue una lista ajena de una eliminada.
var ErrDestinoCarrito = errors.New("la lista de destino no existe")

// ArticuloCarrito guarda sólo referencia y envases, nunca precios o stock obsoletos.
type ArticuloCarrito struct {
	ProductoID string `json:"producto_id"`
	Cantidad   int    `json:"cantidad"`
}

// EnvioCarrito congela la clave y el destino antes de sumar productos a una lista.
type EnvioCarrito struct {
	ListaID string `json:"lista_id"`
	Clave   string `json:"clave"`
}

// ReciboCarrito conserva el resultado de la última confirmación incluso si su respuesta se pierde.
type ReciboCarrito struct {
	EnvioCarrito
	Aplicado   bool      `json:"aplicado"`
	Motivo     string    `json:"motivo,omitempty"`
	Rechazados []Rechazo `json:"rechazados,omitempty"`
}

// Carrito es un borrador versionado independiente del número de listas de la cuenta.
type Carrito struct {
	Version     int64             `json:"version"`
	Articulos   []ArticuloCarrito `json:"articulos"`
	Envio       *EnvioCarrito     `json:"envio"`
	UltimoEnvio *ReciboCarrito    `json:"ultimo_envio"`
}

// RepositorioCarrito permite probar HTTP sin SQL y confirma lista y vaciado en una transacción.
type RepositorioCarrito interface {
	ObtenerCarrito(contexto context.Context, usuarioID string) (Carrito, error)
	GuardarCarrito(contexto context.Context, usuarioID string, version int64, articulos []ArticuloCarrito) (Carrito, error)
	PrepararCarrito(contexto context.Context, usuarioID string, version int64, envio EnvioCarrito) (Carrito, error)
	ConfirmarCarrito(contexto context.Context, usuarioID, clave string, limites Limites, catalogo Catalogo) (Carrito, error)
}

// loteCarrito calcula la huella sobre IDs y envases, compartiendo el contrato idempotente de listas.
func loteCarrito(carrito Carrito) Lote {
	articulos := make([]ArticuloLote, 0, len(carrito.Articulos))
	for _, articulo := range carrito.Articulos {
		articulos = append(articulos, ArticuloLote{ProductoID: articulo.ProductoID, Cantidad: articulo.Cantidad})
	}
	articulos = NormalizarLote(articulos)
	return Lote{Clave: carrito.Envio.Clave, Huella: HuellaDe(articulos), Articulos: articulos}
}
