// Package listas define el modelo de las listas de compra y sus artículos
// (SUP-240, SUP-241), su persistencia en PostgreSQL (postgres.go) y la
// consulta de disponibilidad a Catálogo (catalogo.go).
//
// Los nombres JSON van en snake_case, igual que en Catálogo. La forma de los
// artículos sigue docs/Contratos/CONTRATO_ARTICULOS_LISTA.md.
package listas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// =============================================================================
// Reglas del dominio
// =============================================================================

// LargoMaximoNombre coincide con la columna rutas.listas_compras.nombre VARCHAR(150).
const LargoMaximoNombre = 150

// Estados lista los estados válidos de una lista.
var Estados = map[string]bool{"activa": true, "completada": true, "archivada": true}

// ErrNombre se devuelve cuando el nombre queda vacío o es demasiado largo.
// No repite el valor recibido.
var ErrNombre = errors.New(fmt.Sprintf("debe tener entre 1 y %d caracteres", LargoMaximoNombre))

// Estados de un producto según Catálogo (SUP-312). Motor no los calcula: los
// copia de la consulta de disponibilidad o usa "desconocido" si Catálogo no
// respondió al mostrar la lista.
const (
	EstadoDisponible   = "disponible"
	EstadoSinStock     = "sin_stock"
	EstadoNoEncontrado = "no_encontrado"
	EstadoDesconocido  = "desconocido"
)

// Topes de artículos del contrato (CONTRATO_ARTICULOS_LISTA.md, sección 2).
// Son constantes y no variables de entorno: sólo frenan abusos (cantidades o
// listas enormes que encarecen la consulta a Catálogo), ningún entorno necesita
// otro valor y el frontend muestra los mismos números. Cambiarlos es editar
// aquí y el contrato a la vez.
const (
	MaxArticulosPorLista  = 100 // una compra grande del mes ronda 40–60 productos
	MaxEnvasesPorArticulo = 99  // máximo habitual de un selector de cantidad
)

// LimitesContrato son los topes que usa el servicio en producción.
var LimitesContrato = Limites{MaxArticulos: MaxArticulosPorLista, MaxCantidad: MaxEnvasesPorArticulo}

// Limites agrupa los topes que recibe el servidor. Existe para que las pruebas
// usen valores pequeños; en producción siempre es LimitesContrato.
type Limites struct {
	MaxArticulos int // artículos distintos por lista
	MaxCantidad  int // envases por artículo
}

// ErrMaxArticulos indica que la lista ya tiene el máximo de artículos distintos.
var ErrMaxArticulos = errors.New("la lista alcanzó el máximo de artículos")

// ErrMaxCantidad indica que sumar la cantidad pedida superaría el máximo por artículo.
var ErrMaxCantidad = errors.New("el artículo superaría la cantidad máxima")

// =============================================================================
// Modelos (forma del JSON de respuesta)
// =============================================================================

// Lista es el resumen de una lista de compra.
type Lista struct {
	ID             string    `json:"id"`              // UUID
	Nombre         string    `json:"nombre"`          // ya normalizado
	Estado         string    `json:"estado"`          // activa, completada o archivada
	TotalArticulos int       `json:"total_articulos"` // cantidad de artículos en la lista
	CreadaEl       time.Time `json:"creada_el"`       // siempre en UTC
	ActualizadaEl  time.Time `json:"actualizada_el"`  // siempre en UTC
}

// Articulo es un producto guardado en una lista, tal como lo persiste Motor.
// ProductoID es el UUID de scraper.productos_crudos que publica Catálogo; es
// nil sólo en artículos anteriores a la migración 11, que no se pudieron resolver.
type Articulo struct {
	ID           int64   `json:"id"`
	ProductoID   *string `json:"producto_id"`
	Cantidad     int     `json:"cantidad"`      // envases, no gramos ni porciones
	EstaComprado bool    `json:"esta_comprado"` // marcado por el usuario
}

// ProductoCatalogo son los datos que Catálogo devuelve de un producto, sólo
// para mostrarlos. Motor no los guarda: siempre se leen al momento.
type ProductoCatalogo struct {
	Nombre       string   `json:"nombre"`
	Marca        string   `json:"marca"`
	Supermercado string   `json:"supermercado"`
	Unidad       string   `json:"unidad"`
	Precio       *float64 `json:"precio"` // nil si Catálogo no tiene una captura vigente
	URLImagen    string   `json:"url_imagen"`
}

// ArticuloDetalle es el artículo con su estado en Catálogo. Lo devuelven el
// detalle de la lista y el alta de un artículo.
type ArticuloDetalle struct {
	Articulo
	EstadoProducto string            `json:"estado_producto"`
	Producto       *ProductoCatalogo `json:"producto"` // null si Catálogo no tiene datos
}

// ListaDetalle es la lista con sus artículos (respuesta de GET /listas/{id}).
type ListaDetalle struct {
	Lista
	Articulos []ArticuloDetalle `json:"articulos"` // nunca null: lista vacía es []
}

// =============================================================================
// Contrato de persistencia
// =============================================================================

// Repositorio es lo que el servidor HTTP necesita de la base de datos.
//
// Todas las operaciones filtran por el dueño (usuarioID). Cuando la lista no
// existe para ese usuario se devuelve nil (o false), nunca un error: así una
// lista ajena es indistinguible de una inexistente.
type Repositorio interface {
	Contar(contexto context.Context, usuarioID string) (int, error)
	Listar(contexto context.Context, usuarioID string, limite int) ([]Lista, error)
	Crear(contexto context.Context, usuarioID, nombre string) (Lista, error)
	Obtener(contexto context.Context, usuarioID, listaID string) (*ListaDetalle, error)
	Actualizar(contexto context.Context, usuarioID, listaID string, nombre, estado *string) (*Lista, error)
	Eliminar(contexto context.Context, usuarioID, listaID string) (bool, error)

	// AgregarArticulo guarda el producto o, si ya estaba en la lista, suma la
	// cantidad. creado es true si el artículo es nuevo. Devuelve ErrMaxArticulos
	// o ErrMaxCantidad si se supera un tope.
	AgregarArticulo(contexto context.Context, usuarioID, listaID, productoID string, cantidad int, limites Limites) (articulo *Articulo, creado bool, errorObtenido error)
	// ActualizarArticulo cambia cantidad y/o estado comprado; nil deja el valor.
	ActualizarArticulo(contexto context.Context, usuarioID, listaID string, articuloID int64, cantidad *int, estaComprado *bool) (*Articulo, error)
	// EliminarArticulo quita el artículo y devuelve true si existía en esa lista.
	EliminarArticulo(contexto context.Context, usuarioID, listaID string, articuloID int64) (bool, error)
}

// Catalogo es lo que Motor necesita de Catálogo: el estado de varios productos
// en una sola llamada. Devuelve ErrCatalogoNoDisponible si no pudo consultarlo.
type Catalogo interface {
	Consultar(contexto context.Context, productoIDs []string) (map[string]Disponibilidad, error)
}

// Disponibilidad es el estado de un producto según Catálogo.
type Disponibilidad struct {
	Estado   string
	Producto *ProductoCatalogo
}

// =============================================================================
// Normalización
// =============================================================================

// NormalizarNombre limpia el nombre que escribe el usuario:
//   - elimina caracteres de control y de formato (por ejemplo, el espacio de ancho cero);
//   - convierte tabulaciones y saltos de línea en espacios;
//   - colapsa espacios repetidos y quita los de los extremos;
//   - conserva tildes y ñ.
//
// El largo se mide en caracteres, no en bytes ("ñ" cuenta como 1).
func NormalizarNombre(valor string) (string, error) {
	var constructor strings.Builder
	for _, caracter := range valor {
		switch {
		case unicode.IsSpace(caracter):
			constructor.WriteRune(' ')
		case unicode.Is(unicode.Cc, caracter) || unicode.Is(unicode.Cf, caracter):
			// se descarta: control (Cc) o formato invisible (Cf)
		default:
			constructor.WriteRune(caracter)
		}
	}

	nombreLimpio := strings.Join(strings.Fields(constructor.String()), " ")
	largo := utf8.RuneCountInString(nombreLimpio)
	if largo < 1 || largo > LargoMaximoNombre {
		return "", ErrNombre
	}
	return nombreLimpio, nil
}
