package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/controlscraping"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spool"
)

func floatPtr(value float64) *float64 {
	return &value
}

// TestMemoryGate atraviesa el pipeline real de spool, lotes y entrega con un
// catálogo sintético, usando sólo la API pública de los paquetes. No se
// ejecuta en la suite normal porque mide RSS del proceso; CI lo activa con
// SCRAPER_MEMORY_GATE=1.
func TestMemoryGate(t *testing.T) {
	if os.Getenv("SCRAPER_MEMORY_GATE") != "1" {
		t.Skip("gate de RAM desactivado; usar SCRAPER_MEMORY_GATE=1")
	}
	if runtime.GOOS != "linux" {
		t.Skip("la medición de VmHWM requiere Linux")
	}

	items := envInt("SCRAPER_MEMORY_ITEMS", 200_000)
	limitMB := envFloat("SCRAPER_MEMORY_LIMIT_MB", 150)
	compact := os.Getenv("SCRAPER_MEMORY_COMPACT") == "1"
	if items < 1 {
		t.Fatal("SCRAPER_MEMORY_ITEMS debe ser mayor que cero")
	}
	const batchSize = 1000

	baseline := peakRSSMB(t)
	anonSampler := startAnonSampler(10 * time.Millisecond)
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
			writeJSONResponse(w, map[string]any{"id": workID, "estado": status})
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
			writeJSONResponse(w, map[string]any{
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
			writeJSONResponse(w, map[string]any{"id": workID, "estado": payload.Estado})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stateDir := t.TempDir()
	api := controlscraping.New(server.URL+"/api/v1/scraper/productos", "test", 10*time.Second)
	store, err := spool.Open(workID, stateDir, 1<<20)
	if err != nil {
		t.Fatalf("spool.Open() error = %v", err)
	}

	ctx := context.Background()
	pending := 0
	emit := func(item domain.Item) error {
		item = spiders.CompleteItem(item)
		market := item.Supermercado
		if err := store.Add(item.Product, &market); err != nil {
			return err
		}
		pending++
		if pending >= batchSize {
			if err := store.Seal(batchSize); err != nil {
				return err
			}
			if err := store.Replay(ctx, api); err != nil {
				return err
			}
			pending = 0
		}
		return nil
	}
	spider := memorySpider{items: items, compact: compact, logger: logger}
	if err := spider.Run(ctx, domain.Job{WorkID: workID}, emit); err != nil {
		_ = store.Close()
		t.Fatalf("spider.Run() error = %v", err)
	}
	if err := store.SetProgress("crawl_done", 1); err != nil {
		_ = store.Close()
		t.Fatalf("SetProgress() error = %v", err)
	}
	if err := store.Seal(batchSize); err != nil {
		_ = store.Close()
		t.Fatalf("Seal() error = %v", err)
	}
	if err := store.Replay(ctx, api); err != nil {
		_ = store.Close()
		t.Fatalf("Replay() error = %v", err)
	}
	count, err := store.Count()
	if err != nil {
		_ = store.Close()
		t.Fatalf("Count() error = %v", err)
	}
	if count != items {
		_ = store.Close()
		t.Fatalf("productos en spool = %d, want %d", count, items)
	}
	state, err := api.FinalizeWork(ctx, workID, "completado", nil, nil, "")
	_ = store.Close()
	anonPeak := anonSampler.Stop()
	if err != nil {
		t.Fatalf("FinalizeWork() error = %v", err)
	}
	if state["estado"] != "completado" {
		t.Errorf("estado = %v", state["estado"])
	}
	mu.Lock()
	delivered := received
	final := finalized
	mu.Unlock()
	if final != "completado" || delivered != items {
		t.Fatalf("entrega incompleta: estado=%q productos=%d, want %d", final, delivered, items)
	}

	peak := peakRSSMB(t)
	cgroupLimit := envFloat("SCRAPER_MEMORY_CGROUP_LIMIT_MB", 0)
	cgroupPeak := 0.0
	if cgroupLimit > 0 {
		cgroupPeak = cgroupPeakMB()
	}
	result := memoryGateResult{
		Items:         items,
		Compact:       compact,
		LimitMB:       limitMB,
		BaselineRSSMB: baseline,
		PeakRSSMB:     peak,
		PeakAnonRSSMB: anonPeak,
		CgroupLimitMB: cgroupLimit,
		CgroupPeakMB:  cgroupPeak,
		GoVersion:     runtime.Version(),
		MeasuredAt:    time.Now().UTC().Format(time.RFC3339),
	}
	t.Logf("gate de RAM: %d productos (compacto=%t), RSS pico %.1f MB, RssAnon pico %.1f MB, cgroup pico %.1f MB / límite %.1f MB (base %.1f MB, límite RSS %.1f MB)",
		items, compact, peak, anonPeak, cgroupPeak, cgroupLimit, baseline, limitMB)
	writeMemoryGateResult(t, result)
	if peak > limitMB || anonPeak > limitMB {
		t.Fatalf("RAM pico fuera de límite: RSS %.1f MB, RssAnon %.1f MB, límite %.1f MB",
			peak, anonPeak, limitMB)
	}
}

type memoryGateResult struct {
	Items         int     `json:"items"`
	Compact       bool    `json:"compact,omitempty"`
	LimitMB       float64 `json:"limit_mb"`
	BaselineRSSMB float64 `json:"baseline_rss_mb"`
	PeakRSSMB     float64 `json:"peak_rss_mb"`
	PeakAnonRSSMB float64 `json:"peak_anon_rss_mb"`
	CgroupLimitMB float64 `json:"cgroup_limit_mb,omitempty"`
	CgroupPeakMB  float64 `json:"cgroup_peak_mb,omitempty"`
	GoVersion     string  `json:"go_version"`
	MeasuredAt    string  `json:"measured_at"`
}

func writeMemoryGateResult(t *testing.T, result memoryGateResult) {
	t.Helper()
	path := strings.TrimSpace(os.Getenv("SCRAPER_MEMORY_RESULT"))
	if path == "" {
		return
	}
	body, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal resultado de RAM: %v", err)
	}
	body = append(body, '\n')
	if directory := filepath.Dir(path); directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatalf("crear directorio para resultado de RAM: %v", err)
		}
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("escribir resultado de RAM en %s: %v", path, err)
	}
}

type anonSampler struct {
	stop chan struct{}
	done chan struct{}
	peak atomic.Uint64
}

func startAnonSampler(interval time.Duration) *anonSampler {
	sampler := &anonSampler{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(sampler.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if kb, ok := statusFieldKB("RssAnon"); ok {
				value := uint64(kb)
				for {
					previous := sampler.peak.Load()
					if value <= previous || sampler.peak.CompareAndSwap(previous, value) {
						break
					}
				}
			}
			select {
			case <-sampler.stop:
				return
			case <-ticker.C:
			}
		}
	}()
	return sampler
}

func (s *anonSampler) Stop() float64 {
	close(s.stop)
	<-s.done
	return float64(s.peak.Load()) / 1024
}

func statusFieldKB(name string) (float64, bool) {
	body, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, false
	}
	prefix := name + ":"
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, false
		}
		kilobytes, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return 0, false
		}
		return kilobytes, true
	}
	return 0, false
}

func cgroupPeakMB() float64 {
	for _, path := range []string{
		"/sys/fs/cgroup/memory.peak",
		"/sys/fs/cgroup/memory/memory.max_usage_in_bytes",
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		bytes, err := strconv.ParseFloat(strings.TrimSpace(string(body)), 64)
		if err != nil {
			continue
		}
		return bytes / (1024 * 1024)
	}
	return 0
}

const memorySpiderName = "memory_rsc"

type memorySpider struct {
	items   int
	compact bool
	logger  *slog.Logger
}

func (m memorySpider) Name() string {
	return memorySpiderName
}

func (m memorySpider) Run(_ context.Context, _ domain.Job, emit spiders.Emitter) error {
	base := domain.Product{
		Precio:       floatPtr(1990),
		PrecioNormal: floatPtr(2490),
		PrecioOferta: floatPtr(1990),
		Marca:        domain.StringPtr("Marca de prueba"),
		FormatoCrudo: domain.StringPtr("1 unidad"),
		EnStock:      domain.BoolPtr(true),
		Categoria:    "despensa",
	}
	for index := 0; index < m.items; index++ {
		product := base
		if m.compact {
			// El gate de OOM corre sobre tmpfs para evitar miles de fsync
			// lentos en el volumen de Docker. Como tmpfs se cobra al cgroup,
			// se usa un payload mínimo; el gate del host mantiene el payload
			// realista.
			product = domain.Product{Producto: fmt.Sprintf("p%d", index)}
		} else {
			product.Producto = fmt.Sprintf("Producto de prueba con nombre largo %d", index)
			product.SKU = domain.StringPtr(fmt.Sprintf("%013d", index))
			product.EanGtin = domain.StringPtr(fmt.Sprintf("%013d", index))
			product.URLProducto = domain.StringPtr(fmt.Sprintf("https://www.jumbo.cl/p/producto-%d", index))
			product.Imagen = domain.StringPtr(fmt.Sprintf("https://www.jumbo.cl/img/%d.jpg", index))
		}
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

func envInt(name string, fallback int) int {
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

func envFloat(name string, fallback float64) float64 {
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

func writeJSONResponse(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
