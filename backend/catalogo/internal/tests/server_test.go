package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/server"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func catalogRouter(repo *mockProductoRepository, ping func(context.Context) error) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return server.NewRouter(services.NewProductoService(repo), "catalog-test-only-key", ping, slog.New(slog.NewJSONHandler(io.Discard, nil)), nil)
}

func signedToken(t *testing.T, role string, method jwt.SigningMethod, key string, expires *time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{"user_id": "test-user", "rol": role, "provider": "local"}
	if expires != nil {
		claims["exp"] = expires.Unix()
	}
	token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestCatalogAdministrationAuthorization(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	admin := signedToken(t, "admin", jwt.SigningMethodHS256, "catalog-test-only-key", &future)
	registered := signedToken(t, "registrado", jwt.SigningMethodHS256, "catalog-test-only-key", &future)
	wrongKey := signedToken(t, "admin", jwt.SigningMethodHS256, "different-test-only-key", &future)
	expired := signedToken(t, "admin", jwt.SigningMethodHS256, "catalog-test-only-key", &past)
	wrongAlgorithm := signedToken(t, "admin", jwt.SigningMethodHS384, "catalog-test-only-key", &future)
	missingExpiry := signedToken(t, "admin", jwt.SigningMethodHS256, "catalog-test-only-key", nil)
	for _, tc := range []struct {
		name, cookie string
		status       int
	}{
		{"sin cookie ni confianza en cabeceras", "", 401},
		{"token malformado", "invalid", 401},
		{"firma incorrecta", wrongKey, 401},
		{"expirado", expired, 401},
		{"algoritmo distinto del emisor", wrongAlgorithm, 401},
		{"sin expiración", missingExpiry, 401},
		{"registrado", registered, 403},
		{"admin", admin, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockProductoRepository{adminItems: []domain.ProductoAdminDTO{{ID: "product", SKU: "sku-test"}}, total: 1}
			router := catalogRouter(repo, nil)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/productos/", nil)
			req.Header.Set("X-User-ID", "forged-user")
			req.Header.Set("X-User-Role", "admin")
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "jwt", Value: tc.cookie})
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("HTTP %d; esperado %d: %s", response.Code, tc.status, response.Body)
			}
			if tc.status == 200 {
				var page domain.PaginaProductosAdminDTO
				if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
					t.Fatal(err)
				}
				if len(page.Data) != 1 || page.Data[0].SKU != "sku-test" {
					t.Fatal("administración no consulta el catálogo")
				}
			} else if repo.lastFilter.Limit != 0 {
				t.Fatal("se consultó persistencia antes de autorizar al usuario")
			}
		})
	}
}

func TestCatalogHealthAndDomainIsolation(t *testing.T) {
	for _, dbAvailable := range []bool{true, false} {
		name := "database_down"
		if dbAvailable {
			name = "database_up"
		}
		t.Run(name, func(t *testing.T) {
			router := catalogRouter(&mockProductoRepository{}, func(ctx context.Context) error {
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 2*time.Second {
					t.Error("readiness no limita el tiempo de consulta")
				}
				if !dbAvailable {
					return errors.New("test database unavailable")
				}
				return nil
			})
			for _, endpoint := range []struct {
				method, path string
				status       int
			}{
				{"GET", "/_catalogo/live", 200},
				{"GET", "/_catalogo/ready", map[bool]int{true: 200, false: 503}[dbAvailable]},
				{"POST", "/api/v1/auth/login", 404},
				{"GET", "/api/v1/scraper/trabajos/test-job", 404},
				{"GET", "/api/v1/rutas", 404},
				{"PUT", "/api/v1/productos", 405},
			} {
				req := httptest.NewRequest(endpoint.method, endpoint.path, nil)
				response := httptest.NewRecorder()
				router.ServeHTTP(response, req)
				if response.Code != endpoint.status {
					t.Errorf("%s: HTTP %d; esperado %d", endpoint.path, response.Code, endpoint.status)
				}
			}
		})
	}
}

func TestCatalogRequestCorrelation(t *testing.T) {
	var logs bytes.Buffer
	router := server.NewRouter(services.NewProductoService(&mockProductoRepository{}), "test-key", nil, slog.New(slog.NewJSONHandler(&logs, nil)), nil)
	for _, received := range []string{"gateway-request-267", strings.Repeat("x", 129)} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/productos/buscar?q=le", nil)
		req.Header.Set("X-Request-ID", received)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		returned := response.Header().Get("X-Request-ID")
		if returned == "" || (len(received) <= 128 && returned != received) || (len(received) > 128 && returned == received) {
			t.Fatal("identificador de correlación inválido")
		}
		var entry map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &entry); err != nil {
			t.Fatal(err)
		}
		if entry["request_id"] != returned || entry["path"] != "/api/v1/productos/buscar" || entry["status"] != float64(400) {
			t.Fatal("correlación o ruta de log incorrecta")
		}
		logs.Reset()
	}
}
