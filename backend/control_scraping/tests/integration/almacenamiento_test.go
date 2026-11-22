// Pruebas contra PostgreSQL y Redis reales y desechables, con el rol de
// runtime app_scraping. Se omiten si no se define SCRAPING_TEST_DB_URL; nunca
// deben apuntar a una base o un Redis compartidos.
package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/repositories"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ===== Preparación =====

// plazoPublicacionEsperada cubre un ciclo de recuperación de ejecuciones (5 s).
const plazoPublicacionEsperada = 8 * time.Second

// abrirBasePrueba abre la base aislada y verifica tablas y permisos del rol.
func abrirBasePrueba(prueba *testing.T) *gorm.DB {
	prueba.Helper()
	cadenaConexion := os.Getenv("SCRAPING_TEST_DB_URL")
	if cadenaConexion == "" {
		prueba.Skip("requiere SCRAPING_TEST_DB_URL hacia una base desechable con el rol app_scraping")
	}
	contexto, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	baseDatos, errorObtenido := repositories.Abrir(contexto, cadenaConexion)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	prueba.Cleanup(func() { repositories.Cerrar(baseDatos) })
	if errorObtenido := repositories.VerificarAlmacenamiento(contexto, baseDatos); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	return baseDatos
}

// catalogoFalso simula la API interna de Catálogo y cuenta las ingestas.
func catalogoFalso(prueba *testing.T, ingestas *atomic.Int32) *httptest.Server {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		switch peticion.URL.Path {
		case "/internal/v1/cadenas/resolver":
			_, _ = io.WriteString(respuesta, `{"id":1,"nombre":"Jumbo","esta_activa":true}`)
		case "/internal/v1/sucursales/resolver":
			_, _ = io.WriteString(respuesta, `{"id":10,"cadena_id":1}`)
		case "/internal/v1/ingestas":
			ingestas.Add(1)
			var carga struct {
				LoteID    string `json:"lote_id"`
				Productos []any  `json:"productos"`
			}
			_ = json.NewDecoder(peticion.Body).Decode(&carga)
			if _, errorObtenido := uuid.Parse(carga.LoteID); errorObtenido != nil {
				prueba.Errorf("lote_id ausente: %q", carga.LoteID)
			}
			_ = json.NewEncoder(respuesta).Encode(map[string]int{"sucursal_id": 10, "total_recibidos": len(carga.Productos), "insertados": len(carga.Productos), "precios_registrados": len(carga.Productos)})
		default:
			respuesta.WriteHeader(http.StatusNotFound)
		}
	}))
	prueba.Cleanup(servidor.Close)
	return servidor
}

// decodificarJSON lee el cuerpo de una respuesta como mapa.
func decodificarJSON(prueba *testing.T, cuerpo []byte) map[string]any {
	prueba.Helper()
	var resultado map[string]any
	if errorObtenido := json.Unmarshal(cuerpo, &resultado); errorObtenido != nil {
		prueba.Fatalf("JSON inválido: %s", cuerpo)
	}
	return resultado
}

// ===== Flujo durable completo =====

// TestFlujoDurableConSQLyRedis recorre el contrato con Scraper Go: crear,
// ejecutar (outbox → stream), ingerir con clave estable, reenviar y cerrar.
func TestFlujoDurableConSQLyRedis(prueba *testing.T) {
	baseDatos := abrirBasePrueba(prueba)
	direccionRedis := os.Getenv("SCRAPING_TEST_REDIS_URL")
	if direccionRedis == "" {
		prueba.Skip("requiere SCRAPING_TEST_REDIS_URL hacia un Redis desechable")
	}
	clienteRedis := repositories.NuevoClienteRedis(direccionRedis)
	defer clienteRedis.Close()
	var ingestas atomic.Int32
	configuracion := configuracionPrueba(false)
	configuracion.URLCatalogo = catalogoFalso(prueba, &ingestas).URL
	enrutador, recuperacion := api.Componer(configuracion, baseDatos, clienteRedis)
	contexto, cancelar := context.WithCancel(context.Background())
	recuperacion.Iniciar(contexto)
	defer func() {
		cancelar()
		contextoEspera, cancelarEspera := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelarEspera()
		if errorObtenido := recuperacion.Esperar(contextoEspera); errorObtenido != nil {
			prueba.Errorf("los ciclos no terminaron: %v", errorObtenido)
		}
	}()

	creado := servir(enrutador, http.MethodPost, "/api/v1/scraper/trabajos", `{"cadena_id":1}`)
	if creado.Code != http.StatusCreated {
		prueba.Fatalf("crear: %d %s", creado.Code, creado.Body)
	}
	trabajoID := decodificarJSON(prueba, creado.Body.Bytes())["id"].(string)
	ruta := "/api/v1/scraper/trabajos/" + trabajoID

	if ejecucion := servir(enrutador, http.MethodPost, ruta+"/ejecutar", `{"spider":"jumbo_rsc","producto_url":"https://x.example/p"}`); ejecucion.Code != http.StatusAccepted {
		prueba.Fatalf("ejecutar: %d %s", ejecucion.Code, ejecucion.Body)
	}
	if repetida := servir(enrutador, http.MethodPost, ruta+"/ejecutar", `{"spider":"lider_rsc"}`); repetida.Code != http.StatusConflict {
		prueba.Fatalf("otra ejecución del mismo trabajo debe ser conflicto: %d", repetida.Code)
	}
	esperarEnStream(prueba, clienteRedis, trabajoID)

	clave := uuid.NewString()
	lote := `{"productos":[{"sku":"A","producto":"Arroz","precio_normal":1000},{"sku":"B","producto":"Leche","precio_normal":900}]}`
	for intento := 1; intento <= 2; intento++ {
		peticion := httptest.NewRequest(http.MethodPost, ruta+"/productos", strings.NewReader(lote))
		peticion.Header.Set("Content-Type", "application/json")
		peticion.Header.Set("Authorization", "Bearer "+tokenServicio)
		peticion.Header.Set("Idempotency-Key", clave)
		respuesta := httptest.NewRecorder()
		enrutador.ServeHTTP(respuesta, peticion)
		if respuesta.Code != http.StatusOK || decodificarJSON(prueba, respuesta.Body.Bytes())["insertados"].(float64) != 2 {
			prueba.Fatalf("ingesta %d: %d %s", intento, respuesta.Code, respuesta.Body)
		}
	}
	if ingestas.Load() != 1 {
		prueba.Fatalf("el reenvío con la misma clave llegó a Catálogo: %d ingestas", ingestas.Load())
	}
	cierre := servir(enrutador, http.MethodPut, ruta+"/finalizar", `{"estado":"completado","elementos_extraidos":999}`)
	cuerpoCierre := decodificarJSON(prueba, cierre.Body.Bytes())
	if cierre.Code != http.StatusOK || cuerpoCierre["elementos_extraidos"].(float64) != 2 || cuerpoCierre["cadena_nombre"] != "Jumbo" {
		prueba.Fatalf("el ledger debe prevalecer sobre el contador del worker: %d %s", cierre.Code, cierre.Body)
	}
	var outboxRestante int64
	if errorObtenido := baseDatos.Raw(`SELECT count(*) FROM scraper.entregas_trabajos WHERE trabajo_id = ?`, trabajoID).Scan(&outboxRestante).Error; errorObtenido != nil || outboxRestante != 0 {
		prueba.Fatalf("el cierre debe retirar el outbox: %d %v", outboxRestante, errorObtenido)
	}
	if tardio := servir(enrutador, http.MethodPost, ruta+"/productos", strings.Replace(lote, `"A"`, `"C"`, 1)); tardio.Code != http.StatusConflict {
		prueba.Fatalf("un lote nuevo en un trabajo cerrado debe ser conflicto: %d", tardio.Code)
	}
}

// esperarEnStream espera la entrada del trabajo en el stream con el campo
// payload que lee Scraper Go.
func esperarEnStream(prueba *testing.T, clienteRedis *redis.Client, trabajoID string) {
	prueba.Helper()
	limite := time.Now().Add(plazoPublicacionEsperada)
	for time.Now().Before(limite) {
		entradas, _ := clienteRedis.XRange(context.Background(), "scraper:jobs:stream", "-", "+").Result()
		for _, entrada := range entradas {
			texto, _ := entrada.Values["payload"].(string)
			if strings.Contains(texto, trabajoID) {
				carga := decodificarJSON(prueba, []byte(texto))
				if carga["spider"] != "jumbo_rsc" || carga["product_url"] != "https://x.example/p" {
					prueba.Fatalf("payload publicado inesperado: %s", texto)
				}
				if indice, _ := clienteRedis.HGet(context.Background(), "scraper:jobs:index", trabajoID).Result(); indice != entrada.ID {
					prueba.Fatalf("índice=%q entrada=%q", indice, entrada.ID)
				}
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	prueba.Fatal("la ejecución no llegó a Redis Streams")
}

// ===== Redis bloqueado =====

// TestRedisBloqueadoNoImpideCerrar finaliza un trabajo por SQL mientras el
// publicador está bloqueado en un Redis que nunca responde.
func TestRedisBloqueadoNoImpideCerrar(prueba *testing.T) {
	baseDatos := abrirBasePrueba(prueba)
	var ingestas atomic.Int32
	configuracion := configuracionPrueba(false)
	configuracion.URLCatalogo = catalogoFalso(prueba, &ingestas).URL

	servidorTCP, errorObtenido := net.Listen("tcp", "127.0.0.1:0")
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	defer servidorTCP.Close()
	conexionAceptada, liberarConexion := make(chan struct{}), make(chan struct{})
	defer close(liberarConexion)
	go func() {
		conexion, errorAceptacion := servidorTCP.Accept()
		if errorAceptacion != nil {
			return
		}
		defer conexion.Close()
		if _, errorLectura := io.ReadFull(conexion, make([]byte, 1)); errorLectura != nil {
			return
		}
		close(conexionAceptada)
		<-liberarConexion
	}()
	clienteRedis := redis.NewClient(&redis.Options{Addr: servidorTCP.Addr().String()})
	defer clienteRedis.Close()
	enrutador, recuperacion := api.Componer(configuracion, baseDatos, clienteRedis)
	contexto, cancelar := context.WithCancel(context.Background())
	recuperacion.Iniciar(contexto)
	defer func() {
		cancelar()
		contextoEspera, cancelarEspera := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelarEspera()
		if errorEspera := recuperacion.Esperar(contextoEspera); errorEspera != nil {
			prueba.Errorf("los ciclos no terminaron: %v", errorEspera)
		}
	}()

	creado := servir(enrutador, http.MethodPost, "/api/v1/scraper/trabajos", `{"cadena_id":1}`)
	ruta := "/api/v1/scraper/trabajos/" + decodificarJSON(prueba, creado.Body.Bytes())["id"].(string)
	if ejecucion := servir(enrutador, http.MethodPost, ruta+"/ejecutar", `{"spider":"jumbo_rsc"}`); ejecucion.Code != http.StatusAccepted {
		prueba.Fatalf("no se aceptó la ejecución: %d %s", ejecucion.Code, ejecucion.Body)
	}
	select {
	case <-conexionAceptada:
	case <-time.After(plazoPublicacionEsperada):
		prueba.Fatal("el publicador no llegó a Redis")
	}
	contextoCierre, cancelarCierre := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancelarCierre()
	peticion := httptest.NewRequest(http.MethodPut, ruta+"/finalizar", strings.NewReader(`{"estado":"fallido"}`)).WithContext(contextoCierre)
	peticion.Header.Set("Content-Type", "application/json")
	peticion.Header.Set("Authorization", "Bearer "+tokenServicio)
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, peticion)
	if respuesta.Code != http.StatusOK {
		prueba.Fatalf("Redis bloqueado impidió el cierre SQL: %d %s", respuesta.Code, respuesta.Body)
	}
}
