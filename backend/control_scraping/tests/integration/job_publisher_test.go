// Package integration_test verifica que Redis lento no bloquee el cierre SQL.
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
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/app"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestPublisherStalledRedisAllowsJobFinalization ejerce la API sobre una BD aislada.
func TestPublisherStalledRedisAllowsJobFinalization(t *testing.T) {
	dsn := os.Getenv("SUP270_PUBLISHER_TEST_DB_URL")
	if dsn == "" {
		t.Skip("ejecutado por smoke_scraping_recovery_compose.sh con DB efímera")
	}
	gin.SetMode(gin.TestMode)
	servidorCatalogo := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path != "/internal/v1/cadenas/resolver" {
			respuesta.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(respuesta, `{"id":1,"nombre":"Jumbo","esta_activa":true}`)
	}))
	defer servidorCatalogo.Close()
	t.Setenv("CATALOGO_INTERNAL_URL", servidorCatalogo.URL)
	t.Setenv("CATALOGO_INTERNAL_TOKEN", strings.Repeat("t", 32))
	t.Setenv("SCRAPING_SERVICE_TOKEN", testServiceToken)

	baseDatos, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("no se pudo conectar a PostgreSQL de prueba")
	}
	conexionSQL, err := baseDatos.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer conexionSQL.Close()
	trabajo := domain.TrabajoScraper{
		ID: uuid.New(), CadenaID: 1, Estado: "en_progreso", IniciadoEl: time.Now(),
	}
	if err := baseDatos.Create(&trabajo).Error; err != nil {
		t.Fatal(err)
	}

	servidorTCP, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer servidorTCP.Close()
	conexionAceptada := make(chan struct{})
	liberarConexion := make(chan struct{})
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
	router, coordinacion, err := app.NewApplication(baseDatos, clienteRedis)
	if err != nil {
		t.Fatal(err)
	}
	contexto, cancelar := context.WithCancel(context.Background())
	coordinacion.Start(contexto)
	defer func() {
		cancelar()
		contextoEspera, cancelarEspera := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelarEspera()
		if errorEspera := coordinacion.Wait(contextoEspera); errorEspera != nil {
			t.Errorf("los loops no terminaron: %v", errorEspera)
		}
	}()

	peticionEjecucion := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/scraper/trabajos/"+trabajo.ID.String()+"/ejecutar",
		strings.NewReader(`{"spider":"jumbo_rsc"}`),
	)
	peticionEjecucion.Header.Set("Content-Type", "application/json")
	peticionEjecucion.Header.Set("Authorization", "Bearer "+testServiceToken)
	respuestaEjecucion := httptest.NewRecorder()
	router.ServeHTTP(respuestaEjecucion, peticionEjecucion)
	if respuestaEjecucion.Code != http.StatusAccepted {
		t.Fatalf("no se aceptó la ejecución: %d %s", respuestaEjecucion.Code, respuestaEjecucion.Body)
	}

	select {
	case <-conexionAceptada:
	case <-time.After(7 * time.Second):
		t.Fatal("el publisher no llegó al I/O Redis")
	}
	contextoCierre, cancelarCierre := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancelarCierre()
	peticionCierre := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/scraper/trabajos/"+trabajo.ID.String()+"/finalizar",
		strings.NewReader(`{"estado":"fallido"}`),
	).WithContext(contextoCierre)
	peticionCierre.Header.Set("Content-Type", "application/json")
	peticionCierre.Header.Set("Authorization", "Bearer "+testServiceToken)
	respuestaCierre := httptest.NewRecorder()
	router.ServeHTTP(respuestaCierre, peticionCierre)
	if respuestaCierre.Code != http.StatusOK {
		t.Fatalf("Redis lento bloqueó el cierre SQL: %d %s", respuestaCierre.Code, respuestaCierre.Body)
	}

	var resultado map[string]any
	if err := json.Unmarshal(respuestaCierre.Body.Bytes(), &resultado); err != nil {
		t.Fatal(err)
	}
}
