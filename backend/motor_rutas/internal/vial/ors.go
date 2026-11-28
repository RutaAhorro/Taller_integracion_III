// Package vial consulta ORS por HTTP sin grafos locales, SQL ni cálculos simulados.
package vial

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/optimizacion"
)

// ===== Límites y contrato =====
const (
	rutaServicio          = "/openrouteservice"
	PlazoCalculo          = 12 * time.Second // compartido entre Matrix y Directions por cálculo
	maximoRespuesta       = 8 << 20
	metrosPorKilometro    = 1000
	maximoSucursales      = 10
	maximoParadasTrazado  = 3 // tiendas por recorrido vial; Directions recibe origen, paradas y retorno
	maximoPuntosGeometria = 100000
	precisionPolilinea    = 100000
	decimalesKilometros   = 100
)

// Proveedor separa la comparación por matriz del trazado de la única ruta elegida.
type Proveedor interface {
	Matriz(context.Context, string, optimizacion.Coordenada, []optimizacion.Sucursal) (map[[2]int]optimizacion.TrayectoVial, error)
	Trazar(context.Context, string, optimizacion.Coordenada, *optimizacion.Ruta) error
}

// Cliente guarda la clave exclusivamente en backend; nunca la incorpora a la URL.
type Cliente struct {
	origen string
	clave  string
	http   *http.Client
}

// Nuevo configura ORS sin redirecciones ni reintentos que multipliquen la cuota.
func Nuevo(origen, clave string) *Cliente {
	return &Cliente{origen: strings.TrimRight(origen, "/"), clave: clave, http: &http.Client{
		Timeout:       PlazoCalculo,
		CheckRedirect: func(peticion *http.Request, anteriores []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// PerfilORS valida el modo público y lo traduce al perfil oficial correspondiente.
func PerfilORS(modo string) string {
	switch modo {
	case "vehiculo":
		return "driving-car"
	case "bicicleta":
		return "cycling-regular"
	case "pie":
		return "foot-walking"
	default:
		return ""
	}
}

// ===== Matriz dirigida: una sola consulta =====

// Matriz solicita metros y segundos en bloque. Un null indica falta de conexión.
func (cliente *Cliente) Matriz(contexto context.Context, modo string, origen optimizacion.Coordenada, sucursales []optimizacion.Sucursal) (map[[2]int]optimizacion.TrayectoVial, error) {
	perfil := PerfilORS(modo)
	if perfil == "" || len(sucursales) > maximoSucursales {
		return nil, errors.New("consulta ORS fuera de límites")
	}
	puntos := []optimizacion.Sucursal{{ID: 0, Ubicacion: origen}}
	puntos = append(puntos, sucursales...)
	ubicaciones := make([][2]float64, len(puntos))
	for indice, punto := range puntos {
		ubicaciones[indice] = [2]float64{punto.Ubicacion.Lon, punto.Ubicacion.Lat}
	}
	var datos struct {
		Distancias [][]*float64 `json:"distances"`
		Duraciones [][]*float64 `json:"durations"`
	}
	if errorObtenido := cliente.consultar(contexto, "/v2/matrix/"+perfil, map[string]any{
		"locations": ubicaciones, "metrics": []string{"distance", "duration"}, "units": "m",
	}, &datos); errorObtenido != nil {
		return nil, errorObtenido
	}
	if len(datos.Distancias) != len(puntos) || len(datos.Duraciones) != len(puntos) {
		return nil, errors.New("matriz ORS incompleta")
	}
	resultado := make(map[[2]int]optimizacion.TrayectoVial)
	for indiceDesde, desde := range puntos {
		if len(datos.Distancias[indiceDesde]) != len(puntos) || len(datos.Duraciones[indiceDesde]) != len(puntos) {
			return nil, errors.New("fila ORS incompleta")
		}
		for indiceHasta, hasta := range puntos {
			if indiceDesde == indiceHasta {
				continue
			}
			distancia, duracion := datos.Distancias[indiceDesde][indiceHasta], datos.Duraciones[indiceDesde][indiceHasta]
			if distancia == nil && duracion == nil {
				continue
			}
			if distancia == nil || duracion == nil || !numeroValido(*distancia) || !numeroValido(*duracion) {
				return nil, errors.New("métrica ORS inválida")
			}
			resultado[[2]int{desde.ID, hasta.ID}] = optimizacion.TrayectoVial{DistanciaKm: *distancia / metrosPorKilometro, DuracionSegundos: *duracion}
		}
	}
	return resultado, nil
}

// ===== Geometría: una consulta para todas las paradas y el retorno =====

// Trazar divide el GeoJSON por way_points sin inventar líneas entre sucursales.
// Mantiene el orden elegido por Go; Directions y Matrix usan el mismo perfil.
func (cliente *Cliente) Trazar(contexto context.Context, modo string, origen optimizacion.Coordenada, ruta *optimizacion.Ruta) error {
	perfil := PerfilORS(modo)
	if perfil == "" || len(ruta.Paradas) < 1 || len(ruta.Paradas) > maximoParadasTrazado {
		return errors.New("ruta ORS fuera de límites")
	}
	puntos := [][2]float64{{origen.Lon, origen.Lat}}
	for _, parada := range ruta.Paradas {
		puntos = append(puntos, [2]float64{parada.Lon, parada.Lat})
	}
	puntos = append(puntos, puntos[0])
	var datos struct {
		Tipo  string `json:"type"`
		Rutas []struct {
			Geometria struct {
				Tipo        string      `json:"type"`
				Coordenadas [][]float64 `json:"coordinates"`
			} `json:"geometry"`
			Propiedades struct {
				Extremos  []int `json:"way_points"`
				Segmentos []struct {
					Distancia *float64 `json:"distance"`
					Duracion  *float64 `json:"duration"`
				} `json:"segments"`
			} `json:"properties"`
		} `json:"features"`
	}
	if errorObtenido := cliente.consultar(contexto, "/v2/directions/"+perfil+"/geojson", map[string]any{
		// ORS omite segments si instructions=false; se necesitan sus métricas por tramo.
		"coordinates": puntos, "preference": "fastest", "instructions": true, "elevation": false,
	}, &datos); errorObtenido != nil {
		return errorObtenido
	}
	if datos.Tipo != "FeatureCollection" || len(datos.Rutas) != 1 {
		return errors.New("geometría ORS incompleta")
	}
	trazado := datos.Rutas[0]
	coordenadas, extremos, segmentos := trazado.Geometria.Coordenadas, trazado.Propiedades.Extremos, trazado.Propiedades.Segmentos
	if trazado.Geometria.Tipo != "LineString" || len(coordenadas) < 2 || len(coordenadas) > maximoPuntosGeometria ||
		len(extremos) != len(puntos) || len(segmentos) != len(puntos)-1 || len(ruta.Tramos) != len(segmentos) {
		return errors.New("segmentos ORS incompatibles")
	}
	for _, punto := range coordenadas {
		if len(punto) != 2 || math.IsNaN(punto[0]) || math.IsNaN(punto[1]) || math.IsInf(punto[0], 0) || math.IsInf(punto[1], 0) ||
			math.Abs(punto[0]) > 180 || math.Abs(punto[1]) > 90 {
			return errors.New("coordenadas ORS inválidas")
		}
	}
	if extremos[0] != 0 || extremos[len(extremos)-1] != len(coordenadas)-1 {
		return errors.New("extremos ORS incompletos")
	}
	for indice, segmento := range segmentos {
		if extremos[indice] < 0 || extremos[indice+1] < extremos[indice] || extremos[indice+1] >= len(coordenadas) ||
			segmento.Distancia == nil || segmento.Duracion == nil || !numeroValido(*segmento.Distancia) || !numeroValido(*segmento.Duracion) {
			return errors.New("segmento ORS inválido")
		}
	}
	// Solo mutar el resultado después de validar toda la respuesta.
	ruta.DistanciaKm, ruta.DuracionSegundos = 0, 0
	for indice, segmento := range segmentos {
		geometria := coordenadas[extremos[indice] : extremos[indice+1]+1]
		if len(geometria) == 1 {
			geometria = [][]float64{geometria[0], geometria[0]}
		} // parada coincidente, distancia cero legítima
		ruta.Tramos[indice].Geometrias = []string{codificarPolilinea(geometria)}
		ruta.Tramos[indice].DistanciaKm = *segmento.Distancia / metrosPorKilometro
		ruta.Tramos[indice].DuracionSegundos = *segmento.Duracion
		ruta.DistanciaKm += ruta.Tramos[indice].DistanciaKm
		ruta.DuracionSegundos += *segmento.Duracion
	}
	ruta.DistanciaKm = math.Round(ruta.DistanciaKm*decimalesKilometros) / decimalesKilometros
	for indice := range ruta.Tramos {
		ruta.Tramos[indice].DistanciaKm = math.Round(ruta.Tramos[indice].DistanciaKm*decimalesKilometros) / decimalesKilometros
	}
	return nil
}

// ===== HTTP y utilidades sin exposición de claves =====

// consultar limita tamaño, plazo y errores; jamás devuelve cuerpo, clave ni URL privada.
func (cliente *Cliente) consultar(contexto context.Context, ruta string, cuerpo any, destino any) error {
	crudo, errorObtenido := json.Marshal(cuerpo)
	if errorObtenido != nil {
		return errors.New("consulta ORS inválida")
	}
	peticion, errorObtenido := http.NewRequestWithContext(contexto, http.MethodPost, cliente.origen+rutaServicio+ruta, bytes.NewReader(crudo))
	if errorObtenido != nil {
		return errors.New("petición ORS inválida")
	}
	peticion.Header.Set("Content-Type", "application/json")
	if cliente.clave != "" {
		peticion.Header.Set("Authorization", cliente.clave)
	}
	respuesta, errorObtenido := cliente.http.Do(peticion)
	if errorObtenido != nil {
		var errorCertificado x509.UnknownAuthorityError
		if errors.As(errorObtenido, &errorCertificado) {
			return errors.New("ORS: certificado TLS no confiable")
		}
		if errors.Is(errorObtenido, context.DeadlineExceeded) {
			return errors.New("ORS: plazo de consulta agotado")
		}
		return errors.New("ORS no disponible")
	}
	defer respuesta.Body.Close()
	if respuesta.StatusCode != http.StatusOK {
		return fmt.Errorf("ORS rechazó la consulta (HTTP %d)", respuesta.StatusCode)
	}
	contenido, errorObtenido := io.ReadAll(io.LimitReader(respuesta.Body, maximoRespuesta+1))
	if errorObtenido != nil || len(contenido) > maximoRespuesta {
		return errors.New("respuesta ORS fuera de límites")
	}
	if json.Unmarshal(contenido, destino) != nil {
		return errors.New("respuesta ORS inválida")
	}
	return nil
}

// numeroValido comprueba métricas finitas no negativas (cero es válido al coincidir puntos).
func numeroValido(numero float64) bool {
	return !math.IsNaN(numero) && !math.IsInf(numero, 0) && numero >= 0
}

// codificarPolilinea convierte [lon,lat] de GeoJSON al formato bidimensional de Leaflet.
func codificarPolilinea(puntos [][]float64) string {
	var resultado strings.Builder
	anterior := [2]int64{}
	for _, punto := range puntos {
		actual := [2]int64{int64(math.Round(punto[1] * precisionPolilinea)), int64(math.Round(punto[0] * precisionPolilinea))}
		for indice, valor := range actual {
			delta := valor - anterior[indice]
			codificado := uint64(delta << 1)
			if delta < 0 {
				codificado = uint64(^(delta << 1))
			}
			for codificado >= 32 {
				resultado.WriteByte(byte((32 | (codificado & 31)) + 63))
				codificado >>= 5
			}
			resultado.WriteByte(byte(codificado + 63))
		}
		anterior = actual
	}
	return resultado.String()
}
