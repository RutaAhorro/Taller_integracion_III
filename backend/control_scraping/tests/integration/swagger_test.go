// Package integration_test comprueba que Swagger publique sólo el contrato activo.
package integration_test

import (
	"encoding/json"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/app"
)

// Cada operación publicada debe existir en el router del servicio, incluso al
// generar Swagger después de extraer otro dominio.
func TestSwaggerOnlyDocumentsRegisteredRoutes(t *testing.T) {
	prepareApplicationEnvironment(t)
	t.Setenv("SCRAPING_SWAGGER_ENABLED", "true")
	router, _, err := app.NewApplication(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/swagger/doc.json", nil))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Paths) == 0 || doc.Paths["/api/v1/scraper/trabajos"] == nil {
		t.Fatal("missing coordination contract")
	}
	routes := make(map[string]bool)
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	parameter := regexp.MustCompile(`\{([^}]+)\}`)
	for path, methods := range doc.Paths {
		if strings.HasPrefix(path, "/_") {
			t.Errorf("Swagger público incluye probe privado: %s", path)
		}
		for method := range methods {
			key := strings.ToUpper(method) + " " + parameter.ReplaceAllString(path, ":$1")
			if !routes[key] {
				t.Errorf("Swagger advertises an unregistered operation: %s", key)
			}
		}
	}
}

// SUP-274: sin habilitarlo explícitamente, el contrato no se publica.
func TestSwaggerDisabledByDefault(t *testing.T) {
	prepareApplicationEnvironment(t)
	for _, value := range []string{"", "false"} {
		t.Run("valor "+value, func(t *testing.T) {
			t.Setenv("SCRAPING_SWAGGER_ENABLED", value)
			router, _, err := app.NewApplication(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"/swagger/doc.json", "/swagger/index.html"} {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
				if response.Code != 404 {
					t.Fatalf("%s publicado sin habilitar: %d", path, response.Code)
				}
			}
		})
	}
	t.Run("valor inválido", func(t *testing.T) {
		t.Setenv("SCRAPING_SWAGGER_ENABLED", "yes")
		if _, _, err := app.NewApplication(nil, nil); err == nil {
			t.Fatal("un valor ambiguo debe impedir el arranque")
		}
	})
}
