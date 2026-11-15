// Package integration_test valida contratos HTTP y configuración del servicio.
package integration_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/app"
)

func TestScrapingCORSRejectsBeforeBusinessHandler(t *testing.T) {
	prepareApplicationEnvironment(t)
	t.Setenv("SCRAPING_CORS_ALLOWED_ORIGINS", "https://frontend.example")
	router, _, err := app.NewApplication(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, caso := range []struct {
		origen string
		status int
	}{
		{"https://evil.example", 403},
		{"null", 403},
		{"https://frontend.example.evil.test", 403},
		{"https://frontend.example", 400},
		{"", 400},
	} {
		t.Run(caso.origen, func(t *testing.T) {
			peticion := httptest.NewRequest("POST", "/api/v1/scraper/trabajos", strings.NewReader(`{}`))
			peticion.Header.Set("Origin", caso.origen)
			peticion.Header.Set("Content-Type", "application/json")
			peticion.Header.Set("Authorization", "Bearer "+testServiceToken)
			respuesta := httptest.NewRecorder()
			router.ServeHTTP(respuesta, peticion)
			if respuesta.Code != caso.status {
				t.Fatal(respuesta.Code, respuesta.Body)
			}
			if caso.status == 403 && respuesta.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("origen ajeno reflejado")
			}
		})
	}
}

func TestRedisConfigurationNeverFallsBack(t *testing.T) {
	for _, direccion := range []string{"", "not-a-url", "ftp://redis:6379"} {
		t.Run(direccion, func(t *testing.T) {
			t.Setenv("REDIS_URL", direccion)
			clienteRedis, err := app.NewRedisClientFromEnvironment()
			if clienteRedis != nil {
				if errorCierre := clienteRedis.Close(); errorCierre != nil {
					t.Fatal(errorCierre)
				}
			}
			if err == nil {
				t.Fatal("una configuración inválida de Redis fue aceptada")
			}
		})
	}
}
