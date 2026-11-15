// Distancia aproximada entre dos puntos: línea recta sobre la esfera
// terrestre (Haversine). El factor de corrección se aplica en recorrido.go.
package optimizacion

import "math"

// =============================================================================
// Constantes geográficas
// =============================================================================

// radioTerrestreKm es el radio medio de la Tierra usado por Haversine.
const radioTerrestreKm = 6371.0

// gradosPorMediaVuelta convierte grados a radianes (π radianes = 180°).
const gradosPorMediaVuelta = 180.0

// =============================================================================
// Haversine
// =============================================================================

// haversineKm devuelve la distancia en línea recta, en km, entre dos puntos.
// No considera calles: por eso el contrato exige multiplicarla por un factor.
func haversineKm(desde, hasta Coordenada) float64 {
	latitudDesde := radianes(desde.Lat)
	latitudHasta := radianes(hasta.Lat)
	diferenciaLatitud := radianes(hasta.Lat - desde.Lat)
	diferenciaLongitud := radianes(hasta.Lon - desde.Lon)

	senoMediaLatitud := math.Sin(diferenciaLatitud / 2)
	senoMediaLongitud := math.Sin(diferenciaLongitud / 2)
	semiverseno := senoMediaLatitud*senoMediaLatitud +
		math.Cos(latitudDesde)*math.Cos(latitudHasta)*senoMediaLongitud*senoMediaLongitud
	return 2 * radioTerrestreKm * math.Asin(math.Sqrt(semiverseno))
}

// radianes convierte grados decimales a radianes.
func radianes(grados float64) float64 {
	return grados * math.Pi / gradosPorMediaVuelta
}
