// memory_test.go verifica el presupuesto de memoria (RSS y RssAnon) del
// pipeline de spool, sellado y entrega cuando SCRAPER_MEMORY_GATE=1.
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

	cola "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/cola"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
)

// ===== Auxiliares de Prueba =====

func punteroFlotante(valor float64) *float64 {
	return &valor
}

// ===== Gate de Memoria =====

// TestMemoryGate atraviesa el pipeline real de spool, lotes y entrega con un
// catálogo sintético, usando sólo la API pública de los paquetes. No se
// ejecuta en la suite normal porque mide RSS del proceso; CI lo activa con
// SCRAPER_MEMORY_GATE=1.
func TestMemoryGate(prueba *testing.T) {
	if os.Getenv("SCRAPER_MEMORY_GATE") != "1" {
		prueba.Skip("gate de RAM desactivado; usar SCRAPER_MEMORY_GATE=1")
	}
	if runtime.GOOS != "linux" {
		prueba.Skip("la medición de VmHWM requiere Linux")
	}

	cantidadElementos := leerEnteroEntorno("SCRAPER_MEMORY_ITEMS", 200_000)
	limiteMB := leerFlotanteEntorno("SCRAPER_MEMORY_LIMIT_MB", 150)
	compacto := os.Getenv("SCRAPER_MEMORY_COMPACT") == "1"
	if cantidadElementos < 1 {
		prueba.Fatal("SCRAPER_MEMORY_ITEMS debe ser mayor que cero")
	}
	const tamanoLote = config.MaxBatchSize

	lineaBaseMB := picoRSSMB(prueba)
	muestreadorAnon := iniciarMuestreadorAnon(10 * time.Millisecond)
	trabajoID := uuid.NewString()
	var (
		candado    sync.Mutex
		finalizado string
		recibidos  int
	)
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		rutaBase := "/api/v1/scraper/trabajos/" + trabajoID
		switch {
		case peticion.Method == http.MethodGet && peticion.URL.Path == rutaBase:
			candado.Lock()
			estado := "en_progreso"
			if finalizado != "" {
				estado = finalizado
			}
			candado.Unlock()
			escribirRespuestaJSON(respuesta, map[string]any{"id": trabajoID, "estado": estado})
		case peticion.Method == http.MethodPost && peticion.URL.Path == rutaBase+"/productos":
			var carga struct {
				Productos []dominio.Product `json:"productos"`
			}
			if errorDecodificacion := json.NewDecoder(peticion.Body).Decode(&carga); errorDecodificacion != nil {
				prueba.Errorf("decode productos: %v", errorDecodificacion)
				return
			}
			candado.Lock()
			recibidos += len(carga.Productos)
			candado.Unlock()
			escribirRespuestaJSON(respuesta, map[string]any{
				"total_recibidos":     len(carga.Productos),
				"precios_registrados": len(carga.Productos),
				"insertados":          len(carga.Productos),
				"actualizados":        0,
			})
		case peticion.Method == http.MethodPut && peticion.URL.Path == rutaBase+"/finalizar":
			var carga struct {
				Estado string `json:"estado"`
			}
			if errorDecodificacion := json.NewDecoder(peticion.Body).Decode(&carga); errorDecodificacion != nil {
				prueba.Errorf("decode finalizar: %v", errorDecodificacion)
				return
			}
			candado.Lock()
			finalizado = carga.Estado
			candado.Unlock()
			escribirRespuestaJSON(respuesta, map[string]any{"id": trabajoID, "estado": carga.Estado})
		default:
			http.NotFound(respuesta, peticion)
		}
	}))
	defer servidor.Close()

	registrador := slog.New(slog.NewTextHandler(io.Discard, nil))
	directorioEstado := prueba.TempDir()
	clienteAPI := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "test", 10*time.Second)
	almacen, errorApertura := cola.Open(trabajoID, directorioEstado, 1<<20)
	if errorApertura != nil {
		prueba.Fatalf("cola.Open() error = %v", errorApertura)
	}

	contexto := context.Background()
	pendientes := 0
	emitir := func(elemento dominio.Item) error {
		elemento = extraccion.CompleteItem(elemento)
		supermercado := elemento.Supermercado
		if errorAdicion := almacen.Add(elemento.Product, &supermercado); errorAdicion != nil {
			return errorAdicion
		}
		pendientes++
		if pendientes >= tamanoLote {
			if errorSellado := almacen.Seal(tamanoLote); errorSellado != nil {
				return errorSellado
			}
			if errorReenvio := almacen.Replay(contexto, clienteAPI); errorReenvio != nil {
				return errorReenvio
			}
			pendientes = 0
		}
		return nil
	}
	spider := spiderMemoria{elementos: cantidadElementos, compacto: compacto, registrador: registrador}
	if errorSpider := spider.Run(contexto, dominio.Job{WorkID: trabajoID}, emitir); errorSpider != nil {
		_ = almacen.Close()
		prueba.Fatalf("spider.Run() error = %v", errorSpider)
	}
	if errorProgreso := almacen.SetProgress("crawl_done", 1); errorProgreso != nil {
		_ = almacen.Close()
		prueba.Fatalf("SetProgress() error = %v", errorProgreso)
	}
	if errorSellado := almacen.Seal(tamanoLote); errorSellado != nil {
		_ = almacen.Close()
		prueba.Fatalf("Seal() error = %v", errorSellado)
	}
	if errorReenvio := almacen.Replay(contexto, clienteAPI); errorReenvio != nil {
		_ = almacen.Close()
		prueba.Fatalf("Replay() error = %v", errorReenvio)
	}
	conteoTotal, errorConteo := almacen.Count()
	if errorConteo != nil {
		_ = almacen.Close()
		prueba.Fatalf("Count() error = %v", errorConteo)
	}
	if conteoTotal != cantidadElementos {
		_ = almacen.Close()
		prueba.Fatalf("productos en spool = %d, esperado %d", conteoTotal, cantidadElementos)
	}
	estadoFinal, errorFinalizacion := clienteAPI.FinalizeWork(contexto, trabajoID, "completado", nil, nil, "")
	_ = almacen.Close()
	picoAnonMB := muestreadorAnon.Detener()
	if errorFinalizacion != nil {
		prueba.Fatalf("FinalizeWork() error = %v", errorFinalizacion)
	}
	if estadoFinal["estado"] != "completado" {
		prueba.Errorf("estado = %v", estadoFinal["estado"])
	}
	candado.Lock()
	entregados := recibidos
	estadoConfirmado := finalizado
	candado.Unlock()
	if estadoConfirmado != "completado" || entregados != cantidadElementos {
		prueba.Fatalf("entrega incompleta: estado=%q productos=%d, esperado %d", estadoConfirmado, entregados, cantidadElementos)
	}

	picoMB := picoRSSMB(prueba)
	limiteCgroup := leerFlotanteEntorno("SCRAPER_MEMORY_CGROUP_LIMIT_MB", 0)
	picoCgroup := 0.0
	if limiteCgroup > 0 {
		picoCgroup = picoCgroupMB()
	}
	resultado := resultadoGateMemoria{
		Items:         cantidadElementos,
		Compact:       compacto,
		LimitMB:       limiteMB,
		BaselineRSSMB: lineaBaseMB,
		PeakRSSMB:     picoMB,
		PeakAnonRSSMB: picoAnonMB,
		CgroupLimitMB: limiteCgroup,
		CgroupPeakMB:  picoCgroup,
		GoVersion:     runtime.Version(),
		MeasuredAt:    time.Now().UTC().Format(time.RFC3339),
	}
	prueba.Logf("gate de RAM: %d productos (compacto=%t), RSS pico %.1f MB, RssAnon pico %.1f MB, cgroup pico %.1f MB / límite %.1f MB (base %.1f MB, límite RSS %.1f MB)",
		cantidadElementos, compacto, picoMB, picoAnonMB, picoCgroup, limiteCgroup, lineaBaseMB, limiteMB)
	escribirResultadoGateMemoria(prueba, resultado)
	if picoMB > limiteMB || picoAnonMB > limiteMB {
		prueba.Fatalf("RAM pico fuera de límite: RSS %.1f MB, RssAnon %.1f MB, límite %.1f MB",
			picoMB, picoAnonMB, limiteMB)
	}
}

// ===== Reporte y Muestreo de Memoria =====

type resultadoGateMemoria struct {
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

func escribirResultadoGateMemoria(prueba *testing.T, resultado resultadoGateMemoria) {
	prueba.Helper()
	rutaDestino := strings.TrimSpace(os.Getenv("SCRAPER_MEMORY_RESULT"))
	if rutaDestino == "" {
		return
	}
	cuerpo, errorSerializacion := json.MarshalIndent(resultado, "", "  ")
	if errorSerializacion != nil {
		prueba.Fatalf("marshal resultado de RAM: %v", errorSerializacion)
	}
	cuerpo = append(cuerpo, '\n')
	if directorio := filepath.Dir(rutaDestino); directorio != "." {
		if errorDirectorio := os.MkdirAll(directorio, 0o755); errorDirectorio != nil {
			prueba.Fatalf("crear directorio para resultado de RAM: %v", errorDirectorio)
		}
	}
	if errorEscritura := os.WriteFile(rutaDestino, cuerpo, 0o644); errorEscritura != nil {
		prueba.Fatalf("escribir resultado de RAM en %s: %v", rutaDestino, errorEscritura)
	}
}

type muestreadorAnon struct {
	detener   chan struct{}
	terminado chan struct{}
	pico      atomic.Uint64
}

func iniciarMuestreadorAnon(intervalo time.Duration) *muestreadorAnon {
	muestreador := &muestreadorAnon{detener: make(chan struct{}), terminado: make(chan struct{})}
	go func() {
		defer close(muestreador.terminado)
		temporizador := time.NewTicker(intervalo)
		defer temporizador.Stop()
		for {
			if kilobytes, valido := campoEstadoKB("RssAnon"); valido {
				valorActual := uint64(kilobytes)
				for {
					previo := muestreador.pico.Load()
					if valorActual <= previo || muestreador.pico.CompareAndSwap(previo, valorActual) {
						break
					}
				}
			}
			select {
			case <-muestreador.detener:
				return
			case <-temporizador.C:
			}
		}
	}()
	return muestreador
}

func (muestreador *muestreadorAnon) Detener() float64 {
	close(muestreador.detener)
	<-muestreador.terminado
	return float64(muestreador.pico.Load()) / 1024
}

func campoEstadoKB(nombreCampo string) (float64, bool) {
	cuerpo, errorLectura := os.ReadFile("/proc/self/status")
	if errorLectura != nil {
		return 0, false
	}
	prefijo := nombreCampo + ":"
	for _, linea := range strings.Split(string(cuerpo), "\n") {
		if !strings.HasPrefix(linea, prefijo) {
			continue
		}
		campos := strings.Fields(linea)
		if len(campos) < 2 {
			return 0, false
		}
		kilobytes, errorParseo := strconv.ParseFloat(campos[1], 64)
		if errorParseo != nil {
			return 0, false
		}
		return kilobytes, true
	}
	return 0, false
}

func picoCgroupMB() float64 {
	for _, ruta := range []string{
		"/sys/fs/cgroup/memory.peak",
		"/sys/fs/cgroup/memory/memory.max_usage_in_bytes",
	} {
		cuerpo, errorLectura := os.ReadFile(ruta)
		if errorLectura != nil {
			continue
		}
		bytesLeidos, errorParseo := strconv.ParseFloat(strings.TrimSpace(string(cuerpo)), 64)
		if errorParseo != nil {
			continue
		}
		return bytesLeidos / (1024 * 1024)
	}
	return 0
}

// ===== Spider Sintético de Memoria =====

const nombreSpiderMemoria = "memory_rsc"

type spiderMemoria struct {
	elementos   int
	compacto    bool
	registrador *slog.Logger
}

func (espia spiderMemoria) Name() string {
	return nombreSpiderMemoria
}

func (espia spiderMemoria) Run(_ context.Context, _ dominio.Job, emitir extraccion.Emitter) error {
	productoBase := dominio.Product{
		Precio:       punteroFlotante(1990),
		PrecioNormal: punteroFlotante(2490),
		PrecioOferta: punteroFlotante(1990),
		Marca:        dominio.StringPtr("Marca de prueba"),
		FormatoCrudo: dominio.StringPtr("1 unidad"),
		EnStock:      dominio.BoolPtr(true),
		Categoria:    "despensa",
	}
	for indice := 0; indice < espia.elementos; indice++ {
		producto := productoBase
		if espia.compacto {
			// El gate de OOM corre sobre tmpfs para evitar miles de fsync
			// lentos en el volumen de Docker. Como tmpfs se cobra al cgroup,
			// se usa un payload mínimo; el gate del host mantiene el payload
			// realista.
			producto = dominio.Product{Producto: fmt.Sprintf("p%d", indice)}
		} else {
			producto.Producto = fmt.Sprintf("Producto de prueba con nombre largo %d", indice)
			producto.SKU = dominio.StringPtr(fmt.Sprintf("%013d", indice))
			producto.EanGtin = dominio.StringPtr(fmt.Sprintf("%013d", indice))
			producto.URLProducto = dominio.StringPtr(fmt.Sprintf("https://www.jumbo.cl/p/producto-%d", indice))
			producto.Imagen = dominio.StringPtr(fmt.Sprintf("https://www.jumbo.cl/img/%d.jpg", indice))
		}
		if errorEmision := emitir(dominio.Item{Product: producto, Supermercado: "Jumbo"}); errorEmision != nil {
			return errorEmision
		}
	}
	return nil
}

func picoRSSMB(prueba *testing.T) float64 {
	prueba.Helper()
	cuerpo, errorLectura := os.ReadFile("/proc/self/status")
	if errorLectura != nil {
		prueba.Fatalf("leer /proc/self/status: %v", errorLectura)
	}
	for _, linea := range strings.Split(string(cuerpo), "\n") {
		if !strings.HasPrefix(linea, "VmHWM:") {
			continue
		}
		campos := strings.Fields(linea)
		if len(campos) < 2 {
			break
		}
		kilobytes, errorParseo := strconv.ParseFloat(campos[1], 64)
		if errorParseo != nil {
			prueba.Fatalf("parsear VmHWM: %v", errorParseo)
		}
		return kilobytes / 1024
	}
	prueba.Fatal("VmHWM no encontrado en /proc/self/status")
	return 0
}

func leerEnteroEntorno(nombre string, predeterminado int) int {
	crudo := strings.TrimSpace(os.Getenv(nombre))
	if crudo == "" {
		return predeterminado
	}
	valor, errorParseo := strconv.Atoi(crudo)
	if errorParseo != nil {
		return predeterminado
	}
	return valor
}

func leerFlotanteEntorno(nombre string, predeterminado float64) float64 {
	crudo := strings.TrimSpace(os.Getenv(nombre))
	if crudo == "" {
		return predeterminado
	}
	valor, errorParseo := strconv.ParseFloat(crudo, 64)
	if errorParseo != nil {
		return predeterminado
	}
	return valor
}

func escribirRespuestaJSON(respuesta http.ResponseWriter, carga any) {
	respuesta.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(respuesta).Encode(carga)
}
