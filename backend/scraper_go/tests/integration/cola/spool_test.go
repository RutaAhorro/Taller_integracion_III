// spool_test.go verifica el almacenamiento SQLite por trabajo: huellas de
// deduplicación, sellado por supermercado, persistencia, reenvío idempotente
// y rechazo ante presión de disco.
package cola_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	cola "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/cola"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
)

// ===== Auxiliares de Prueba para Spool =====

func punteroFlotante(valor float64) *float64 {
	return &valor
}

func reciboIngesta(cantidad int) map[string]any {
	return map[string]any{
		"total_recibidos":     cantidad,
		"precios_registrados": cantidad,
		"insertados":          cantidad,
		"actualizados":        0,
	}
}

// ===== Sellado y Reenvío =====

func TestSealAndReplay(prueba *testing.T) {
	trabajoID := uuid.NewString()
	almacen, errorApertura := cola.Open(trabajoID, prueba.TempDir(), 1<<20)
	if errorApertura != nil {
		prueba.Fatalf("Open() error = %v", errorApertura)
	}
	defer almacen.Close()

	supermercado := "Jumbo"
	leche := dominio.Product{Producto: "Leche", Categoria: "lacteos", Precio: punteroFlotante(1290)}
	pan := dominio.Product{Producto: "Pan", Categoria: "panaderia"}
	if errorAdicion := almacen.Add(leche, &supermercado); errorAdicion != nil {
		prueba.Fatalf("Add(leche) error = %v", errorAdicion)
	}
	if errorDuplicado := almacen.Add(leche, &supermercado); errorDuplicado != nil {
		prueba.Fatalf("Add(leche duplicado) error = %v", errorDuplicado)
	}
	if errorPan := almacen.Add(pan, &supermercado); errorPan != nil {
		prueba.Fatalf("Add(pan) error = %v", errorPan)
	}
	if cantidad, errorConteo := almacen.Count(); errorConteo != nil || cantidad != 2 {
		prueba.Fatalf("Count() = %d, %v; esperado 2 (la huella evita duplicados)", cantidad, errorConteo)
	}
	if errorSelladoCero := almacen.Seal(0); errorSelladoCero == nil {
		prueba.Fatal("Seal(0) debió fallar")
	}
	if errorSellado := almacen.Seal(1); errorSellado != nil {
		prueba.Fatalf("Seal() error = %v", errorSellado)
	}

	var llamadas int
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		llamadas++
		var carga struct {
			Productos    []dominio.Product `json:"productos"`
			Supermercado string            `json:"supermercado"`
		}
		if errorDecodificacion := json.NewDecoder(peticion.Body).Decode(&carga); errorDecodificacion != nil {
			prueba.Errorf("decode payload: %v", errorDecodificacion)
		}
		if carga.Supermercado != "Jumbo" || len(carga.Productos) != 1 {
			prueba.Errorf("payload = %+v", carga)
		}
		respuesta.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(respuesta).Encode(reciboIngesta(1))
	}))
	defer servidor.Close()

	cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
	if errorReenvio := almacen.Replay(context.Background(), cliente); errorReenvio != nil {
		prueba.Fatalf("Replay() error = %v", errorReenvio)
	}
	if llamadas != 2 {
		prueba.Fatalf("calls = %d, esperado 2 (un lote por producto)", llamadas)
	}
	if errorSegundoReenvio := almacen.Replay(context.Background(), cliente); errorSegundoReenvio != nil {
		prueba.Fatalf("segundo Replay() error = %v", errorSegundoReenvio)
	}
	if llamadas != 2 {
		prueba.Fatalf("calls tras reenviar = %d, esperado 2 (los lotes ya se entregaron)", llamadas)
	}

	if errorProgreso := almacen.SetProgress("crawl_attempts", 2); errorProgreso != nil {
		prueba.Fatalf("SetProgress() error = %v", errorProgreso)
	}
	intentos, errorLectura := almacen.Progress("crawl_attempts")
	if errorLectura != nil || intentos != 2 {
		prueba.Fatalf("Progress() = %d, %v", intentos, errorLectura)
	}
	if inexistente, _ := almacen.Progress("crawl_done"); inexistente != 0 {
		prueba.Fatalf("crawl_done = %d, esperado 0", inexistente)
	}
}

func TestSealNoMezclaSupermercadosYRechazaLoteMayorAlMaximo(prueba *testing.T) {
	trabajoID := uuid.NewString()
	almacen, errorApertura := cola.Open(trabajoID, prueba.TempDir(), 1<<20)
	if errorApertura != nil {
		prueba.Fatalf("Open() error = %v", errorApertura)
	}
	defer almacen.Close()

	jumbo := "Jumbo"
	lider := "Lider"
	if errorAdicion := almacen.Add(dominio.Product{Producto: "Arroz Jumbo", Precio: punteroFlotante(1500)}, &jumbo); errorAdicion != nil {
		prueba.Fatalf("Add(Jumbo) error = %v", errorAdicion)
	}
	if errorAdicion := almacen.Add(dominio.Product{Producto: "Arroz Lider", Precio: punteroFlotante(1400)}, &lider); errorAdicion != nil {
		prueba.Fatalf("Add(Lider) error = %v", errorAdicion)
	}

	if errorExceso := almacen.Seal(config.MaxBatchSize + 1); errorExceso == nil {
		prueba.Fatalf("Seal(%d) debió fallar por superar el máximo permitido", config.MaxBatchSize+1)
	}
	if errorSellado := almacen.Seal(config.MaxBatchSize); errorSellado != nil {
		prueba.Fatalf("Seal(%d) error = %v", config.MaxBatchSize, errorSellado)
	}

	var mercadosRecibidos []string
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		var carga struct {
			Productos    []dominio.Product `json:"productos"`
			Supermercado string            `json:"supermercado"`
		}
		if errorDecodificacion := json.NewDecoder(peticion.Body).Decode(&carga); errorDecodificacion != nil {
			prueba.Errorf("decode payload: %v", errorDecodificacion)
		}
		if len(carga.Productos) != 1 {
			prueba.Errorf("cada lote debe contener solo 1 supermercado; productos = %d", len(carga.Productos))
		}
		mercadosRecibidos = append(mercadosRecibidos, carga.Supermercado)
		respuesta.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(respuesta).Encode(reciboIngesta(len(carga.Productos)))
	}))
	defer servidor.Close()

	cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
	if errorReenvio := almacen.Replay(context.Background(), cliente); errorReenvio != nil {
		prueba.Fatalf("Replay() error = %v", errorReenvio)
	}
	if len(mercadosRecibidos) != 2 || mercadosRecibidos[0] != "Jumbo" || mercadosRecibidos[1] != "Lider" {
		prueba.Fatalf("lotes recibidos = %v, esperado [Jumbo Lider]", mercadosRecibidos)
	}
}

func TestSpoolPersisteTrasCerrarYReabrir(prueba *testing.T) {
	trabajoID := uuid.NewString()
	directorioEstado := prueba.TempDir()

	almacenInicial, errorApertura := cola.Open(trabajoID, directorioEstado, 1<<20)
	if errorApertura != nil {
		prueba.Fatalf("Open(inicial) error = %v", errorApertura)
	}
	supermercado := "Santa Isabel"
	if errorAdicion := almacenInicial.Add(dominio.Product{Producto: "Café", Precio: punteroFlotante(2990)}, &supermercado); errorAdicion != nil {
		prueba.Fatalf("Add() error = %v", errorAdicion)
	}
	if errorProgreso := almacenInicial.SetProgress("crawl_done", 1); errorProgreso != nil {
		prueba.Fatalf("SetProgress() error = %v", errorProgreso)
	}
	if errorSellado := almacenInicial.Seal(10); errorSellado != nil {
		prueba.Fatalf("Seal() error = %v", errorSellado)
	}
	if errorCierre := almacenInicial.Close(); errorCierre != nil {
		prueba.Fatalf("Close(inicial) error = %v", errorCierre)
	}

	almacenReabierto, errorReapertura := cola.Open(trabajoID, directorioEstado, 1<<20)
	if errorReapertura != nil {
		prueba.Fatalf("Open(reabierto) error = %v", errorReapertura)
	}
	defer almacenReabierto.Close()

	if cantidad, errorConteo := almacenReabierto.Count(); errorConteo != nil || cantidad != 1 {
		prueba.Fatalf("Count() tras reabrir = %d, %v; esperado 1", cantidad, errorConteo)
	}
	if progreso, errorProgreso := almacenReabierto.Progress("crawl_done"); errorProgreso != nil || progreso != 1 {
		prueba.Fatalf("Progress(crawl_done) tras reabrir = %d, %v; esperado 1", progreso, errorProgreso)
	}

	var lotesEntregados int
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
		lotesEntregados++
		respuesta.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(respuesta).Encode(reciboIngesta(1))
	}))
	defer servidor.Close()

	cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
	if errorReenvio := almacenReabierto.Replay(context.Background(), cliente); errorReenvio != nil {
		prueba.Fatalf("Replay() tras reabrir error = %v", errorReenvio)
	}
	if lotesEntregados != 1 {
		prueba.Fatalf("lotes entregados tras reabrir = %d, esperado 1", lotesEntregados)
	}
}

// TestSpoolPresionDeDiscoEnOpen comprueba que Open rechace el spool cuando la
// reserva pedida supera cualquier volumen. Usa una reserva imposible para no
// depender del espacio libre real del disco, que cambia mientras corren otros
// procesos (SUP-395).
func TestSpoolPresionDeDiscoEnOpen(prueba *testing.T) {
	const reservaImposible = int64(1) << 62
	if _, errorOpen := cola.Open(uuid.NewString(), prueba.TempDir(), reservaImposible); !errors.Is(errorOpen, cola.ErrStoragePressure) {
		prueba.Fatalf("Open() bajo presión = %v, esperado ErrStoragePressure", errorOpen)
	}
}

// ===== Idempotencia y Cancelación en Replay =====

func TestReplayRetriesSamePayloadAndIdempotencyKey(prueba *testing.T) {
	trabajoID := uuid.NewString()
	almacen, errorApertura := cola.Open(trabajoID, prueba.TempDir(), 1<<20)
	if errorApertura != nil {
		prueba.Fatalf("Open() error = %v", errorApertura)
	}
	defer almacen.Close()
	supermercado := "Jumbo"
	if errorAdicion := almacen.Add(dominio.Product{Producto: "Leche", Precio: punteroFlotante(1290)}, &supermercado); errorAdicion != nil {
		prueba.Fatalf("Add() error = %v", errorAdicion)
	}
	if errorSellado := almacen.Seal(100); errorSellado != nil {
		prueba.Fatalf("Seal() error = %v", errorSellado)
	}

	var (
		llamadas int
		claves   []string
		cuerpos  []string
	)
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		llamadas++
		claves = append(claves, peticion.Header.Get("Idempotency-Key"))
		var carga map[string]any
		if errorDecodificacion := json.NewDecoder(peticion.Body).Decode(&carga); errorDecodificacion != nil {
			prueba.Errorf("decode payload: %v", errorDecodificacion)
		}
		codificado, _ := json.Marshal(carga)
		cuerpos = append(cuerpos, string(codificado))
		if llamadas == 1 {
			respuesta.WriteHeader(http.StatusServiceUnavailable)
			_, _ = respuesta.Write([]byte(`{"codigo":"catalogo_no_disponible"}`))
			return
		}
		respuesta.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(respuesta).Encode(reciboIngesta(1))
	}))
	defer servidor.Close()

	cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
	if errorPrimerReenvio := almacen.Replay(context.Background(), cliente); errorPrimerReenvio == nil {
		prueba.Fatal("el primer Replay debió fallar con 503")
	}
	if errorSegundoReenvio := almacen.Replay(context.Background(), cliente); errorSegundoReenvio != nil {
		prueba.Fatalf("el segundo Replay debe recuperar: %v", errorSegundoReenvio)
	}
	if llamadas != 2 {
		prueba.Fatalf("calls = %d, esperado 2", llamadas)
	}
	if claves[0] == "" || claves[0] != claves[1] {
		prueba.Errorf("Idempotency-Key cambió entre reintentos: %q vs %q", claves[0], claves[1])
	}
	if cuerpos[0] != cuerpos[1] {
		prueba.Errorf("payload cambió entre reintentos:\n%s\n%s", cuerpos[0], cuerpos[1])
	}
	if errorTercerReenvio := almacen.Replay(context.Background(), cliente); errorTercerReenvio != nil {
		prueba.Fatalf("tercer Replay() error = %v", errorTercerReenvio)
	}
	if llamadas != 2 {
		prueba.Fatalf("calls tras el tercer Replay = %d, esperado 2 (lote ya entregado)", llamadas)
	}
}

func TestReplayDoesNotSendWithCancelledContext(prueba *testing.T) {
	trabajoID := uuid.NewString()
	almacen, errorApertura := cola.Open(trabajoID, prueba.TempDir(), 1<<20)
	if errorApertura != nil {
		prueba.Fatalf("Open() error = %v", errorApertura)
	}
	defer almacen.Close()
	supermercado := "Jumbo"
	if errorAdicion := almacen.Add(dominio.Product{Producto: "Leche"}, &supermercado); errorAdicion != nil {
		prueba.Fatalf("Add() error = %v", errorAdicion)
	}
	if errorSellado := almacen.Seal(100); errorSellado != nil {
		prueba.Fatalf("Seal() error = %v", errorSellado)
	}

	llamado := false
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
		llamado = true
		http.Error(respuesta, "no debe llamarse", http.StatusInternalServerError)
	}))
	defer servidor.Close()

	contexto, cancelar := context.WithCancel(context.Background())
	cancelar()
	cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
	if errorReenvio := almacen.Replay(contexto, cliente); errorReenvio == nil {
		prueba.Fatal("Replay() debió respetar el contexto cancelado")
	}
	if llamado {
		prueba.Fatal("Replay() envió un lote con la lease cancelada")
	}
}

func TestOpenRejectsInvalidWorkID(prueba *testing.T) {
	if _, errorApertura := cola.Open("no-es-uuid", prueba.TempDir(), 1<<20); errorApertura == nil {
		prueba.Fatal("Open() debió rechazar un trabajo_id inválido")
	}
}
