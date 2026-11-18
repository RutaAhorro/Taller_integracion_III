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

func skipIfRedisUnavailable(prueba *testing.T, err error) {
	prueba.Helper()
	if os.Getenv("REDIS_TEST_URL") != "" {
		prueba.Fatalf("Redis configurado en REDIS_TEST_URL no está disponible: %v", err)
	}
	prueba.Skipf("Redis no disponible: %v", err)
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

func (estado *apiState) currentStatus() string {
	estado.Lock()
	defer estado.Unlock()
	if estado.finalized != "" {
		return estado.finalized
	}
	return estado.status
}

func newAPI(prueba *testing.T, workID string) (*httptest.Server, *apiState) {
	prueba.Helper()
	state := &apiState{status: "en_progreso"}
	server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		base := "/api/v1/scraper/trabajos/" + workID
		switch {
		case peticion.Method == http.MethodGet && peticion.URL.Path == base:
			writeJSON(respuesta, map[string]any{"id": workID, "estado": state.currentStatus()})
		case peticion.Method == http.MethodPost && peticion.URL.Path == base+"/productos":
			var payload struct {
				Productos    []domain.Product `json:"productos"`
				Supermercado string           `json:"supermercado"`
			}
			if err := json.NewDecoder(peticion.Body).Decode(&payload); err != nil {
				prueba.Errorf("decode productos: %v", err)
				return
			}
			state.Lock()
			state.batches++
			state.products = append(state.products, payload.Productos...)
			state.supermarket = payload.Supermercado
			state.idempotency = peticion.Header.Get("Idempotency-Key")
			state.Unlock()
			writeJSON(respuesta, map[string]any{
				"total_recibidos":     len(payload.Productos),
				"precios_registrados": len(payload.Productos),
				"insertados":          len(payload.Productos),
				"actualizados":        0,
			})
		case peticion.Method == http.MethodPut && peticion.URL.Path == base+"/finalizar":
			var payload struct {
				Estado               string  `json:"estado"`
				RespuestasBloqueadas *int    `json:"respuestas_bloqueadas"`
				RegistroErrores      *string `json:"registro_errores"`
			}
			if err := json.NewDecoder(peticion.Body).Decode(&payload); err != nil {
				prueba.Errorf("decode finalizar: %v", err)
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
			writeJSON(respuesta, map[string]any{"id": workID, "estado": payload.Estado})
		default:
			http.NotFound(respuesta, peticion)
		}
	}))
	return server, state
}

func writeJSON(respuesta http.ResponseWriter, payload any) {
	respuesta.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(respuesta).Encode(payload)
}

type fakeSpider struct {
	name  string
	items []domain.Item
	err   error

	mu    sync.Mutex
	calls int
	job   domain.Job
}

func (falso *fakeSpider) Name() string { return falso.name }

func (falso *fakeSpider) Run(_ context.Context, job domain.Job, emit spiders.Emitter) error {
	falso.mu.Lock()
	falso.calls++
	falso.job = job
	falso.mu.Unlock()
	for _, item := range falso.items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return falso.err
}

func (falso *fakeSpider) runCount() int {
	falso.mu.Lock()
	defer falso.mu.Unlock()
	return falso.calls
}

func (falso *fakeSpider) lastJob() domain.Job {
	falso.mu.Lock()
	defer falso.mu.Unlock()
	return falso.job
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

func newHarness(prueba *testing.T, spider spiders.Spider, mutate func(*config.Config)) *harness {
	prueba.Helper()
	workID := uuid.NewString()
	server, state := newAPI(prueba, workID)

	redisURL := os.Getenv("REDIS_TEST_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/15"
	}
	redisOptions, err := redis.ParseURL(redisURL)
	if err != nil {
		prueba.Fatalf("REDIS_TEST_URL inválida: %v", err)
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
		skipIfRedisUnavailable(prueba, err)
	}

	prefix := "scraper:test:worker:" + uuid.NewString()
	arnes := &harness{
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
	prueba.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = redisClient.Del(cleanupCtx, arnes.stream, arnes.index, arnes.dead, arnes.closedPrefix+arnes.workID).Err()
		_ = redisClient.Close()
		server.Close()
	})

	arnes.queue, err = queue.New(redisClient, queue.Options{
		Stream:       arnes.stream,
		Group:        arnes.group,
		Lease:        10 * time.Second,
		Consumer:     "worker-test",
		IndexKey:     arnes.index,
		DeadLetter:   arnes.dead,
		ClosedPrefix: arnes.closedPrefix,
	})
	if err != nil {
		prueba.Fatalf("queue.New() error = %v", err)
	}
	if err := arnes.queue.EnsureGroup(context.Background()); err != nil {
		prueba.Fatalf("EnsureGroup() error = %v", err)
	}

	cfg := config.Default()
	cfg.APIURL = server.URL + "/api/v1/scraper/productos"
	cfg.Token = "token"
	cfg.StateDir = prueba.TempDir()
	cfg.MinFreeBytes = 1 << 20
	cfg.BatchSize = 2
	cfg.APITimeout = 2 * time.Second
	cfg.CrawlAttempts = 3
	if mutate != nil {
		mutate(&cfg)
	}
	arnes.cfg = cfg

	registry := spiders.NewRegistry(nil, prueba.TempDir(), 100, quietLogger())
	registry.Register(spider)
	arnes.worker, err = worker.New(
		cfg,
		controlscraping.New(cfg.APIURL, cfg.Token, cfg.APITimeout),
		arnes.queue,
		registry,
		quietLogger(),
	)
	if err != nil {
		prueba.Fatalf("worker.New() error = %v", err)
	}
	return arnes
}

func (arnes *harness) addJob(prueba *testing.T, raw string) string {
	prueba.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	messageID, err := arnes.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: arnes.stream,
		Values: map[string]any{"payload": raw},
	}).Result()
	if err != nil {
		prueba.Fatalf("XAdd() error = %v", err)
	}
	if err := arnes.redis.HSet(ctx, arnes.index, arnes.workID, messageID).Err(); err != nil {
		prueba.Fatalf("HSet() error = %v", err)
	}
	return messageID
}

func (arnes *harness) jobPayload(extra string) string {
	raw := `{"trabajo_id":"` + arnes.workID + `","spider":"` + arnes.spider.Name() + `"`
	if extra != "" {
		raw += "," + extra
	}
	return raw + "}"
}

// runJob encola el mensaje, lo reclama como el consumidor y lo entrega al
// worker, igual que el ciclo real.
func (arnes *harness) runJob(prueba *testing.T, raw string) {
	prueba.Helper()
	arnes.addJob(prueba, raw)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	message, err := arnes.queue.Receive(ctx)
	if err != nil || message == nil {
		prueba.Fatalf("Receive() = %+v, %v", message, err)
	}
	arnes.worker.HandleMessage(ctx, message)
}

func (arnes *harness) deadLetters(prueba *testing.T) []redis.XMessage {
	prueba.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	messages, err := arnes.redis.XRange(ctx, arnes.dead, "-", "+").Result()
	if err != nil {
		prueba.Fatalf("XRange(dead) error = %v", err)
	}
	return messages
}

func (arnes *harness) streamLength(prueba *testing.T) int64 {
	prueba.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	length, err := arnes.redis.XLen(ctx, arnes.stream).Result()
	if err != nil {
		prueba.Fatalf("XLen() error = %v", err)
	}
	return length
}

func (arnes *harness) pendingMessages(prueba *testing.T) int64 {
	prueba.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pending, err := arnes.redis.XPending(ctx, arnes.stream, arnes.group).Result()
	if err != nil {
		prueba.Fatalf("XPending() error = %v", err)
	}
	return pending.Count
}
