package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping/internal/services"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBatchKeyIsSentWithoutJobIdentityAndConflictIsNotRetried(t *testing.T) {
	key := uuid.New().String()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload["lote_id"] != key || payload["trabajo_id"] != nil {
			t.Errorf("contrato interno=%v", payload)
		}
		w.WriteHeader(409)
	}))
	defer srv.Close()
	client, err := NewCatalogoClient(srv.URL, strings.Repeat("t", 32), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	_, err = client.IngestarLote(domain.ConLoteID(context.Background(), key), &id, 1, []domain.ProductoScrapeadoDTO{{SKU: "sku"}})
	if !errors.Is(err, services.ErrIdempotencia) || calls != 1 {
		t.Fatal(err, calls)
	}
}
