package controlscraping_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/controlscraping"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

type ingestPayload struct {
	Productos    []domain.Product `json:"productos"`
	Supermercado *string          `json:"supermercado"`
}

func floatPtr(value float64) *float64 {
	return &value
}

func receipt(count int) map[string]any {
	return map[string]any{
		"total_recibidos":     count,
		"precios_registrados": count,
		"insertados":          count,
		"actualizados":        0,
	}
}

func TestIngestBatchAndFinalizeWork(t *testing.T) {
	var (
		received    ingestPayload
		idempotency string
		final       map[string]any
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/scraper/trabajos/work-1/productos":
			if got := r.Header.Get("Authorization"); got != "Bearer token" {
				t.Errorf("Authorization = %q", got)
			}
			idempotency = r.Header.Get("Idempotency-Key")
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Errorf("decode payload: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(receipt(len(received.Productos)))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/scraper/trabajos/work-1/finalizar":
			if err := json.NewDecoder(r.Body).Decode(&final); err != nil {
				t.Errorf("decode finalizar: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "work-1", "estado": "completado"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "token", 2*time.Second)
	supermarket := "Jumbo"
	products := []domain.Product{
		{Producto: "Leche", Categoria: "lacteos", Precio: floatPtr(1290)},
		{Producto: "Pan", Categoria: "panaderia"},
	}
	result, err := client.IngestBatch(context.Background(), products, &supermarket, "work-1", "batch-1")
	if err != nil {
		t.Fatalf("IngestBatch() error = %v", err)
	}
	if idempotency != "batch-1" {
		t.Errorf("Idempotency-Key = %q", idempotency)
	}
	if received.Supermercado == nil || *received.Supermercado != "Jumbo" {
		t.Errorf("supermercado = %v", received.Supermercado)
	}
	if len(received.Productos) != 2 || received.Productos[1].Precio != nil {
		t.Errorf("productos = %+v", received.Productos)
	}
	if result["insertados"] != float64(2) {
		t.Errorf("resultado = %+v", result)
	}

	blocked := 4
	closed, err := client.FinalizeWork(context.Background(), "work-1", "completado", nil, &blocked, "")
	if err != nil {
		t.Fatalf("FinalizeWork() error = %v", err)
	}
	if closed["estado"] != "completado" {
		t.Errorf("estado = %v", closed["estado"])
	}
	if final["respuestas_bloqueadas"] != float64(4) {
		t.Errorf("respuestas_bloqueadas = %v", final["respuestas_bloqueadas"])
	}
}

func TestIngestBatchLimitaLasColumnasDeLaBase(t *testing.T) {
	var received ingestPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(receipt(len(received.Productos)))
	}))
	defer server.Close()

	long := strings.Repeat("á", 400)
	sku := strings.Repeat("s", 120)
	ean := strings.Repeat("9", 60)
	productURL := "https://www.santaisabel.cl/producto-1993238/p"
	client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
	_, err := client.IngestBatch(context.Background(), []domain.Product{{
		Producto:          long,
		SKU:               &sku,
		EanGtin:           &ean,
		Marca:             &long,
		Categoria:         long,
		FormatoCrudo:      &long,
		MecanicaPromocion: &long,
	}, {
		Producto:    "Agua saborizada",
		URLProducto: &productURL,
	}}, nil, "work-1", "batch-1")
	if err != nil {
		t.Fatalf("IngestBatch() error = %v", err)
	}

	if len(received.Productos) != 2 {
		t.Fatalf("productos = %d", len(received.Productos))
	}
	product := received.Productos[0]
	checks := []struct {
		name string
		got  string
		max  int
	}{
		{"producto", product.Producto, 255},
		{"sku", value(product.SKU), 100},
		{"ean_gtin", value(product.EanGtin), 50},
		{"marca", value(product.Marca), 100},
		{"categoria", product.Categoria, 100},
		{"formato_crudo", value(product.FormatoCrudo), 100},
		{"mecanica_promocion", value(product.MecanicaPromocion), 255},
	}
	for _, check := range checks {
		if got := len([]rune(check.got)); got != check.max {
			t.Errorf("%s tiene %d caracteres, want %d", check.name, got, check.max)
		}
	}
	if got := received.Productos[1].SKU; got == nil || *got != "1993238" {
		t.Errorf("sku derivado de URL = %v, want 1993238", got)
	}
}

func TestIngestBatchVacioNoLlamaALaAPI(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	defer server.Close()

	client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
	result, err := client.IngestBatch(context.Background(), nil, nil, "work-1", "batch-1")
	if err != nil {
		t.Fatalf("IngestBatch() error = %v", err)
	}
	if calls != 0 {
		t.Errorf("calls = %d, want 0", calls)
	}
	if result["total_recibidos"] != 0 {
		t.Errorf("resultado = %+v", result)
	}
}

func TestIngestBatchRechazaRecibosInconsistentes(t *testing.T) {
	cases := map[string]map[string]any{
		"total distinto": {
			"total_recibidos":     0,
			"precios_registrados": 0,
			"insertados":          0,
			"actualizados":        0,
		},
		"sin campos": {"total_recibidos": 1},
		"suma rota": {
			"total_recibidos":     1,
			"precios_registrados": 2,
			"insertados":          1,
			"actualizados":        0,
		},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(body)
			}))
			defer server.Close()

			client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
			_, err := client.IngestBatch(context.Background(), []domain.Product{{Producto: "Leche"}}, nil, "work-1", "batch-1")
			if err == nil {
				t.Fatal("IngestBatch() debió rechazar un recibo inconsistente")
			}
		})
	}
}

func TestIngestBatchRechazaJSONInvalido(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("no es json"))
	}))
	defer server.Close()

	client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
	if _, err := client.IngestBatch(context.Background(), []domain.Product{{Producto: "Leche"}}, nil, "work-1", "batch-1"); err == nil {
		t.Fatal("IngestBatch() debió rechazar un JSON inválido")
	}
}

func TestPermanentAPIError(t *testing.T) {
	cases := []struct {
		name   string
		status int
		code   string
		want   bool
	}{
		{"bad request", http.StatusBadRequest, "", true},
		{"not found", http.StatusNotFound, "", true},
		{"too large", http.StatusRequestEntityTooLarge, "", true},
		{"conflicto de idempotencia", http.StatusConflict, "idempotency_conflict", true},
		{"lote rechazado", http.StatusUnprocessableEntity, "lote_rechazado", true},
		{"servicio caído", http.StatusServiceUnavailable, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				body := map[string]any{}
				if tc.code != "" {
					body["codigo"] = tc.code
				}
				_ = json.NewEncoder(w).Encode(body)
			}))
			defer server.Close()

			client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
			_, err := client.IngestBatch(context.Background(), []domain.Product{{Producto: "Leche"}}, nil, "work-1", "batch-1")
			var apiErr *controlscraping.Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %T %v, want *Error", err, err)
			}
			if got := apiErr.Permanent(); got != tc.want {
				t.Errorf("Permanent() = %t, want %t (error=%v)", got, tc.want, apiErr)
			}
		})
	}
}

func TestGetWorkValidaEstadoYIdentidad(t *testing.T) {
	cases := []struct {
		name    string
		payload map[string]any
		wantErr bool
	}{
		{"en progreso", map[string]any{"id": "work-1", "estado": "en_progreso"}, false},
		{"completado", map[string]any{"id": "work-1", "estado": "completado"}, false},
		{"id ajeno", map[string]any{"id": "otro", "estado": "en_progreso"}, true},
		{"estado desconocido", map[string]any{"id": "work-1", "estado": "pendiente"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(tc.payload)
			}))
			defer server.Close()

			client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
			_, err := client.GetWork(context.Background(), "work-1")
			if tc.wantErr && err == nil {
				t.Fatal("GetWork() debió rechazar la respuesta")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("GetWork() error = %v", err)
			}
		})
	}
}

func TestFinalizeWorkExigeConfirmacion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "work-1", "estado": "en_progreso"})
	}))
	defer server.Close()

	client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
	if _, err := client.FinalizeWork(context.Background(), "work-1", "completado", nil, nil, ""); err == nil {
		t.Fatal("FinalizeWork() debió rechazar un cierre no confirmado")
	}
}

func value(pointer *string) string {
	if pointer == nil {
		return ""
	}
	return *pointer
}
