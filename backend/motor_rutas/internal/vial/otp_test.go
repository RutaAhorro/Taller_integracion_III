// Pruebas HTTP controladas del adaptador OTP: no requieren mapa ni red externa.
package vial

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/optimizacion"
)

// TestPerfilesDirigidos exige un par de consultas distintas (ida y vuelta).
func TestPerfilesDirigidos(prueba *testing.T) {
	for _, modo := range []string{"vehiculo", "bicicleta", "pie"} {
		prueba.Run(modo, func(prueba *testing.T) {
			servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
				cuerpo, _ := io.ReadAll(peticion.Body)
				if peticion.URL.Path != "/otp/gtfs/v1" || !strings.Contains(string(cuerpo), ModoOTP(modo)) {
					prueba.Error("perfil o ruta incorrecto")
				}
				fmt.Fprintf(respuesta, `{"data":{"planConnection":{"edges":[{"node":{"start":"2026-11-28T08:00:00-03:00","end":"2026-11-28T08:01:00-03:00","legs":[{"mode":%q,"distance":1234.5,"legGeometry":{"points":"_p~iF~ps|U_ulLnnqC_mqNvxq`+"`"+`@"}}]}}]}}}`, ModoOTP(modo))
			}))
			defer servidor.Close()
			matriz, errorObtenido := Nuevo(servidor.URL).Matriz(context.Background(), modo, optimizacion.Coordenada{}, []optimizacion.Sucursal{{ID: 1}})
			if errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
			if len(matriz) != 2 || matriz[[2]int{0, 1}].DistanciaKm != 1.2345 || matriz[[2]int{1, 0}].DuracionSegundos != 60 {
				prueba.Fatalf("matriz incorrecta: %+v", matriz)
			}
		})
	}
}

// TestFallosNoProducenMatriz evita publicar errores HTTP o GraphQL como éxito.
func TestFallosNoProducenMatriz(prueba *testing.T) {
	for _, cuerpo := range []string{`{"errors":[{"message":"fallo"}]}`, `not-json`, `{"data":{"planConnection":null}}`} {
		servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
			fmt.Fprint(respuesta, cuerpo)
		}))
		matriz, errorObtenido := Nuevo(servidor.URL).Matriz(context.Background(), "vehiculo", optimizacion.Coordenada{}, []optimizacion.Sucursal{{ID: 1}})
		servidor.Close()
		if errorObtenido == nil || matriz != nil {
			prueba.Fatal("aceptó respuesta inválida")
		}
	}
}
