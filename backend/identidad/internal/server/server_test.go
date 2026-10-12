package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPBoundariesCorsAndJWT(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("j", 32))
	cfg := config.Config{AllowedOrigins: []string{"https://frontend.example"}}
	r := NewRouter(cfg, nil, nil, func(context.Context) error { return nil }, nil)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": "fixture", "rol": "registrado", "provider": "local", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(strings.Repeat("j", 32)))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, method, path, origin, body, cookie string
		status                                   int
	}{
		{"live", "GET", "/_identidad/live", "", "", "", 200}, {"ready without Redis", "GET", "/_identidad/ready", "", "", "", 503},
		{"missing cookie", "GET", "/api/v1/auth/me", "", "", "", 401}, {"valid cookie", "GET", "/api/v1/auth/me", "", "", token, 200},
		{"allowed CORS", "GET", "/api/v1/auth/me", "https://frontend.example", "", token, 200}, {"hostile CORS", "GET", "/api/v1/auth/me", "https://evil.example", "", token, 403},
		{"hostile mutation", "PUT", "/api/v1/auth/me", "https://evil.example", `{"nombre_completo":"forged"}`, token, 403},
		{"allowed preflight", "OPTIONS", "/api/v1/auth/me", "https://frontend.example", "", "", 204}, {"hostile preflight", "OPTIONS", "/api/v1/auth/me", "https://evil.example", "", "", 403},
		{"fail closed registration", "POST", "/api/v1/auth/register", "", `{}`, "", 500}, {"fail closed login", "POST", "/api/v1/auth/login", "", `{}`, "", 500}, {"fail closed google", "POST", "/api/v1/auth/google", "", `{}`, "", 500},
		{"catalog not owned", "GET", "/api/v1/productos", "", "", "", 404}, {"scraping not owned", "POST", "/api/v1/scraper/trabajos", "", `{}`, "", 404}, {"method not allowed", "DELETE", "/api/v1/auth/me", "", "", "", 405},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("X-User-ID", "forged")
			req.Header.Set("X-Role", "admin")
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "jwt", Value: tc.cookie})
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
			if tc.origin != "https://frontend.example" && w.Header().Get("Access-Control-Allow-Credentials") != "" {
				t.Fatal("credenciales CORS fuera de allowlist")
			}
			if w.Header().Get("X-Request-ID") == "" {
				t.Fatal("correlación ausente")
			}
			if w.Code == 500 && strings.Contains(w.Body.String(), "detalle") {
				t.Fatal("error interno expuesto")
			}
			if tc.name == "valid cookie" {
				var body map[string]any
				json.Unmarshal(w.Body.Bytes(), &body)
				if body["rol"] != "registrado" || body["user_id"] != "fixture" {
					t.Fatal("headers reemplazaron JWT")
				}
			}
		})
	}
}
func TestRecoveryDoesNotExposePanic(t *testing.T) {
	r := NewRouter(config.Config{}, nil, nil, nil, nil)
	r.GET("/panic", func(c *gin.Context) { panic("private SQL table and credentials") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/panic", nil))
	if w.Code != 500 || strings.Contains(w.Body.String(), "private") {
		t.Fatal(w.Code, w.Body)
	}
}
func TestReadinessDeadlineAndLiveness(t *testing.T) {
	calls := 0
	r := NewRouter(config.Config{}, nil, nil, func(ctx context.Context) error { calls++; <-ctx.Done(); return errors.New("private diagnostic") }, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	w := httptest.NewRecorder()
	start := time.Now()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/_identidad/ready", nil).WithContext(ctx))
	if w.Code != 503 || time.Since(start) > time.Second {
		t.Fatal("deadline perdido")
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/_identidad/live", nil))
	if calls != 1 || w.Code != 200 {
		t.Fatal("live consulta DB")
	}
}

func TestUnclassifiedContextErrorDoesNotExposeSQL(t *testing.T) {
	r := NewRouter(config.Config{}, nil, nil, nil, nil)
	r.GET("/context-error", func(c *gin.Context) { _ = c.Error(errors.New("private SQL table and credentials")) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/context-error", nil))
	if w.Code != 500 || strings.Contains(w.Body.String(), "private") || strings.Contains(w.Body.String(), "detalle") {
		t.Fatal(w.Code, w.Body)
	}
}
