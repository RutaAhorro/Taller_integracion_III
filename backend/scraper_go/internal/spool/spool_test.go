package spool

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
)

func floatPtr(value float64) *float64 {
	return &value
}

func TestSealAndReplay(t *testing.T) {
	directory := t.TempDir()
	workID := uuid.NewString()
	store, err := Open(workID, directory, 1<<20)
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

	var unbatched int
	if err := store.db.QueryRow("SELECT count(*) FROM items WHERE batch_id IS NULL").Scan(&unbatched); err != nil {
		t.Fatalf("contar sin lote: %v", err)
	}
	if unbatched != 2 {
		t.Fatalf("items sin lote = %d, want 2", unbatched)
	}
	if err := store.Seal(1); err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	if err := store.db.QueryRow("SELECT count(*) FROM items WHERE batch_id IS NULL").Scan(&unbatched); err != nil {
		t.Fatalf("contar sin lote después de Seal: %v", err)
	}
	if unbatched != 0 {
		t.Fatalf("items sin lote después de Seal = %d", unbatched)
	}
	var batches int
	if err := store.db.QueryRow("SELECT count(*) FROM batches").Scan(&batches); err != nil {
		t.Fatalf("contar batches: %v", err)
	}
	if batches != 2 {
		t.Fatalf("batches = %d, want 2", batches)
	}
	if err := store.Seal(0); err == nil {
		t.Fatal("Seal(0) debió fallar")
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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_recibidos":     1,
			"precios_registrados": 1,
			"insertados":          1,
			"actualizados":        0,
		})
	}))
	defer server.Close()

	client := controlscraping.New(server.URL+"/api/v1/scraper/productos", "", time.Second)
	if err := store.Replay(context.Background(), client); err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	var delivered int
	if err := store.db.QueryRow("SELECT count(*) FROM batches WHERE delivered = 0").Scan(&delivered); err != nil {
		t.Fatalf("contar pendientes: %v", err)
	}
	if delivered != 0 {
		t.Fatalf("lotes pendientes = %d", delivered)
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
	directory := t.TempDir()
	workID := uuid.NewString()
	store, err := Open(workID, directory, 1<<20)
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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_recibidos":     1,
			"precios_registrados": 1,
			"insertados":          1,
			"actualizados":        0,
		})
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
	var delivered int
	if err := store.db.QueryRow("SELECT count(*) FROM batches WHERE delivered = 1").Scan(&delivered); err != nil {
		t.Fatalf("contar entregados: %v", err)
	}
	if delivered != 1 {
		t.Errorf("lotes entregados = %d, want 1", delivered)
	}
}

func TestReplayDoesNotSendWithCancelledContext(t *testing.T) {
	directory := t.TempDir()
	workID := uuid.NewString()
	store, err := Open(workID, directory, 1<<20)
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
	if _, err := Open("no-es-uuid", t.TempDir(), 1<<20); err == nil {
		t.Fatal("Open() debió rechazar un trabajo_id inválido")
	}
}
