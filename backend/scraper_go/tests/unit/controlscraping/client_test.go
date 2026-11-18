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

func TestIngestBatchAndFinalizeWork(prueba *testing.T) {
	var (
		received    ingestPayload
		idempotency string
		final       map[string]any
	)
	server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		switch {
		case peticion.Method == http.MethodPost && peticion.URL.Path == "/api/v1/scraper/trabajos/work-1/productos":
			if got := peticion.Header.Get("Authorization"); got != "Bearer token" {
				prueba.Errorf("Authorization = %q", got)
			}
			idempotency = peticion.Header.Get("Idempotency-Key")
			if err := json.NewDecoder(peticion.Body).Decode(&received); err != nil {
				prueba.Errorf("decode payload: %v", err)
			}
			respuesta.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(respuesta).Encode(receipt(len(received.Productos)))
		case peticion.Method == http.MethodPut && peticion.URL.Path == "/api/v1/scraper/trabajos/work-1/finalizar":
			if err := json.NewDecoder(peticion.Body).Decode(&final); err != nil {
				prueba.Errorf("decode finalizar: %v", err)
			}
			respuesta.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(respuesta).Encode(map[string]any{"id": "work-1", "estado": "completado"})
		default:
			http.NotFound(respuesta, peticion)
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
		prueba.Fatalf("IngestBatch() error = %v", err)
	}
	if idempotency != "batch-1" {
		prueba.Errorf("Idempotency-Key = %q", idempotency)
	}
	if received.Supermercado == nil || *received.Supermercado != "Jumbo" {
		prueba.Errorf("supermercado = %v", received.Supermercado)
	}
	if len(received.Productos) != 2 || received.Productos[1].Precio != nil {
		prueba.Errorf("productos = %+v", received.Productos)
	}
	if result["insertados"] != float64(2) {
		prueba.Errorf("resultado = %+v", result)
	}

	blocked := 4
	closed, err := client.FinalizeWork(context.Background(), "work-1", "completado", nil, &blocked, "")
	if err != nil {
		prueba.Fatalf("FinalizeWork() error = %v", err)
	}
	if closed["estado"] != "completado" {
		prueba.Errorf("estado = %v", closed["estado"])
	}
	if final["respuestas_bloqueadas"] != float64(4) {
		prueba.Errorf("respuestas_bloqueadas = %v", final["respuestas_bloqueadas"])
	}
}

func TestIngestBatchLimitaLasColumnasDeLaBase(prueba *testing.T) {
	var received ingestPayload
	server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if err := json.NewDecoder(peticion.Body).Decode(&received); err != nil {
			prueba.Errorf("decode payload: %v", err)
		}
		respuesta.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(respuesta).Encode(receipt(len(received.Productos)))
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
		prueba.Fatalf("IngestBatch() error = %v", err)
	}

	if len(received.Productos) != 2 {
		prueba.Fatalf("productos = %d", len(received.Productos))
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
			prueba.Errorf("%s tiene %d caracteres, want %d", check.name, got, check.max)
		}
	}
	if got := received.Productos[1].SKU; got == nil || *got != "1993238" {
		prueba.Errorf("sku derivado de URL = %v, want 1993238", got)
	}
}

func TestIngestBatchVacioNoLlamaALaAPI(prueba *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	defer server.Close()

	client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
	result, err := client.IngestBatch(context.Background(), nil, nil, "work-1", "batch-1")
	if err != nil {
		prueba.Fatalf("IngestBatch() error = %v", err)
	}
	if calls != 0 {
		prueba.Errorf("calls = %d, want 0", calls)
	}
	if result["total_recibidos"] != 0 {
		prueba.Errorf("resultado = %+v", result)
	}
}

func TestIngestBatchRechazaRecibosInconsistentes(prueba *testing.T) {
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
		prueba.Run(name, func(prueba *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
				respuesta.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(respuesta).Encode(body)
			}))
			defer server.Close()

			client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
			_, err := client.IngestBatch(context.Background(), []domain.Product{{Producto: "Leche"}}, nil, "work-1", "batch-1")
			if err == nil {
				prueba.Fatal("IngestBatch() debió rechazar un recibo inconsistente")
			}
		})
	}
}

func TestIngestBatchRechazaJSONInvalido(prueba *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
		_, _ = respuesta.Write([]byte("no es json"))
	}))
	defer server.Close()

	client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
	if _, err := client.IngestBatch(context.Background(), []domain.Product{{Producto: "Leche"}}, nil, "work-1", "batch-1"); err == nil {
		prueba.Fatal("IngestBatch() debió rechazar un JSON inválido")
	}
}

func TestPermanentAPIError(prueba *testing.T) {
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
		prueba.Run(tc.name, func(prueba *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
				respuesta.Header().Set("Content-Type", "application/json")
				respuesta.WriteHeader(tc.status)
				body := map[string]any{}
				if tc.code != "" {
					body["codigo"] = tc.code
				}
				_ = json.NewEncoder(respuesta).Encode(body)
			}))
			defer server.Close()

			client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
			_, err := client.IngestBatch(context.Background(), []domain.Product{{Producto: "Leche"}}, nil, "work-1", "batch-1")
			var apiErr *controlscraping.Error
			if !errors.As(err, &apiErr) {
				prueba.Fatalf("error = %T %v, want *Error", err, err)
			}
			if got := apiErr.Permanent(); got != tc.want {
				prueba.Errorf("Permanent() = %t, want %t (error=%v)", got, tc.want, apiErr)
			}
		})
	}
}

func TestGetWorkValidaEstadoYIdentidad(prueba *testing.T) {
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
		prueba.Run(tc.name, func(prueba *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
				respuesta.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(respuesta).Encode(tc.payload)
			}))
			defer server.Close()

			client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
			_, err := client.GetWork(context.Background(), "work-1")
			if tc.wantErr && err == nil {
				prueba.Fatal("GetWork() debió rechazar la respuesta")
			}
			if !tc.wantErr && err != nil {
				prueba.Fatalf("GetWork() error = %v", err)
			}
		})
	}
}

func TestFinalizeWorkExigeConfirmacion(prueba *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
		respuesta.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(respuesta).Encode(map[string]any{"id": "work-1", "estado": "en_progreso"})
	}))
	defer server.Close()

	client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
	if _, err := client.FinalizeWork(context.Background(), "work-1", "completado", nil, nil, ""); err == nil {
		prueba.Fatal("FinalizeWork() debió rechazar un cierre no confirmado")
	}
}

func value(pointer *string) string {
	if pointer == nil {
		return ""
	}
	return *pointer
}
