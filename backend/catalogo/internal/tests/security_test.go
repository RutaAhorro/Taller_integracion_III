package tests

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/middleware"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/server"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestCatalogCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expires := time.Now().Add(time.Hour)
	token := signedToken(t, "admin", jwt.SigningMethodHS256, "test-key", &expires)
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
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockProductoRepository{}
			router := server.NewRouter(services.NewProductoService(repo), "test-key", nil, slog.New(slog.NewJSONHandler(io.Discard, nil)), tc.allowed)
			req := httptest.NewRequest(tc.method, "/api/v1/admin/productos", nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Access-Control-Request-Method", "GET")
			req.AddCookie(&http.Cookie{Name: "jwt", Value: token})
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("HTTP %d: %s", w.Code, w.Body)
			}
			if !strings.Contains(w.Header().Get("Vary"), "Origin") {
				t.Fatal("falta Vary: Origin")
			}
			if tc.cors {
				if w.Header().Get("Access-Control-Allow-Origin") != tc.origin || w.Header().Get("Access-Control-Allow-Credentials") != "true" {
					t.Fatal("origen permitido sin CORS con credenciales")
				}
			} else if w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Access-Control-Allow-Credentials") != "" {
				t.Fatal("CORS concedido a un origen no autorizado")
			}
			if tc.status == 403 && repo.lastFilter.Limit != 0 {
				t.Fatal("un origen rechazado alcanzó el repositorio")
			}
		})
	}
}

func TestCatalogInternalErrorsArePrivate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const diagnostic = "SQLSTATE 42P01: relation scraper.private_table does not exist"
	expires := time.Now().Add(time.Hour)
	token := signedToken(t, "admin", jwt.SigningMethodHS256, "test-key", &expires)
	for _, path := range []string{"/api/v1/admin/productos", "/api/v1/productos", "/api/v1/productos/buscar?q=leche", "/api/v1/productos/private-id", "/panic", "/context-error", "/explicit-error"} {
		t.Run(path, func(t *testing.T) {
			var logs bytes.Buffer
			router := server.NewRouter(services.NewProductoService(&mockProductoRepository{err: errors.New(diagnostic)}), "test-key", nil, slog.New(slog.NewJSONHandler(&logs, nil)), nil)
			router.GET("/panic", func(c *gin.Context) { panic(diagnostic) })
			router.GET("/context-error", func(c *gin.Context) { _ = c.Error(errors.New(diagnostic)) })
			router.GET("/explicit-error", func(c *gin.Context) { middleware.ResponderError(c, 500, diagnostic, errors.New(diagnostic)) })
			req := httptest.NewRequest("GET", path, nil)
			req.AddCookie(&http.Cookie{Name: "jwt", Value: token})
			req.Header.Set("X-Request-ID", "security-review-267")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != 500 || strings.Contains(w.Body.String(), "SQLSTATE") || strings.Contains(w.Body.String(), "private_table") {
				t.Fatalf("error interno expuesto: HTTP %d %s", w.Code, w.Body)
			}
			var response map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if _, exists := response["detalle"]; exists {
				t.Fatal("un error interno incluye detalle")
			}
			found := false
			for _, line := range bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n")) {
				var entry map[string]any
				if err := json.Unmarshal(line, &entry); err != nil {
					t.Fatal(err)
				}
				if entry["msg"] == "http_internal_error" {
					found = true
					if entry["request_id"] != "security-review-267" || !strings.Contains(entry["error"].(string), diagnostic) {
						t.Fatal("diagnóstico interno sin correlación")
					}
				}
			}
			if !found {
				t.Fatal("se perdió el diagnóstico interno")
			}
		})
	}
}

func TestOnlyControlledValidationDetailsArePublic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name   string
		err    error
		detail string
	}{
		{"controlled", middleware.ErrorValidacion{Mensaje: "El filtro es inválido."}, "El filtro es inválido."},
		{"unexpected", errors.New("private SQL diagnostic"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/validation", func(c *gin.Context) { middleware.ResponderError(c, 400, "Solicitud inválida.", tc.err) })
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", "/validation", nil))
			var response middleware.RespuestaError
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if w.Code != 400 || response.Detalle != tc.detail {
				t.Fatalf("detalle inválido: %s", w.Body)
			}
		})
	}
}
