// Package integration_test valida contratos HTTP y configuración del servicio.
package integration_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/app"
)

func TestScrapingCORSRejectsBeforeBusinessHandler(prueba *testing.T) {
	prepareApplicationEnvironment(prueba)
	prueba.Setenv("SCRAPING_CORS_ALLOWED_ORIGINS", "https://frontend.example")
	enrutador, _, errorObtenido := app.NewApplication(nil, nil)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	for _, caso := range []struct {
		origen       string
		codigoEstado int
	}{
		{"https://evil.example", 403},
		{"null", 403},
		{"https://frontend.example.evil.test", 403},
		{"https://frontend.example", 400},
		{"", 400},
	} {
		prueba.Run(caso.origen, func(prueba *testing.T) {
			peticion := httptest.NewRequest("POST", "/api/v1/scraper/trabajos", strings.NewReader(`{}`))
			peticion.Header.Set("Origin", caso.origen)
			peticion.Header.Set("Content-Type", "application/json")
			peticion.Header.Set("Authorization", "Bearer "+testServiceToken)
			respuesta := httptest.NewRecorder()
			enrutador.ServeHTTP(respuesta, peticion)
			if respuesta.Code != caso.codigoEstado {
				prueba.Fatal(respuesta.Code, respuesta.Body)
			}
			if caso.codigoEstado == 403 && respuesta.Header().Get("Access-Control-Allow-Origin") != "" {
				prueba.Fatal("origen ajeno reflejado")
			}
		})
	}
}

func TestRedisConfigurationNeverFallsBack(prueba *testing.T) {
	for _, direccion := range []string{"", "not-a-url", "ftp://redis:6379"} {
		prueba.Run(direccion, func(prueba *testing.T) {
			prueba.Setenv("REDIS_URL", direccion)
			clienteRedis, errorObtenido := app.NewRedisClientFromEnvironment()
			if clienteRedis != nil {
				if errorCierre := clienteRedis.Close(); errorCierre != nil {
					prueba.Fatal(errorCierre)
				}
			}
			if errorObtenido == nil {
				prueba.Fatal("una configuración inválida de Redis fue aceptada")
			}
		})
	}
}
