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
	catalogoToken := strings.Repeat("t", 32)
	prueba.Setenv("SCRAPING_CORS_ALLOWED_ORIGINS", "https://frontend.example")
	prueba.Setenv("CATALOGO_INTERNAL_TOKEN", catalogoToken)
	prueba.Setenv("SCRAPING_SWAGGER_ENABLED", "true")
	router, _, err := app.NewApplication(nil, nil)
	if err != nil {
		prueba.Fatal(err)
	}
	routes := []struct{ method, path string }{
		{"POST", "/api/v1/scraper/trabajos"},
		{"GET", "/api/v1/scraper/trabajos/00000000-0000-0000-0000-000000000000"},
		{"PUT", "/api/v1/scraper/trabajos/00000000-0000-0000-0000-000000000000/finalizar"},
		{"POST", "/api/v1/scraper/trabajos/00000000-0000-0000-0000-000000000000/ejecutar"},
		{"POST", "/api/v1/scraper/trabajos/00000000-0000-0000-0000-000000000000/productos"},
		{"POST", "/api/v1/scraper/productos"},
	}
	headers := map[string]string{
		"sin header":         "",
		"token incorrecto":   "Bearer " + strings.Repeat("x", 32),
		"token de Catálogo":  "Bearer " + catalogoToken,
		"sin esquema Bearer": testServiceToken,
		"espacio extra":      "Bearer  " + testServiceToken,
	}
	for _, route := range routes {
		for name, value := range headers {
			prueba.Run(route.method+" "+route.path+" "+name, func(prueba *testing.T) {
				req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Origin", "https://frontend.example")
				if value != "" {
					req.Header.Set("Authorization", value)
				}
				response := httptest.NewRecorder()
				router.ServeHTTP(response, req)
				if response.Code != 401 {
					prueba.Fatalf("HTTP %d; esperado 401: %s", response.Code, response.Body)
				}
				if response.Header().Get("WWW-Authenticate") == "" {
					prueba.Fatal("falta WWW-Authenticate")
				}
			})
		}
	}
	// Probes y Swagger no exigen token.
	for _, path := range []string{"/_scraping/live", "/swagger/doc.json"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code == 401 {
			prueba.Fatalf("%s no debe exigir token de servicio", path)
		}
	}
}

func TestServiceTokenConfigurationFailsClosed(prueba *testing.T) {
	prepareApplicationEnvironment(prueba)
	catalogoToken := strings.Repeat("t", 32)
	prueba.Setenv("CATALOGO_INTERNAL_TOKEN", catalogoToken)
	for name, value := range map[string]string{
		"vacío":                "",
		"corto":                strings.Repeat("s", 31),
		"con espacios":         strings.Repeat("s", 20) + " " + strings.Repeat("s", 20),
		"igual al de Catálogo": catalogoToken,
	} {
		prueba.Run(name, func(prueba *testing.T) {
			prueba.Setenv("SCRAPING_SERVICE_TOKEN", value)
			if _, _, err := app.NewApplication(nil, nil); err == nil {
				prueba.Fatal("la configuración insegura debe impedir el arranque")
			}
		})
	}
}
