package integration_test

import (
	"context"
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

func TestHTTPBoundariesCorsAndJWT(prueba *testing.T) {
	cfg := config.Config{JWTSecret: strings.Repeat("j", 32), AllowedOrigins: []string{"https://frontend.example"}}
	peticion := enrutadorDePrueba(prueba, cfg, func(context.Context) error { return nil })
	// Con la base inalcanzable un token válido no se autoriza y la
	// petición termina en 500 fail-closed, nunca en un bypass de la revalidación.
	const usuarioID = "c14bd236-294a-41b8-8cbb-5473bb610415"
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": usuarioID, "rol": "registrado", "provider": "local", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(strings.Repeat("j", 32)))
	if err != nil {
		prueba.Fatal(err)
	}
	for _, tc := range []struct {
		name, method, path, origin, body, cookie string
		status                                   int
	}{
		{"live", "GET", "/_identidad/live", "", "", "", 200}, {"ready without Redis", "GET", "/_identidad/ready", "", "", "", 503},
		{"missing cookie", "GET", "/api/v1/auth/me", "", "", "", 401}, {"valid cookie fails closed without DB", "GET", "/api/v1/auth/me", "", "", token, 500},
		{"allowed CORS", "GET", "/api/v1/auth/me", "https://frontend.example", "", token, 500}, {"hostile CORS", "GET", "/api/v1/auth/me", "https://evil.example", "", token, 403},
		{"hostile mutation", "PUT", "/api/v1/auth/me", "https://evil.example", `{"nombre_completo":"forged"}`, token, 403},
		{"allowed preflight", "OPTIONS", "/api/v1/auth/me", "https://frontend.example", "", "", 204}, {"hostile preflight", "OPTIONS", "/api/v1/auth/me", "https://evil.example", "", "", 403},
		{"fail closed registration", "POST", "/api/v1/auth/register", "", `{}`, "", 500}, {"fail closed login", "POST", "/api/v1/auth/login", "", `{}`, "", 500}, {"fail closed google", "POST", "/api/v1/auth/google", "", `{}`, "", 500},
		{"catalog not owned", "GET", "/api/v1/productos", "", "", "", 404}, {"scraping not owned", "POST", "/api/v1/scraper/trabajos", "", `{}`, "", 404}, {"method not allowed", "DELETE", "/api/v1/auth/me", "", "", "", 405},
	} {
		prueba.Run(tc.name, func(prueba *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("X-User-ID", "forged")
			req.Header.Set("X-Role", "admin")
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "jwt", Value: tc.cookie})
			}
			escritor := httptest.NewRecorder()
			peticion.ServeHTTP(escritor, req)
			if escritor.Code != tc.status {
				prueba.Fatalf("%d %s", escritor.Code, escritor.Body)
			}
			if tc.origin != "https://frontend.example" && escritor.Header().Get("Access-Control-Allow-Credentials") != "" {
				prueba.Fatal("credenciales CORS fuera de allowlist")
			}
			if escritor.Header().Get("X-Request-ID") == "" {
				prueba.Fatal("correlación ausente")
			}
			if escritor.Code == 500 && strings.Contains(escritor.Body.String(), "detalle") {
				prueba.Fatal("error interno expuesto")
			}
		})
	}
}
func TestRecoveryDoesNotExposePanic(prueba *testing.T) {
	peticion := enrutadorDePrueba(prueba, config.Config{}, nil)
	peticion.GET("/panic", func(contexto *gin.Context) { panic("private SQL table and credentials") })
	escritor := httptest.NewRecorder()
	peticion.ServeHTTP(escritor, httptest.NewRequest("GET", "/panic", nil))
	if escritor.Code != 500 || strings.Contains(escritor.Body.String(), "private") {
		prueba.Fatal(escritor.Code, escritor.Body)
	}
}
func TestReadinessDeadlineAndLiveness(prueba *testing.T) {
	calls := 0
	peticion := enrutadorDePrueba(prueba, config.Config{}, func(ctx context.Context) error { calls++; <-ctx.Done(); return errors.New("private diagnostic") })
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	escritor := httptest.NewRecorder()
	start := time.Now()
	peticion.ServeHTTP(escritor, httptest.NewRequest("GET", "/_identidad/ready", nil).WithContext(ctx))
	if escritor.Code != 503 || time.Since(start) > time.Second {
		prueba.Fatal("deadline perdido")
	}
	escritor = httptest.NewRecorder()
	peticion.ServeHTTP(escritor, httptest.NewRequest("GET", "/_identidad/live", nil))
	if calls != 1 || escritor.Code != 200 {
		prueba.Fatal("live consulta DB")
	}
}

func TestUnclassifiedContextErrorDoesNotExposeSQL(prueba *testing.T) {
	peticion := enrutadorDePrueba(prueba, config.Config{}, nil)
	peticion.GET("/context-error", func(contexto *gin.Context) { _ = contexto.Error(errors.New("private SQL table and credentials")) })
	escritor := httptest.NewRecorder()
	peticion.ServeHTTP(escritor, httptest.NewRequest("GET", "/context-error", nil))
	if escritor.Code != 500 || strings.Contains(escritor.Body.String(), "private") || strings.Contains(escritor.Body.String(), "detalle") {
		prueba.Fatal(escritor.Code, escritor.Body)
	}
}
