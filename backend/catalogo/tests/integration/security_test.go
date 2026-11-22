package integration_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/api"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestCatalogCORS(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	expires := time.Now().Add(time.Hour)
	token := signedToken(prueba, "admin", jwt.SigningMethodHS256, claveJWTDePrueba, &expires)
	for _, tc := range []struct {
		name, origin, method string
		allowed              []string
		status               int
		cors                 bool
	}{
		{"allowed admin", "https://frontend.example", "GET", []string{"https://frontend.example"}, 200, true},
		{"allowed preflight", "https://frontend.example", "OPTIONS", []string{"https://frontend.example"}, 204, true},
		{"localhost port", "http://localhost:5173", "GET", []string{"http://localhost:5173"}, 200, true},
		{"attacker with admin cookie", "https://evil.example", "GET", []string{"https://frontend.example"}, 403, false},
		{"attacker preflight", "https://evil.example", "OPTIONS", []string{"https://frontend.example"}, 403, false},
		{"suffix spoof", "https://frontend.example.evil.test", "GET", []string{"https://frontend.example"}, 403, false},
		{"wrong port", "http://localhost:5174", "GET", []string{"http://localhost:5173"}, 403, false},
		{"null origin", "null", "GET", []string{"https://frontend.example"}, 403, false},
		{"closed default", "https://frontend.example", "GET", nil, 403, false},
		{"without origin", "", "GET", nil, 200, false},
	} {
		prueba.Run(tc.name, func(subprueba *testing.T) {
			repo := &mockProductoRepository{}
			router := nuevoEnrutadorDePrueba(api.Dependencias{Productos: services.NuevoServicioProductos(repo, limitesProductosDePrueba), OrigenesPermitidos: tc.allowed})
			req := httptest.NewRequest(tc.method, "/api/v1/admin/productos", nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Access-Control-Request-Method", "GET")
			req.AddCookie(&http.Cookie{Name: "jwt", Value: token})
			respuesta := httptest.NewRecorder()
			router.ServeHTTP(respuesta, req)
			if respuesta.Code != tc.status {
				subprueba.Fatalf("HTTP %d: %s", respuesta.Code, respuesta.Body)
			}
			if !strings.Contains(respuesta.Header().Get("Vary"), "Origin") {
				subprueba.Fatal("falta Vary: Origin")
			}
			if tc.cors {
				if respuesta.Header().Get("Access-Control-Allow-Origin") != tc.origin || respuesta.Header().Get("Access-Control-Allow-Credentials") != "true" {
					subprueba.Fatal("origen permitido sin CORS con credenciales")
				}
			} else if respuesta.Header().Get("Access-Control-Allow-Origin") != "" || respuesta.Header().Get("Access-Control-Allow-Credentials") != "" {
				subprueba.Fatal("CORS concedido a un origen no autorizado")
			}
			if tc.status == 403 && repo.lastFilter.Limit != 0 {
				subprueba.Fatal("un origen rechazado alcanzó el repositorio")
			}
		})
	}
}

func TestCatalogInternalErrorsArePrivate(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	const diagnostic = "SQLSTATE 42P01: relation scraper.private_table does not exist"
	expires := time.Now().Add(time.Hour)
	token := signedToken(prueba, "admin", jwt.SigningMethodHS256, claveJWTDePrueba, &expires)
	for _, path := range []string{"/api/v1/admin/productos", "/api/v1/productos", "/api/v1/productos/buscar?q=leche", "/api/v1/productos/private-id", "/panic"} {
		prueba.Run(path, func(subprueba *testing.T) {
			var logs bytes.Buffer
			router := nuevoEnrutadorDePrueba(api.Dependencias{Productos: services.NuevoServicioProductos(&mockProductoRepository{err: errors.New(diagnostic)}, limitesProductosDePrueba), Registrador: slog.New(slog.NewJSONHandler(&logs, nil))})
			router.GET("/panic", func(contexto *gin.Context) { panic(diagnostic) })
			req := httptest.NewRequest("GET", path, nil)
			req.AddCookie(&http.Cookie{Name: "jwt", Value: token})
			req.Header.Set("X-Request-ID", "security-review-267")
			respuesta := httptest.NewRecorder()
			router.ServeHTTP(respuesta, req)
			if respuesta.Code != 500 || strings.Contains(respuesta.Body.String(), "SQLSTATE") || strings.Contains(respuesta.Body.String(), "private_table") {
				subprueba.Fatalf("error interno expuesto: HTTP %d %s", respuesta.Code, respuesta.Body)
			}
			var response map[string]any
			if err := json.Unmarshal(respuesta.Body.Bytes(), &response); err != nil {
				subprueba.Fatal(err)
			}
			if _, exists := response["detalle"]; exists {
				subprueba.Fatal("un error interno incluye detalle")
			}
			found := false
			for _, line := range bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n")) {
				var entry map[string]any
				if err := json.Unmarshal(line, &entry); err != nil {
					subprueba.Fatal(err)
				}
				if entry["msg"] == "http_internal_error" {
					found = true
					if entry["request_id"] != "security-review-267" || !strings.Contains(entry["error"].(string), diagnostic) {
						subprueba.Fatal("diagnóstico interno sin correlación")
					}
				}
			}
			if !found {
				subprueba.Fatal("se perdió el diagnóstico interno")
			}
		})
	}
}
