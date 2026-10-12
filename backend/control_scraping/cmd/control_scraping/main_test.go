package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Comprueba la composición real y que la API ya no aloje Identidad ni Catálogo.
func TestModuleCompositionHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("CATALOGO_INTERNAL_URL", "http://catalogo:8080")
	t.Setenv("CATALOGO_INTERNAL_TOKEN", strings.Repeat("t", 32))
	t.Setenv("CATALOGO_INGEST_TIMEOUT", "")
	router, err := setupRouter()
	if err != nil {
		t.Fatal(err)
	}
	token := "cookie-que-control-no-interpreta"

	cases := []struct {
		name, method, path, body, cookie string
		status                           int
	}{
		{"liveness sin dependencias", "GET", "/_api/live", "", "", 200},
		{"readiness sin bases", "GET", "/health", "", "", 503},
		{"readiness v1 sin bases", "GET", "/api/v1/health", "", "", 503},
		{"perfil fuera de API", "GET", "/api/v1/auth/me", "", "", 404},
		{"actualización fuera de API", "PUT", "/api/v1/auth/me", "{}", "", 404},
		{"admin fuera de API", "GET", "/api/v1/admin/productos", "", "", 404},
		{"admin fuera de API con cookie", "GET", "/api/v1/admin/productos", "", token, 404},
		{"admin barra fuera de API", "GET", "/api/v1/admin/productos/", "", token, 404},
		{"perfil con cookie fuera de API", "GET", "/api/v1/auth/me", "", token, 404},
		{"login fuera de API", "POST", "/api/v1/auth/login", "{}", "", 404},
		{"registro fuera de API", "POST", "/api/v1/auth/register", "{}", "", 404},
		{"google fuera de API", "POST", "/api/v1/auth/google", "{}", "", 404},
		{"búsqueda fuera de API", "GET", "/api/v1/productos/buscar?q=le", "", "", 404},
		{"catálogo fuera de API", "GET", "/api/v1/productos", "", "", 404},
		{"detalle fuera de API", "GET", "/api/v1/productos/product-id", "", "", 404},
		{"trabajo inválido", "GET", "/api/v1/scraper/trabajos/invalid", "", "", 400},
		{"creación inválida", "POST", "/api/v1/scraper/trabajos", "{}", "", 400},
		{"finalización inválida", "PUT", "/api/v1/scraper/trabajos/invalid/finalizar", "{}", "", 400},
		{"ingesta con trabajo inválida", "POST", "/api/v1/scraper/trabajos/invalid/productos", "{}", "", 400},
		{"ingesta directa inválida", "POST", "/api/v1/scraper/productos", "{}", "", 400},
		{"ejecución con ID inválido sin Redis", "POST", "/api/v1/scraper/trabajos/invalid/ejecutar", "{}", "", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "jwt", Value: tc.cookie})
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("HTTP %d; esperado %d: %s", response.Code, tc.status, response.Body)
			}

		})
	}
}
