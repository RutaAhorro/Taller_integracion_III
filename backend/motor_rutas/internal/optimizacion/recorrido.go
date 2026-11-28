// SUP-328: elige el orden de visita de un conjunto de tiendas y calcula su
// distancia (contrato, sección 2.3). El recorrido sale del origen, pasa por
// todas las tiendas y vuelve al origen.
package optimizacion

import (
	"math"
	"slices"
)

// =============================================================================
// Tolerancias
// =============================================================================

// toleranciaKm considera iguales dos distancias que solo difieren por el
// redondeo de punto flotante (un recorrido y su inverso miden lo mismo).
const toleranciaKm = 1e-9

// =============================================================================
// Orden de visita
// =============================================================================

// Recorrido es un orden de visita con sus tramos y su distancia total, ya
// multiplicada por el factor de corrección. Las distancias no se redondean
// aquí: el redondeo es solo para presentar el resultado.
type Recorrido struct {
	Orden       []Sucursal
	TramosKm    []float64 // origen → primera, entre tiendas y última → origen
	DistanciaKm float64
	Viales      []TrayectoVial
}

// ordenarVisitasViales usa la matriz dirigida, incluidos ida y regreso distintos.
// Un tramo ausente invalida ese orden; nunca se rellena con Haversine.
func ordenarVisitasViales(conjunto []Sucursal, matriz map[[2]int]TrayectoVial) (Recorrido, bool) {
	var mejor Recorrido
	encontrado := false
	for _, orden := range permutaciones(conjunto) {
		candidato := Recorrido{Orden: orden}
		anterior := 0
		valido := true
		destinos := append(idsDe(orden), 0)
		for _, destino := range destinos {
			tramo, existe := matriz[[2]int{anterior, destino}]
			if !existe {
				valido = false
				break
			}
			candidato.TramosKm = append(candidato.TramosKm, tramo.DistanciaKm)
			candidato.Viales = append(candidato.Viales, tramo)
			candidato.DistanciaKm += tramo.DistanciaKm
			anterior = destino
		}
		if valido && (!encontrado || candidato.DistanciaKm < mejor.DistanciaKm-toleranciaKm) {
			mejor, encontrado = candidato, true
		}
	}
	return mejor, encontrado
}

// OrdenarVisitas prueba todos los órdenes del conjunto (6 como máximo con 3
// paradas) y devuelve el de menor distancia. Ante la misma distancia, por
// ejemplo un recorrido y su inverso, visita primero la tienda más cercana al
// origen; si aún empatan, gana la secuencia de sucursal_id menor.
func OrdenarVisitas(origen Coordenada, conjunto []Sucursal, factorDistancia float64) Recorrido {
	var mejor Recorrido
	for indice, orden := range permutaciones(conjunto) {
		candidato := medirRecorrido(origen, orden, factorDistancia)
		if indice == 0 || recorridoPreferido(origen, candidato, mejor) {
			mejor = candidato
		}
	}
	return mejor
}

// recorridoPreferido indica si candidato gana a actual según el desempate.
func recorridoPreferido(origen Coordenada, candidato, actual Recorrido) bool {
	diferencia := candidato.DistanciaKm - actual.DistanciaKm
	if math.Abs(diferencia) > toleranciaKm {
		return diferencia < 0
	}
	primeraCandidato := haversineKm(origen, candidato.Orden[0].Ubicacion)
	primeraActual := haversineKm(origen, actual.Orden[0].Ubicacion)
	if math.Abs(primeraCandidato-primeraActual) > toleranciaKm {
		return primeraCandidato < primeraActual
	}
	return slices.Compare(idsDe(candidato.Orden), idsDe(actual.Orden)) < 0
}

// medirRecorrido suma los tramos origen → tiendas → origen con el factor.
func medirRecorrido(origen Coordenada, orden []Sucursal, factorDistancia float64) Recorrido {
	recorrido := Recorrido{Orden: orden}
	anterior := origen
	for _, sucursal := range orden {
		recorrido.TramosKm = append(recorrido.TramosKm, haversineKm(anterior, sucursal.Ubicacion)*factorDistancia)
		anterior = sucursal.Ubicacion
	}
	recorrido.TramosKm = append(recorrido.TramosKm, haversineKm(anterior, origen)*factorDistancia)
	for _, tramo := range recorrido.TramosKm {
		recorrido.DistanciaKm += tramo
	}
	return recorrido
}

// =============================================================================
// Costo de traslado
// =============================================================================

// CostoTraslado aplica la regla de negocio del contrato:
// (distancia_km ÷ rendimiento_km_l) × precio_combustible_l. Sin redondear.
// Cuando PrecioCombustibleL es 0 (modo MVP sin combustible) devuelve 0.
func CostoTraslado(distanciaKm float64, parametros Parametros) float64 {
	if parametros.PrecioCombustibleL == 0 {
		return 0
	}
	return distanciaKm / parametros.RendimientoKmL * parametros.PrecioCombustibleL
}

// =============================================================================
// Utilidades
// =============================================================================

// permutaciones devuelve todos los órdenes posibles del conjunto.
func permutaciones(conjunto []Sucursal) [][]Sucursal {
	if len(conjunto) <= 1 {
		return [][]Sucursal{slices.Clone(conjunto)}
	}
	var resultado [][]Sucursal
	for indice, primera := range conjunto {
		resto := slices.Concat(conjunto[:indice], conjunto[indice+1:])
		for _, cola := range permutaciones(resto) {
			resultado = append(resultado, append([]Sucursal{primera}, cola...))
		}
	}
	return resultado
}

// idsDe devuelve los sucursal_id en el orden recibido.
func idsDe(sucursales []Sucursal) []int {
	ids := make([]int, 0, len(sucursales))
	for _, sucursal := range sucursales {
		ids = append(ids, sucursal.ID)
	}
	return ids
}
