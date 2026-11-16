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

// New valida la configuración y construye el worker con sus dependencias
// derivadas: el cliente de recolección y el recolector de spools.
func New(
	configuracion config.Config,
	api *controlscraping.Client,
	clienteCola *queue.Queue,
	registro *spiders.Registry,
	registrador *slog.Logger,
) (*Worker, error) {
	if errorObtenido := configuracion.Validate(); errorObtenido != nil {
		return nil, fmt.Errorf("configuración inválida: %w", errorObtenido)
	}
	if registrador == nil {
		registrador = slog.Default()
	}
	tiempoEsperaGC := configuracion.GCTimeout
	if configuracion.APITimeout > 0 && configuracion.APITimeout < tiempoEsperaGC {
		tiempoEsperaGC = configuracion.APITimeout
	}
	return &Worker{
		cfg:      configuracion,
		api:      api,
		gcAPI:    controlscraping.New(configuracion.APIURL, configuracion.Token, tiempoEsperaGC),
		queue:    clienteCola,
		registry: registro,
		collector: storage.NewCollector(
			configuracion.StateDir,
			configuracion.MinFreeBytes,
			configuracion.FailedRetention,
			configuracion.CollectTimeout,
			registrador,
		),
		logger: registrador,
	}, nil
}

// ===== Bucle Principal =====

// Run asegura el grupo de consumo, recolecta spools vencidos y procesa
// mensajes hasta que el contexto se cancela. Los fallos de dependencias se
// reintentan con RetryInterval.
func (trabajador *Worker) Run(contexto context.Context) error {
	if trabajador.cfg.Token == "" {
		return fmt.Errorf("SCRAPING_SERVICE_TOKEN no configurado: Control de scraping rechazaría al worker")
	}
	if errorObtenido := os.MkdirAll(trabajador.cfg.StateDir, 0o700); errorObtenido != nil {
		return errorObtenido
	}

	trabajador.logger.Info("worker_started",
		"stream", trabajador.queue.Stream(),
		"group", trabajador.queue.Group(),
		"consumer", trabajador.queue.Consumer(),
	)

	siguienteRecoleccion := time.Time{}
	for {
		if contexto.Err() != nil {
			return nil
		}
		if errorObtenido := trabajador.queue.EnsureGroup(contexto); errorObtenido != nil {
			trabajador.logger.Warn("worker_group_unavailable", "error", errorObtenido)
			if !sleep(contexto, trabajador.cfg.RetryInterval) {
				return nil
			}
			continue
		}

		if time.Now().After(siguienteRecoleccion) {
			if _, errorObtenido := trabajador.collector.Collect(contexto, trabajador.gcAPI, trabajador.queue, trabajador.cfg.CollectLimit); errorObtenido != nil {
				trabajador.logger.Warn("worker_collection_pending", "error", errorObtenido)
			}
			siguienteRecoleccion = time.Now().Add(trabajador.cfg.CollectInterval)
		}

		if errorObtenido := storage.EnsureCapacity(trabajador.cfg.StateDir, trabajador.cfg.MinFreeBytes, 0); errorObtenido != nil {
			if errors.Is(errorObtenido, storage.ErrStoragePressure) {
				trabajador.logger.Error("worker_storage_pressure", "error", errorObtenido)
			} else {
				trabajador.logger.Warn("worker_storage_unavailable", "error", errorObtenido)
			}
			if !sleep(contexto, trabajador.cfg.RetryInterval) {
				return nil
			}
			continue
		}

		mensaje, errorObtenido := trabajador.queue.Receive(contexto)
		if errorObtenido != nil {
			if contexto.Err() != nil {
				return nil
			}
			trabajador.logger.Warn("worker_dependency_unavailable", "error", errorObtenido)
			if !sleep(contexto, trabajador.cfg.RetryInterval) {
				return nil
			}
			continue
		}
		if mensaje == nil {
			if !sleep(contexto, trabajador.cfg.PollInterval) {
				return nil
			}
			continue
		}
		trabajador.HandleMessage(contexto, mensaje)
	}
}

// HandleMessage procesa un mensaje ya reclamado: valida el payload, ejecuta
// el trabajo y confirma o deriva el mensaje según el resultado.
func (trabajador *Worker) HandleMessage(contexto context.Context, mensaje *redis.XMessage) {
	crudo, _ := mensaje.Values["payload"].(string)
	trabajo, errorObtenido := trabajador.parseJob(crudo)
	if errorObtenido != nil {
		trabajador.logger.Warn("worker_invalid_payload", "message_id", mensaje.ID, "error", errorObtenido)
		if _, errorConfirmacion := trabajador.queue.Ack(contexto, mensaje.ID, crudo, "", "invalid_payload"); errorConfirmacion != nil {
			trabajador.logger.Warn("worker_ack_failed", "message_id", mensaje.ID, "error", errorConfirmacion)
		}
		return
	}

	contextoLatido, cancelar, perdido, errorObtenido := trabajador.queue.Heartbeat(contexto, mensaje.ID)
	if errorObtenido != nil {
		trabajador.logger.Warn("worker_heartbeat_failed", "trabajo_id", trabajo.WorkID, "error", errorObtenido)
		return
	}
	defer cancelar()

	estado, errorObtenido := trabajador.processJob(contextoLatido, perdido, trabajo)
	if errorObtenido != nil {
		if queue.IsLost(perdido) || errors.Is(errorObtenido, ErrLeaseLost) {
			trabajador.logger.Warn("worker_lease_lost", "trabajo_id", trabajo.WorkID)
			return
		}

		var errorAPI *controlscraping.Error
		falloAPI := errors.As(errorObtenido, &errorAPI)
		var bloqueado *crawler.BlockedError
		falloBloqueo := errors.As(errorObtenido, &bloqueado)
		sinProductos := errors.Is(errorObtenido, ErrNoProducts)
		permanente := errors.Is(errorObtenido, ErrCrawlExhausted) || sinProductos || falloBloqueo ||
			(falloAPI && errorAPI.Permanent())
		if permanente {
			mensajeFallo := "Trabajo enviado a revisión; consultar spool y entregas"
			motivo := "delivery_rejected"
			var respuestasBloqueadas *int
			switch {
			case falloBloqueo:
				cantidadBloqueados := bloqueado.BlockedResponses
				if cantidadBloqueados < 1 {
					cantidadBloqueados = 1
				}
				respuestasBloqueadas = &cantidadBloqueados
				mensajeFallo = fmt.Sprintf(
					"bloqueo: %d respuestas bloqueadas (motivo=%s)",
					cantidadBloqueados,
					bloqueado.Reason,
				)
				motivo = "blocked"
				trabajador.logger.Warn("worker_job_blocked",
					"trabajo_id", trabajo.WorkID,
					"url", bloqueado.URL,
					"status", bloqueado.Status,
					"motivo", bloqueado.Reason,
					"respuestas_bloqueadas", cantidadBloqueados,
				)
			case sinProductos:
				mensajeFallo = "sin productos: el crawl no extrajo elementos; no se ingirió un lote vacío"
				motivo = "no_products"
				trabajador.logger.Warn("worker_job_without_products", "trabajo_id", trabajo.WorkID)
			case errors.Is(errorObtenido, ErrCrawlExhausted):
				mensajeFallo = "crawl agotado: se conserva el spool para revisión"
				motivo = "attempts_exhausted"
			}
			if !falloAPI || errorAPI.Status != 404 {
				if _, errorFinalizacion := trabajador.api.FinalizeWork(
					contextoLatido,
					trabajo.WorkID,
					"fallido",
					nil,
					respuestasBloqueadas,
					mensajeFallo,
				); errorFinalizacion != nil {
					trabajador.logger.Warn("worker_finalize_failed", "trabajo_id", trabajo.WorkID, "error", errorFinalizacion)
					return
				}
			}
			if _, errorConfirmacion := trabajador.queue.Ack(contextoLatido, mensaje.ID, crudo, trabajo.WorkID, motivo); errorConfirmacion != nil {
				trabajador.logger.Warn("worker_ack_failed", "trabajo_id", trabajo.WorkID, "error", errorConfirmacion)
			}
			return
		}
		if falloAPI && (errorAPI.Status == 401 || errorAPI.Status == 403) {
			trabajador.logger.Error("worker_auth_rejected",
				"trabajo_id", trabajo.WorkID,
				"status", errorAPI.Status,
				"error", "revisar SCRAPING_SERVICE_TOKEN",
			)
			return
		}
		trabajador.logger.Warn("worker_delivery_pending", "trabajo_id", trabajo.WorkID, "error", errorObtenido)
		return
	}

	if queue.IsLost(perdido) {
		trabajador.logger.Warn("worker_lease_lost", "trabajo_id", trabajo.WorkID)
		return
	}
	confirmado, errorObtenido := trabajador.queue.Ack(contextoLatido, mensaje.ID, crudo, trabajo.WorkID, "")
	if errorObtenido != nil {
		trabajador.logger.Warn("worker_ack_failed", "trabajo_id", trabajo.WorkID, "error", errorObtenido)
		return
	}
	if confirmado && estado == "completado" {
		if _, errorObtenido := storage.CleanupSpool(trabajador.cfg.StateDir, trabajo.WorkID); errorObtenido != nil {
			trabajador.logger.Warn("worker_spool_cleanup_failed", "trabajo_id", trabajo.WorkID, "error", errorObtenido)
		}
	}
}

// ===== Procesamiento de Trabajos =====

// processJob ejecuta la secuencia de extracción, acumulación en spool y envío al control.
func (trabajador *Worker) processJob(contexto context.Context, perdido <-chan struct{}, trabajo domain.Job) (string, error) {
	estado, errorObtenido := trabajador.api.GetWork(contexto, trabajo.WorkID)
	if errorObtenido != nil {
		return "", errorObtenido
	}
	if isTerminal(estado) {
		return stateOf(estado), nil
	}

	almacen, errorObtenido := spool.Open(trabajo.WorkID, trabajador.cfg.StateDir, trabajador.cfg.MinFreeBytes)
	if errorObtenido != nil {
		return "", errorObtenido
	}
	defer almacen.Close()

	bloqueo, errorObtenido := lockExecution(almacen.Directory())
	if errorObtenido != nil {
		return "", errorObtenido
	}
	defer bloqueo.Close()

	if queue.IsLost(perdido) {
		return "", ErrLeaseLost
	}
	estado, errorObtenido = trabajador.api.GetWork(contexto, trabajo.WorkID)
	if errorObtenido != nil {
		return "", errorObtenido
	}
	if isTerminal(estado) {
		return stateOf(estado), nil
	}

	if errorObtenido := almacen.Seal(trabajador.cfg.BatchSize); errorObtenido != nil {
		return "", errorObtenido
	}
	if errorObtenido := almacen.Replay(contexto, trabajador.api); errorObtenido != nil {
		return "", errorObtenido
	}

	terminado, errorObtenido := almacen.Progress("crawl_done")
	if errorObtenido != nil {
		return "", errorObtenido
	}
	if terminado == 0 {
		intentos, errorObtenido := almacen.Progress("crawl_attempts")
		if errorObtenido != nil {
			return "", errorObtenido
		}
		if intentos >= trabajador.cfg.CrawlAttempts {
			return "", ErrCrawlExhausted
		}
		if errorObtenido := almacen.SetProgress("crawl_attempts", intentos+1); errorObtenido != nil {
			return "", errorObtenido
		}

		spider, valido := trabajador.registry.Get(trabajo.Spider)
		if !valido {
			return "", fmt.Errorf("spider aún no implementado en Go: %s", trabajo.Spider)
		}
		pendiente := 0
		emitir := func(elemento domain.Item) error {
			elemento = spiders.CompleteItem(elemento)
			mercado := elemento.Supermercado
			if errorObtenido := almacen.Add(elemento.Product, &mercado); errorObtenido != nil {
				return errorObtenido
			}
			pendiente++
			if pendiente >= trabajador.cfg.BatchSize {
				if errorObtenido := almacen.Seal(trabajador.cfg.BatchSize); errorObtenido != nil {
					return errorObtenido
				}
				if errorObtenido := almacen.Replay(contexto, trabajador.api); errorObtenido != nil {
					return errorObtenido
				}
				pendiente = 0
			}
			return nil
		}
		if errorObtenido := spider.Run(contexto, trabajo, emitir); errorObtenido != nil {
			var bloqueado *crawler.BlockedError
			if errors.As(errorObtenido, &bloqueado) {
				// Se conserva en el spool para diagnóstico y para que el
				// operador pueda distinguir el bloqueo de un fallo de red.
				_ = almacen.SetProgress("blocked_responses", bloqueado.BlockedResponses)
			}
			return "", errorObtenido
		}
		if errorObtenido := almacen.SetProgress("crawl_done", 1); errorObtenido != nil {
			return "", errorObtenido
		}
	}

	if errorObtenido := almacen.Seal(trabajador.cfg.BatchSize); errorObtenido != nil {
		return "", errorObtenido
	}
	if errorObtenido := almacen.Replay(contexto, trabajador.api); errorObtenido != nil {
		return "", errorObtenido
	}
	if queue.IsLost(perdido) {
		return "", ErrLeaseLost
	}
	// Un crawl con categorías vacías puede terminar sin error y sin lotes. No
	// se declara completado: se cierra como fallido para no publicar un
	// catálogo vacío como éxito.
	cantidadElementos, errorObtenido := almacen.Count()
	if errorObtenido != nil {
		return "", errorObtenido
	}
	if cantidadElementos == 0 {
		return "", ErrNoProducts
	}
	final, errorObtenido := trabajador.api.FinalizeWork(contexto, trabajo.WorkID, "completado", nil, nil, "")
	if errorObtenido != nil {
		return "", errorObtenido
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
func (trabajador *Worker) parseJob(crudo string) (domain.Job, error) {
	var carga jobPayload
	if errorObtenido := json.Unmarshal([]byte(crudo), &carga); errorObtenido != nil {
		return domain.Job{}, errorObtenido
	}
	if _, errorObtenido := uuid.Parse(carga.WorkID); errorObtenido != nil {
		return domain.Job{}, fmt.Errorf("trabajo_id inválido: %w", errorObtenido)
	}
	spider := carga.Spider
	if spider == "" {
		spider = spiders.JumboName
	}
	if trabajador.registry == nil {
		return domain.Job{}, fmt.Errorf("registro de spiders no configurado")
	}
	if _, valido := trabajador.registry.Get(spider); !valido {
		return domain.Job{}, fmt.Errorf("spider inválido: %s", spider)
	}
	if carga.ProductID != "" && carga.ProductURL == "" {
		return domain.Job{}, fmt.Errorf("product_id requiere product_url")
	}
	return domain.Job{
		WorkID:     carga.WorkID,
		Spider:     spider,
		ProductURL: carga.ProductURL,
		AddURL:     carga.AddURL,
		EnrichEAN:  parseEnrich(carga.EnrichEAN),
	}, nil
}

// parseEnrich extrae los parámetros específicos de enriquecimiento individual de producto.
func parseEnrich(valor any) bool {
	switch tipado := valor.(type) {
	case bool:
		return tipado
	case string:
		switch strings.ToLower(strings.TrimSpace(tipado)) {
		case "1", "true", "yes":
			return true
		}
	case float64:
		return tipado == 1
	}
	return false
}

// isTerminal verifica si un estado reportado por control indica la culminación del trabajo.
func isTerminal(estado map[string]any) bool {
	estadoFinal := stateOf(estado)
	return estadoFinal == "completado" || estadoFinal == "fallido"
}

// stateOf recupera el estado vigente de un trabajo consultando el servicio de control.
func stateOf(estado map[string]any) string {
	estadoTrabajo, _ := estado["estado"].(string)
	return estadoTrabajo
}

// ===== Bloqueos de Concurrencia =====

// executionLock gestiona el descriptor del archivo de bloqueo para evitar concurrencia.
type executionLock struct {
	file *os.File
}

// lockExecution adquiere un bloqueo exclusivo sobre el sistema de archivos para el trabajo.
func lockExecution(directorio string) (*executionLock, error) {
	archivo, errorObtenido := os.OpenFile(filepath.Join(directorio, "execution.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if errorObtenido := syscall.Flock(int(archivo.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); errorObtenido != nil {
		_ = archivo.Close()
		if errors.Is(errorObtenido, syscall.EWOULDBLOCK) || errors.Is(errorObtenido, syscall.EAGAIN) {
			return nil, fmt.Errorf("el spool ya está en ejecución por otro worker")
		}
		return nil, errorObtenido
	}
	return &executionLock{file: archivo}, nil
}

// Close libera los recursos asociados al descriptor o conexión.
func (bloqueo *executionLock) Close() error {
	_ = syscall.Flock(int(bloqueo.file.Fd()), syscall.LOCK_UN)
	return bloqueo.file.Close()
}

// sleep realiza una pausa controlada que respeta la cancelación del contexto.
func sleep(contexto context.Context, duracion time.Duration) bool {
	temporizador := time.NewTimer(duracion)
	defer temporizador.Stop()
	select {
	case <-contexto.Done():
		return false
	case <-temporizador.C:
		return true
	}
}
