// cola_test.go verifica el flujo de Redis Streams (ACK, lease, dead-letter)
// y la recolección de spools terminales mediante Collector.Collect.
package cola_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	cola "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/cola"
)

// ===== Accesorios de Prueba Redis =====

type accesorioCola struct {
	cliente       *redis.Client
	colaTrabajos  *cola.Queue
	flujo         string
	indice        string
	colaMuerta    string
	prefijoCierre string
	trabajoID     string
}

// omitirSiRedisNoDisponible omite la prueba local si Redis no responde y REDIS_TEST_URL no fue exigida.
func omitirSiRedisNoDisponible(prueba *testing.T, urlCruda string, errorConexion error) {
	prueba.Helper()
	if os.Getenv("REDIS_TEST_URL") != "" {
		prueba.Fatalf("Redis configurado en REDIS_TEST_URL=%s no está disponible: %v", urlCruda, errorConexion)
	}
	prueba.Skipf("Redis no disponible en %s: %v", urlCruda, errorConexion)
}

// nuevoAccesorioCola inicializa un cliente Redis y una cola aislada para la prueba.
func nuevoAccesorioCola(prueba *testing.T, consumidor string) *accesorioCola {
	prueba.Helper()
	urlCruda := os.Getenv("REDIS_TEST_URL")
	if urlCruda == "" {
		urlCruda = "redis://localhost:6379/15"
	}
	opcionesRedis, errorParseo := redis.ParseURL(urlCruda)
	if errorParseo != nil {
		prueba.Fatalf("REDIS_TEST_URL inválida: %v", errorParseo)
	}
	opcionesRedis.DialTimeout = 500 * time.Millisecond
	opcionesRedis.ReadTimeout = 2 * time.Second
	opcionesRedis.WriteTimeout = 2 * time.Second
	cliente := redis.NewClient(opcionesRedis)

	contextoPing, cancelarPing := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelarPing()
	if errorPing := cliente.Ping(contextoPing).Err(); errorPing != nil {
		_ = cliente.Close()
		omitirSiRedisNoDisponible(prueba, urlCruda, errorPing)
	}

	prefijo := "scraper:test:" + uuid.NewString()
	accesorio := &accesorioCola{
		cliente:       cliente,
		flujo:         prefijo + ":stream",
		indice:        prefijo + ":index",
		colaMuerta:    prefijo + ":dead",
		prefijoCierre: prefijo + ":closed:",
		trabajoID:     uuid.NewString(),
	}
	var errorCola error
	accesorio.colaTrabajos, errorCola = cola.NewQueue(cliente, cola.Options{
		Stream:       accesorio.flujo,
		Group:        prefijo + ":group",
		Lease:        10 * time.Second,
		Consumer:     consumidor,
		IndexKey:     accesorio.indice,
		DeadLetter:   accesorio.colaMuerta,
		ClosedPrefix: accesorio.prefijoCierre,
	})
	if errorCola != nil {
		prueba.Fatalf("cola.NewQueue() error = %v", errorCola)
	}
	if errorGrupo := accesorio.colaTrabajos.EnsureGroup(context.Background()); errorGrupo != nil {
		prueba.Fatalf("EnsureGroup() error = %v", errorGrupo)
	}
	prueba.Cleanup(func() {
		contextoLimpieza, cancelarLimpieza := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelarLimpieza()
		_ = cliente.Del(contextoLimpieza,
			accesorio.flujo,
			accesorio.indice,
			accesorio.colaMuerta,
			accesorio.prefijoCierre+accesorio.trabajoID,
		).Err()
		_ = cliente.Close()
	})
	return accesorio
}

// agregarTrabajoParaID encola un mensaje en el stream y registra el trabajoID en el hash de índice.
func (accesorio *accesorioCola) agregarTrabajoParaID(prueba *testing.T, trabajoID, carga string) string {
	prueba.Helper()
	contexto, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelar()
	mensajeID, errorAdicion := accesorio.cliente.XAdd(contexto, &redis.XAddArgs{
		Stream: accesorio.flujo,
		Values: map[string]any{"payload": carga},
	}).Result()
	if errorAdicion != nil {
		prueba.Fatalf("XAdd() error = %v", errorAdicion)
	}
	if errorIndice := accesorio.cliente.HSet(contexto, accesorio.indice, trabajoID, mensajeID).Err(); errorIndice != nil {
		prueba.Fatalf("HSet() error = %v", errorIndice)
	}
	return mensajeID
}

// agregarTrabajo encola un mensaje para el trabajoID principal del accesorio.
func (accesorio *accesorioCola) agregarTrabajo(prueba *testing.T, carga string) string {
	prueba.Helper()
	return accesorio.agregarTrabajoParaID(prueba, accesorio.trabajoID, carga)
}

// ===== Propiedad de Lease, ACK y Dead-Letter =====

func TestAckOwnershipAndDeadLetter(prueba *testing.T) {
	accesorio := nuevoAccesorioCola(prueba, "consumer-a")
	contexto, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()

	cargaCruda := `{"trabajo_id":"` + accesorio.trabajoID + `","spider":"jumbo_rsc"}`
	mensajeID := accesorio.agregarTrabajo(prueba, cargaCruda)

	tieneMensaje, errorConsulta := accesorio.colaTrabajos.HasMessage(contexto, accesorio.trabajoID)
	if errorConsulta != nil || !tieneMensaje {
		prueba.Fatalf("HasMessage() = %v, %v; esperado true", tieneMensaje, errorConsulta)
	}
	mensaje, errorRecepcion := accesorio.colaTrabajos.Receive(contexto)
	if errorRecepcion != nil {
		prueba.Fatalf("Receive() error = %v", errorRecepcion)
	}
	if mensaje == nil || mensaje.ID != mensajeID {
		prueba.Fatalf("message = %+v, esperado %s", mensaje, mensajeID)
	}
	renovado, errorRenovacion := accesorio.colaTrabajos.Renew(contexto, mensajeID)
	if errorRenovacion != nil || !renovado {
		prueba.Fatalf("Renew() = %v, %v; esperado true", renovado, errorRenovacion)
	}

	otroConsumidor, errorOtro := cola.NewQueue(accesorio.cliente, cola.Options{
		Stream:       accesorio.flujo,
		Group:        accesorio.colaTrabajos.Group(),
		Lease:        10 * time.Second,
		Consumer:     "consumer-b",
		IndexKey:     accesorio.indice,
		DeadLetter:   accesorio.colaMuerta,
		ClosedPrefix: accesorio.prefijoCierre,
	})
	if errorOtro != nil {
		prueba.Fatalf("cola.NewQueue(otroConsumidor) error = %v", errorOtro)
	}
	if pudoRenovar, _ := otroConsumidor.Renew(contexto, mensajeID); pudoRenovar {
		prueba.Fatal("otro consumidor no debe poder renovar la lease")
	}
	if pudoConfirmar, _ := otroConsumidor.Ack(contexto, mensajeID, cargaCruda, accesorio.trabajoID, ""); pudoConfirmar {
		prueba.Fatal("otro consumidor no debe poder hacer ACK")
	}

	confirmado, errorAck := accesorio.colaTrabajos.Ack(contexto, mensajeID, cargaCruda, accesorio.trabajoID, "attempts_exhausted")
	if errorAck != nil || !confirmado {
		prueba.Fatalf("Ack() = %v, %v; esperado true", confirmado, errorAck)
	}
	if sigueEnIndice, _ := accesorio.colaTrabajos.HasMessage(contexto, accesorio.trabajoID); sigueEnIndice {
		prueba.Fatal("el índice debe limpiarse después del ACK")
	}
	if marcadorCierre, _ := accesorio.cliente.Exists(contexto, accesorio.prefijoCierre+accesorio.trabajoID).Result(); marcadorCierre != 1 {
		prueba.Errorf("marcador de cierre existe = %d, esperado 1", marcadorCierre)
	}
	if longitudFlujo, _ := accesorio.cliente.XLen(contexto, accesorio.flujo).Result(); longitudFlujo != 0 {
		prueba.Errorf("stream length = %d, esperado 0", longitudFlujo)
	}

	mensajesMuertos, errorRango := accesorio.cliente.XRange(contexto, accesorio.colaMuerta, "-", "+").Result()
	if errorRango != nil {
		prueba.Fatalf("XRange(dead) error = %v", errorRango)
	}
	if len(mensajesMuertos) != 1 {
		prueba.Fatalf("dead letter = %d mensajes, esperado 1", len(mensajesMuertos))
	}
	if mensajesMuertos[0].Values["reason"] != "attempts_exhausted" {
		prueba.Errorf("reason = %v", mensajesMuertos[0].Values["reason"])
	}
	if mensajesMuertos[0].Values["source_id"] != mensajeID {
		prueba.Errorf("source_id = %v, esperado %s", mensajesMuertos[0].Values["source_id"], mensajeID)
	}
	if mensajesMuertos[0].Values["payload"] != cargaCruda {
		prueba.Errorf("payload = %v", mensajesMuertos[0].Values["payload"])
	}
}

func TestAckSuccessDoesNotCreateDeadLetter(prueba *testing.T) {
	accesorio := nuevoAccesorioCola(prueba, "consumer-a")
	contexto, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()

	cargaCruda := `{"trabajo_id":"` + accesorio.trabajoID + `","spider":"jumbo_rsc"}`
	mensajeID := accesorio.agregarTrabajo(prueba, cargaCruda)
	mensaje, errorRecepcion := accesorio.colaTrabajos.Receive(contexto)
	if errorRecepcion != nil || mensaje == nil {
		prueba.Fatalf("Receive() = %+v, %v", mensaje, errorRecepcion)
	}
	confirmado, errorAck := accesorio.colaTrabajos.Ack(contexto, mensajeID, cargaCruda, accesorio.trabajoID, "")
	if errorAck != nil || !confirmado {
		prueba.Fatalf("Ack() = %v, %v; esperado true", confirmado, errorAck)
	}
	if existeColaMuerta, _ := accesorio.cliente.Exists(contexto, accesorio.colaMuerta).Result(); existeColaMuerta != 0 {
		prueba.Errorf("dead letter existe = %d, esperado 0", existeColaMuerta)
	}
	if marcadorCierre, _ := accesorio.cliente.Exists(contexto, accesorio.prefijoCierre+accesorio.trabajoID).Result(); marcadorCierre != 1 {
		prueba.Errorf("marcador de cierre existe = %d, esperado 1", marcadorCierre)
	}
}

func TestRejectsShortLease(prueba *testing.T) {
	cliente := redis.NewClient(&redis.Options{Addr: "localhost:0"})
	defer cliente.Close()
	if _, errorCola := cola.NewQueue(cliente, cola.Options{Lease: time.Second}); errorCola == nil {
		prueba.Fatal("NewQueue() debió rechazar una lease menor a 10 s")
	}
}

// ===== Recolección de Spools Terminales (Collector.Collect) =====

// crearSpoolDePrueba abre y cierra un spool válido en directorioEstado para un trabajoID.
func crearSpoolDePrueba(prueba *testing.T, directorioEstado string) string {
	prueba.Helper()
	trabajoID := uuid.NewString()
	almacen, errorApertura := cola.Open(trabajoID, directorioEstado, 1)
	if errorApertura != nil {
		prueba.Fatalf("Open(%s) error = %v", trabajoID, errorApertura)
	}
	if errorCierre := almacen.Close(); errorCierre != nil {
		prueba.Fatalf("Close(%s) error = %v", trabajoID, errorCierre)
	}
	return trabajoID
}

// existeDirectorioSpool informa si la carpeta del spool sigue presente en disco.
func existeDirectorioSpool(directorioEstado, trabajoID string) bool {
	_, errorEstadistica := os.Stat(filepath.Join(directorioEstado, trabajoID))
	return errorEstadistica == nil
}

// registradorSilenciosoCola devuelve un logger que descarta la salida durante los tests.
func registradorSilenciosoCola() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCollectorCollectConservaEnProgresoYAnteErrorDeAPI(prueba *testing.T) {
	directorioEstado := prueba.TempDir()
	trabajoEnProgreso := crearSpoolDePrueba(prueba, directorioEstado)

	fallarAPI := false
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
		if fallarAPI {
			respuesta.WriteHeader(http.StatusServiceUnavailable)
			_, _ = respuesta.Write([]byte(`{"detail":"control caido"}`))
			return
		}
		respuesta.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(respuesta).Encode(map[string]any{
			"id":     trabajoEnProgreso,
			"estado": "en_progreso",
		})
	}))
	defer servidor.Close()

	clienteControl := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "token", time.Second)
	recolector := cola.NewCollector(directorioEstado, 1, time.Hour, 5*time.Second, registradorSilenciosoCola())

	// Caso 3: Conserva spools de trabajos en_progreso.
	eliminados, errorRecoleccion := recolector.Collect(context.Background(), clienteControl, nil, 10)
	if errorRecoleccion != nil || eliminados != 0 {
		prueba.Fatalf("Collect(en_progreso) = %d, %v; esperado 0, nil", eliminados, errorRecoleccion)
	}
	if !existeDirectorioSpool(directorioEstado, trabajoEnProgreso) {
		prueba.Fatal("el spool en_progreso fue eliminado indebidamente")
	}

	// Caso 5: Si api.GetWork devuelve error, conserva todos los archivos y propaga el error.
	fallarAPI = true
	eliminados, errorRecoleccion = recolector.Collect(context.Background(), clienteControl, nil, 10)
	if errorRecoleccion == nil || eliminados != 0 {
		prueba.Fatalf("Collect(error API) = %d, %v; esperado 0 y error", eliminados, errorRecoleccion)
	}
	if !existeDirectorioSpool(directorioEstado, trabajoEnProgreso) {
		prueba.Fatal("el spool fue eliminado cuando api.GetWork falló")
	}
}

func TestCollectorCollectCompletadosYFallidos(prueba *testing.T) {
	accesorio := nuevoAccesorioCola(prueba, "consumer-collector")
	directorioEstado := prueba.TempDir()

	trabajoCompletadoLibre := crearSpoolDePrueba(prueba, directorioEstado)
	trabajoCompletadoPendiente := crearSpoolDePrueba(prueba, directorioEstado)
	trabajoFallidoReciente := crearSpoolDePrueba(prueba, directorioEstado)
	trabajoFallidoVencido := crearSpoolDePrueba(prueba, directorioEstado)

	// El trabajoCompletadoPendiente aún tiene mensaje en el índice de Redis.
	accesorio.agregarTrabajoParaID(prueba, trabajoCompletadoPendiente, `{"trabajo_id":"`+trabajoCompletadoPendiente+`"}`)

	ahora := time.Now().UTC()
	estadosPorTrabajo := map[string]map[string]any{
		trabajoCompletadoLibre: {
			"id":     trabajoCompletadoLibre,
			"estado": "completado",
		},
		trabajoCompletadoPendiente: {
			"id":     trabajoCompletadoPendiente,
			"estado": "completado",
		},
		trabajoFallidoReciente: {
			"id":            trabajoFallidoReciente,
			"estado":        "fallido",
			"finalizado_el": ahora.Add(-5 * time.Minute).Format(time.RFC3339),
		},
		trabajoFallidoVencido: {
			"id":            trabajoFallidoVencido,
			"estado":        "fallido",
			"finalizado_el": ahora.Add(-2 * time.Hour).Format(time.RFC3339),
		},
	}

	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		identificador := filepath.Base(peticion.URL.Path)
		estado, existe := estadosPorTrabajo[identificador]
		if !existe {
			http.NotFound(respuesta, peticion)
			return
		}
		respuesta.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(respuesta).Encode(estado)
	}))
	defer servidor.Close()

	clienteControl := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "token", time.Second)

	// Pasada sin presión de disco y retención de 1 hora:
	// - Caso 1: elimina completado sin mensaje en cola (trabajoCompletadoLibre).
	// - Caso 2: conserva completado con HasMessage == true (trabajoCompletadoPendiente).
	// - Caso 4a: conserva fallido dentro de la ventana de retención (trabajoFallidoReciente)
	//            y elimina fallido cuya retención ya venció (trabajoFallidoVencido).
	recolectorSinPresion := cola.NewCollector(directorioEstado, 1, time.Hour, 5*time.Second, registradorSilenciosoCola())
	eliminados, errorRecoleccion := recolectorSinPresion.Collect(context.Background(), clienteControl, accesorio.colaTrabajos, 10)
	if errorRecoleccion != nil {
		prueba.Fatalf("Collect(sin presión) error = %v", errorRecoleccion)
	}
	if eliminados != 2 {
		prueba.Fatalf("eliminados = %d, esperado 2 (completado libre + fallido vencido)", eliminados)
	}
	if existeDirectorioSpool(directorioEstado, trabajoCompletadoLibre) {
		prueba.Error("trabajoCompletadoLibre debió eliminarse")
	}
	if !existeDirectorioSpool(directorioEstado, trabajoCompletadoPendiente) {
		prueba.Error("trabajoCompletadoPendiente debió conservarse porque sigue en Redis")
	}
	if !existeDirectorioSpool(directorioEstado, trabajoFallidoReciente) {
		prueba.Error("trabajoFallidoReciente debió conservarse dentro de la ventana de retención")
	}
	if existeDirectorioSpool(directorioEstado, trabajoFallidoVencido) {
		prueba.Error("trabajoFallidoVencido debió eliminarse tras vencer la retención")
	}

	// Caso 4b: Bajo presión de disco (minFree mayor al espacio disponible), el fallido reciente sí se elimina.
	recolectorConPresion := cola.NewCollector(directorioEstado, 1<<62, time.Hour, 5*time.Second, registradorSilenciosoCola())
	eliminadosConPresion, errorPresion := recolectorConPresion.Collect(context.Background(), clienteControl, accesorio.colaTrabajos, 10)
	if errorPresion != nil {
		prueba.Fatalf("Collect(con presión) error = %v", errorPresion)
	}
	if eliminadosConPresion != 1 {
		prueba.Fatalf("eliminados bajo presión = %d, esperado 1 (trabajoFallidoReciente)", eliminadosConPresion)
	}
	if existeDirectorioSpool(directorioEstado, trabajoFallidoReciente) {
		prueba.Error("trabajoFallidoReciente debió eliminarse bajo presión de disco")
	}
	if !existeDirectorioSpool(directorioEstado, trabajoCompletadoPendiente) {
		prueba.Error("trabajoCompletadoPendiente nunca debe eliminarse mientras tenga mensaje en Redis")
	}
}
