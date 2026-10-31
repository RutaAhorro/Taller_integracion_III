package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/controlscraping"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spool"
)

type recoveryProcessAPI struct {
	sync.Mutex
	status       string
	finalized    string
	finalMessage string
	products     int
}

func newRecoveryProcessAPI(t *testing.T, workID string) (*httptest.Server, *recoveryProcessAPI) {
	t.Helper()
	state := &recoveryProcessAPI{status: "en_progreso"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "/api/v1/scraper/trabajos/" + workID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == base:
			state.Lock()
			status := state.status
			if state.finalized != "" {
				status = state.finalized
			}
			state.Unlock()
			writeJSON(w, map[string]any{"id": workID, "estado": status})
		case r.Method == http.MethodPost && r.URL.Path == base+"/productos":
			var payload struct {
				Productos []domain.Product `json:"productos"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode productos: %v", err)
				return
			}
			state.Lock()
			state.products += len(payload.Productos)
			state.Unlock()
			writeJSON(w, map[string]any{
				"total_recibidos":     len(payload.Productos),
				"precios_registrados": len(payload.Productos),
				"insertados":          len(payload.Productos),
				"actualizados":        0,
			})
		case r.Method == http.MethodPut && r.URL.Path == base+"/finalizar":
			var payload struct {
				Estado          string  `json:"estado"`
				RegistroErrores *string `json:"registro_errores"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode finalizar: %v", err)
				return
			}
			state.Lock()
			state.finalized = payload.Estado
			if payload.RegistroErrores != nil {
				state.finalMessage = *payload.RegistroErrores
			}
			state.Unlock()
			writeJSON(w, map[string]any{"id": workID, "estado": payload.Estado})
		default:
			http.NotFound(w, r)
		}
	}))
	return server, state
}

func recoveryProcessWorker(t *testing.T, serverURL string, spider spiders.Spider) (*Worker, config.Config) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := spiders.NewRegistry(nil, t.TempDir(), 100, logger)
	registry.Register(spider)
	cfg := config.Config{
		APIURL:          serverURL + "/api/v1/scraper/productos",
		Token:           "token",
		StateDir:        t.TempDir(),
		MinFreeBytes:    1 << 20,
		FailedRetention: time.Hour,
		APITimeout:      2 * time.Second,
		BatchSize:       100,
		Lease:           10 * time.Second,
	}
	return &Worker{
		cfg:      cfg,
		api:      controlscraping.New(cfg.APIURL, cfg.Token, cfg.APITimeout),
		registry: registry,
		logger:   logger,
	}, cfg
}

func TestProcessJobDoesNotCrawlTerminalJob(t *testing.T) {
	workID := uuid.NewString()
	server, state := newRecoveryProcessAPI(t, workID)
	defer server.Close()
	state.status = "completado"

	spider := &fakeSpider{name: spiders.JumboName}
	scraperWorker, cfg := recoveryProcessWorker(t, server.URL, spider)

	got, err := scraperWorker.processJob(context.Background(), nil, domain.Job{
		WorkID: workID,
		Spider: spiders.JumboName,
	})
	if err != nil {
		t.Fatalf("processJob() error = %v", err)
	}
	if got != "completado" {
		t.Errorf("estado = %q", got)
	}
	if spider.calls != 0 {
		t.Errorf("el spider se ejecutó %d veces para un trabajo terminal", spider.calls)
	}
	if _, err := os.Stat(filepath.Join(cfg.StateDir, workID)); !os.IsNotExist(err) {
		t.Errorf("un trabajo terminal no debe abrir el spool: %v", err)
	}
	if state.finalized != "" {
		t.Errorf("no se debe finalizar de nuevo un trabajo terminal: %q", state.finalized)
	}
}

func TestProcessJobWithCrawlDoneFinalizesWithoutCrawling(t *testing.T) {
	workID := uuid.NewString()
	server, state := newRecoveryProcessAPI(t, workID)
	defer server.Close()

	spider := &fakeSpider{name: spiders.JumboName}
	scraperWorker, cfg := recoveryProcessWorker(t, server.URL, spider)

	store, err := spool.Open(workID, cfg.StateDir, cfg.MinFreeBytes)
	if err != nil {
		t.Fatalf("spool.Open() error = %v", err)
	}
	market := "Jumbo"
	if err := store.Add(domain.Product{
		Producto: "Producto recuperado",
		Precio:   domain.FloatPtr(1000),
	}, &market); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := store.SetProgress("crawl_done", 1); err != nil {
		t.Fatalf("SetProgress(crawl_done) error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	got, err := scraperWorker.processJob(context.Background(), nil, domain.Job{
		WorkID: workID,
		Spider: spiders.JumboName,
	})
	if err != nil {
		t.Fatalf("processJob() error = %v", err)
	}
	if got != "completado" {
		t.Errorf("estado = %q", got)
	}
	if spider.calls != 0 {
		t.Errorf("el spider se ejecutó %d veces con crawl_done=1", spider.calls)
	}
	state.Lock()
	defer state.Unlock()
	if state.products != 1 {
		t.Errorf("productos entregados = %d, want 1", state.products)
	}
	if state.finalized != "completado" {
		t.Errorf("estado final = %q", state.finalized)
	}
}

func TestProcessJobStopsAfterThreeCrawlAttempts(t *testing.T) {
	workID := uuid.NewString()
	server, state := newRecoveryProcessAPI(t, workID)
	defer server.Close()

	spider := &fakeSpider{name: spiders.JumboName}
	scraperWorker, cfg := recoveryProcessWorker(t, server.URL, spider)

	store, err := spool.Open(workID, cfg.StateDir, cfg.MinFreeBytes)
	if err != nil {
		t.Fatalf("spool.Open() error = %v", err)
	}
	if err := store.SetProgress("crawl_attempts", 3); err != nil {
		t.Fatalf("SetProgress(crawl_attempts) error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, err = scraperWorker.processJob(context.Background(), nil, domain.Job{
		WorkID: workID,
		Spider: spiders.JumboName,
	})
	if !errors.Is(err, ErrCrawlExhausted) {
		t.Fatalf("error = %v, want ErrCrawlExhausted", err)
	}
	if spider.calls != 0 {
		t.Errorf("el spider se ejecutó %d veces con los intentos agotados", spider.calls)
	}
	state.Lock()
	defer state.Unlock()
	if state.finalized != "" {
		t.Errorf("processJob no debe finalizar; handleMessage lo hace: %q", state.finalized)
	}
}
