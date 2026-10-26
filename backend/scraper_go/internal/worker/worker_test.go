package worker

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/controlscraping"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/queue"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
)

func testWorker(t *testing.T) *Worker {
	t.Helper()
	registry := spiders.NewRegistry(nil, t.TempDir(), 100, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &Worker{
		registry: registry,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestParseJob(t *testing.T) {
	w := testWorker(t)
	workID := uuid.NewString()

	job, err := w.parseJob(`{"trabajo_id":"` + workID + `","product_url":"https://www.jumbo.cl/p/lechuga","enrich_ean":"true"}`)
	if err != nil {
		t.Fatalf("parseJob() error = %v", err)
	}
	if job.WorkID != workID {
		t.Errorf("WorkID = %q", job.WorkID)
	}
	if job.Spider != spiders.JumboName {
		t.Errorf("Spider = %q", job.Spider)
	}
	if !job.EnrichEAN {
		t.Error("EnrichEAN = false")
	}
	if job.ProductURL != "https://www.jumbo.cl/p/lechuga" {
		t.Errorf("ProductURL = %q", job.ProductURL)
	}
}

func TestParseJobAcceptsKnownSpider(t *testing.T) {
	w := testWorker(t)
	job, err := w.parseJob(`{"trabajo_id":"` + uuid.NewString() + `","spider":"santa_isabel_rsc"}`)
	if err != nil {
		t.Fatalf("parseJob() error = %v", err)
	}
	if job.Spider != spiders.SantaIsabelName {
		t.Errorf("Spider = %q", job.Spider)
	}
}

func TestParseJobRejectsInvalidPayloads(t *testing.T) {
	w := testWorker(t)
	cases := map[string]string{
		"json inválido":       `{`,
		"trabajo_id inválido": `{"trabajo_id":"no-es-uuid"}`,
		"spider desconocido":  `{"trabajo_id":"` + uuid.NewString() + `","spider":"desconocido"}`,
		"product_id sin url":  `{"trabajo_id":"` + uuid.NewString() + `","product_id":"123"}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := w.parseJob(raw); err == nil {
				t.Fatalf("parseJob(%s) debió fallar", raw)
			}
		})
	}
}

func TestParseEnrich(t *testing.T) {
	cases := []struct {
		value any
		want  bool
	}{
		{true, true},
		{false, false},
		{"true", true},
		{"1", true},
		{"yes", true},
		{"false", false},
		{float64(1), true},
		{float64(0), false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := parseEnrich(tc.value); got != tc.want {
			t.Errorf("parseEnrich(%v) = %v, want %v", tc.value, got, tc.want)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	if !isTerminal(map[string]any{"estado": "completado"}) {
		t.Error("completado debería ser terminal")
	}
	if !isTerminal(map[string]any{"estado": "fallido"}) {
		t.Error("fallido debería ser terminal")
	}
	if isTerminal(map[string]any{"estado": "en_progreso"}) {
		t.Error("en_progreso no debería ser terminal")
	}
}

type fakeSpider struct {
	name  string
	items []domain.Item
	err   error
	calls int
}

func (f *fakeSpider) Name() string {
	return f.name
}

func (f *fakeSpider) Run(_ context.Context, _ domain.Job, emit spiders.Emitter) error {
	f.calls++
	for _, item := range f.items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return f.err
}

func TestWorkerProcessesJobEndToEnd(t *testing.T) {
	redisURL := os.Getenv("REDIS_TEST_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/15"
	}
	redisOptions, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("REDIS_TEST_URL inválida: %v", err)
	}
	redisOptions.DialTimeout = 500 * time.Millisecond
	redisOptions.ReadTimeout = 2 * time.Second
	redisOptions.WriteTimeout = 2 * time.Second
	redisClient := redis.NewClient(redisOptions)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer pingCancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		_ = redisClient.Close()
		t.Skipf("Redis no disponible: %v", err)
	}

	workID := uuid.NewString()
	prefix := "scraper:test:worker:" + uuid.NewString()
	stream := prefix + ":stream"
	group := prefix + ":group"
	index := prefix + ":index"
	dead := prefix + ":dead"
	closedPrefix := prefix + ":closed:"
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = redisClient.Del(cleanupCtx, stream, index, dead, closedPrefix+workID).Err()
		_ = redisClient.Close()
	})

	type apiState struct {
		sync.Mutex
		finalized   string
		batches     int
		products    []domain.Product
		supermarket string
		idempotency string
	}
	state := &apiState{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "/api/v1/scraper/trabajos/" + workID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == base:
			state.Lock()
			status := "en_progreso"
			if state.finalized != "" {
				status = state.finalized
			}
			state.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": workID, "estado": status})
		case r.Method == http.MethodPost && r.URL.Path == base+"/productos":
			var payload struct {
				Productos    []domain.Product `json:"productos"`
				Supermercado string           `json:"supermercado"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode productos: %v", err)
			}
			state.Lock()
			state.batches++
			state.products = append(state.products, payload.Productos...)
			state.supermarket = payload.Supermercado
			state.idempotency = r.Header.Get("Idempotency-Key")
			state.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
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
			}
			state.Lock()
			state.finalized = payload.Estado
			state.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": workID, "estado": payload.Estado})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	queueClient, err := queue.New(redisClient, queue.Options{
		Stream:       stream,
		Group:        group,
		Lease:        10 * time.Second,
		Consumer:     "worker-test",
		IndexKey:     index,
		DeadLetter:   dead,
		ClosedPrefix: closedPrefix,
	})
	if err != nil {
		t.Fatalf("queue.New() error = %v", err)
	}
	if err := queueClient.EnsureGroup(context.Background()); err != nil {
		t.Fatalf("EnsureGroup() error = %v", err)
	}
	raw := `{"trabajo_id":"` + workID + `","spider":"` + spiders.SantaIsabelName + `"}`
	messageID, err := redisClient.XAdd(context.Background(), &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"payload": raw},
	}).Result()
	if err != nil {
		t.Fatalf("XAdd() error = %v", err)
	}
	if err := redisClient.HSet(context.Background(), index, workID, messageID).Err(); err != nil {
		t.Fatalf("HSet() error = %v", err)
	}

	registry := spiders.NewRegistry(nil, t.TempDir(), 100, slog.New(slog.NewTextHandler(io.Discard, nil)))
	registry.Register(&fakeSpider{
		name: spiders.SantaIsabelName,
		items: []domain.Item{
			{Product: domain.Product{Producto: "Leche", Categoria: "lacteos", Precio: domain.FloatPtr(1290)}, Supermercado: "Santa Isabel"},
			{Product: domain.Product{Producto: "Pan", Categoria: "panaderia", Precio: domain.FloatPtr(990)}, Supermercado: "Santa Isabel"},
		},
	})
	cfg := config.Config{
		APIURL:          server.URL + "/api/v1/scraper/productos",
		Token:           "token",
		StateDir:        t.TempDir(),
		MinFreeBytes:    1 << 20,
		FailedRetention: time.Hour,
		APITimeout:      2 * time.Second,
		BatchSize:       2,
		Lease:           10 * time.Second,
	}
	apiClient := controlscraping.New(cfg.APIURL, cfg.Token, cfg.APITimeout)
	scraperWorker := New(cfg, apiClient, queueClient, registry, slog.New(slog.NewTextHandler(io.Discard, nil)))

	receiveCtx, receiveCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer receiveCancel()
	message, err := queueClient.Receive(receiveCtx)
	if err != nil || message == nil {
		t.Fatalf("Receive() = %+v, %v", message, err)
	}
	scraperWorker.HandleMessage(receiveCtx, message)

	state.Lock()
	defer state.Unlock()
	if state.finalized != "completado" {
		t.Errorf("estado final = %q", state.finalized)
	}
	if state.batches != 1 || len(state.products) != 2 {
		t.Errorf("lotes = %d, productos = %d", state.batches, len(state.products))
	}
	if state.supermarket != "Santa Isabel" {
		t.Errorf("supermercado = %q", state.supermarket)
	}
	if state.idempotency == "" {
		t.Error("el lote debe enviar Idempotency-Key")
	}
	if length, _ := redisClient.XLen(context.Background(), stream).Result(); length != 0 {
		t.Errorf("stream length = %d, want 0", length)
	}
	if _, err := os.Stat(filepath.Join(cfg.StateDir, workID)); !os.IsNotExist(err) {
		t.Errorf("el spool debería haberse limpiado: %v", err)
	}
}

type workerRunState struct {
	sync.Mutex
	finalized    string
	finalMessage string
	batches      int
	products     int
	streamLength int64
}

func runWorkerJob(t *testing.T, workID string, spider spiders.Spider) *workerRunState {
	t.Helper()
	redisURL := os.Getenv("REDIS_TEST_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/15"
	}
	redisOptions, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("REDIS_TEST_URL inválida: %v", err)
	}
	redisOptions.DialTimeout = 500 * time.Millisecond
	redisOptions.ReadTimeout = 2 * time.Second
	redisOptions.WriteTimeout = 2 * time.Second
	redisClient := redis.NewClient(redisOptions)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer pingCancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		_ = redisClient.Close()
		t.Skipf("Redis no disponible: %v", err)
	}

	prefix := "scraper:test:worker:" + uuid.NewString()
	stream := prefix + ":stream"
	group := prefix + ":group"
	index := prefix + ":index"
	dead := prefix + ":dead"
	closedPrefix := prefix + ":closed:"
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = redisClient.Del(cleanupCtx, stream, index, dead, closedPrefix+workID).Err()
		_ = redisClient.Close()
	})

	state := &workerRunState{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "/api/v1/scraper/trabajos/" + workID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == base:
			state.Lock()
			status := "en_progreso"
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
			state.batches++
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
	defer server.Close()

	queueClient, err := queue.New(redisClient, queue.Options{
		Stream:       stream,
		Group:        group,
		Lease:        10 * time.Second,
		Consumer:     "worker-test",
		IndexKey:     index,
		DeadLetter:   dead,
		ClosedPrefix: closedPrefix,
	})
	if err != nil {
		t.Fatalf("queue.New() error = %v", err)
	}
	if err := queueClient.EnsureGroup(context.Background()); err != nil {
		t.Fatalf("EnsureGroup() error = %v", err)
	}
	raw := `{"trabajo_id":"` + workID + `","spider":"` + spider.Name() + `"}`
	messageID, err := redisClient.XAdd(context.Background(), &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"payload": raw},
	}).Result()
	if err != nil {
		t.Fatalf("XAdd() error = %v", err)
	}
	if err := redisClient.HSet(context.Background(), index, workID, messageID).Err(); err != nil {
		t.Fatalf("HSet() error = %v", err)
	}

	registry := spiders.NewRegistry(nil, t.TempDir(), 100, slog.New(slog.NewTextHandler(io.Discard, nil)))
	registry.Register(spider)
	cfg := config.Config{
		APIURL:          server.URL + "/api/v1/scraper/productos",
		Token:           "token",
		StateDir:        t.TempDir(),
		MinFreeBytes:    1 << 20,
		FailedRetention: time.Hour,
		APITimeout:      2 * time.Second,
		BatchSize:       2,
		Lease:           10 * time.Second,
	}
	scraperWorker := New(cfg, controlscraping.New(cfg.APIURL, cfg.Token, cfg.APITimeout), queueClient,
		registry, slog.New(slog.NewTextHandler(io.Discard, nil)))

	receiveCtx, receiveCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer receiveCancel()
	message, err := queueClient.Receive(receiveCtx)
	if err != nil || message == nil {
		t.Fatalf("Receive() = %+v, %v", message, err)
	}
	scraperWorker.HandleMessage(receiveCtx, message)
	state.Lock()
	state.streamLength, _ = redisClient.XLen(context.Background(), stream).Result()
	state.Unlock()
	return state
}

func TestWorkerFinalizesBlockedJob(t *testing.T) {
	state := runWorkerJob(t, uuid.NewString(), &fakeSpider{
		name: spiders.SantaIsabelName,
		err: &crawler.BlockedError{
			URL:              "https://www.santaisabel.cl/despensa",
			Status:           429,
			Reason:           "http_429",
			BlockedResponses: 3,
		},
	})

	state.Lock()
	defer state.Unlock()
	if state.finalized != "fallido" {
		t.Errorf("estado final = %q, want fallido", state.finalized)
	}
	if !strings.Contains(state.finalMessage, "bloqueo: 3 respuestas bloqueadas") {
		t.Errorf("registro_errores = %q", state.finalMessage)
	}
	if state.batches != 0 {
		t.Errorf("lotes = %d, want 0", state.batches)
	}
	if state.streamLength != 0 {
		t.Errorf("stream length = %d, want 0 (mensaje ACK)", state.streamLength)
	}
}

func TestWorkerRejectsEmptyCrawl(t *testing.T) {
	state := runWorkerJob(t, uuid.NewString(), &fakeSpider{name: spiders.CugatName})

	state.Lock()
	defer state.Unlock()
	if state.finalized != "fallido" {
		t.Errorf("estado final = %q, want fallido", state.finalized)
	}
	if !strings.Contains(state.finalMessage, "sin productos") {
		t.Errorf("registro_errores = %q", state.finalMessage)
	}
	if state.batches != 0 {
		t.Errorf("lotes = %d, want 0", state.batches)
	}
	if state.streamLength != 0 {
		t.Errorf("stream length = %d, want 0 (mensaje ACK)", state.streamLength)
	}
}

func TestLockExecution(t *testing.T) {
	directory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(directory, "spool"), 0o700); err != nil {
		t.Fatal(err)
	}
	spoolDir := filepath.Join(directory, "spool")

	first, err := lockExecution(spoolDir)
	if err != nil {
		t.Fatalf("primer lock: %v", err)
	}
	if _, err := lockExecution(spoolDir); err == nil {
		t.Fatal("el segundo lock debió fallar")
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	third, err := lockExecution(spoolDir)
	if err != nil {
		t.Fatalf("lock tras liberar: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatalf("Close() tercero: %v", err)
	}
}
