// Verifica que las calles dirigidas cambien el orden sin inventar conectividad.
package optimizacion

import (
	"errors"
	"testing"
)

// entradaVialPrueba ofrece una compra más barata repartida entre dos tiendas.
func entradaVialPrueba() Entrada {
	return Entrada{
		Origen:     Coordenada{Lat: -38.73, Lon: -72.59},
		Articulos:  []Articulo{{ProductoID: "a", Cantidad: 1}, {ProductoID: "b", Cantidad: 1}},
		Sucursales: []Sucursal{{ID: 1, Ubicacion: Coordenada{Lat: -38.74, Lon: -72.59}}, {ID: 2, Ubicacion: Coordenada{Lat: -38.75, Lon: -72.59}}},
		Ofertas: []Oferta{
			{ProductoID: "a", ProductoIDOferta: "a", SucursalID: 1, Precio: 100, EnStock: true},
			{ProductoID: "a", ProductoIDOferta: "a", SucursalID: 2, Precio: 900, EnStock: true},
			{ProductoID: "b", ProductoIDOferta: "b", SucursalID: 1, Precio: 900, EnStock: true},
			{ProductoID: "b", ProductoIDOferta: "b", SucursalID: 2, Precio: 100, EnStock: true},
		},
		Parametros: Parametros{MaxParadas: 2, FactorDistancia: 1.3},
	}
}

// TestMatrizDirigidaCambiaOrden comprueba ida/regreso distintos y costos intactos.
func TestMatrizDirigidaCambiaOrden(prueba *testing.T) {
	entrada := entradaVialPrueba()
	entrada.Viales = make(map[[2]int]TrayectoVial)
	for _, par := range [][2]int{{0, 1}, {1, 0}, {0, 2}, {2, 0}, {1, 2}, {2, 1}} {
		entrada.Viales[par] = TrayectoVial{DistanciaKm: 1, DuracionSegundos: 60, Geometrias: []string{"polilinea"}}
	}
	entrada.Viales[[2]int{0, 1}] = TrayectoVial{DistanciaKm: 10}
	entrada.Viales[[2]int{2, 0}] = TrayectoVial{DistanciaKm: 10}
	resultado, errorObtenido := Calcular(entrada)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if len(resultado.Ruta.Paradas) != 2 || resultado.Ruta.Paradas[0].SucursalID != 2 || resultado.Ruta.DistanciaKm != 3 {
		prueba.Fatalf("no usó la matriz dirigida: %+v", resultado.Ruta)
	}
	if resultado.Ruta.Total != 200 || resultado.Ruta.DuracionSegundos != 180 || len(resultado.Ruta.Tramos[0].Geometrias) != 1 {
		prueba.Fatalf("perdió importes o geometría: %+v", resultado.Ruta)
	}
}

// TestSinRutaNoUsaHaversine impide una sustitución silenciosa al fallar el grafo.
func TestSinRutaNoUsaHaversine(prueba *testing.T) {
	entrada := entradaVialPrueba()
	entrada.Viales = map[[2]int]TrayectoVial{}
	_, errorObtenido := Calcular(entrada)
	if !errors.Is(errorObtenido, ErrSinRecorrido) {
		prueba.Fatalf("error obtenido: %v", errorObtenido)
	}
}
