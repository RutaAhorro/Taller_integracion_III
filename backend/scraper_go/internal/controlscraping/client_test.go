package controlscraping

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

func TestIngestBatchAndFinalizeWork(t *testing.T) {
	var received ingestPayload
	var idempotency string
	var finalPayload map[string]any
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
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total_recibidos":     len(received.Productos),
				"precios_registrados": len(received.Productos),
				"insertados":          len(received.Productos),
				"actualizados":        0,
			})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/scraper/trabajos/work-1/finalizar":
			if err := json.NewDecoder(r.Body).Decode(&finalPayload); err != nil {
				t.Errorf("decode finalizar: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "work-1", "estado": "completado"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(server.URL+"/api/v1/scraper/productos", "token", 2*time.Second)
	supermarket := "Jumbo"
	products := []domain.Product{
		{Producto: "Leche", Categoria: "lacteos", Precio: domain.FloatPtr(1290)},
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
	final, err := client.FinalizeWork(context.Background(), "work-1", "completado", nil, &blocked, "")
	if err != nil {
		t.Fatalf("FinalizeWork() error = %v", err)
	}
	if final["estado"] != "completado" {
		t.Errorf("estado = %v", final["estado"])
	}
	if finalPayload["respuestas_bloqueadas"] != float64(4) {
		t.Errorf("respuestas_bloqueadas = %v", finalPayload["respuestas_bloqueadas"])
	}
}

func TestIngestBatchLimitsDatabaseColumns(t *testing.T) {
	var received ingestPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_recibidos":     len(received.Productos),
			"precios_registrados": len(received.Productos),
			"insertados":          len(received.Productos),
			"actualizados":        0,
		})
	}))
	defer server.Close()

	long := strings.Repeat("á", 400)
	sku := strings.Repeat("s", 120)
	ean := strings.Repeat("9", 60)
	client := New(server.URL+"/api/v1/scraper/productos", "", time.Second)
	_, err := client.IngestBatch(context.Background(), []domain.Product{{
		Producto:          long,
		SKU:               &sku,
		EanGtin:           &ean,
		Marca:             &long,
		Categoria:         long,
		FormatoCrudo:      &long,
		MecanicaPromocion: &long,
	}}, nil, "work-1", "batch-1")
	if err != nil {
		t.Fatalf("IngestBatch() error = %v", err)
	}

	if len(received.Productos) != 1 {
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
		if !utf8.ValidString(check.got) {
			t.Errorf("%s no es UTF-8 válido", check.name)
		}
		if got := utf8.RuneCountInString(check.got); got != check.max {
			t.Errorf("%s tiene %d caracteres, want %d", check.name, got, check.max)
		}
	}
}

func value(pointer *string) string {
	if pointer == nil {
		return ""
	}
	return *pointer
}

func TestIngestBatchRejectsInconsistentReceipt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_recibidos":     0,
			"precios_registrados": 0,
			"insertados":          0,
			"actualizados":        0,
		})
	}))
	defer server.Close()

	client := New(server.URL+"/api/v1/scraper/productos", "", time.Second)
	_, err := client.IngestBatch(context.Background(), []domain.Product{{Producto: "Leche"}}, nil, "work-1", "batch-1")
	if err == nil {
		t.Fatal("IngestBatch() debió rechazar un recibo inconsistente")
	}
}

func TestPermanentAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"codigo": "lote_rechazado"})
	}))
	defer server.Close()

	client := New(server.URL+"/api/v1/scraper/productos", "", time.Second)
	_, err := client.IngestBatch(context.Background(), []domain.Product{{Producto: "Leche"}}, nil, "work-1", "batch-1")
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("error = %T, want *Error", err)
	}
	if !apiErr.Permanent() {
		t.Errorf("Permanent() = false, error = %v", apiErr)
	}
}
