// Package worker orquesta el ciclo completo: lease, spool, crawl, entrega y
// finalización. Es el único punto que conoce todas las dependencias; los
// adaptadores y spiders quedan detrás de interfaces.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/controlscraping"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/queue"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spool"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/storage"
)

var (
	ErrCrawlExhausted = errors.New("el crawl agotó 3 intentos; el spool se conserva")
	ErrLeaseLost      = errors.New("se perdió la lease del trabajo")
	ErrNoProducts     = errors.New("el crawl terminó sin productos")
)

type Worker struct {
	cfg       config.Config
	api       *controlscraping.Client
	gcAPI     *controlscraping.Client
	queue     *queue.Queue
	registry  *spiders.Registry
	collector *storage.Collector
	logger    *slog.Logger
}

func New(
	cfg config.Config,
	api *controlscraping.Client,
	queueClient *queue.Queue,
	registry *spiders.Registry,
	logger *slog.Logger,
) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	gcTimeout := 2 * time.Second
	if cfg.APITimeout > 0 && cfg.APITimeout < gcTimeout {
		gcTimeout = cfg.APITimeout
	}
	return &Worker{
		cfg:       cfg,
		api:       api,
		gcAPI:     controlscraping.New(cfg.APIURL, cfg.Token, gcTimeout),
		queue:     queueClient,
		registry:  registry,
		collector: storage.NewCollector(cfg.StateDir, cfg.MinFreeBytes, cfg.FailedRetention, logger),
		logger:    logger,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	if w.cfg.Token == "" {
		return fmt.Errorf("SCRAPING_SERVICE_TOKEN no configurado: Control de scraping rechazaría al worker")
	}
	if err := os.MkdirAll(w.cfg.StateDir, 0o700); err != nil {
		return err
	}

	w.logger.Info("worker_started",
		"stream", w.queue.Stream(),
		"group", w.queue.Group(),
		"consumer", w.queue.Consumer(),
	)

	nextCollection := time.Time{}
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := w.queue.EnsureGroup(ctx); err != nil {
			w.logger.Warn("worker_group_unavailable", "error", err)
			if !sleep(ctx, 5*time.Second) {
				return nil
			}
			continue
		}

		if time.Now().After(nextCollection) {
			if _, err := w.collector.Collect(ctx, w.gcAPI, w.queue, 20); err != nil {
				w.logger.Warn("worker_collection_pending", "error", err)
			}
			nextCollection = time.Now().Add(10 * time.Second)
		}

		if err := storage.EnsureCapacity(w.cfg.StateDir, w.cfg.MinFreeBytes, 0); err != nil {
			if errors.Is(err, storage.ErrStoragePressure) {
				w.logger.Error("worker_storage_pressure", "error", err)
			} else {
				w.logger.Warn("worker_storage_unavailable", "error", err)
			}
			if !sleep(ctx, 5*time.Second) {
				return nil
			}
			continue
		}

		message, err := w.queue.Receive(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			w.logger.Warn("worker_dependency_unavailable", "error", err)
			if !sleep(ctx, 5*time.Second) {
				return nil
			}
			continue
		}
		if message == nil {
			if !sleep(ctx, 200*time.Millisecond) {
				return nil
			}
			continue
		}
		w.HandleMessage(ctx, message)
	}
}

func (w *Worker) HandleMessage(ctx context.Context, message *redis.XMessage) {
	raw, _ := message.Values["payload"].(string)
	job, err := w.parseJob(raw)
	if err != nil {
		w.logger.Warn("worker_invalid_payload", "message_id", message.ID, "error", err)
		if _, ackErr := w.queue.Ack(ctx, message.ID, raw, "", "invalid_payload"); ackErr != nil {
			w.logger.Warn("worker_ack_failed", "message_id", message.ID, "error", ackErr)
		}
		return
	}

	heartbeatCtx, cancel, lost, err := w.queue.Heartbeat(ctx, message.ID)
	if err != nil {
		w.logger.Warn("worker_heartbeat_failed", "trabajo_id", job.WorkID, "error", err)
		return
	}
	defer cancel()

	state, err := w.processJob(heartbeatCtx, lost, job)
	if err != nil {
		if queue.IsLost(lost) || errors.Is(err, ErrLeaseLost) {
			w.logger.Warn("worker_lease_lost", "trabajo_id", job.WorkID)
			return
		}

		var apiErr *controlscraping.Error
		apiFailure := errors.As(err, &apiErr)
		var blocked *crawler.BlockedError
		blockedFailure := errors.As(err, &blocked)
		noProducts := errors.Is(err, ErrNoProducts)
		permanent := errors.Is(err, ErrCrawlExhausted) || noProducts || blockedFailure ||
			(apiFailure && apiErr.Permanent())
		if permanent {
			failureMessage := "Trabajo enviado a revisión; consultar spool y entregas"
			reason := "delivery_rejected"
			var blockedResponses *int
			switch {
			case blockedFailure:
				blockedCount := blocked.BlockedResponses
				if blockedCount < 1 {
					blockedCount = 1
				}
				blockedResponses = &blockedCount
				failureMessage = fmt.Sprintf(
					"bloqueo: %d respuestas bloqueadas (motivo=%s)",
					blockedCount,
					blocked.Reason,
				)
				reason = "blocked"
				w.logger.Warn("worker_job_blocked",
					"trabajo_id", job.WorkID,
					"url", blocked.URL,
					"status", blocked.Status,
					"motivo", blocked.Reason,
					"respuestas_bloqueadas", blockedCount,
				)
			case noProducts:
				failureMessage = "sin productos: el crawl no extrajo elementos; no se ingirió un lote vacío"
				reason = "no_products"
				w.logger.Warn("worker_job_without_products", "trabajo_id", job.WorkID)
			case errors.Is(err, ErrCrawlExhausted):
				failureMessage = "crawl agotado: se conserva el spool para revisión"
				reason = "attempts_exhausted"
			}
			if !apiFailure || apiErr.Status != 404 {
				if _, finalizeErr := w.api.FinalizeWork(
					heartbeatCtx,
					job.WorkID,
					"fallido",
					nil,
					blockedResponses,
					failureMessage,
				); finalizeErr != nil {
					w.logger.Warn("worker_finalize_failed", "trabajo_id", job.WorkID, "error", finalizeErr)
					return
				}
			}
			if _, ackErr := w.queue.Ack(heartbeatCtx, message.ID, raw, job.WorkID, reason); ackErr != nil {
				w.logger.Warn("worker_ack_failed", "trabajo_id", job.WorkID, "error", ackErr)
			}
			return
		}
		if apiFailure && (apiErr.Status == 401 || apiErr.Status == 403) {
			w.logger.Error("worker_auth_rejected",
				"trabajo_id", job.WorkID,
				"status", apiErr.Status,
				"error", "revisar SCRAPING_SERVICE_TOKEN",
			)
			return
		}
		w.logger.Warn("worker_delivery_pending", "trabajo_id", job.WorkID, "error", err)
		return
	}

	if queue.IsLost(lost) {
		w.logger.Warn("worker_lease_lost", "trabajo_id", job.WorkID)
		return
	}
	acked, err := w.queue.Ack(heartbeatCtx, message.ID, raw, job.WorkID, "")
	if err != nil {
		w.logger.Warn("worker_ack_failed", "trabajo_id", job.WorkID, "error", err)
		return
	}
	if acked && state == "completado" {
		if _, err := storage.CleanupSpool(w.cfg.StateDir, job.WorkID); err != nil {
			w.logger.Warn("worker_spool_cleanup_failed", "trabajo_id", job.WorkID, "error", err)
		}
	}
}

func (w *Worker) processJob(ctx context.Context, lost <-chan struct{}, job domain.Job) (string, error) {
	status, err := w.api.GetWork(ctx, job.WorkID)
	if err != nil {
		return "", err
	}
	if isTerminal(status) {
		return stateOf(status), nil
	}

	store, err := spool.Open(job.WorkID, w.cfg.StateDir, w.cfg.MinFreeBytes)
	if err != nil {
		return "", err
	}
	defer store.Close()

	lock, err := lockExecution(store.Directory())
	if err != nil {
		return "", err
	}
	defer lock.Close()

	if queue.IsLost(lost) {
		return "", ErrLeaseLost
	}
	status, err = w.api.GetWork(ctx, job.WorkID)
	if err != nil {
		return "", err
	}
	if isTerminal(status) {
		return stateOf(status), nil
	}

	if err := store.Seal(w.cfg.BatchSize); err != nil {
		return "", err
	}
	if err := store.Replay(ctx, w.api); err != nil {
		return "", err
	}

	done, err := store.Progress("crawl_done")
	if err != nil {
		return "", err
	}
	if done == 0 {
		attempts, err := store.Progress("crawl_attempts")
		if err != nil {
			return "", err
		}
		if attempts >= 3 {
			return "", ErrCrawlExhausted
		}
		if err := store.SetProgress("crawl_attempts", attempts+1); err != nil {
			return "", err
		}

		spider, ok := w.registry.Get(job.Spider)
		if !ok {
			return "", fmt.Errorf("spider aún no implementado en Go: %s", job.Spider)
		}
		pending := 0
		emit := func(item domain.Item) error {
			market := item.Supermercado
			if err := store.Add(item.Product, &market); err != nil {
				return err
			}
			pending++
			if pending >= w.cfg.BatchSize {
				if err := store.Seal(w.cfg.BatchSize); err != nil {
					return err
				}
				if err := store.Replay(ctx, w.api); err != nil {
					return err
				}
				pending = 0
			}
			return nil
		}
		if err := spider.Run(ctx, job, emit); err != nil {
			var blocked *crawler.BlockedError
			if errors.As(err, &blocked) {
				// Se conserva en el spool para diagnóstico y para que el
				// operador pueda distinguir el bloqueo de un fallo de red.
				_ = store.SetProgress("blocked_responses", blocked.BlockedResponses)
			}
			return "", err
		}
		if err := store.SetProgress("crawl_done", 1); err != nil {
			return "", err
		}
	}

	if err := store.Seal(w.cfg.BatchSize); err != nil {
		return "", err
	}
	if err := store.Replay(ctx, w.api); err != nil {
		return "", err
	}
	if queue.IsLost(lost) {
		return "", ErrLeaseLost
	}
	// Un crawl sin selectores puede terminar sin error y sin lotes. No se
	// declara completado: se cierra como fallido para no publicar un catálogo
	// vacío como éxito.
	itemCount, err := store.Count()
	if err != nil {
		return "", err
	}
	if itemCount == 0 {
		return "", ErrNoProducts
	}
	final, err := w.api.FinalizeWork(ctx, job.WorkID, "completado", nil, nil, "")
	if err != nil {
		return "", err
	}
	return fmt.Sprint(final["estado"]), nil
}

type jobPayload struct {
	WorkID     string `json:"trabajo_id"`
	Spider     string `json:"spider"`
	ProductID  string `json:"product_id"`
	ProductURL string `json:"product_url"`
	AddURL     string `json:"add_url"`
	EnrichEAN  any    `json:"enrich_ean"`
}

func (w *Worker) parseJob(raw string) (domain.Job, error) {
	var payload jobPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return domain.Job{}, err
	}
	if _, err := uuid.Parse(payload.WorkID); err != nil {
		return domain.Job{}, fmt.Errorf("trabajo_id inválido: %w", err)
	}
	spider := payload.Spider
	if spider == "" {
		spider = spiders.JumboName
	}
	if !spiders.IsKnown(spider) {
		return domain.Job{}, fmt.Errorf("spider inválido: %s", spider)
	}
	if payload.ProductID != "" && payload.ProductURL == "" {
		return domain.Job{}, fmt.Errorf("product_id requiere product_url")
	}
	return domain.Job{
		WorkID:     payload.WorkID,
		Spider:     spider,
		ProductID:  payload.ProductID,
		ProductURL: payload.ProductURL,
		AddURL:     payload.AddURL,
		EnrichEAN:  parseEnrich(payload.EnrichEAN),
	}, nil
}

func parseEnrich(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes":
			return true
		}
	case float64:
		return typed == 1
	}
	return false
}

func isTerminal(status map[string]any) bool {
	state := stateOf(status)
	return state == "completado" || state == "fallido"
}

func stateOf(status map[string]any) string {
	state, _ := status["estado"].(string)
	return state
}

type executionLock struct {
	file *os.File
}

func lockExecution(directory string) (*executionLock, error) {
	file, err := os.OpenFile(filepath.Join(directory, "execution.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("el spool ya está en ejecución por otro worker")
		}
		return nil, err
	}
	return &executionLock{file: file}, nil
}

func (l *executionLock) Close() error {
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	return l.file.Close()
}

func sleep(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
