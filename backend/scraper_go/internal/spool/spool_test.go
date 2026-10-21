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

func TestSealAndReplay(t *testing.T) {
	directory := t.TempDir()
	workID := uuid.NewString()
	store, err := Open(workID, directory, 1<<20)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	market := "Jumbo"
	leche := domain.Product{Producto: "Leche", Categoria: "lacteos", Precio: domain.FloatPtr(1290)}
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

	count, err := store.Unbatched()
	if err != nil || count != 2 {
		t.Fatalf("Unbatched() = %d, %v; want 2", count, err)
	}
	if err := store.Seal(1); err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	if count, _ := store.Unbatched(); count != 0 {
		t.Fatalf("Unbatched() después de Seal = %d", count)
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

func TestOpenRejectsInvalidWorkID(t *testing.T) {
	if _, err := Open("no-es-uuid", t.TempDir(), 1<<20); err == nil {
		t.Fatal("Open() debió rechazar un trabajo_id inválido")
	}
}
