package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/controlscraping"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
)

// TestMemoryGate atraviesa el pipeline real de spool, lotes y entrega con un
// catálogo sintético. No se ejecuta en la suite normal porque mide RSS del
// proceso; CI lo activa con SCRAPER_MEMORY_GATE=1.
func TestMemoryGate(t *testing.T) {
	if os.Getenv("SCRAPER_MEMORY_GATE") != "1" {
		t.Skip("gate de RAM desactivado; usar SCRAPER_MEMORY_GATE=1")
	}
	if runtime.GOOS != "linux" {
		t.Skip("la medición de VmHWM requiere Linux")
	}

	items := envIntTest("SCRAPER_MEMORY_ITEMS", 50_000)
	limitMB := envFloatTest("SCRAPER_MEMORY_LIMIT_MB", 150)
	if items < 1 {
		t.Fatal("SCRAPER_MEMORY_ITEMS debe ser mayor que cero")
	}

	baseline := peakRSSMB(t)
	workID := uuid.NewString()
	var (
		mu        sync.Mutex
		finalized string
		received  int
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "/api/v1/scraper/trabajos/" + workID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == base:
			mu.Lock()
			status := "en_progreso"
			if finalized != "" {
				status = finalized
			}
			mu.Unlock()
			writeJSON(w, map[string]any{"id": workID, "estado": status})
		case r.Method == http.MethodPost && r.URL.Path == base+"/productos":
			var payload struct {
				Productos []domain.Product `json:"productos"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode productos: %v", err)
				return
			}
			mu.Lock()
			received += len(payload.Productos)
			mu.Unlock()
			writeJSON(w, map[string]any{
				"total_recibidos":     len(payload.Productos),
				"precios_registrados": len(payload.Productos),
				"insertados":          len(payload.Productos),
				"actualizados":        0,
			})
		case r.Method == http.MethodPut && r.URL.Path == base+"/finalizar":
			var payload struct {
				Estado string `json:"estado"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode finalizar: %v", err)
				return
			}
			mu.Lock()
			finalized = payload.Estado
			mu.Unlock()
			writeJSON(w, map[string]any{"id": workID, "estado": payload.Estado})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := config.Config{
		APIURL:          server.URL + "/api/v1/scraper/productos",
		Token:           "test",
		StateDir:        t.TempDir(),
		MinFreeBytes:    1 << 20,
		FailedRetention: time.Hour,
		APITimeout:      10 * time.Second,
		BatchSize:       1000,
		Lease:           10 * time.Second,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := spiders.NewRegistry(nil, t.TempDir(), 100, logger)
	registry.Register(memorySpider{items: items})

	scraperWorker := &Worker{
		cfg:      cfg,
		api:      controlscraping.New(cfg.APIURL, cfg.Token, cfg.APITimeout),
		registry: registry,
		logger:   logger,
	}
	state, err := scraperWorker.processJob(context.Background(), make(chan struct{}), domain.Job{
		WorkID: workID,
		Spider: memorySpiderName,
	})
	if err != nil {
		t.Fatalf("processJob() error = %v", err)
	}
	if state != "completado" {
		t.Errorf("estado = %q", state)
	}
	mu.Lock()
	delivered := received
	final := finalized
	mu.Unlock()
	if final != "completado" || delivered != items {
		t.Fatalf("entrega incompleta: estado=%q productos=%d, want %d", final, delivered, items)
	}

	peak := peakRSSMB(t)
	t.Logf("gate de RAM: %d productos, RSS pico %.1f MB (base %.1f MB, límite %.1f MB)",
		items, peak, baseline, limitMB)
	if peak > limitMB {
		t.Fatalf("RSS pico %.1f MB > límite %.1f MB", peak, limitMB)
	}
}

const memorySpiderName = "memory_rsc"

type memorySpider struct {
	items int
}

func (m memorySpider) Name() string {
	return memorySpiderName
}

func (m memorySpider) Run(_ context.Context, _ domain.Job, emit spiders.Emitter) error {
	base := domain.Product{
		Precio:       domain.FloatPtr(1990),
		PrecioNormal: domain.FloatPtr(2490),
		PrecioOferta: domain.FloatPtr(1990),
		Marca:        domain.StringPtr("Marca de prueba"),
		FormatoCrudo: domain.StringPtr("1 unidad"),
		EnStock:      domain.BoolPtr(true),
		Categoria:    "despensa",
	}
	for index := 0; index < m.items; index++ {
		product := base
		product.Producto = fmt.Sprintf("Producto de prueba con nombre largo %d", index)
		product.SKU = domain.StringPtr(fmt.Sprintf("%013d", index))
		product.EanGtin = domain.StringPtr(fmt.Sprintf("%013d", index))
		product.URLProducto = domain.StringPtr(fmt.Sprintf("https://www.jumbo.cl/p/producto-%d", index))
		product.Imagen = domain.StringPtr(fmt.Sprintf("https://www.jumbo.cl/img/%d.jpg", index))
		if err := emit(domain.Item{Product: product, Supermercado: "Jumbo"}); err != nil {
			return err
		}
	}
	return nil
}

func peakRSSMB(t *testing.T) float64 {
	t.Helper()
	body, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Fatalf("leer /proc/self/status: %v", err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, "VmHWM:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			break
		}
		kilobytes, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			t.Fatalf("parsear VmHWM: %v", err)
		}
		return kilobytes / 1024
	}
	t.Fatal("VmHWM no encontrado en /proc/self/status")
	return 0
}

func envIntTest(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envFloatTest(name string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
