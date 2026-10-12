package main

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

var testServiceToken = strings.Repeat("s", 32)

// El Gateway es público: sin el token de servicio ninguna ruta de coordinación
// llega al handler, aunque el cuerpo sea válido o el origen esté permitido.
func TestScraperRequiresServiceToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	catalogoToken := strings.Repeat("t", 32)
	t.Setenv("SCRAPING_CORS_ALLOWED_ORIGINS", "https://frontend.example")
	t.Setenv("CATALOGO_INTERNAL_URL", "http://catalogo:8080")
	t.Setenv("CATALOGO_INTERNAL_TOKEN", catalogoToken)
	t.Setenv("SCRAPING_SERVICE_TOKEN", testServiceToken)
	router, err := setupRouter()
	if err != nil {
		t.Fatal(err)
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
			t.Run(route.method+" "+route.path+" "+name, func(t *testing.T) {
				req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Origin", "https://frontend.example")
				if value != "" {
					req.Header.Set("Authorization", value)
				}
				response := httptest.NewRecorder()
				router.ServeHTTP(response, req)
				if response.Code != 401 {
					t.Fatalf("HTTP %d; esperado 401: %s", response.Code, response.Body)
				}
				if response.Header().Get("WWW-Authenticate") == "" {
					t.Fatal("falta WWW-Authenticate")
				}
			})
		}
	}
	// Probes y Swagger no exigen token.
	for _, path := range []string{"/_scraping/live", "/swagger/doc.json"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code == 401 {
			t.Fatalf("%s no debe exigir token de servicio", path)
		}
	}
}

func TestServiceTokenConfigurationFailsClosed(t *testing.T) {
	catalogoToken := strings.Repeat("t", 32)
	t.Setenv("CATALOGO_INTERNAL_URL", "http://catalogo:8080")
	t.Setenv("CATALOGO_INTERNAL_TOKEN", catalogoToken)
	for name, value := range map[string]string{
		"vacío":                "",
		"corto":                strings.Repeat("s", 31),
		"con espacios":         strings.Repeat("s", 20) + " " + strings.Repeat("s", 20),
		"igual al de Catálogo": catalogoToken,
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("SCRAPING_SERVICE_TOKEN", value)
			if _, err := setupRouter(); err == nil {
				t.Fatal("la configuración insegura debe impedir el arranque")
			}
		})
	}
}
