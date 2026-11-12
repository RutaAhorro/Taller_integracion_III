package integration_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/ingesta"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

type testStore struct {
	calls int
	err   error
}

func TestInternalSQLDataErrorsArePermanentAndPrivate(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		code   string
		status int
	}{
		{"22001", 400}, {"22003", 400}, {"23502", 400}, {"23503", 400}, {"23505", 400}, {"23514", 400},
		{"08006", 500}, {"40001", 500}, {"40P01", 500}, {"42P01", 500}, {"XX000", 500},
	} {
		prueba.Run(tc.code, func(subprueba *testing.T) {
			token := strings.Repeat("t", 32)
			err := fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: tc.code, Message: "secret SQL", TableName: "productos_crudos"})
			router := gin.New()
			ingesta.Register(router, ingesta.Config{Repository: &testStore{err: err}, Token: token, MaxProductos: productosPorLoteDePrueba}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			req := httptest.NewRequest("POST", "/internal/v1/ingestas", strings.NewReader(`{"sucursal_id":1,"productos":[{"sku":"SKU"}]}`))
			req.Header.Set("Authorization", "Bearer "+token)
			respuesta := httptest.NewRecorder()
			router.ServeHTTP(respuesta, req)
			if respuesta.Code != tc.status || strings.Contains(respuesta.Body.String(), "secret") || strings.Contains(respuesta.Body.String(), "productos_crudos") {
				subprueba.Fatal(respuesta.Code, respuesta.Body.String())
			}
		})
	}
}

func (almacen *testStore) ObtenerCadena(context.Context, int, string) (*ingesta.CadenaSupermercado, error) {
	almacen.calls++
	return &ingesta.CadenaSupermercado{ID: 1}, almacen.err
}
func (almacen *testStore) ObtenerSucursal(context.Context, int, *int, *string) (*ingesta.SucursalSupermercado, error) {
	almacen.calls++
	return &ingesta.SucursalSupermercado{ID: 1}, almacen.err
}
func (almacen *testStore) IngestarLote(_ context.Context, input ingesta.Request) (*ingesta.IngestaResultadoDTO, error) {
	almacen.calls++
	return &ingesta.IngestaResultadoDTO{SucursalID: input.SucursalID, TotalRecibidos: len(input.Productos), Insertados: 1, PreciosRegistrados: 1}, almacen.err
}

func TestInternalIngestSecurityAndValidation(prueba *testing.T) {
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
		{"body excesivo", "ingestas", `{"productos":"` + strings.Repeat("x", ingesta.MaxBodyBytes) + `"}`, "Bearer " + token, "", 413, 0},
		{"ingesta válida", "ingestas", `{"sucursal_id":1,"productos":[{"sku":"SKU","producto":"Leche","precio_normal":1000,"en_stock":false}]}`, "Bearer " + token, "", 200, 1},
		{"cadena válida", "cadenas/resolver", `{"cadena_id":1}`, "Bearer " + token, "", 200, 1},
		{"sucursal válida", "sucursales/resolver", `{"sucursal_id":1}`, "Bearer " + token, "", 200, 1},
	} {
		prueba.Run(tc.name, func(subprueba *testing.T) {
			store := &testStore{}
			router := gin.New()
			ingesta.Register(router, ingesta.Config{Repository: store, Token: token, MaxProductos: productosPorLoteDePrueba}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			req := httptest.NewRequest("POST", "/internal/v1/"+tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", tc.auth)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("X-Role", "admin")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != tc.status || store.calls != tc.calls {
				subprueba.Fatalf("status=%d calls=%d body=%s", response.Code, store.calls, response.Body.String())
			}
		})
	}
}

func TestInternalBatchConflictIsControlled(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	token := strings.Repeat("t", 32)
	router := gin.New()
	ingesta.Register(router, ingesta.Config{Repository: &testStore{err: ingesta.ErrIdempotencia}, Token: token, MaxProductos: productosPorLoteDePrueba}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest("POST", "/internal/v1/ingestas", strings.NewReader(`{"lote_id":"00000000-0000-4000-8000-000000000270","sucursal_id":1,"productos":[{"sku":"SKU"}]}`))
	req.Header.Set("Authorization", "Bearer "+token)
	respuesta := httptest.NewRecorder()
	router.ServeHTTP(respuesta, req)
	if respuesta.Code != 409 || !strings.Contains(respuesta.Body.String(), "idempotency_conflict") {
		prueba.Fatal(respuesta.Code, respuesta.Body.String())
	}
}

func TestInternalIngestClosedAndGenericError(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, token := range []string{"", strings.Repeat("t", 32)} {
		store := &testStore{err: errors.New("SQL secret scraper.productos_crudos")}
		router := gin.New()
		ingesta.Register(router, ingesta.Config{Repository: store, Token: token, MaxProductos: productosPorLoteDePrueba}, slog.New(slog.NewTextHandler(io.Discard, nil)))
		req := httptest.NewRequest("POST", "/internal/v1/cadenas/resolver", strings.NewReader(`{"cadena_id":1}`))
		req.Header.Set("Authorization", "Bearer "+token)
		respuesta := httptest.NewRecorder()
		router.ServeHTTP(respuesta, req)
		expected := 500
		if token == "" {
			expected = 404
		}
		if respuesta.Code != expected || strings.Contains(respuesta.Body.String(), "SQL") || strings.Contains(respuesta.Body.String(), "secret") {
			prueba.Fatal(respuesta.Code, respuesta.Body.String())
		}
	}
}
