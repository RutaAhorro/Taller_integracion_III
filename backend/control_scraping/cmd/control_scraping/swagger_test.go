package main

import (
	"encoding/json"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// Cada operación publicada debe existir en el router del servicio, incluso al
// generar Swagger después de extraer otro dominio.
func TestSwaggerOnlyDocumentsRegisteredRoutes(t *testing.T) {
	t.Setenv("CATALOGO_INTERNAL_URL", "http://catalogo:8080")
	t.Setenv("CATALOGO_INTERNAL_TOKEN", strings.Repeat("t", 32))
	t.Setenv("SCRAPING_SERVICE_TOKEN", testServiceToken)
	router, err := setupRouter()
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
