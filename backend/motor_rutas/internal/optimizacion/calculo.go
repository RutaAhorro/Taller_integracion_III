// SUP-328: une la asignación (SUP-313) con el recorrido, elige la alternativa
// ganadora, calcula la base de una tienda y el ahorro, y aplica el umbral de
// ahorro de RN-08 (contrato, secciones 2.4 y 2.5).
package optimizacion

import (
	"fmt"
	"math"
	"slices"
)

// =============================================================================
// Tolerancias y redondeo
// =============================================================================

// toleranciaPesos considera iguales dos totales que solo difieren por el
// redondeo de punto flotante.
const toleranciaPesos = 1e-6

// decimalesDistancia es la escala para informar distancias con dos decimales.
const decimalesDistancia = 100

// =============================================================================
// Cálculo completo
// =============================================================================

// evaluada es una alternativa con su recorrido y sus costos sin redondear.
type evaluada struct {
	alternativa Alternativa
	recorrido   Recorrido
	traslado    float64 // CLP
	total       float64 // CLP, productos + traslado
}

// Calcular devuelve la ruta recomendada para la entrada.
//
// Gana la alternativa de menor total. Si la ganadora tiene varias tiendas y
// solo ahorra RUTAS_UMBRAL_AHORRO pesos o menos frente a la mejor tienda sola,
// se recomienda la tienda sola: visitar más tiendas no compensa un ahorro
// pequeño y el traslado es solo una estimación.
func Calcular(entrada Entrada) (Resultado, error) {
	if errorParametros := validarParametros(entrada); errorParametros != nil {
		return Resultado{}, errorParametros
	}
	asignacion, errorAsignacion := AsignarConjuntos(entrada)
	if errorAsignacion != nil {
		return Resultado{}, errorAsignacion
	}

	evaluadas := make([]evaluada, 0, len(asignacion.Alternativas))
	for _, alternativa := range asignacion.Alternativas {
		recorrido := OrdenarVisitas(entrada.Origen, alternativa.Sucursales, entrada.Parametros.FactorDistancia)
		traslado := CostoTraslado(recorrido.DistanciaKm, entrada.Parametros)
		evaluadas = append(evaluadas, evaluada{
			alternativa: alternativa,
			recorrido:   recorrido,
			traslado:    traslado,
			total:       float64(alternativa.CostoProductos) + traslado,
		})
	}
	slices.SortStableFunc(evaluadas, compararEvaluadas)

	// --- Base de una tienda: la mejor alternativa con una sola parada ---
	var base *evaluada
	for indice := range evaluadas {
		if len(evaluadas[indice].alternativa.Sucursales) == 1 {
			base = &evaluadas[indice]
			break // ya están ordenadas: la primera es la mejor
		}
	}

	// --- Umbral de ahorro (RN-08) sobre los montos que verá el usuario ---
	elegida := &evaluadas[0]
	if base != nil && len(elegida.alternativa.Sucursales) > 1 &&
		pesos(base.total)-pesos(elegida.total) <= entrada.Parametros.UmbralAhorro {
		elegida = base
	}

	resultado := Resultado{
		Ruta:                construirRuta(*elegida),
		NoCubiertos:         asignacion.NoCubiertos,
		CandidatasEvaluadas: asignacion.CandidatasEvaluadas,
	}
	if base != nil {
		sucursalBase := base.alternativa.Sucursales[0]
		resultado.BaseUnaTienda = &BaseUnaTienda{SucursalID: sucursalBase.ID, Nombre: sucursalBase.Nombre, Total: pesos(base.total)}
		ahorro := pesos(base.total) - resultado.Ruta.Total // 0 si la elegida es la base
		resultado.Ahorro = &ahorro
	}
	return resultado, nil
}

// compararEvaluadas ordena de mejor a peor: menor total; luego menos paradas;
// luego menor distancia; luego la lista de sucursal_id menor (contrato, 2.4).
func compararEvaluadas(primera, segunda evaluada) int {
	if diferencia := primera.total - segunda.total; math.Abs(diferencia) > toleranciaPesos {
		return signo(diferencia)
	}
	if paradas := len(primera.alternativa.Sucursales) - len(segunda.alternativa.Sucursales); paradas != 0 {
		return paradas
	}
	if diferencia := primera.recorrido.DistanciaKm - segunda.recorrido.DistanciaKm; math.Abs(diferencia) > toleranciaKm {
		return signo(diferencia)
	}
	return slices.Compare(idsDe(primera.alternativa.Sucursales), idsDe(segunda.alternativa.Sucursales))
}

// =============================================================================
// Presentación del resultado
// =============================================================================

// construirRuta arma la ruta del JSON con paradas en orden de visita. Los
// montos y distancias se redondean solo aquí, al final.
func construirRuta(elegida evaluada) Ruta {
	ruta := Ruta{
		Paradas:        []Parada{},
		Tramos:         []Tramo{},
		DistanciaKm:    kilometros(elegida.recorrido.DistanciaKm),
		CostoProductos: elegida.alternativa.CostoProductos,
		CostoTraslado:  pesos(elegida.traslado),
		Total:          pesos(elegida.total),
	}
	anterior := ExtremoTramo{} // el origen
	for indice, sucursal := range elegida.recorrido.Orden {
		compras := elegida.alternativa.Compras[sucursal.ID]
		subtotal := 0
		for _, compra := range compras {
			subtotal += compra.Subtotal
		}
		ruta.Paradas = append(ruta.Paradas, Parada{
			Orden:      indice + 1,
			SucursalID: sucursal.ID,
			Nombre:     sucursal.Nombre,
			Cadena:     sucursal.Cadena,
			Lat:        sucursal.Ubicacion.Lat,
			Lon:        sucursal.Ubicacion.Lon,
			Subtotal:   subtotal,
			Articulos:  compras,
		})
		actual := ExtremoTramo{SucursalID: sucursal.ID}
		ruta.Tramos = append(ruta.Tramos, Tramo{Desde: anterior, Hasta: actual, DistanciaKm: kilometros(elegida.recorrido.TramosKm[indice])})
		anterior = actual
	}
	ultimoTramo := elegida.recorrido.TramosKm[len(elegida.recorrido.TramosKm)-1]
	ruta.Tramos = append(ruta.Tramos, Tramo{Desde: anterior, Hasta: ExtremoTramo{}, DistanciaKm: kilometros(ultimoTramo)})
	return ruta
}

// =============================================================================
// Validación y utilidades
// =============================================================================

// validarParametros falla rápido ante valores que harían el cálculo absurdo,
// como dividir por un rendimiento cero o un umbral negativo.
func validarParametros(entrada Entrada) error {
	parametros := entrada.Parametros
	switch {
	case !coordenadaValida(entrada.Origen):
		return fmt.Errorf("%w: origen fuera de rango", ErrParametros)
	case parametros.MaxParadas < 1:
		return fmt.Errorf("%w: MaxParadas debe ser al menos 1", ErrParametros)
	case !(parametros.FactorDistancia > 0), math.IsInf(parametros.FactorDistancia, 0):
		return fmt.Errorf("%w: FactorDistancia debe ser positivo", ErrParametros)
	case !(parametros.RendimientoKmL > 0), math.IsInf(parametros.RendimientoKmL, 0):
		return fmt.Errorf("%w: RendimientoKmL debe ser positivo", ErrParametros)
	case !(parametros.PrecioCombustibleL > 0), math.IsInf(parametros.PrecioCombustibleL, 0):
		return fmt.Errorf("%w: PrecioCombustibleL debe ser positivo", ErrParametros)
	case parametros.UmbralAhorro < 0:
		return fmt.Errorf("%w: UmbralAhorro no puede ser negativo", ErrParametros)
	}
	return nil
}

// pesos redondea un monto al peso entero.
func pesos(monto float64) int {
	return int(math.Round(monto))
}

// kilometros redondea una distancia a dos decimales.
func kilometros(distancia float64) float64 {
	return math.Round(distancia*decimalesDistancia) / decimalesDistancia
}

// signo devuelve -1, 0 o 1 según el signo del número.
func signo(numero float64) int {
	switch {
	case numero < 0:
		return -1
	case numero > 0:
		return 1
	}
	return 0
}
