package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBatchKeyIsSentWithoutJobIdentityAndConflictIsNotRetried(prueba *testing.T) {
	key := uuid.New().String()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(respuestaHTTP http.ResponseWriter, peticion *http.Request) {
		calls++
		var payload map[string]any
		if err := json.NewDecoder(peticion.Body).Decode(&payload); err != nil {
			prueba.Error(err)
		}
		if payload["lote_id"] != key || payload["trabajo_id"] != nil {
			prueba.Errorf("contrato interno=%v", payload)
		}
		respuestaHTTP.WriteHeader(409)
	}))
	defer srv.Close()
	client, err := repositories.NewCatalogoClient(srv.URL, strings.Repeat("t", 32), time.Second)
	if err != nil {
		prueba.Fatal(err)
	}
	id := uuid.New()
	_, err = client.IngestarLote(domain.ConLoteID(context.Background(), key), &id, 1, []domain.ProductoScrapeadoDTO{{SKU: "sku"}})
	if !errors.Is(err, services.ErrIdempotencia) || calls != 1 {
		prueba.Fatal(err, calls)
	}
}

func TestCatalogDataRejectionIsDistinctFromInfrastructureFailure(prueba *testing.T) {
	for _, status := range []int{400, 413, 500, 503} {
		prueba.Run(http.StatusText(status), func(prueba *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(respuestaHTTP http.ResponseWriter, peticion *http.Request) { respuestaHTTP.WriteHeader(status) }))
			defer srv.Close()
			client, err := repositories.NewCatalogoClient(srv.URL, strings.Repeat("t", 32), time.Second)
			if err != nil {
				prueba.Fatal(err)
			}
			_, err = client.IngestarLote(context.Background(), nil, 1, []domain.ProductoScrapeadoDTO{{SKU: "sku"}})
			want := services.ErrCatalogoNoDisponible
			if status == 400 || status == 413 {
				want = services.ErrCatalogoRechazo
			}
			if !errors.Is(err, want) {
				prueba.Fatal(err)
			}
		})
	}
}
