package spool_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/controlscraping"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spool"
)

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

func TestSealAndReplay(t *testing.T) {
	workID := uuid.NewString()
	store, err := spool.Open(workID, t.TempDir(), 1<<20)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	market := "Jumbo"
	leche := domain.Product{Producto: "Leche", Categoria: "lacteos", Precio: floatPtr(1290)}
	pan := domain.Product{Producto: "Pan", Categoria: "panaderia"}
	if err := store.Add(leche, &market); err != nil {
		t.Fatalf("Add(leche) error = %v", err)
	}
	if err := store.Add(leche, &market); err != nil {
		t.Fatalf("Add(leche duplicado) error = %v", err)
	}
	if err := store.Add(pan, &market); err != nil {
		t.Fatalf("Add(pan) error = %v", err)
	}
	if count, err := store.Count(); err != nil || count != 2 {
		t.Fatalf("Count() = %d, %v; want 2 (la huella evita duplicados)", count, err)
	}
	if err := store.Seal(0); err == nil {
		t.Fatal("Seal(0) debió fallar")
	}
	if err := store.Seal(1); err != nil {
		t.Fatalf("Seal() error = %v", err)
	}

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload struct {
			Productos    []domain.Product `json:"productos"`
			Supermercado string           `json:"supermercado"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		if payload.Supermercado != "Jumbo" || len(payload.Productos) != 1 {
			t.Errorf("payload = %+v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(receipt(1))
	}))
	defer server.Close()

	client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
	if err := store.Replay(context.Background(), client); err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (un lote por producto)", calls)
	}
	if err := store.Replay(context.Background(), client); err != nil {
		t.Fatalf("segundo Replay() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls tras reenviar = %d, want 2 (los lotes ya se entregaron)", calls)
	}

	if err := store.SetProgress("crawl_attempts", 2); err != nil {
		t.Fatalf("SetProgress() error = %v", err)
	}
	attempts, err := store.Progress("crawl_attempts")
	if err != nil || attempts != 2 {
		t.Fatalf("Progress() = %d, %v", attempts, err)
	}
	if missing, _ := store.Progress("crawl_done"); missing != 0 {
		t.Fatalf("crawl_done = %d, want 0", missing)
	}
}

func TestReplayRetriesSamePayloadAndIdempotencyKey(t *testing.T) {
	workID := uuid.NewString()
	store, err := spool.Open(workID, t.TempDir(), 1<<20)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	market := "Jumbo"
	if err := store.Add(domain.Product{Producto: "Leche", Precio: floatPtr(1290)}, &market); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := store.Seal(100); err != nil {
		t.Fatalf("Seal() error = %v", err)
	}

	var (
		calls  int
		keys   []string
		bodies []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		encoded, _ := json.Marshal(payload)
		bodies = append(bodies, string(encoded))
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"codigo":"catalogo_no_disponible"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(receipt(1))
	}))
	defer server.Close()

	client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
	if err := store.Replay(context.Background(), client); err == nil {
		t.Fatal("el primer Replay debió fallar con 503")
	}
	if err := store.Replay(context.Background(), client); err != nil {
		t.Fatalf("el segundo Replay debe recuperar: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if keys[0] == "" || keys[0] != keys[1] {
		t.Errorf("Idempotency-Key cambió entre reintentos: %q vs %q", keys[0], keys[1])
	}
	if bodies[0] != bodies[1] {
		t.Errorf("payload cambió entre reintentos:\n%s\n%s", bodies[0], bodies[1])
	}
	if err := store.Replay(context.Background(), client); err != nil {
		t.Fatalf("tercer Replay() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls tras el tercer Replay = %d, want 2 (lote ya entregado)", calls)
	}
}

func TestReplayDoesNotSendWithCancelledContext(t *testing.T) {
	workID := uuid.NewString()
	store, err := spool.Open(workID, t.TempDir(), 1<<20)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	market := "Jumbo"
	if err := store.Add(domain.Product{Producto: "Leche"}, &market); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := store.Seal(100); err != nil {
		t.Fatalf("Seal() error = %v", err)
	}

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		http.Error(w, "no debe llamarse", http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
	if err := store.Replay(ctx, client); err == nil {
		t.Fatal("Replay() debió respetar el contexto cancelado")
	}
	if called {
		t.Fatal("Replay() envió un lote con la lease cancelada")
	}
}

func TestOpenRejectsInvalidWorkID(t *testing.T) {
	if _, err := spool.Open("no-es-uuid", t.TempDir(), 1<<20); err == nil {
		t.Fatal("Open() debió rechazar un trabajo_id inválido")
	}
}
