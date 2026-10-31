package tests

import (
	"bytes"
	"encoding/json"
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

func fuenteHTTPRouter(repo *mockFuenteRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return server.NewRouter(
		services.NewProductoService(&mockProductoRepository{}),
		services.NewFuenteService(repo),
		"catalog-test-only-key",
		nil,
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
		nil,
	)
}

func solicitarFuente(t *testing.T, router http.Handler, method, path, cookie, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "jwt", Value: cookie})
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func TestFuenteHandler_HTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	future := time.Now().Add(time.Hour)
	admin := signedToken(t, "admin", jwt.SigningMethodHS256, "catalog-test-only-key", &future)
	registered := signedToken(t, "registrado", jwt.SigningMethodHS256, "catalog-test-only-key", &future)
	fecha := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	nuevaRepo := func() *mockFuenteRepository {
		return &mockFuenteRepository{
			fuentes: []domain.FuenteDTO{
				{ID: 1, Nombre: "Jumbo", Habilitada: true, TotalSucursales: 8, TotalProductos: 120, CreadoEl: fecha},
				{ID: 2, Nombre: "Lider", Habilitada: false, TotalSucursales: 12, TotalProductos: 340, CreadoEl: fecha},
			},
		}
	}

	t.Run("GET sin cookie", func(t *testing.T) {
		response := solicitarFuente(t, fuenteHTTPRouter(nuevaRepo()), http.MethodGet, "/api/v1/admin/productos/fuentes", "", "")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("HTTP %d; esperado 401: %s", response.Code, response.Body)
		}
	})

	t.Run("GET con rol registrado", func(t *testing.T) {
		response := solicitarFuente(t, fuenteHTTPRouter(nuevaRepo()), http.MethodGet, "/api/v1/admin/productos/fuentes", registered, "")
		if response.Code != http.StatusForbidden {
			t.Fatalf("HTTP %d; esperado 403: %s", response.Code, response.Body)
		}
	})

	t.Run("GET con admin", func(t *testing.T) {
		response := solicitarFuente(t, fuenteHTTPRouter(nuevaRepo()), http.MethodGet, "/api/v1/admin/productos/fuentes", admin, "")
		if response.Code != http.StatusOK {
			t.Fatalf("HTTP %d; esperado 200: %s", response.Code, response.Body)
		}
		var fuentes []domain.FuenteDTO
		if err := json.Unmarshal(response.Body.Bytes(), &fuentes); err != nil {
			t.Fatal(err)
		}
		if len(fuentes) != 2 || fuentes[0].Nombre != "Jumbo" || !fuentes[0].Habilitada {
			t.Fatalf("listado de fuentes inesperado: %+v", fuentes)
		}
	})

	t.Run("PUT deshabilita y persiste", func(t *testing.T) {
		repo := nuevaRepo()
		response := solicitarFuente(t, fuenteHTTPRouter(repo), http.MethodPut, "/api/v1/admin/productos/fuentes/1", admin, `{"habilitada":false}`)
		if response.Code != http.StatusOK {
			t.Fatalf("HTTP %d; esperado 200: %s", response.Code, response.Body)
		}
		var fuente domain.FuenteDTO
		if err := json.Unmarshal(response.Body.Bytes(), &fuente); err != nil {
			t.Fatal(err)
		}
		if fuente.Habilitada || repo.fuentes[0].Habilitada {
			t.Fatalf("la fuente no quedó deshabilitada: respuesta=%+v repo=%+v", fuente, repo.fuentes[0])
		}
	})

	for _, tc := range []struct {
		name, path, cookie, body string
		status                   int
		contains                 string
	}{
		{"PUT sin cookie", "/api/v1/admin/productos/fuentes/1", "", `{"habilitada":false}`, http.StatusUnauthorized, ""},
		{"PUT con rol registrado", "/api/v1/admin/productos/fuentes/1", registered, `{"habilitada":false}`, http.StatusForbidden, ""},
		{"PUT con ID inválido", "/api/v1/admin/productos/fuentes/abc", admin, `{"habilitada":false}`, http.StatusBadRequest, "ID de fuente inválido"},
		{"PUT sin estado", "/api/v1/admin/productos/fuentes/1", admin, `{}`, http.StatusBadRequest, "estado de la fuente"},
		{"PUT de fuente inexistente", "/api/v1/admin/productos/fuentes/99", admin, `{"habilitada":true}`, http.StatusNotFound, "no encontrada"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := solicitarFuente(t, fuenteHTTPRouter(nuevaRepo()), http.MethodPut, tc.path, tc.cookie, tc.body)
			if response.Code != tc.status {
				t.Fatalf("HTTP %d; esperado %d: %s", response.Code, tc.status, response.Body)
			}
			if tc.contains != "" && !strings.Contains(response.Body.String(), tc.contains) {
				t.Fatalf("respuesta sin %q: %s", tc.contains, response.Body)
			}
		})
	}
}
