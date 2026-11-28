// Package vial obtiene recorridos por calles de OTP, sin SQL ni datos GTFS.
package vial

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/optimizacion"
)

// ===== Límites y contrato =====
const (
	plazoMatriz        = 8 * time.Second
	maximoRespuesta    = 2 << 20
	concurrencia       = 4
	metrosPorKilometro = 1000
	maximoSucursales   = 10
)

// Proveedor permite probar el endpoint sin red ni servicios reales.
type Proveedor interface {
	Matriz(context.Context, string, optimizacion.Coordenada, []optimizacion.Sucursal) (map[[2]int]optimizacion.TrayectoVial, error)
}

// Cliente consulta exclusivamente el origen OTP configurado por el operador.
type Cliente struct {
	origen string
	http   *http.Client
}

// Nuevo configura un cliente acotado; nunca sigue redirecciones a otro host.
func Nuevo(origen string) *Cliente {
	return &Cliente{origen: strings.TrimRight(origen, "/"), http: &http.Client{
		Timeout:       plazoMatriz,
		CheckRedirect: func(peticion *http.Request, anteriores []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// ModoOTP valida el contrato público sin permitir modos arbitrarios en GraphQL.
func ModoOTP(modo string) string {
	switch modo {
	case "vehiculo":
		return "CAR"
	case "bicicleta":
		return "BICYCLE"
	case "pie":
		return "WALK"
	default:
		return ""
	}
}

// ===== Matriz dirigida =====

// Matriz resuelve los pares con concurrencia y plazo global limitados.
// Solo un resultado sin itinerario representa desconexión; un fallo HTTP o
// GraphQL aborta la consulta completa, sin devolver una matriz parcial.
func (cliente *Cliente) Matriz(contexto context.Context, modo string, origen optimizacion.Coordenada, sucursales []optimizacion.Sucursal) (map[[2]int]optimizacion.TrayectoVial, error) {
	if ModoOTP(modo) == "" || len(sucursales) > maximoSucursales {
		return nil, errors.New("consulta vial fuera de límites")
	}
	contexto, cancelar := context.WithTimeout(contexto, plazoMatriz)
	defer cancelar()
	puntos := []optimizacion.Sucursal{{ID: 0, Ubicacion: origen}}
	puntos = append(puntos, sucursales...)
	resultado := make(map[[2]int]optimizacion.TrayectoVial)
	var bloqueo sync.Mutex
	var grupo sync.WaitGroup
	var primerError error
	cupos := make(chan struct{}, concurrencia)
	for _, desde := range puntos {
		for _, hasta := range puntos {
			if desde.ID == hasta.ID {
				continue
			}
			grupo.Add(1)
			go func() {
				defer grupo.Done()
				select {
				case cupos <- struct{}{}:
				case <-contexto.Done():
					return
				}
				defer func() { <-cupos }()
				tramo, encontrado, errorObtenido := cliente.consultar(contexto, ModoOTP(modo), desde.Ubicacion, hasta.Ubicacion)
				bloqueo.Lock()
				defer bloqueo.Unlock()
				if errorObtenido != nil {
					if primerError == nil {
						primerError = errorObtenido
						cancelar()
					}
					return
				}
				if encontrado {
					resultado[[2]int{desde.ID, hasta.ID}] = tramo
				}
			}()
		}
	}
	grupo.Wait()
	if primerError != nil {
		return nil, primerError
	}
	if contexto.Err() != nil {
		return nil, contexto.Err()
	}
	return resultado, nil
}

// ===== Adaptación de OTP =====

// respuestaOTP solo decodifica los campos usados y nunca expone errores privados.
type respuestaOTP struct {
	Errores []json.RawMessage `json:"errors"`
	Datos   struct {
		Plan *struct {
			Alternativas []struct {
				Itinerario struct {
					Inicio string `json:"start"`
					Fin    string `json:"end"`
					Etapas []struct {
						Modo      string   `json:"mode"`
						Distancia *float64 `json:"distance"`
						Geometria struct {
							Puntos string `json:"points"`
						} `json:"legGeometry"`
					} `json:"legs"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"planConnection"`
	} `json:"data"`
}

// consultar elige el itinerario de menor distancia con el perfil exacto.
func (cliente *Cliente) consultar(contexto context.Context, modo string, desde, hasta optimizacion.Coordenada) (optimizacion.TrayectoVial, bool, error) {
	consulta := fmt.Sprintf(`{ planConnection(
		origin:{location:{coordinate:{latitude:%f,longitude:%f}}},
		destination:{location:{coordinate:{latitude:%f,longitude:%f}}},
		modes:{direct:[%s]}
	) { edges { node { start end legs { mode distance legGeometry { points } } } } } }`,
		desde.Lat, desde.Lon, hasta.Lat, hasta.Lon, modo)
	cuerpo, _ := json.Marshal(map[string]string{"query": consulta})
	peticion, errorObtenido := http.NewRequestWithContext(contexto, http.MethodPost, cliente.origen+"/otp/gtfs/v1", bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return optimizacion.TrayectoVial{}, false, errors.New("petición OTP inválida")
	}
	peticion.Header.Set("Content-Type", "application/json")
	respuesta, errorObtenido := cliente.http.Do(peticion)
	if errorObtenido != nil {
		return optimizacion.TrayectoVial{}, false, errors.New("OTP no disponible")
	}
	defer respuesta.Body.Close()
	if respuesta.StatusCode != http.StatusOK {
		return optimizacion.TrayectoVial{}, false, errors.New("OTP rechazó la consulta")
	}
	crudo, errorObtenido := io.ReadAll(io.LimitReader(respuesta.Body, maximoRespuesta+1))
	if errorObtenido != nil || len(crudo) > maximoRespuesta {
		return optimizacion.TrayectoVial{}, false, errors.New("respuesta OTP fuera de límites")
	}
	var datos respuestaOTP
	if json.Unmarshal(crudo, &datos) != nil || len(datos.Errores) > 0 || datos.Datos.Plan == nil {
		return optimizacion.TrayectoVial{}, false, errors.New("respuesta OTP inválida")
	}
	var mejor optimizacion.TrayectoVial
	encontrado := false
	for _, alternativa := range datos.Datos.Plan.Alternativas {
		itinerario := alternativa.Itinerario
		inicio, errorInicio := time.Parse(time.RFC3339, itinerario.Inicio)
		fin, errorFin := time.Parse(time.RFC3339, itinerario.Fin)
		if errorInicio != nil || errorFin != nil || fin.Before(inicio) || len(itinerario.Etapas) == 0 {
			return mejor, false, errors.New("itinerario OTP incompleto")
		}
		tramo := optimizacion.TrayectoVial{DuracionSegundos: fin.Sub(inicio).Seconds()}
		contieneModo := false
		for _, etapa := range itinerario.Etapas {
			if etapa.Modo == modo {
				contieneModo = true
			}
			if (etapa.Modo != modo && etapa.Modo != "WALK") || etapa.Distancia == nil ||
				math.IsNaN(*etapa.Distancia) || math.IsInf(*etapa.Distancia, 0) || *etapa.Distancia < 0 || etapa.Geometria.Puntos == "" {
				return mejor, false, errors.New("tramo OTP incompatible")
			}
			tramo.DistanciaKm += *etapa.Distancia / metrosPorKilometro
			tramo.Geometrias = append(tramo.Geometrias, etapa.Geometria.Puntos)
		}
		if !contieneModo {
			continue
		}
		if !encontrado || tramo.DistanciaKm < mejor.DistanciaKm {
			mejor, encontrado = tramo, true
		}
	}
	return mejor, encontrado, nil
}
