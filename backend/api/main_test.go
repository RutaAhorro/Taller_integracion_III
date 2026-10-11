package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Comprueba la composición real: no sustituye los interceptores de Identidad
// por mocks ni necesita PostgreSQL para probar rechazos previos a persistencia.
func TestModuleCompositionHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_SECRET", "sup266-test-only-key")
	router := setupRouter()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "c14bd236-294a-41b8-8cbb-5473bb610415",
		"rol":     "registrado", "provider": "local",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("sup266-test-only-key"))
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, method, path, body, cookie string
		status                           int
	}{
		{"perfil sin sesión", "GET", "/api/v1/auth/me", "", "", 401},
		{"actualización sin sesión", "PUT", "/api/v1/auth/me", "{}", "", 401},
		{"admin fuera de API", "GET", "/api/v1/admin/productos", "", "", 404},
		{"admin fuera de API con cookie", "GET", "/api/v1/admin/productos", "", token, 404},
		{"admin barra fuera de API", "GET", "/api/v1/admin/productos/", "", token, 404},
		{"perfil con sesión", "GET", "/api/v1/auth/me", "", token, 200},
		{"login sin Redis falla cerrado", "POST", "/api/v1/auth/login", "{}", "", 500},
		{"registro inválido", "POST", "/api/v1/auth/register", "{}", "", 400},
		{"google inválido", "POST", "/api/v1/auth/google", "{}", "", 400},
		{"búsqueda fuera de API", "GET", "/api/v1/productos/buscar?q=le", "", "", 404},
		{"catálogo fuera de API", "GET", "/api/v1/productos", "", "", 404},
		{"detalle fuera de API", "GET", "/api/v1/productos/product-id", "", "", 404},
		{"trabajo inválido", "GET", "/api/v1/scraper/trabajos/invalid", "", "", 400},
		{"creación inválida", "POST", "/api/v1/scraper/trabajos", "{}", "", 400},
		{"finalización inválida", "PUT", "/api/v1/scraper/trabajos/invalid/finalizar", "{}", "", 400},
		{"ingesta con trabajo inválida", "POST", "/api/v1/scraper/trabajos/invalid/productos", "{}", "", 400},
		{"ingesta directa inválida", "POST", "/api/v1/scraper/productos", "{}", "", 400},
		{"ejecución sin Redis", "POST", "/api/v1/scraper/trabajos/invalid/ejecutar", "{}", "", 503},
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
			if tc.name == "perfil con sesión" {
				var body map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body["user_id"] != "c14bd236-294a-41b8-8cbb-5473bb610415" || body["rol"] != "registrado" || body["provider"] != "local" {
					t.Fatalf("claims de sesión alterados: %v", body)
				}
			}
		})
	}
}
