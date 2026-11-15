// SUP-313: genera los conjuntos de tiendas candidatos y asigna cada artículo
// a la oferta con stock más barata del conjunto (contrato, sección 2.2).
package optimizacion

import (
	"fmt"
	"math"
	"slices"
)

// =============================================================================
// Tipos de la asignación
// =============================================================================

// Alternativa es un conjunto de tiendas que cubre todos los artículos
// cubribles, con lo que se compra en cada una. Todavía no tiene orden de
// visita ni traslado: eso lo agrega recorrido.go (SUP-328).
type Alternativa struct {
	Sucursales     []Sucursal                 // ordenadas por ID ascendente
	Compras        map[int][]ArticuloAsignado // por sucursal_id, en el orden de la lista
	CostoProductos int                        // CLP, suma de precio × cantidad
}

// Asignacion es el resultado de SUP-313: las alternativas completas y los
// artículos que salieron del cálculo por no tener oferta.
type Asignacion struct {
	Alternativas        []Alternativa
	NoCubiertos         []NoCubierto // nunca null
	CandidatasEvaluadas int          // sucursales candidatas recibidas
}

// mejorOferta es la oferta más barata de un artículo en una sucursal.
type mejorOferta struct {
	productoIDOferta string
	precio           int
}

// =============================================================================
// Asignación
// =============================================================================

// AsignarConjuntos prueba todos los conjuntos de 1 a MaxParadas tiendas y
// devuelve los que cubren la compra completa. Un artículo sin ninguna oferta
// con stock sale del cálculo como no cubierto; el resto se sigue calculando.
//
// Se descarta un conjunto si no cubre todos los artículos cubribles o si
// incluye una tienda donde no se compra nada: ese caso ya se evaluó en el
// conjunto sin esa tienda, que además recorre menos.
func AsignarConjuntos(entrada Entrada) (Asignacion, error) {
	articulos, errorArticulos := agruparArticulos(entrada.Articulos)
	if errorArticulos != nil {
		return Asignacion{}, errorArticulos
	}
	sucursalesPorID, errorSucursales := indexarSucursales(entrada.Sucursales)
	if errorSucursales != nil {
		return Asignacion{}, errorSucursales
	}
	if entrada.Parametros.MaxParadas < 1 {
		return Asignacion{}, fmt.Errorf("%w: MaxParadas debe ser al menos 1", ErrParametros)
	}
	ofertas, errorOfertas := mejoresOfertas(entrada.Ofertas, articulos, sucursalesPorID)
	if errorOfertas != nil {
		return Asignacion{}, errorOfertas
	}

	// --- Artículos sin ninguna oferta: salen del cálculo y se informan ---
	asignacion := Asignacion{NoCubiertos: []NoCubierto{}, CandidatasEvaluadas: len(entrada.Sucursales)}
	var cubribles []Articulo
	for _, articulo := range articulos {
		if len(ofertas[articulo.ProductoID]) == 0 {
			asignacion.NoCubiertos = append(asignacion.NoCubiertos, NoCubierto{ProductoID: articulo.ProductoID, Motivo: MotivoSinOfertaEnRadio})
			continue
		}
		cubribles = append(cubribles, articulo)
	}
	if len(cubribles) == 0 {
		return Asignacion{}, &ErrorSinCobertura{NoCubiertos: idsNoCubiertos(asignacion.NoCubiertos)}
	}

	// --- Solo cuentan las tiendas que venden algo de la lista ---
	var utiles []Sucursal
	for _, sucursal := range entrada.Sucursales {
		for _, articulo := range cubribles {
			if _, existe := ofertas[articulo.ProductoID][sucursal.ID]; existe {
				utiles = append(utiles, sucursal)
				break
			}
		}
	}
	slices.SortFunc(utiles, func(primera, segunda Sucursal) int { return primera.ID - segunda.ID })

	maximoParadas := min(entrada.Parametros.MaxParadas, len(utiles))
	for tamano := 1; tamano <= maximoParadas; tamano++ {
		for _, conjunto := range combinaciones(utiles, tamano) {
			if alternativa, completa := asignarEnConjunto(conjunto, cubribles, ofertas); completa {
				asignacion.Alternativas = append(asignacion.Alternativas, alternativa)
			}
		}
	}
	if len(asignacion.Alternativas) == 0 {
		return Asignacion{}, &ErrorSinCobertura{NoCubiertos: idsNoCubiertos(asignacion.NoCubiertos)}
	}
	return asignacion, nil
}

// asignarEnConjunto compra cada artículo en la tienda más barata del conjunto.
// Ante el mismo subtotal gana la sucursal de menor ID, para que el resultado
// no dependa del orden de llegada de los datos. Devuelve false si falta un
// artículo o si sobra una tienda.
func asignarEnConjunto(conjunto []Sucursal, cubribles []Articulo, ofertas map[string]map[int]mejorOferta) (Alternativa, bool) {
	alternativa := Alternativa{Sucursales: conjunto, Compras: map[int][]ArticuloAsignado{}}
	for _, articulo := range cubribles {
		elegidaID := 0
		var elegida mejorOferta
		for _, sucursal := range conjunto { // ya ordenado por ID
			oferta, existe := ofertas[articulo.ProductoID][sucursal.ID]
			if existe && (elegidaID == 0 || oferta.precio < elegida.precio) {
				elegidaID, elegida = sucursal.ID, oferta
			}
		}
		if elegidaID == 0 {
			return Alternativa{}, false
		}
		subtotal := elegida.precio * articulo.Cantidad
		alternativa.Compras[elegidaID] = append(alternativa.Compras[elegidaID], ArticuloAsignado{
			ProductoID:       articulo.ProductoID,
			ProductoIDOferta: elegida.productoIDOferta,
			Cantidad:         articulo.Cantidad,
			PrecioUnitario:   elegida.precio,
			Subtotal:         subtotal,
		})
		alternativa.CostoProductos += subtotal
	}
	for _, sucursal := range conjunto {
		if len(alternativa.Compras[sucursal.ID]) == 0 {
			return Alternativa{}, false
		}
	}
	return alternativa, true
}

// =============================================================================
// Preparación de la entrada
// =============================================================================

// agruparArticulos valida las cantidades y suma las repetidas de un mismo
// producto, conservando el orden en que aparecen en la lista.
func agruparArticulos(articulos []Articulo) ([]Articulo, error) {
	if len(articulos) == 0 {
		return nil, ErrSinArticulos
	}
	posicionPorProducto := map[string]int{}
	var agrupados []Articulo
	for _, articulo := range articulos {
		if articulo.ProductoID == "" || articulo.Cantidad < 1 {
			return nil, fmt.Errorf("%w: cada artículo necesita producto_id y cantidad positiva", ErrParametros)
		}
		if posicion, repetido := posicionPorProducto[articulo.ProductoID]; repetido {
			agrupados[posicion].Cantidad += articulo.Cantidad
			continue
		}
		posicionPorProducto[articulo.ProductoID] = len(agrupados)
		agrupados = append(agrupados, articulo)
	}
	return agrupados, nil
}

// indexarSucursales rechaza IDs repetidos o no positivos y coordenadas fuera
// de rango: una sucursal sin coordenadas reales no debe llegar al cálculo.
func indexarSucursales(sucursales []Sucursal) (map[int]Sucursal, error) {
	if len(sucursales) == 0 {
		return nil, ErrSinSucursales
	}
	sucursalesPorID := make(map[int]Sucursal, len(sucursales))
	for _, sucursal := range sucursales {
		if _, repetida := sucursalesPorID[sucursal.ID]; repetida || sucursal.ID < 1 {
			return nil, fmt.Errorf("%w: sucursal_id repetido o no positivo (%d)", ErrParametros, sucursal.ID)
		}
		if !coordenadaValida(sucursal.Ubicacion) {
			return nil, fmt.Errorf("%w: coordenadas inválidas en la sucursal %d", ErrParametros, sucursal.ID)
		}
		sucursalesPorID[sucursal.ID] = sucursal
	}
	return sucursalesPorID, nil
}

// mejoresOfertas deja, por artículo y sucursal, la oferta con stock más
// barata. Ignora ofertas de sucursales que no son candidatas o de productos
// que no están en la lista. Ante el mismo precio gana el producto_id_oferta
// menor, para que el resultado sea siempre el mismo.
func mejoresOfertas(ofertas []Oferta, articulos []Articulo, sucursales map[int]Sucursal) (map[string]map[int]mejorOferta, error) {
	resultado := make(map[string]map[int]mejorOferta, len(articulos))
	for _, articulo := range articulos {
		resultado[articulo.ProductoID] = map[int]mejorOferta{}
	}
	for _, oferta := range ofertas {
		porSucursal, enLista := resultado[oferta.ProductoID]
		if _, candidata := sucursales[oferta.SucursalID]; !enLista || !candidata || !oferta.EnStock {
			continue
		}
		if oferta.Precio < 1 || oferta.ProductoIDOferta == "" {
			return nil, fmt.Errorf("%w: oferta sin precio positivo o sin producto_id_oferta", ErrParametros)
		}
		actual, existe := porSucursal[oferta.SucursalID]
		if !existe || oferta.Precio < actual.precio ||
			(oferta.Precio == actual.precio && oferta.ProductoIDOferta < actual.productoIDOferta) {
			porSucursal[oferta.SucursalID] = mejorOferta{productoIDOferta: oferta.ProductoIDOferta, precio: oferta.Precio}
		}
	}
	return resultado, nil
}

// =============================================================================
// Utilidades
// =============================================================================

// Límites de latitud y longitud en grados decimales.
const (
	latitudMaxima  = 90.0
	longitudMaxima = 180.0
)

// coordenadaValida comprueba rangos y descarta NaN o infinitos.
func coordenadaValida(coordenada Coordenada) bool {
	if math.IsNaN(coordenada.Lat) || math.IsNaN(coordenada.Lon) {
		return false
	}
	return math.Abs(coordenada.Lat) <= latitudMaxima && math.Abs(coordenada.Lon) <= longitudMaxima
}

// combinaciones devuelve todos los subconjuntos de tamano elementos, en orden
// lexicográfico de posiciones. Con 10 candidatas y 3 paradas son 175.
func combinaciones(sucursales []Sucursal, tamano int) [][]Sucursal {
	var resultado [][]Sucursal
	actual := make([]Sucursal, 0, tamano)
	var elegir func(desde int)
	elegir = func(desde int) {
		if len(actual) == tamano {
			resultado = append(resultado, slices.Clone(actual))
			return
		}
		for indice := desde; indice < len(sucursales); indice++ {
			actual = append(actual, sucursales[indice])
			elegir(indice + 1)
			actual = actual[:len(actual)-1]
		}
	}
	elegir(0)
	return resultado
}

// idsNoCubiertos extrae los producto_id para el detalle del error.
func idsNoCubiertos(noCubiertos []NoCubierto) []string {
	ids := make([]string, 0, len(noCubiertos))
	for _, noCubierto := range noCubiertos {
		ids = append(ids, noCubierto.ProductoID)
	}
	return ids
}
