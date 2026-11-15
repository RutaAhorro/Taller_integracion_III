// Package integration_test valida readiness y liveness con dependencias controladas.
package integration_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/app"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ===== Dependencias simuladas para probes =====

// healthConnector permite controlar la respuesta de Ping sin abrir una base real.
type healthConnector struct {
	falloPing     atomic.Bool
	consultasPing atomic.Int32
}

// Connect crea una conexión de prueba asociada al estado del conector.
func (conector *healthConnector) Connect(context.Context) (driver.Conn, error) {
	return healthConn{conector: conector}, nil
}

// Driver satisface driver.Connector con un driver que no abre conexiones directamente.
func (conector *healthConnector) Driver() driver.Driver { return healthDriver{} }

// healthDriver sólo permite abrir conexiones mediante el conector de prueba.
type healthDriver struct{}

// Open rechaza conexiones directas para asegurar que la prueba use el conector.
func (healthDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use connector")
}

// healthConn implementa las operaciones SQL mínimas usadas por el probe.
type healthConn struct {
	conector *healthConnector
}

// Prepare rechaza SQL porque el probe no debe ejecutar consultas.
func (healthConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected SQL")
}

// Close cierra la conexión simulada sin efectos laterales.
func (healthConn) Close() error { return nil }

// Begin rechaza transacciones porque el probe no debe iniciarlas.
func (healthConn) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}

// Ping permite probar readiness exitosa, fallida y cancelada.
func (conexion healthConn) Ping(contexto context.Context) error {
	conexion.conector.consultasPing.Add(1)
	if errorObtenido := contexto.Err(); errorObtenido != nil {
		return errorObtenido
	}
	if conexion.conector.falloPing.Load() {
		return errors.New("private PostgreSQL diagnostic")
	}
	return nil
}

// probeDatabase crea una conexión GORM que usa el conector controlado por la prueba.
func probeDatabase(prueba *testing.T, conector *healthConnector) *gorm.DB {
	prueba.Helper()
	conexionSQL := sql.OpenDB(conector)
	prueba.Cleanup(func() { _ = conexionSQL.Close() })
	baseDatos, errorObtenido := gorm.Open(postgres.New(postgres.Config{Conn: conexionSQL}), &gorm.Config{DisableAutomaticPing: true})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	return baseDatos
}

// ===== Contratos de readiness y diagnóstico =====

// TestHealthRedisTimeoutDoesNotRemoveReadiness verifica que Redis sólo sea diagnóstico.
func TestHealthRedisTimeoutDoesNotRemoveReadiness(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	baseDatos := probeDatabase(prueba, &healthConnector{})
	servidorTCP, errorObtenido := net.Listen("tcp", "127.0.0.1:0")
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	defer servidorTCP.Close()
	go func() {
		for {
			conexion, errorAceptacion := servidorTCP.Accept()
			if errorAceptacion != nil {
				return
			}
			go func() {
				defer conexion.Close()
				_, _ = io.Copy(io.Discard, conexion)
			}()
		}
	}()
	clienteRedis := redis.NewClient(&redis.Options{
		Addr: servidorTCP.Addr().String(), MaxRetries: -1,
		DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second,
		ContextTimeoutEnabled: true,
	})
	defer clienteRedis.Close()
	enrutador := gin.New()
	enrutador.GET("/health", app.HealthHandler(baseDatos, clienteRedis, "control-scraping"))
	contexto, cancelar := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelar()
	respuesta := httptest.NewRecorder()
	inicio := time.Now()
	enrutador.ServeHTTP(respuesta, httptest.NewRequest("GET", "/health", nil).WithContext(contexto))
	if transcurrido := time.Since(inicio); transcurrido > time.Second {
		prueba.Fatalf("Redis lento bloqueó el probe durante %v", transcurrido)
	}
	var cuerpo struct {
		Checks map[string]string `json:"checks"`
	}
	if errorObtenido := json.Unmarshal(respuesta.Body.Bytes(), &cuerpo); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if respuesta.Code != 200 || cuerpo.Checks["redis"] != "error" || cuerpo.Checks["database"] != "ok" {
		prueba.Fatalf("Redis retiró readiness con PostgreSQL sano: %d %s", respuesta.Code, respuesta.Body)
	}
}

// TestHealthRequiresScrapingDatabase verifica readiness SQL y liveness independiente.
func TestHealthRequiresScrapingDatabase(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	conector := &healthConnector{}
	baseDatos := probeDatabase(prueba, conector)
	for _, caso := range []struct {
		nombre           string
		baseDatosFallida bool
		baseDatosAusente bool
		codigoEstado     int
	}{
		{"healthy", false, false, 200},
		{"scraping unavailable", true, false, 503},
		{"scraping not configured", false, true, 503},
		{"recovered", false, false, 200},
	} {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			conector.falloPing.Store(caso.baseDatosFallida)
			baseDatosActual := baseDatos
			if caso.baseDatosAusente {
				baseDatosActual = nil
			}
			enrutador := gin.New()
			enrutador.GET("/health", app.HealthHandler(baseDatosActual, nil, "control-scraping"))
			enrutador.GET("/_api/live", app.LivenessHandler)
			respuesta := httptest.NewRecorder()
			enrutador.ServeHTTP(respuesta, httptest.NewRequest("GET", "/health", nil))
			if respuesta.Code != caso.codigoEstado || strings.Contains(respuesta.Body.String(), "private PostgreSQL") {
				prueba.Fatalf("readiness incorrecto: %d %s", respuesta.Code, respuesta.Body)
			}
			var cuerpo struct {
				Estado string `json:"status"`
			}
			if errorObtenido := json.Unmarshal(respuesta.Body.Bytes(), &cuerpo); errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
			if (cuerpo.Estado == "ok") != (caso.codigoEstado == 200) {
				prueba.Fatal("estado JSON contradice HTTP")
			}
			consultasAntes := conector.consultasPing.Load()
			respuesta = httptest.NewRecorder()
			enrutador.ServeHTTP(respuesta, httptest.NewRequest("GET", "/_api/live", nil))
			if respuesta.Code != 200 || consultasAntes != conector.consultasPing.Load() {
				prueba.Fatal("liveness depende de PostgreSQL")
			}
		})
	}
}
