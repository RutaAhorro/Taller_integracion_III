// Package optimizacion calcula en qué tiendas conviene comprar una lista, en
// qué orden visitarlas y cuánto cuesta en total, según el contrato del motor
// de rutas (docs/Contratos/CONTRATO_MOTOR_RUTAS.md, SUP-323).
//
// El paquete no hace HTTP ni SQL: recibe sucursales, ofertas y artículos ya
// resueltos y devuelve la alternativa elegida. Así se prueba aislado y el
// endpoint (SUP-316) solo se ocupa de obtener los datos y responder.
//
//   - SUP-313 (asignacion.go): conjuntos de tiendas y asignación de artículos.
//   - SUP-328 (recorrido.go y calculo.go): orden de visita, traslado, elección,
//     base de una tienda, ahorro y umbral de ahorro (RN-08).
//
// El resultado es el mejor solo entre los candidatos y reglas evaluados, no
// entre todas las tiendas que existen. Distancias y costos son estimados.
package optimizacion

import (
	"encoding/json"
	"errors"
	"strings"
)

// =============================================================================
// Entrada del cálculo
// =============================================================================

// Coordenada es un punto geográfico en grados decimales.
type Coordenada struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// Sucursal es una tienda física candidata, con coordenadas reales informadas
// por Catálogo. Las tiendas solo en línea no deben llegar aquí.
type Sucursal struct {
	ID        int        // sucursal_id de Catálogo
	Nombre    string     // nombre visible de la sucursal
	Cadena    string     // cadena a la que pertenece
	Ubicacion Coordenada // coordenadas reales; nunca inventadas
}

// Oferta es el precio de un producto de la lista en una sucursal. Puede ser
// el mismo producto u otro equivalente por EAN exacto (ProductoIDOferta).
type Oferta struct {
	ProductoID       string // producto_id guardado en la lista
	ProductoIDOferta string // producto que efectivamente se compra en la sucursal
	SucursalID       int    // sucursal donde está la oferta
	Precio           int    // CLP por envase
	EnStock          bool   // solo las ofertas con stock se consideran
}

// Articulo es un producto pendiente de la lista con su cantidad en envases.
type Articulo struct {
	ProductoID string // producto_id guardado en la lista
	Cantidad   int    // envases; el mismo EAN implica el mismo envase
}

// Parametros son los valores del cálculo. Los entrega quien llama (SUP-316
// los lee de las variables RUTAS_* y de la solicitud); aquí no se leen del
// entorno para que el algoritmo se pueda probar con valores fijos.
type Parametros struct {
	MaxParadas         int     // RUTAS_MAX_PARADAS
	FactorDistancia    float64 // RUTAS_FACTOR_DISTANCIA, corrige la línea recta
	RendimientoKmL     float64 // de la solicitud o RUTAS_RENDIMIENTO_KM_L_DEFECTO
	PrecioCombustibleL float64 // CLP por litro, de la CNE o del respaldo
	UmbralAhorro       int     // RUTAS_UMBRAL_AHORRO, CLP (RN-08)
}

// Entrada reúne todo lo necesario para un cálculo.
type Entrada struct {
	Origen     Coordenada
	Articulos  []Articulo
	Sucursales []Sucursal
	Ofertas    []Oferta
	Parametros Parametros
}

// =============================================================================
// Resultado (forma del JSON de la sección 3.2 del contrato)
// =============================================================================

// MotivoSinOfertaEnRadio indica que ninguna candidata tiene el producto con stock.
const MotivoSinOfertaEnRadio = "sin_oferta_en_radio"

// ArticuloAsignado es un artículo comprado en una parada.
type ArticuloAsignado struct {
	ProductoID       string `json:"producto_id"`
	ProductoIDOferta string `json:"producto_id_oferta"`
	Cantidad         int    `json:"cantidad"`
	PrecioUnitario   int    `json:"precio_unitario"`
	Subtotal         int    `json:"subtotal"`
}

// Parada es una tienda del recorrido con lo que se compra en ella.
type Parada struct {
	Orden      int                `json:"orden"`
	SucursalID int                `json:"sucursal_id"`
	Nombre     string             `json:"nombre"`
	Cadena     string             `json:"cadena"`
	Lat        float64            `json:"lat"`
	Lon        float64            `json:"lon"`
	Subtotal   int                `json:"subtotal"`
	Articulos  []ArticuloAsignado `json:"articulos"`
}

// ExtremoTramo es el inicio o el fin de un tramo: el origen o una sucursal.
// En JSON el origen se escribe "origen" y una sucursal, su sucursal_id.
type ExtremoTramo struct {
	SucursalID int // 0 representa el origen del usuario
}

// extremoOrigen es el texto con el que el contrato nombra el punto de partida.
const extremoOrigen = "origen"

// MarshalJSON escribe "origen" o el número de sucursal.
func (extremo ExtremoTramo) MarshalJSON() ([]byte, error) {
	if extremo.SucursalID == 0 {
		return json.Marshal(extremoOrigen)
	}
	return json.Marshal(extremo.SucursalID)
}

// Tramo es un trayecto entre dos puntos del recorrido, ya corregido por el factor.
type Tramo struct {
	Desde       ExtremoTramo `json:"desde"`
	Hasta       ExtremoTramo `json:"hasta"`
	DistanciaKm float64      `json:"distancia_km"` // dos decimales
}

// Ruta es la alternativa elegida: paradas en orden, tramos y costos.
type Ruta struct {
	Paradas        []Parada `json:"paradas"`
	Tramos         []Tramo  `json:"tramos"`
	DistanciaKm    float64  `json:"distancia_km"`    // dos decimales
	CostoProductos int      `json:"costo_productos"` // CLP
	CostoTraslado  int      `json:"costo_traslado"`  // CLP, redondeado al final
	Total          int      `json:"total"`           // CLP, redondeado al final
}

// BaseUnaTienda es la mejor alternativa de una sola tienda, usada para el ahorro.
type BaseUnaTienda struct {
	SucursalID int    `json:"sucursal_id"`
	Nombre     string `json:"nombre"`
	Total      int    `json:"total"`
}

// NoCubierto es un artículo que salió del cálculo y se informa al usuario.
type NoCubierto struct {
	ProductoID string `json:"producto_id"`
	Motivo     string `json:"motivo"`
}

// Resultado es la respuesta del cálculo. Los parámetros usados y la fuente del
// combustible los agrega el endpoint, que es quien los conoce.
type Resultado struct {
	Ruta                Ruta           `json:"ruta"`
	BaseUnaTienda       *BaseUnaTienda `json:"base_una_tienda"` // null si ninguna tienda sola alcanza
	Ahorro              *int           `json:"ahorro"`          // null cuando no hay base
	NoCubiertos         []NoCubierto   `json:"no_cubiertos"`    // nunca null
	CandidatasEvaluadas int            `json:"candidatas_evaluadas"`
}

// =============================================================================
// Errores
// =============================================================================

// ErrSinArticulos: la lista no tiene artículos por comprar.
var ErrSinArticulos = errors.New("la lista no tiene artículos por comprar")

// ErrSinSucursales: no hay sucursales candidatas dentro del radio.
var ErrSinSucursales = errors.New("no hay sucursales candidatas")

// ErrParametros: los parámetros o la entrada no permiten un cálculo válido.
// Es un error de programación o de configuración, no del usuario.
var ErrParametros = errors.New("parámetros del cálculo inválidos")

// ErrorSinCobertura indica que no hay forma de comprar la lista: ningún
// artículo tiene oferta, o ningún conjunto de hasta MaxParadas tiendas cubre
// todos los que sí la tienen. NoCubiertos lleva los productos sin oferta.
type ErrorSinCobertura struct {
	NoCubiertos []string
}

// Error describe el fallo sin datos sensibles.
func (fallo *ErrorSinCobertura) Error() string {
	if len(fallo.NoCubiertos) == 0 {
		return "ningún conjunto de tiendas cubre la compra"
	}
	return "productos sin cobertura: " + strings.Join(fallo.NoCubiertos, ", ")
}
