// Package listas define el modelo de las listas de compra y sus artículos
// (SUP-240, SUP-241, lote SUP-318), su persistencia en PostgreSQL (postgres.go) y la
// consulta de disponibilidad a Catálogo (catalogo.go).
//
// Los nombres JSON van en snake_case, igual que en Catálogo. La forma de los
// artículos sigue docs/Contratos/CONTRATO_ARTICULOS_LISTA.md.
package listas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
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
	// ListaEsDelUsuario indica si la lista existe y pertenece al usuario. Se usa
	// antes de consultar a Catálogo: una lista ajena responde 404 aunque
	// Catálogo esté caído o el producto no exista.
	ListaEsDelUsuario(contexto context.Context, usuarioID, listaID string) (bool, error)
	AgregarArticulo(contexto context.Context, usuarioID, listaID, productoID string, cantidad int, limites Limites) (articulo *Articulo, creado bool, errorObtenido error)
	// ActualizarArticulo cambia cantidad y/o estado comprado; nil deja el valor.
	ActualizarArticulo(contexto context.Context, usuarioID, listaID string, articuloID int64, cantidad *int, estaComprado *bool) (*Articulo, error)
	// EliminarArticulo quita el artículo y devuelve true si existía en esa lista.
	EliminarArticulo(contexto context.Context, usuarioID, listaID string, articuloID int64) (bool, error)

	// HuellaLote devuelve la huella del lote ya aplicado con esa clave en la
	// lista del usuario; existe es false si la clave no se usó todavía.
	HuellaLote(contexto context.Context, usuarioID, listaID, clave string) (huella string, existe bool, errorObtenido error)
	// AgregarLote aplica todos los artículos del lote o ninguno (SUP-318) y
	// registra la clave de idempotencia en la misma transacción.
	AgregarLote(contexto context.Context, usuarioID, listaID string, lote Lote, limites Limites) (ResultadoLote, error)
}

// =============================================================================
// Agregado en lote (SUP-318)
// =============================================================================

// Motivos de rechazo de un producto del lote (detalle del 409 del contrato).
const (
	MotivoNoEncontrado   = "no_encontrado"   // Catálogo no tiene el producto
	MotivoCantidadMaxima = "cantidad_maxima" // la cantidad, sumada a la que ya hay, supera el tope
)

// ArticuloLote es un producto pedido en el lote, ya validado y sin repetidos.
type ArticuloLote struct {
	ProductoID string
	Cantidad   int
}

// Lote es lo que se guarda de una vez: los artículos, la Idempotency-Key y la
// huella del contenido que se registra junto a la clave.
type Lote struct {
	Clave     string
	Huella    string
	Articulos []ArticuloLote
}

// Rechazo indica un producto del lote que no se pudo agregar y por qué.
type Rechazo struct {
	ProductoID string `json:"producto_id"`
	Motivo     string `json:"motivo"`
}

// ResultadoLote informa qué pasó con el lote. Si Rechazados no está vacío no
// se guardó nada.
type ResultadoLote struct {
	ListaExiste bool      // false si la lista no existe o es de otro usuario
	Repetido    bool      // la clave ya se había aplicado con la misma huella: no se sumó otra vez
	Rechazados  []Rechazo // productos que superan la cantidad máxima
}

// ErrClaveReutilizada indica que la Idempotency-Key ya se usó en esa lista con
// un lote distinto. Devolver el resultado anterior engañaría al cliente.
var ErrClaveReutilizada = errors.New("la clave de idempotencia ya se usó con otro lote")

// NormalizarLote suma los productos repetidos (regla del contrato) y los
// ordena por producto_id. El orden fijo hace que la huella no dependa del
// orden en que el frontend envía los artículos. La suma se satura en
// math.MaxInt en vez de desbordar: así una suma enorme sigue superando la
// cantidad máxima y el lote se rechaza con 409, en lugar de dar la vuelta a un
// número pequeño y guardarse.
func NormalizarLote(articulos []ArticuloLote) []ArticuloLote {
	sumados := map[string]int{}
	for _, articulo := range articulos {
		sumados[articulo.ProductoID] = sumarSinDesbordar(sumados[articulo.ProductoID], articulo.Cantidad)
	}
	normalizados := make([]ArticuloLote, 0, len(sumados))
	for productoID, cantidad := range sumados {
		normalizados = append(normalizados, ArticuloLote{ProductoID: productoID, Cantidad: cantidad})
	}
	sort.Slice(normalizados, func(izquierda, derecha int) bool {
		return normalizados[izquierda].ProductoID < normalizados[derecha].ProductoID
	})
	return normalizados
}

// sumarSinDesbordar suma dos cantidades positivas y devuelve math.MaxInt si el
// resultado no cabe en un int.
func sumarSinDesbordar(acumulado, cantidad int) int {
	if cantidad > math.MaxInt-acumulado {
		return math.MaxInt
	}
	return acumulado + cantidad
}

// HuellaDe calcula el SHA-256 hexadecimal de un lote ya normalizado.
func HuellaDe(articulos []ArticuloLote) string {
	resumen := sha256.New()
	for _, articulo := range articulos {
		fmt.Fprintf(resumen, "%s:%d;", articulo.ProductoID, articulo.Cantidad)
	}
	return hex.EncodeToString(resumen.Sum(nil))
}

// Catalogo es lo que Motor necesita de Catálogo: el estado de varios productos
// en una sola llamada. Devuelve ErrCatalogoNoDisponible si no pudo consultarlo.
type Catalogo interface {
	Consultar(contexto context.Context, productoIDs []string) (map[string]Disponibilidad, error)
}

// Disponibilidad es el estado de un producto según Catálogo.
type Disponibilidad struct {
	Estado   string            `json:"estado_producto"`
	Producto *ProductoCatalogo `json:"producto"`
}

// =============================================================================
// Contrato de precios comparables con Catálogo (SUP-317 / optimización)
// =============================================================================

// ConsultaPreciosComparables es el cuerpo que Motor envía a Catálogo para
// obtener sucursales candidatas y ofertas equivalentes por EAN dentro del radio.
type ConsultaPreciosComparables struct {
	ProductoIDs      []string `json:"producto_ids"`
	Lat              float64  `json:"lat"`
	Lon              float64  `json:"lon"`
	RadioKm          float64  `json:"radio_km"`
	MaximoSucursales int      `json:"maximo_sucursales"`
}

// SucursalCandidata es una tienda física y activa dentro del radio informado
// por Catálogo.
type SucursalCandidata struct {
	SucursalID  int     `json:"sucursal_id"`
	Nombre      string  `json:"nombre"`
	Cadena      string  `json:"cadena"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	DistanciaKm float64 `json:"distancia_km"`
}

// OfertaComparable es una oferta concreta devuelta por Catálogo para un
// producto de la lista en una sucursal candidata. Precio y EnStock pueden ser
// nil cuando la captura no es vigente o no trae dato.
type OfertaComparable struct {
	ProductoIDOferta string   `json:"producto_id_oferta"`
	SucursalID       int      `json:"sucursal_id"`
	Cadena           string   `json:"cadena"`
	Sucursal         string   `json:"sucursal"`
	Nombre           string   `json:"nombre"`
	Unidad           string   `json:"unidad"`
	Precio           *float64 `json:"precio"`
	EnStock          *bool    `json:"en_stock"`
	EstadoPrecio     string   `json:"estado_precio"`
}

// ProductoPreciosComparable agrupa las ofertas vigentes de un producto pedido.
type ProductoPreciosComparable struct {
	ProductoID string             `json:"producto_id"`
	Estado     string             `json:"estado"`
	Ofertas    []OfertaComparable `json:"ofertas"`
}

// RespuestaPreciosComparables reúne sucursales candidatas, ofertas por producto
// y productos sin ofertas en el radio.
type RespuestaPreciosComparables struct {
	Sucursales []SucursalCandidata         `json:"sucursales"`
	Productos  []ProductoPreciosComparable `json:"productos"`
	SinOfertas []string                    `json:"sin_ofertas"`
}

// CatalogoPrecios consulta precios comparables por sucursal en Catálogo.
// Se declara separada de Catalogo para que los tests de listas/artículos no
// necesiten implementar esta consulta.
type CatalogoPrecios interface {
	ConsultarPreciosComparables(contexto context.Context, consulta ConsultaPreciosComparables) (RespuestaPreciosComparables, error)
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
