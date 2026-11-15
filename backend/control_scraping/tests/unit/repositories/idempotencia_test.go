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
	clave := uuid.New().String()
	llamadas := 0
	servidor := httptest.NewServer(http.HandlerFunc(func(respuestaHTTP http.ResponseWriter, peticion *http.Request) {
		llamadas++
		var carga map[string]any
		if errorObtenido := json.NewDecoder(peticion.Body).Decode(&carga); errorObtenido != nil {
			prueba.Error(errorObtenido)
		}
		if carga["lote_id"] != clave || carga["trabajo_id"] != nil {
			prueba.Errorf("contrato interno=%v", carga)
		}
		respuestaHTTP.WriteHeader(409)
	}))
	defer servidor.Close()
	cliente, errorObtenido := repositories.NewCatalogoClient(servidor.URL, strings.Repeat("t", 32), time.Second)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	identificador := uuid.New()
	_, errorObtenido = cliente.IngestarLote(domain.ConLoteID(context.Background(), clave), &identificador, 1, []domain.ProductoScrapeadoDTO{{SKU: "sku"}})
	if !errors.Is(errorObtenido, services.ErrIdempotencia) || llamadas != 1 {
		prueba.Fatal(errorObtenido, llamadas)
	}
}

func TestCatalogDataRejectionIsDistinctFromInfrastructureFailure(prueba *testing.T) {
	for _, estadoHTTP := range []int{400, 413, 500, 503} {
		prueba.Run(http.StatusText(estadoHTTP), func(prueba *testing.T) {
			servidor := httptest.NewServer(http.HandlerFunc(func(respuestaHTTP http.ResponseWriter, peticion *http.Request) { respuestaHTTP.WriteHeader(estadoHTTP) }))
			defer servidor.Close()
			cliente, errorObtenido := repositories.NewCatalogoClient(servidor.URL, strings.Repeat("t", 32), time.Second)
			if errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
			_, errorObtenido = cliente.IngestarLote(context.Background(), nil, 1, []domain.ProductoScrapeadoDTO{{SKU: "sku"}})
			esperado := services.ErrCatalogoNoDisponible
			if estadoHTTP == 400 || estadoHTTP == 413 {
				esperado = services.ErrCatalogoRechazo
			}
			if !errors.Is(errorObtenido, esperado) {
				prueba.Fatal(errorObtenido)
			}
		})
	}
}
