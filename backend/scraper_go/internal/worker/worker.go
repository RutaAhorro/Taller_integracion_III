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
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	cola "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/cola"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
)

// ===== Errores de Orquestación =====

var (
	// ErrCrawlExhausted indica que el trabajo superó el número máximo de intentos de crawl permitidos.
	ErrCrawlExhausted = errors.New("el crawl agotó los intentos configurados; el spool se conserva")
	// ErrLeaseLost indica que otro consumidor reclamó el mensaje en Redis durante la ejecución.
	ErrLeaseLost = errors.New("se perdió la lease del trabajo")
	// ErrNoProducts indica que el spider finalizó sin extraer ningún producto válido.
	ErrNoProducts = errors.New("el crawl terminó sin productos")
)

// ===== Estructura de Orquestador =====

// Worker consume trabajos de la cola y ejecuta el ciclo completo: lease,
// spool, crawl, entrega y cierre.
type Worker struct {
	configuracion config.Config
	api           *cola.Client
	apiGC         *cola.Client
	clienteCola   *cola.Queue
	registro      *extraccion.Registry
	recolector    *cola.Collector
	registrador   *slog.Logger
}

// ===== Constructor =====

// New valida la configuración y las dependencias obligatorias, y construye el
// worker junto con el cliente de recolección y el recolector de spools.
func New(
	configuracion config.Config,
	api *cola.Client,
	clienteCola *cola.Queue,
	registro *extraccion.Registry,
	registrador *slog.Logger,
) (*Worker, error) {
	if errorObtenido := configuracion.Validate(); errorObtenido != nil {
		return nil, fmt.Errorf("configuración inválida: %w", errorObtenido)
	}
	if strings.TrimSpace(configuracion.Token) == "" {
		return nil, fmt.Errorf("SCRAPING_SERVICE_TOKEN no configurado: Control de scraping rechazaría al worker")
	}
	if api == nil {
		return nil, fmt.Errorf("cliente de Control de Scraping no configurado")
	}
	if clienteCola == nil {
		return nil, fmt.Errorf("cola de trabajos no configurada")
	}
	if registro == nil {
		return nil, fmt.Errorf("registro de spiders no configurado")
	}
	if registrador == nil {
		registrador = slog.Default()
	}
	tiempoEsperaGC := configuracion.GCTimeout
	if configuracion.APITimeout > 0 && configuracion.APITimeout < tiempoEsperaGC {
		tiempoEsperaGC = configuracion.APITimeout
	}
	return &Worker{
		configuracion: configuracion,
		api:           api,
		apiGC:         cola.NewControlClient(configuracion.APIURL, configuracion.Token, tiempoEsperaGC),
		clienteCola:   clienteCola,
		registro:      registro,
		recolector: cola.NewCollector(
			configuracion.StateDir,
			configuracion.MinFreeBytes,
			configuracion.FailedRetention,
			configuracion.CollectTimeout,
			registrador,
		),
		registrador: registrador,
	}, nil
}

// ===== Bucle Principal =====

// Run asegura el directorio de estado y el grupo de consumo, recolecta spools
// vencidos y procesa mensajes hasta que el contexto se cancela. Los fallos de
// dependencias se reintentan con RetryInterval.
func (trabajador *Worker) Run(contexto context.Context) error {
	if errorObtenido := os.MkdirAll(trabajador.configuracion.StateDir, 0o700); errorObtenido != nil {
		return errorObtenido
	}

	trabajador.registrador.Info("worker_started",
		"stream", trabajador.clienteCola.Stream(),
		"group", trabajador.clienteCola.Group(),
		"consumer", trabajador.clienteCola.Consumer(),
	)

	siguienteRecoleccion := time.Time{}
	for {
		if contexto.Err() != nil {
			return nil
		}
		if errorObtenido := trabajador.clienteCola.EnsureGroup(contexto); errorObtenido != nil {
			trabajador.registrador.Warn("worker_group_unavailable", "error", errorObtenido)
			if !pausarConContexto(contexto, trabajador.configuracion.RetryInterval) {
				return nil
			}
			continue
		}

		if time.Now().After(siguienteRecoleccion) {
			if _, errorObtenido := trabajador.recolector.Collect(contexto, trabajador.apiGC, trabajador.clienteCola, trabajador.configuracion.CollectLimit); errorObtenido != nil {
				trabajador.registrador.Warn("worker_collection_pending", "error", errorObtenido)
			}
			siguienteRecoleccion = time.Now().Add(trabajador.configuracion.CollectInterval)
		}

		if errorObtenido := cola.EnsureCapacity(trabajador.configuracion.StateDir, trabajador.configuracion.MinFreeBytes, 0); errorObtenido != nil {
			if errors.Is(errorObtenido, cola.ErrStoragePressure) {
				trabajador.registrador.Error("worker_storage_pressure", "error", errorObtenido)
			} else {
				trabajador.registrador.Warn("worker_storage_unavailable", "error", errorObtenido)
			}
			if !pausarConContexto(contexto, trabajador.configuracion.RetryInterval) {
				return nil
			}
			continue
		}

		mensaje, errorObtenido := trabajador.clienteCola.Receive(contexto)
		if errorObtenido != nil {
			if contexto.Err() != nil {
				return nil
			}
			trabajador.registrador.Warn("worker_dependency_unavailable", "error", errorObtenido)
			if !pausarConContexto(contexto, trabajador.configuracion.RetryInterval) {
				return nil
			}
			continue
		}
		if mensaje == nil {
			if !pausarConContexto(contexto, trabajador.configuracion.PollInterval) {
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
	trabajo, errorObtenido := trabajador.analizarTrabajo(crudo)
	if errorObtenido != nil {
		trabajador.registrador.Warn("worker_invalid_payload", "message_id", mensaje.ID, "error", errorObtenido)
		if _, errorConfirmacion := trabajador.clienteCola.Ack(contexto, mensaje.ID, crudo, "", "invalid_payload"); errorConfirmacion != nil {
			trabajador.registrador.Warn("worker_ack_failed", "message_id", mensaje.ID, "error", errorConfirmacion)
		}
		return
	}

	contextoLatido, cancelar, perdido, errorObtenido := trabajador.clienteCola.Heartbeat(contexto, mensaje.ID)
	if errorObtenido != nil {
		trabajador.registrador.Warn("worker_heartbeat_failed", "trabajo_id", trabajo.WorkID, "error", errorObtenido)
		return
	}
	defer cancelar()

	estado, errorObtenido := trabajador.procesarTrabajo(contextoLatido, perdido, trabajo)
	if errorObtenido != nil {
		if cola.IsLost(perdido) || errors.Is(errorObtenido, ErrLeaseLost) {
			trabajador.registrador.Warn("worker_lease_lost", "trabajo_id", trabajo.WorkID)
			return
		}

		var errorAPI *cola.Error
		falloAPI := errors.As(errorObtenido, &errorAPI)
		var bloqueado *extraccion.BlockedError
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
				trabajador.registrador.Warn("worker_job_blocked",
					"trabajo_id", trabajo.WorkID,
					"url", bloqueado.URL,
					"status", bloqueado.Status,
					"motivo", bloqueado.Reason,
					"respuestas_bloqueadas", cantidadBloqueados,
				)
			case sinProductos:
				mensajeFallo = "sin productos: el crawl no extrajo elementos; no se ingirió un lote vacío"
				motivo = "no_products"
				trabajador.registrador.Warn("worker_job_without_products", "trabajo_id", trabajo.WorkID)
			case errors.Is(errorObtenido, ErrCrawlExhausted):
				mensajeFallo = "crawl agotado: se conserva el spool para revisión"
				motivo = "attempts_exhausted"
			}
			if !falloAPI || errorAPI.Status != http.StatusNotFound {
				if _, errorFinalizacion := trabajador.api.FinalizeWork(
					contextoLatido,
					trabajo.WorkID,
					"fallido",
					nil,
					respuestasBloqueadas,
					mensajeFallo,
				); errorFinalizacion != nil {
					trabajador.registrador.Warn("worker_finalize_failed", "trabajo_id", trabajo.WorkID, "error", errorFinalizacion)
					return
				}
			}
			if _, errorConfirmacion := trabajador.clienteCola.Ack(contextoLatido, mensaje.ID, crudo, trabajo.WorkID, motivo); errorConfirmacion != nil {
				trabajador.registrador.Warn("worker_ack_failed", "trabajo_id", trabajo.WorkID, "error", errorConfirmacion)
			}
			return
		}
		if falloAPI && (errorAPI.Status == http.StatusUnauthorized || errorAPI.Status == http.StatusForbidden) {
			trabajador.registrador.Error("worker_auth_rejected",
				"trabajo_id", trabajo.WorkID,
				"status", errorAPI.Status,
				"error", "revisar SCRAPING_SERVICE_TOKEN",
			)
			return
		}
		trabajador.registrador.Warn("worker_delivery_pending", "trabajo_id", trabajo.WorkID, "error", errorObtenido)
		return
	}

	if cola.IsLost(perdido) {
		trabajador.registrador.Warn("worker_lease_lost", "trabajo_id", trabajo.WorkID)
		return
	}
	confirmado, errorObtenido := trabajador.clienteCola.Ack(contextoLatido, mensaje.ID, crudo, trabajo.WorkID, "")
	if errorObtenido != nil {
		trabajador.registrador.Warn("worker_ack_failed", "trabajo_id", trabajo.WorkID, "error", errorObtenido)
		return
	}
	if confirmado && estado == "completado" {
		if _, errorObtenido := cola.CleanupSpool(trabajador.configuracion.StateDir, trabajo.WorkID); errorObtenido != nil {
			trabajador.registrador.Warn("worker_spool_cleanup_failed", "trabajo_id", trabajo.WorkID, "error", errorObtenido)
		}
	}
}

// ===== Procesamiento de Trabajos =====

// procesarTrabajo ejecuta la secuencia de extracción, acumulación en spool y envío al control.
func (trabajador *Worker) procesarTrabajo(contexto context.Context, perdido <-chan struct{}, trabajo dominio.Job) (string, error) {
	estado, errorObtenido := trabajador.api.GetWork(contexto, trabajo.WorkID)
	if errorObtenido != nil {
		return "", errorObtenido
	}
	if esEstadoTerminal(estado) {
		return estadoDeTrabajo(estado), nil
	}

	almacen, errorObtenido := cola.Open(trabajo.WorkID, trabajador.configuracion.StateDir, trabajador.configuracion.MinFreeBytes)
	if errorObtenido != nil {
		return "", errorObtenido
	}
	defer almacen.Close()

	bloqueo, errorObtenido := bloquearEjecucion(almacen.Directory())
	if errorObtenido != nil {
		return "", errorObtenido
	}
	defer bloqueo.cerrar()

	if cola.IsLost(perdido) {
		return "", ErrLeaseLost
	}
	estado, errorObtenido = trabajador.api.GetWork(contexto, trabajo.WorkID)
	if errorObtenido != nil {
		return "", errorObtenido
	}
	if esEstadoTerminal(estado) {
		return estadoDeTrabajo(estado), nil
	}

	if errorObtenido := almacen.Seal(trabajador.configuracion.BatchSize); errorObtenido != nil {
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
		if intentos >= trabajador.configuracion.CrawlAttempts {
			return "", ErrCrawlExhausted
		}
		if errorObtenido := almacen.SetProgress("crawl_attempts", intentos+1); errorObtenido != nil {
			return "", errorObtenido
		}

		spider, valido := trabajador.registro.Get(trabajo.Spider)
		if !valido {
			return "", fmt.Errorf("spider aún no implementado en Go: %s", trabajo.Spider)
		}
		pendiente := 0
		emitir := func(elemento dominio.Item) error {
			elemento = extraccion.CompleteItem(elemento)
			mercado := elemento.Supermercado
			if errorObtenido := almacen.Add(elemento.Product, &mercado); errorObtenido != nil {
				return errorObtenido
			}
			pendiente++
			if pendiente >= trabajador.configuracion.BatchSize {
				if errorObtenido := almacen.Seal(trabajador.configuracion.BatchSize); errorObtenido != nil {
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
			var bloqueado *extraccion.BlockedError
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

	if errorObtenido := almacen.Seal(trabajador.configuracion.BatchSize); errorObtenido != nil {
		return "", errorObtenido
	}
	if errorObtenido := almacen.Replay(contexto, trabajador.api); errorObtenido != nil {
		return "", errorObtenido
	}
	if cola.IsLost(perdido) {
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

// cargaTrabajo modela el esquema JSON del mensaje de trabajo recibido desde la cola.
type cargaTrabajo struct {
	WorkID     string `json:"trabajo_id"`
	Spider     string `json:"spider"`
	ProductID  string `json:"product_id"`
	ProductURL string `json:"product_url"`
	AddURL     string `json:"add_url"`
	EnrichEAN  any    `json:"enrich_ean"`
}

// analizarTrabajo deserializa y valida la carga útil de un trabajo recibido en la cola.
func (trabajador *Worker) analizarTrabajo(crudo string) (dominio.Job, error) {
	var carga cargaTrabajo
	if errorObtenido := json.Unmarshal([]byte(crudo), &carga); errorObtenido != nil {
		return dominio.Job{}, errorObtenido
	}
	if _, errorObtenido := uuid.Parse(carga.WorkID); errorObtenido != nil {
		return dominio.Job{}, fmt.Errorf("trabajo_id inválido: %w", errorObtenido)
	}
	spider := carga.Spider
	if spider == "" {
		spider = extraccion.JumboName
	}
	if _, valido := trabajador.registro.Get(spider); !valido {
		return dominio.Job{}, fmt.Errorf("spider inválido: %s", spider)
	}
	if carga.ProductID != "" && carga.ProductURL == "" {
		return dominio.Job{}, fmt.Errorf("product_id requiere product_url")
	}
	return dominio.Job{
		WorkID:     carga.WorkID,
		Spider:     spider,
		ProductURL: carga.ProductURL,
		AddURL:     carga.AddURL,
		EnrichEAN:  analizarEnriquecimiento(carga.EnrichEAN),
	}, nil
}

// analizarEnriquecimiento extrae los parámetros específicos de enriquecimiento individual de producto.
func analizarEnriquecimiento(valor any) bool {
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

// esEstadoTerminal verifica si un estado reportado por control indica la culminación del trabajo.
func esEstadoTerminal(estado map[string]any) bool {
	estadoFinal := estadoDeTrabajo(estado)
	return estadoFinal == "completado" || estadoFinal == "fallido"
}

// estadoDeTrabajo recupera el estado vigente de un trabajo consultando el servicio de control.
func estadoDeTrabajo(estado map[string]any) string {
	estadoTrabajo, _ := estado["estado"].(string)
	return estadoTrabajo
}

// ===== Bloqueos de Concurrencia =====

// bloqueoEjecucion gestiona el descriptor del archivo de bloqueo para evitar concurrencia.
type bloqueoEjecucion struct {
	archivo *os.File
}

// bloquearEjecucion adquiere un bloqueo exclusivo sobre el sistema de archivos para el trabajo.
func bloquearEjecucion(directorio string) (*bloqueoEjecucion, error) {
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
	return &bloqueoEjecucion{archivo: archivo}, nil
}

// cerrar libera los recursos asociados al descriptor de bloqueo.
func (bloqueo *bloqueoEjecucion) cerrar() error {
	_ = syscall.Flock(int(bloqueo.archivo.Fd()), syscall.LOCK_UN)
	return bloqueo.archivo.Close()
}

// pausarConContexto realiza una pausa controlada que respeta la cancelación del contexto.
func pausarConContexto(contexto context.Context, duracion time.Duration) bool {
	temporizador := time.NewTimer(duracion)
	defer temporizador.Stop()
	select {
	case <-contexto.Done():
		return false
	case <-temporizador.C:
		return true
	}
}
