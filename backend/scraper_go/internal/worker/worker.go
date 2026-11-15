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
	ErrCrawlExhausted = errors.New("el crawl agotó los intentos configurados; el spool se conserva")
	ErrLeaseLost      = errors.New("se perdió la lease del trabajo")
	ErrNoProducts     = errors.New("el crawl terminó sin productos")
)

// ===== Estructura de Orquestador =====

// Worker consume trabajos de la cola y ejecuta el ciclo completo: lease,
// spool, crawl, entrega y cierre.
type Worker struct {
	cfg       config.Config
	api       *controlscraping.Client
	gcAPI     *controlscraping.Client
	queue     *queue.Queue
	registry  *spiders.Registry
	collector *storage.Collector
	logger    *slog.Logger
}

// ===== Constructor =====

// New construye el worker y las dependencias derivadas de la configuración:
// el cliente que usa el recolector y el recolector de spools.
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
	gcTimeout := cfg.GCTimeout
	if cfg.APITimeout > 0 && cfg.APITimeout < gcTimeout {
		gcTimeout = cfg.APITimeout
	}
	return &Worker{
		cfg:      cfg,
		api:      api,
		gcAPI:    controlscraping.New(cfg.APIURL, cfg.Token, gcTimeout),
		queue:    queueClient,
		registry: registry,
		collector: storage.NewCollector(
			cfg.StateDir,
			cfg.MinFreeBytes,
			cfg.FailedRetention,
			cfg.CollectTimeout,
			logger,
		),
		logger: logger,
	}
}

// ===== Bucle Principal =====

// Run asegura el grupo de consumo, recolecta spools vencidos y procesa
// mensajes hasta que el contexto se cancela. Los fallos de dependencias se
// reintentan con RetryInterval.
func (trabajador *Worker) Run(ctx context.Context) error {
	if trabajador.cfg.Token == "" {
		return fmt.Errorf("SCRAPING_SERVICE_TOKEN no configurado: Control de scraping rechazaría al worker")
	}
	if err := os.MkdirAll(trabajador.cfg.StateDir, 0o700); err != nil {
		return err
	}

	trabajador.logger.Info("worker_started",
		"stream", trabajador.queue.Stream(),
		"group", trabajador.queue.Group(),
		"consumer", trabajador.queue.Consumer(),
	)

	nextCollection := time.Time{}
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := trabajador.queue.EnsureGroup(ctx); err != nil {
			trabajador.logger.Warn("worker_group_unavailable", "error", err)
			if !sleep(ctx, trabajador.cfg.RetryInterval) {
				return nil
			}
			continue
		}

		if time.Now().After(nextCollection) {
			if _, err := trabajador.collector.Collect(ctx, trabajador.gcAPI, trabajador.queue, trabajador.cfg.CollectLimit); err != nil {
				trabajador.logger.Warn("worker_collection_pending", "error", err)
			}
			nextCollection = time.Now().Add(trabajador.cfg.CollectInterval)
		}

		if err := storage.EnsureCapacity(trabajador.cfg.StateDir, trabajador.cfg.MinFreeBytes, 0); err != nil {
			if errors.Is(err, storage.ErrStoragePressure) {
				trabajador.logger.Error("worker_storage_pressure", "error", err)
			} else {
				trabajador.logger.Warn("worker_storage_unavailable", "error", err)
			}
			if !sleep(ctx, trabajador.cfg.RetryInterval) {
				return nil
			}
			continue
		}

		message, err := trabajador.queue.Receive(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			trabajador.logger.Warn("worker_dependency_unavailable", "error", err)
			if !sleep(ctx, trabajador.cfg.RetryInterval) {
				return nil
			}
			continue
		}
		if message == nil {
			if !sleep(ctx, trabajador.cfg.PollInterval) {
				return nil
			}
			continue
		}
		trabajador.HandleMessage(ctx, message)
	}
}

// HandleMessage procesa un mensaje ya reclamado: valida el payload, ejecuta
// el trabajo y confirma o deriva el mensaje según el resultado.
func (trabajador *Worker) HandleMessage(ctx context.Context, message *redis.XMessage) {
	raw, _ := message.Values["payload"].(string)
	job, err := trabajador.parseJob(raw)
	if err != nil {
		trabajador.logger.Warn("worker_invalid_payload", "message_id", message.ID, "error", err)
		if _, ackErr := trabajador.queue.Ack(ctx, message.ID, raw, "", "invalid_payload"); ackErr != nil {
			trabajador.logger.Warn("worker_ack_failed", "message_id", message.ID, "error", ackErr)
		}
		return
	}

	heartbeatCtx, cancel, lost, err := trabajador.queue.Heartbeat(ctx, message.ID)
	if err != nil {
		trabajador.logger.Warn("worker_heartbeat_failed", "trabajo_id", job.WorkID, "error", err)
		return
	}
	defer cancel()

	state, err := trabajador.processJob(heartbeatCtx, lost, job)
	if err != nil {
		if queue.IsLost(lost) || errors.Is(err, ErrLeaseLost) {
			trabajador.logger.Warn("worker_lease_lost", "trabajo_id", job.WorkID)
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
				trabajador.logger.Warn("worker_job_blocked",
					"trabajo_id", job.WorkID,
					"url", blocked.URL,
					"status", blocked.Status,
					"motivo", blocked.Reason,
					"respuestas_bloqueadas", blockedCount,
				)
			case noProducts:
				failureMessage = "sin productos: el crawl no extrajo elementos; no se ingirió un lote vacío"
				reason = "no_products"
				trabajador.logger.Warn("worker_job_without_products", "trabajo_id", job.WorkID)
			case errors.Is(err, ErrCrawlExhausted):
				failureMessage = "crawl agotado: se conserva el spool para revisión"
				reason = "attempts_exhausted"
			}
			if !apiFailure || apiErr.Status != 404 {
				if _, finalizeErr := trabajador.api.FinalizeWork(
					heartbeatCtx,
					job.WorkID,
					"fallido",
					nil,
					blockedResponses,
					failureMessage,
				); finalizeErr != nil {
					trabajador.logger.Warn("worker_finalize_failed", "trabajo_id", job.WorkID, "error", finalizeErr)
					return
				}
			}
			if _, ackErr := trabajador.queue.Ack(heartbeatCtx, message.ID, raw, job.WorkID, reason); ackErr != nil {
				trabajador.logger.Warn("worker_ack_failed", "trabajo_id", job.WorkID, "error", ackErr)
			}
			return
		}
		if apiFailure && (apiErr.Status == 401 || apiErr.Status == 403) {
			trabajador.logger.Error("worker_auth_rejected",
				"trabajo_id", job.WorkID,
				"status", apiErr.Status,
				"error", "revisar SCRAPING_SERVICE_TOKEN",
			)
			return
		}
		trabajador.logger.Warn("worker_delivery_pending", "trabajo_id", job.WorkID, "error", err)
		return
	}

	if queue.IsLost(lost) {
		trabajador.logger.Warn("worker_lease_lost", "trabajo_id", job.WorkID)
		return
	}
	acked, err := trabajador.queue.Ack(heartbeatCtx, message.ID, raw, job.WorkID, "")
	if err != nil {
		trabajador.logger.Warn("worker_ack_failed", "trabajo_id", job.WorkID, "error", err)
		return
	}
	if acked && state == "completado" {
		if _, err := storage.CleanupSpool(trabajador.cfg.StateDir, job.WorkID); err != nil {
			trabajador.logger.Warn("worker_spool_cleanup_failed", "trabajo_id", job.WorkID, "error", err)
		}
	}
}

// ===== Procesamiento de Trabajos =====

// processJob ejecuta la secuencia de extracción, acumulación en spool y envío al control.
func (trabajador *Worker) processJob(ctx context.Context, lost <-chan struct{}, job domain.Job) (string, error) {
	status, err := trabajador.api.GetWork(ctx, job.WorkID)
	if err != nil {
		return "", err
	}
	if isTerminal(status) {
		return stateOf(status), nil
	}

	store, err := spool.Open(job.WorkID, trabajador.cfg.StateDir, trabajador.cfg.MinFreeBytes)
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
	status, err = trabajador.api.GetWork(ctx, job.WorkID)
	if err != nil {
		return "", err
	}
	if isTerminal(status) {
		return stateOf(status), nil
	}

	if err := store.Seal(trabajador.cfg.BatchSize); err != nil {
		return "", err
	}
	if err := store.Replay(ctx, trabajador.api); err != nil {
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
		if attempts >= trabajador.cfg.CrawlAttempts {
			return "", ErrCrawlExhausted
		}
		if err := store.SetProgress("crawl_attempts", attempts+1); err != nil {
			return "", err
		}

		spider, ok := trabajador.registry.Get(job.Spider)
		if !ok {
			return "", fmt.Errorf("spider aún no implementado en Go: %s", job.Spider)
		}
		pending := 0
		emit := func(item domain.Item) error {
			item = spiders.CompleteItem(item)
			market := item.Supermercado
			if err := store.Add(item.Product, &market); err != nil {
				return err
			}
			pending++
			if pending >= trabajador.cfg.BatchSize {
				if err := store.Seal(trabajador.cfg.BatchSize); err != nil {
					return err
				}
				if err := store.Replay(ctx, trabajador.api); err != nil {
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

	if err := store.Seal(trabajador.cfg.BatchSize); err != nil {
		return "", err
	}
	if err := store.Replay(ctx, trabajador.api); err != nil {
		return "", err
	}
	if queue.IsLost(lost) {
		return "", ErrLeaseLost
	}
	// Un crawl con categorías vacías puede terminar sin error y sin lotes. No
	// se declara completado: se cierra como fallido para no publicar un
	// catálogo vacío como éxito.
	itemCount, err := store.Count()
	if err != nil {
		return "", err
	}
	if itemCount == 0 {
		return "", ErrNoProducts
	}
	final, err := trabajador.api.FinalizeWork(ctx, job.WorkID, "completado", nil, nil, "")
	if err != nil {
		return "", err
	}
	return fmt.Sprint(final["estado"]), nil
}

// ===== Deserialización y Validación =====

// jobPayload modela el esquema JSON del mensaje de trabajo recibido desde la cola.
type jobPayload struct {
	WorkID     string `json:"trabajo_id"`
	Spider     string `json:"spider"`
	ProductID  string `json:"product_id"`
	ProductURL string `json:"product_url"`
	AddURL     string `json:"add_url"`
	EnrichEAN  any    `json:"enrich_ean"`
}

// parseJob deserializa y valida la carga útil de un trabajo recibido en la cola.
func (trabajador *Worker) parseJob(raw string) (domain.Job, error) {
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
		ProductURL: payload.ProductURL,
		AddURL:     payload.AddURL,
		EnrichEAN:  parseEnrich(payload.EnrichEAN),
	}, nil
}

// parseEnrich extrae los parámetros específicos de enriquecimiento individual de producto.
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

// isTerminal verifica si un estado reportado por control indica la culminación del trabajo.
func isTerminal(status map[string]any) bool {
	state := stateOf(status)
	return state == "completado" || state == "fallido"
}

// stateOf recupera el estado vigente de un trabajo consultando el servicio de control.
func stateOf(status map[string]any) string {
	state, _ := status["estado"].(string)
	return state
}

// ===== Bloqueos de Concurrencia =====

// executionLock gestiona el descriptor del archivo de bloqueo para evitar concurrencia.
type executionLock struct {
	file *os.File
}

// lockExecution adquiere un bloqueo exclusivo sobre el sistema de archivos para el trabajo.
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

// Close libera los recursos asociados al descriptor o conexión.
func (bloqueo *executionLock) Close() error {
	_ = syscall.Flock(int(bloqueo.file.Fd()), syscall.LOCK_UN)
	return bloqueo.file.Close()
}

// sleep realiza una pausa controlada que respeta la cancelación del contexto.
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
