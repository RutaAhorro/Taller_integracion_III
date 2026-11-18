// Package integration_test valida autenticación de servicio en las rutas públicas.
package integration_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/app"
)

// El Gateway es público: sin el token de servicio ninguna ruta de coordinación
// llega al handler, aunque el cuerpo sea válido o el origen esté permitido.
func TestScraperRequiresServiceToken(prueba *testing.T) {
	prepareApplicationEnvironment(prueba)
	tokenCatalogo := strings.Repeat("t", 32)
	prueba.Setenv("SCRAPING_CORS_ALLOWED_ORIGINS", "https://frontend.example")
	prueba.Setenv("CATALOGO_INTERNAL_TOKEN", tokenCatalogo)
	prueba.Setenv("SCRAPING_SWAGGER_ENABLED", "true")
	enrutador, _, errorObtenido := app.NewApplication(nil, nil)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	rutas := []struct{ metodo, ruta string }{
		{"POST", "/api/v1/scraper/trabajos"},
		{"GET", "/api/v1/scraper/trabajos/00000000-0000-0000-0000-000000000000"},
		{"PUT", "/api/v1/scraper/trabajos/00000000-0000-0000-0000-000000000000/finalizar"},
		{"POST", "/api/v1/scraper/trabajos/00000000-0000-0000-0000-000000000000/ejecutar"},
		{"POST", "/api/v1/scraper/trabajos/00000000-0000-0000-0000-000000000000/productos"},
		{"POST", "/api/v1/scraper/productos"},
	}
	cabeceras := map[string]string{
		"sin header":         "",
		"token incorrecto":   "Bearer " + strings.Repeat("x", 32),
		"token de Catálogo":  "Bearer " + tokenCatalogo,
		"sin esquema Bearer": testServiceToken,
		"espacio extra":      "Bearer  " + testServiceToken,
	}
	for _, ruta := range rutas {
		for nombre, valor := range cabeceras {
			prueba.Run(ruta.metodo+" "+ruta.ruta+" "+nombre, func(prueba *testing.T) {
				peticion := httptest.NewRequest(ruta.metodo, ruta.ruta, strings.NewReader(`{}`))
				peticion.Header.Set("Content-Type", "application/json")
				peticion.Header.Set("Origin", "https://frontend.example")
				if valor != "" {
					peticion.Header.Set("Authorization", valor)
				}
				respuesta := httptest.NewRecorder()
				enrutador.ServeHTTP(respuesta, peticion)
				if respuesta.Code != 401 {
					prueba.Fatalf("HTTP %d; esperado 401: %s", respuesta.Code, respuesta.Body)
				}
				if respuesta.Header().Get("WWW-Authenticate") == "" {
					prueba.Fatal("falta WWW-Authenticate")
				}
			})
		}
	}
	// Probes y Swagger no exigen token.
	for _, ruta := range []string{"/_scraping/live", "/swagger/doc.json"} {
		respuesta := httptest.NewRecorder()
		enrutador.ServeHTTP(respuesta, httptest.NewRequest("GET", ruta, nil))
		if respuesta.Code == 401 {
			prueba.Fatalf("%s no debe exigir token de servicio", ruta)
		}
	}
}

func TestServiceTokenConfigurationFailsClosed(prueba *testing.T) {
	prepareApplicationEnvironment(prueba)
	tokenCatalogo := strings.Repeat("t", 32)
	prueba.Setenv("CATALOGO_INTERNAL_TOKEN", tokenCatalogo)
	for nombre, valor := range map[string]string{
		"vacío":                "",
		"corto":                strings.Repeat("s", 31),
		"con espacios":         strings.Repeat("s", 20) + " " + strings.Repeat("s", 20),
		"igual al de Catálogo": tokenCatalogo,
	} {
		prueba.Run(nombre, func(prueba *testing.T) {
			prueba.Setenv("SCRAPING_SERVICE_TOKEN", valor)
			if _, _, errorObtenido := app.NewApplication(nil, nil); errorObtenido == nil {
				prueba.Fatal("la configuración insegura debe impedir el arranque")
			}
		})
	}
}
