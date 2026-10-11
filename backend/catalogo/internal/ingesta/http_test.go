package ingesta

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type testStore struct {
	calls int
	err   error
}

func (s *testStore) ObtenerCadena(context.Context, int, string) (*CadenaSupermercado, error) {
	s.calls++
	return &CadenaSupermercado{ID: 1}, s.err
}
func (s *testStore) ObtenerSucursal(context.Context, int, *int, *string) (*SucursalSupermercado, error) {
	s.calls++
	return &SucursalSupermercado{ID: 1}, s.err
}
func (s *testStore) IngestarLote(_ context.Context, input Request) (*IngestaResultadoDTO, error) {
	s.calls++
	return &IngestaResultadoDTO{SucursalID: input.SucursalID, TotalRecibidos: len(input.Productos), Insertados: 1, PreciosRegistrados: 1}, s.err
}

func TestInternalIngestSecurityAndValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := strings.Repeat("t", 32)
	for _, tc := range []struct {
		name, path, body, auth, origin string
		status, calls                  int
	}{
		{"sin token", "ingestas", `{"sucursal_id":1,"productos":[{"sku":"SKU"}]}`, "", "", 401, 0},
		{"JWT o rol no reemplaza token", "ingestas", `{}`, "Bearer user-jwt", "", 401, 0},
		{"navegador rechazado", "ingestas", `{}`, "Bearer " + token, "https://frontend.example", 403, 0},
		{"JSON inválido", "ingestas", `{`, "Bearer " + token, "", 400, 0},
		{"dos documentos", "ingestas", `{} {}`, "Bearer " + token, "", 400, 0},
		{"lote vacío", "ingestas", `{"sucursal_id":1,"productos":[]}`, "Bearer " + token, "", 400, 0},
		{"clave inválida", "ingestas", `{"lote_id":"invalid","sucursal_id":1,"productos":[{"sku":"SKU"}]}`, "Bearer " + token, "", 400, 0},
		{"trabajo fuera de dominio", "ingestas", `{"trabajo_id":"x","sucursal_id":1,"productos":[]}`, "Bearer " + token, "", 400, 0},
		{"body excesivo", "ingestas", `{"productos":"` + strings.Repeat("x", MaxBodyBytes) + `"}`, "Bearer " + token, "", 413, 0},
		{"ingesta válida", "ingestas", `{"sucursal_id":1,"productos":[{"sku":"SKU","producto":"Leche","precio_normal":1000,"en_stock":false}]}`, "Bearer " + token, "", 200, 1},
		{"cadena válida", "cadenas/resolver", `{"cadena_id":1}`, "Bearer " + token, "", 200, 1},
		{"sucursal válida", "sucursales/resolver", `{"sucursal_id":1}`, "Bearer " + token, "", 200, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &testStore{}
			r := gin.New()
			Register(r, Config{Repository: store, Token: token}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			req := httptest.NewRequest("POST", "/internal/v1/"+tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", tc.auth)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("X-Role", "admin")
			response := httptest.NewRecorder()
			r.ServeHTTP(response, req)
			if response.Code != tc.status || store.calls != tc.calls {
				t.Fatalf("status=%d calls=%d body=%s", response.Code, store.calls, response.Body.String())
			}
		})
	}
}

func TestInternalBatchConflictIsControlled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := strings.Repeat("t", 32)
	r := gin.New()
	Register(r, Config{Repository: &testStore{err: ErrIdempotencia}, Token: token}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest("POST", "/internal/v1/ingestas", strings.NewReader(`{"lote_id":"00000000-0000-4000-8000-000000000270","sucursal_id":1,"productos":[{"sku":"SKU"}]}`))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "idempotency_conflict") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestInternalIngestClosedAndGenericError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, token := range []string{"", strings.Repeat("t", 32)} {
		store := &testStore{err: errors.New("SQL secret scraper.productos_crudos")}
		r := gin.New()
		Register(r, Config{Repository: store, Token: token}, slog.New(slog.NewTextHandler(io.Discard, nil)))
		req := httptest.NewRequest("POST", "/internal/v1/cadenas/resolver", strings.NewReader(`{"cadena_id":1}`))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		expected := 500
		if token == "" {
			expected = 404
		}
		if w.Code != expected || strings.Contains(w.Body.String(), "SQL") || strings.Contains(w.Body.String(), "secret") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
