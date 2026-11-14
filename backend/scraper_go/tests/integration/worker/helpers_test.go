package worker_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/controlscraping"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/queue"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/worker"
)

func floatPtr(value float64) *float64 {
	return &value
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func skipIfRedisUnavailable(t *testing.T, err error) {
	t.Helper()
	if os.Getenv("REDIS_TEST_URL") != "" {
		t.Fatalf("Redis configurado en REDIS_TEST_URL no está disponible: %v", err)
	}
	t.Skipf("Redis no disponible: %v", err)
}

type apiState struct {
	sync.Mutex
	status       string
	finalized    string
	finalMessage string
	blocked      int
	batches      int
	products     []domain.Product
	supermarket  string
	idempotency  string
}

func (s *apiState) currentStatus() string {
	s.Lock()
	defer s.Unlock()
	if s.finalized != "" {
		return s.finalized
	}
	return s.status
}

func newAPI(t *testing.T, workID string) (*httptest.Server, *apiState) {
	t.Helper()
	state := &apiState{status: "en_progreso"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "/api/v1/scraper/trabajos/" + workID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == base:
			writeJSON(w, map[string]any{"id": workID, "estado": state.currentStatus()})
		case r.Method == http.MethodPost && r.URL.Path == base+"/productos":
			var payload struct {
				Productos    []domain.Product `json:"productos"`
				Supermercado string           `json:"supermercado"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode productos: %v", err)
				return
			}
			state.Lock()
			state.batches++
			state.products = append(state.products, payload.Productos...)
			state.supermarket = payload.Supermercado
			state.idempotency = r.Header.Get("Idempotency-Key")
			state.Unlock()
			writeJSON(w, map[string]any{
				"total_recibidos":     len(payload.Productos),
				"precios_registrados": len(payload.Productos),
				"insertados":          len(payload.Productos),
				"actualizados":        0,
			})
		case r.Method == http.MethodPut && r.URL.Path == base+"/finalizar":
			var payload struct {
				Estado               string  `json:"estado"`
				RespuestasBloqueadas *int    `json:"respuestas_bloqueadas"`
				RegistroErrores      *string `json:"registro_errores"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode finalizar: %v", err)
				return
			}
			state.Lock()
			state.finalized = payload.Estado
			if payload.RespuestasBloqueadas != nil {
				state.blocked = *payload.RespuestasBloqueadas
			}
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

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

type fakeSpider struct {
	name  string
	items []domain.Item
	err   error

	mu    sync.Mutex
	calls int
	job   domain.Job
}

func (f *fakeSpider) Name() string { return f.name }

func (f *fakeSpider) Run(_ context.Context, job domain.Job, emit spiders.Emitter) error {
	f.mu.Lock()
	f.calls++
	f.job = job
	f.mu.Unlock()
	for _, item := range f.items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return f.err
}

func (f *fakeSpider) runCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeSpider) lastJob() domain.Job {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.job
}

type harness struct {
	workID       string
	stream       string
	group        string
	index        string
	dead         string
	closedPrefix string

	redis    *redis.Client
	queue    *queue.Queue
	worker   *worker.Worker
	cfg      config.Config
	server   *httptest.Server
	apiState *apiState
	spider   spiders.Spider
}

func newHarness(t *testing.T, spider spiders.Spider, mutate func(*config.Config)) *harness {
	t.Helper()
	workID := uuid.NewString()
	server, state := newAPI(t, workID)

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
		server.Close()
		skipIfRedisUnavailable(t, err)
	}

	prefix := "scraper:test:worker:" + uuid.NewString()
	h := &harness{
		workID:       workID,
		stream:       prefix + ":stream",
		group:        prefix + ":group",
		index:        prefix + ":index",
		dead:         prefix + ":dead",
		closedPrefix: prefix + ":closed:",
		redis:        redisClient,
		server:       server,
		apiState:     state,
		spider:       spider,
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = redisClient.Del(cleanupCtx, h.stream, h.index, h.dead, h.closedPrefix+h.workID).Err()
		_ = redisClient.Close()
		server.Close()
	})

	h.queue, err = queue.New(redisClient, queue.Options{
		Stream:       h.stream,
		Group:        h.group,
		Lease:        10 * time.Second,
		Consumer:     "worker-test",
		IndexKey:     h.index,
		DeadLetter:   h.dead,
		ClosedPrefix: h.closedPrefix,
	})
	if err != nil {
		t.Fatalf("queue.New() error = %v", err)
	}
	if err := h.queue.EnsureGroup(context.Background()); err != nil {
		t.Fatalf("EnsureGroup() error = %v", err)
	}

	cfg := config.Default()
	cfg.APIURL = server.URL + "/api/v1/scraper/productos"
	cfg.Token = "token"
	cfg.StateDir = t.TempDir()
	cfg.MinFreeBytes = 1 << 20
	cfg.BatchSize = 2
	cfg.APITimeout = 2 * time.Second
	cfg.CrawlAttempts = 3
	if mutate != nil {
		mutate(&cfg)
	}
	h.cfg = cfg

	registry := spiders.NewRegistry(nil, t.TempDir(), 100, quietLogger())
	registry.Register(spider)
	h.worker = worker.New(
		cfg,
		controlscraping.New(cfg.APIURL, cfg.Token, cfg.APITimeout),
		h.queue,
		registry,
		quietLogger(),
	)
	return h
}

func (h *harness) addJob(t *testing.T, raw string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	messageID, err := h.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: h.stream,
		Values: map[string]any{"payload": raw},
	}).Result()
	if err != nil {
		t.Fatalf("XAdd() error = %v", err)
	}
	if err := h.redis.HSet(ctx, h.index, h.workID, messageID).Err(); err != nil {
		t.Fatalf("HSet() error = %v", err)
	}
	return messageID
}

func (h *harness) jobPayload(extra string) string {
	raw := `{"trabajo_id":"` + h.workID + `","spider":"` + h.spider.Name() + `"`
	if extra != "" {
		raw += "," + extra
	}
	return raw + "}"
}

// runJob encola el mensaje, lo reclama como el consumidor y lo entrega al
// worker, igual que el ciclo real.
func (h *harness) runJob(t *testing.T, raw string) {
	t.Helper()
	h.addJob(t, raw)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	message, err := h.queue.Receive(ctx)
	if err != nil || message == nil {
		t.Fatalf("Receive() = %+v, %v", message, err)
	}
	h.worker.HandleMessage(ctx, message)
}

func (h *harness) deadLetters(t *testing.T) []redis.XMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	messages, err := h.redis.XRange(ctx, h.dead, "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange(dead) error = %v", err)
	}
	return messages
}

func (h *harness) streamLength(t *testing.T) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	length, err := h.redis.XLen(ctx, h.stream).Result()
	if err != nil {
		t.Fatalf("XLen() error = %v", err)
	}
	return length
}

func (h *harness) pendingMessages(t *testing.T) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pending, err := h.redis.XPending(ctx, h.stream, h.group).Result()
	if err != nil {
		t.Fatalf("XPending() error = %v", err)
	}
	return pending.Count
}
