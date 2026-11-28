// Verifica el contrato ORS sin red externa ni consumo de cuota.
package vial

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/optimizacion"
)

// TestMatrixYDirections usa exactamente una llamada por endpoint y perfil.
func TestMatrixYDirections(prueba *testing.T) {
	for _, modo := range []string{"vehiculo", "bicicleta", "pie"} {
		prueba.Run(modo, func(prueba *testing.T) {
			llamadas := []string{}
			servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
				llamadas = append(llamadas, peticion.URL.Path)
				if peticion.Header.Get("Authorization") != "clave-de-prueba" || peticion.URL.RawQuery != "" {
					prueba.Error("autorización fuera del encabezado")
				}
				var cuerpo map[string]json.RawMessage
				if json.NewDecoder(peticion.Body).Decode(&cuerpo) != nil {
					prueba.Error("cuerpo JSON inválido")
				}
				switch peticion.URL.Path {
				case "/openrouteservice/v2/matrix/" + PerfilORS(modo):
					var puntos [][2]float64
					json.Unmarshal(cuerpo["locations"], &puntos)
					if len(puntos) != 2 || puntos[0] != [2]float64{-72, -38} {
						prueba.Error("invirtió lon/lat")
					}
					fmt.Fprint(respuesta, `{"distances":[[0,1000],[2000,0]],"durations":[[0,60],[120,0]]}`)
				case "/openrouteservice/v2/directions/" + PerfilORS(modo) + "/geojson":
					var puntos [][2]float64
					json.Unmarshal(cuerpo["coordinates"], &puntos)
					if len(puntos) != 3 || puntos[0] != puntos[2] || string(cuerpo["preference"]) != `"fastest"` {
						prueba.Error("perdió el retorno o cambió criterio")
					}
					if string(cuerpo["instructions"]) != "true" {
						prueba.Error("ORS no devolvería segmentos")
					}
					fmt.Fprint(respuesta, `{"type":"FeatureCollection","features":[{"geometry":{"type":"LineString","coordinates":[[-72,-38],[-72.1,-38.1],[-72,-38]]},"properties":{"way_points":[0,1,2],"segments":[{"distance":1001,"duration":61},{"distance":2001,"duration":121}]}}]}`)
				default:
					prueba.Error("endpoint inesperado")
					respuesta.WriteHeader(404)
				}
			}))
			defer servidor.Close()
			cliente := Nuevo(servidor.URL, "clave-de-prueba")
			origen := optimizacion.Coordenada{Lat: -38, Lon: -72}
			matriz, errorObtenido := cliente.Matriz(context.Background(), modo, origen, []optimizacion.Sucursal{{ID: 1, Ubicacion: optimizacion.Coordenada{Lat: -38.1, Lon: -72.1}}})
			if errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
			if matriz[[2]int{0, 1}].DistanciaKm != 1 || matriz[[2]int{1, 0}].DistanciaKm != 2 {
				prueba.Fatal("simetrizó la matriz")
			}
			ruta := optimizacion.Ruta{Paradas: []optimizacion.Parada{{SucursalID: 1, Lat: -38.1, Lon: -72.1}}, Tramos: make([]optimizacion.Tramo, 2), Total: 1234}
			if errorObtenido = cliente.Trazar(context.Background(), modo, origen, &ruta); errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
			if ruta.Total != 1234 || ruta.DuracionSegundos != 182 || len(ruta.Tramos[0].Geometrias) != 1 || len(llamadas) != 2 {
				prueba.Fatalf("resultado inválido o exceso de llamadas: %+v", ruta)
			}
		})
	}
}

// TestErroresORSNoFiltranClaves rechaza fallos HTTP, respuestas truncadas y datos inválidos.
func TestErroresORSNoFiltranClaves(prueba *testing.T) {
	for _, caso := range []struct {
		estado int
		cuerpo string
	}{
		{401, `clave-de-prueba`}, {429, `clave-de-prueba`}, {500, `error`},
		{200, `not-json`}, {200, `{}`}, {200, `{"distances":[[0,-1],[1,0]],"durations":[[0,1],[1,0]]}`},
	} {
		servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
			respuesta.WriteHeader(caso.estado)
			fmt.Fprint(respuesta, caso.cuerpo)
		}))
		_, errorObtenido := Nuevo(servidor.URL, "clave-de-prueba").Matriz(context.Background(), "pie", optimizacion.Coordenada{}, []optimizacion.Sucursal{{ID: 1}})
		servidor.Close()
		if errorObtenido == nil || strings.Contains(errorObtenido.Error(), "clave-de-prueba") {
			prueba.Fatal("aceptó respuesta inválida o filtró clave")
		}
	}
}

// TestMatrizSinConexion conserva null como tramo ausente, sin inventar distancia.
func TestMatrizSinConexion(prueba *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		fmt.Fprint(respuesta, `{"distances":[[0,null],[1,0]],"durations":[[0,null],[1,0]]}`)
	}))
	defer servidor.Close()
	matriz, errorObtenido := Nuevo(servidor.URL, "").Matriz(context.Background(), "pie", optimizacion.Coordenada{}, []optimizacion.Sucursal{{ID: 1}})
	if errorObtenido != nil || len(matriz) != 1 {
		prueba.Fatalf("null inválido: %v", errorObtenido)
	}
	if _, existe := matriz[[2]int{0, 1}]; existe {
		prueba.Fatal("inventó conectividad")
	}
}

// TestPolilineaORS conserva el formato 2D que consume Leaflet.
func TestPolilineaORS(prueba *testing.T) {
	if codificarPolilinea([][]float64{{-120.2, 38.5}, {-120.95, 40.7}, {-126.453, 43.252}}) != "_p~iF~ps|U_ulLnnqC_mqNvxq`@" {
		prueba.Fatal("polilínea incorrecta")
	}
}

// TestTrazadoIncompletoNoMutaRuta impide publicar distancias sin geometría válida.
func TestTrazadoIncompletoNoMutaRuta(prueba *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		fmt.Fprint(respuesta, `{"type":"FeatureCollection","features":[{"geometry":{"type":"LineString","coordinates":[[-72,-38],[-72.1,-38.1]]},"properties":{"way_points":[0,1,1]}}]}`)
	}))
	defer servidor.Close()
	ruta := optimizacion.Ruta{Paradas: []optimizacion.Parada{{Lat: -38.1, Lon: -72.1}}, Tramos: make([]optimizacion.Tramo, 2), Total: 1234, DistanciaKm: 7}
	errorObtenido := Nuevo(servidor.URL, "").Trazar(context.Background(), "pie", optimizacion.Coordenada{Lat: -38, Lon: -72}, &ruta)
	if errorObtenido == nil || ruta.Total != 1234 || ruta.DistanciaKm != 7 || ruta.Tramos[0].Geometrias != nil {
		prueba.Fatal("aceptó o publicó una respuesta incompleta")
	}
}
