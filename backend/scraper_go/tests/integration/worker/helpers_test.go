// helpers_test.go provee el arnés de integración del worker con Redis real,
// servidor HTTP de Control falso y spiders simulados.
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

	cola "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/cola"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/worker"
)

// ===== Auxiliares de Prueba =====

func punteroFlotante(valor float64) *float64 {
	return &valor
}

func registradorSilencioso() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func omitirSiRedisNoDisponible(prueba *testing.T, errorConexion error) {
	prueba.Helper()
	if os.Getenv("REDIS_TEST_URL") != "" {
		prueba.Fatalf("Redis configurado en REDIS_TEST_URL no está disponible: %v", errorConexion)
	}
	prueba.Skipf("Redis no disponible: %v", errorConexion)
}

// ===== Servidor Falso de Control =====

type estadoAPI struct {
	sync.Mutex
	estado       string
	finalizado   string
	mensajeFinal string
	bloqueadas   int
	lotes        int
	productos    []dominio.Product
	supermercado string
	idempotencia string
}

func (estado *estadoAPI) estadoActual() string {
	estado.Lock()
	defer estado.Unlock()
	if estado.finalizado != "" {
		return estado.finalizado
	}
	return estado.estado
}

func nuevaAPIFalsa(prueba *testing.T, trabajoID string) (*httptest.Server, *estadoAPI) {
	prueba.Helper()
	estado := &estadoAPI{estado: "en_progreso"}
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		rutaBase := "/api/v1/scraper/trabajos/" + trabajoID
		switch {
		case peticion.Method == http.MethodGet && peticion.URL.Path == rutaBase:
			escribirJSON(respuesta, map[string]any{"id": trabajoID, "estado": estado.estadoActual()})
		case peticion.Method == http.MethodPost && peticion.URL.Path == rutaBase+"/productos":
			var carga struct {
				Productos    []dominio.Product `json:"productos"`
				Supermercado string            `json:"supermercado"`
			}
			if errorDecodificacion := json.NewDecoder(peticion.Body).Decode(&carga); errorDecodificacion != nil {
				prueba.Errorf("decode productos: %v", errorDecodificacion)
				return
			}
			estado.Lock()
			estado.lotes++
			estado.productos = append(estado.productos, carga.Productos...)
			estado.supermercado = carga.Supermercado
			estado.idempotencia = peticion.Header.Get("Idempotency-Key")
			estado.Unlock()
			escribirJSON(respuesta, map[string]any{
				"total_recibidos":     len(carga.Productos),
				"precios_registrados": len(carga.Productos),
				"insertados":          len(carga.Productos),
				"actualizados":        0,
			})
		case peticion.Method == http.MethodPut && peticion.URL.Path == rutaBase+"/finalizar":
			var carga struct {
				Estado               string  `json:"estado"`
				RespuestasBloqueadas *int    `json:"respuestas_bloqueadas"`
				RegistroErrores      *string `json:"registro_errores"`
			}
			if errorDecodificacion := json.NewDecoder(peticion.Body).Decode(&carga); errorDecodificacion != nil {
				prueba.Errorf("decode finalizar: %v", errorDecodificacion)
				return
			}
			estado.Lock()
			estado.finalizado = carga.Estado
			if carga.RespuestasBloqueadas != nil {
				estado.bloqueadas = *carga.RespuestasBloqueadas
			}
			if carga.RegistroErrores != nil {
				estado.mensajeFinal = *carga.RegistroErrores
			}
			estado.Unlock()
			escribirJSON(respuesta, map[string]any{"id": trabajoID, "estado": carga.Estado})
		default:
			http.NotFound(respuesta, peticion)
		}
	}))
	return servidor, estado
}

func escribirJSON(respuesta http.ResponseWriter, carga any) {
	respuesta.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(respuesta).Encode(carga)
}

// ===== Spider Falso =====

type spiderFalso struct {
	nombre       string
	elementos    []dominio.Item
	errorForzado error

	candado       sync.Mutex
	llamadas      int
	ultimoTrabajo dominio.Job
}

func (falso *spiderFalso) Name() string {
	return falso.nombre
}

func (falso *spiderFalso) Run(_ context.Context, trabajo dominio.Job, emitir extraccion.Emitter) error {
	falso.candado.Lock()
	falso.llamadas++
	falso.ultimoTrabajo = trabajo
	falso.candado.Unlock()

	for _, elemento := range falso.elementos {
		if errorEmision := emitir(elemento); errorEmision != nil {
			return errorEmision
		}
	}
	return falso.errorForzado
}

func (falso *spiderFalso) cantidadEjecuciones() int {
	falso.candado.Lock()
	defer falso.candado.Unlock()
	return falso.llamadas
}

func (falso *spiderFalso) trabajoRecibido() dominio.Job {
	falso.candado.Lock()
	defer falso.candado.Unlock()
	return falso.ultimoTrabajo
}

// ===== Arnés de Integración del Worker =====

type arnesWorker struct {
	trabajoID     string
	flujo         string
	grupo         string
	indice        string
	colaMuerta    string
	prefijoCierre string
	clienteRedis  *redis.Client
	colaTrabajos  *cola.Queue
	servidor      *httptest.Server
	estadoControl *estadoAPI
	spider        *spiderFalso
	configuracion config.Config
	trabajador    *worker.Worker
}

func nuevoArnesWorker(prueba *testing.T, spider *spiderFalso, mutar func(*config.Config)) *arnesWorker {
	prueba.Helper()
	trabajoID := uuid.NewString()
	servidor, estado := nuevaAPIFalsa(prueba, trabajoID)

	urlRedis := os.Getenv("REDIS_TEST_URL")
	if urlRedis == "" {
		urlRedis = "redis://localhost:6379/15"
	}
	opcionesRedis, errorParseo := redis.ParseURL(urlRedis)
	if errorParseo != nil {
		servidor.Close()
		prueba.Fatalf("REDIS_TEST_URL inválida: %v", errorParseo)
	}
	opcionesRedis.DialTimeout = 500 * time.Millisecond
	opcionesRedis.ReadTimeout = 2 * time.Second
	opcionesRedis.WriteTimeout = 2 * time.Second
	clienteRedis := redis.NewClient(opcionesRedis)

	contextoPing, cancelarPing := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelarPing()
	if errorPing := clienteRedis.Ping(contextoPing).Err(); errorPing != nil {
		_ = clienteRedis.Close()
		servidor.Close()
		omitirSiRedisNoDisponible(prueba, errorPing)
	}

	prefijo := "scraper:test:worker:" + uuid.NewString()
	arnes := &arnesWorker{
		trabajoID:     trabajoID,
		flujo:         prefijo + ":stream",
		grupo:         prefijo + ":group",
		indice:        prefijo + ":index",
		colaMuerta:    prefijo + ":dead",
		prefijoCierre: prefijo + ":closed:",
		clienteRedis:  clienteRedis,
		servidor:      servidor,
		estadoControl: estado,
		spider:        spider,
	}
	prueba.Cleanup(func() {
		contextoLimpieza, cancelarLimpieza := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelarLimpieza()
		_ = clienteRedis.Del(contextoLimpieza, arnes.flujo, arnes.indice, arnes.colaMuerta, arnes.prefijoCierre+arnes.trabajoID).Err()
		_ = clienteRedis.Close()
		servidor.Close()
	})

	var errorCola error
	arnes.colaTrabajos, errorCola = cola.NewQueue(clienteRedis, cola.Options{
		Stream:       arnes.flujo,
		Group:        arnes.grupo,
		Lease:        10 * time.Second,
		Consumer:     "worker-test",
		IndexKey:     arnes.indice,
		DeadLetter:   arnes.colaMuerta,
		ClosedPrefix: arnes.prefijoCierre,
	})
	if errorCola != nil {
		prueba.Fatalf("cola.NewQueue() error = %v", errorCola)
	}
	if errorGrupo := arnes.colaTrabajos.EnsureGroup(context.Background()); errorGrupo != nil {
		prueba.Fatalf("EnsureGroup() error = %v", errorGrupo)
	}

	configuracion := config.Default()
	configuracion.APIURL = servidor.URL + "/api/v1/scraper/productos"
	configuracion.Token = "token"
	configuracion.StateDir = prueba.TempDir()
	configuracion.MinFreeBytes = 1 << 20
	configuracion.BatchSize = 2
	configuracion.APITimeout = 2 * time.Second
	configuracion.CrawlAttempts = 3
	if mutar != nil {
		mutar(&configuracion)
	}
	arnes.configuracion = configuracion

	registro := &extraccion.Registry{}
	registro.Register(spider)
	var errorWorker error
	arnes.trabajador, errorWorker = worker.New(
		configuracion,
		cola.NewControlClient(configuracion.APIURL, configuracion.Token, configuracion.APITimeout),
		arnes.colaTrabajos,
		registro,
		registradorSilencioso(),
	)
	if errorWorker != nil {
		prueba.Fatalf("worker.New() error = %v", errorWorker)
	}
	return arnes
}

func (arnes *arnesWorker) agregarTrabajo(prueba *testing.T, cargaCruda string) string {
	prueba.Helper()
	contexto, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelar()
	mensajeID, errorAdicion := arnes.clienteRedis.XAdd(contexto, &redis.XAddArgs{
		Stream: arnes.flujo,
		Values: map[string]any{"payload": cargaCruda},
	}).Result()
	if errorAdicion != nil {
		prueba.Fatalf("XAdd() error = %v", errorAdicion)
	}
	if errorIndice := arnes.clienteRedis.HSet(contexto, arnes.indice, arnes.trabajoID, mensajeID).Err(); errorIndice != nil {
		prueba.Fatalf("HSet() error = %v", errorIndice)
	}
	return mensajeID
}

func (arnes *arnesWorker) cargaTrabajo(extra string) string {
	cargaCruda := `{"trabajo_id":"` + arnes.trabajoID + `","spider":"` + arnes.spider.Name() + `"`
	if extra != "" {
		cargaCruda += "," + extra
	}
	return cargaCruda + "}"
}

// ejecutarTrabajo encola el mensaje, lo reclama como el consumidor y lo entrega
// al worker, igual que el ciclo real.
func (arnes *arnesWorker) ejecutarTrabajo(prueba *testing.T, cargaCruda string) {
	prueba.Helper()
	arnes.agregarTrabajo(prueba, cargaCruda)
	contexto, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()
	mensaje, errorRecepcion := arnes.colaTrabajos.Receive(contexto)
	if errorRecepcion != nil || mensaje == nil {
		prueba.Fatalf("Receive() = %+v, %v", mensaje, errorRecepcion)
	}
	arnes.trabajador.HandleMessage(contexto, mensaje)
}

func (arnes *arnesWorker) mensajesMuertos(prueba *testing.T) []redis.XMessage {
	prueba.Helper()
	contexto, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelar()
	mensajes, errorRango := arnes.clienteRedis.XRange(contexto, arnes.colaMuerta, "-", "+").Result()
	if errorRango != nil {
		prueba.Fatalf("XRange(dead) error = %v", errorRango)
	}
	return mensajes
}

func (arnes *arnesWorker) longitudFlujo(prueba *testing.T) int64 {
	prueba.Helper()
	contexto, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelar()
	longitud, errorLongitud := arnes.clienteRedis.XLen(contexto, arnes.flujo).Result()
	if errorLongitud != nil {
		prueba.Fatalf("XLen() error = %v", errorLongitud)
	}
	return longitud
}

func (arnes *arnesWorker) mensajesPendientes(prueba *testing.T) int64 {
	prueba.Helper()
	contexto, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelar()
	pendientes, errorPendientes := arnes.clienteRedis.XPending(contexto, arnes.flujo, arnes.grupo).Result()
	if errorPendientes != nil {
		prueba.Fatalf("XPending() error = %v", errorPendientes)
	}
	return pendientes.Count
}
