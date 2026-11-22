// Package integration_test prueba el servicio compuesto como en main: límites
// de arquitectura, rutas registradas, sondas y Swagger. Las pruebas con
// PostgreSQL y Redis reales están en almacenamiento_test.go.
package integration_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ===== Preparación =====

const (
	moduloServicio = "github.com/danirc2024/Taller_integracion_III/backend/control_scraping/"
	tokenServicio  = "ssssssssssssssssssssssssssssssss"
)

// configuracionPrueba devuelve una configuración válida sin dependencias reales.
func configuracionPrueba(swagger bool) config.Configuracion {
	return config.Configuracion{
		URLCatalogo: "http://catalogo.invalid:8080", TokenCatalogo: strings.Repeat("t", 32), TokenServicio: tokenServicio,
		PlazoCatalogo: time.Second, SwaggerHabilitado: swagger,
	}
}

// servir ejecuta una petición autenticada contra el router.
func servir(enrutador *gin.Engine, metodo, ruta, cuerpo string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	peticion := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	peticion.Header.Set("Content-Type", "application/json")
	peticion.Header.Set("Authorization", "Bearer "+tokenServicio)
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, peticion)
	return respuesta
}

// ===== Arquitectura =====

// TestDependenciasEntreCapas impide importar otros servicios y fija el sentido
// de las dependencias: domain no importa infraestructura, services sólo domain
// y repositories nunca services ni api.
func TestDependenciasEntreCapas(prueba *testing.T) {
	propio := moduloServicio + "internal/"
	dependenciasDominio := map[string]bool{"context": true, "errors": true, "strconv": true, "time": true, "github.com/google/uuid": true}
	errorObtenido := filepath.WalkDir(filepath.Join("..", "..", "internal"), func(ruta string, entrada fs.DirEntry, errorObtenido error) error {
		if errorObtenido != nil || entrada.IsDir() || !strings.HasSuffix(ruta, ".go") {
			return errorObtenido
		}
		archivo, errorObtenido := parser.ParseFile(token.NewFileSet(), ruta, nil, parser.ImportsOnly)
		if errorObtenido != nil {
			return errorObtenido
		}
		capa := filepath.Base(filepath.Dir(ruta))
		for _, especificacion := range archivo.Imports {
			dependencia, _ := strconv.Unquote(especificacion.Path.Value)
			if strings.HasPrefix(dependencia, "github.com/danirc2024/Taller_integracion_III/backend/") && !strings.HasPrefix(dependencia, moduloServicio) {
				prueba.Errorf("%s importa otro servicio: %s", ruta, dependencia)
			}
			esPropia := strings.HasPrefix(dependencia, propio)
			switch capa {
			case "domain":
				if !dependenciasDominio[dependencia] {
					prueba.Errorf("%s: domain depende de %s", ruta, dependencia)
				}
			case "services":
				if (esPropia && dependencia != propio+"domain") || strings.HasPrefix(dependencia, "gorm.io/") || strings.HasPrefix(dependencia, "github.com/gin-gonic/") || strings.HasPrefix(dependencia, "github.com/redis/") {
					prueba.Errorf("%s: services depende de %s", ruta, dependencia)
				}
			case "repositories":
				if esPropia && dependencia != propio+"domain" {
					prueba.Errorf("%s: repositories depende de %s", ruta, dependencia)
				}
			}
		}
		return nil
	})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
}

// ===== Composición HTTP =====

// TestComposicionSinDependencias registra sólo rutas de Control y valida antes de tocar SQL.
func TestComposicionSinDependencias(prueba *testing.T) {
	enrutador, _ := api.Componer(configuracionPrueba(false), nil, nil)
	casos := []struct {
		nombre, metodo, ruta, cuerpo string
		esperado                     int
	}{
		{"liveness histórica", http.MethodGet, "/_api/live", "", http.StatusOK},
		{"liveness", http.MethodGet, "/_scraping/live", "", http.StatusOK},
		{"readiness sin base", http.MethodGet, "/_scraping/ready", "", http.StatusServiceUnavailable},
		{"health sin base", http.MethodGet, "/health", "", http.StatusServiceUnavailable},
		{"health v1 sin base", http.MethodGet, "/api/v1/health", "", http.StatusServiceUnavailable},
		{"raíz", http.MethodGet, "/", "", http.StatusOK},
		{"perfil fuera de Control", http.MethodGet, "/api/v1/auth/me", "", http.StatusNotFound},
		{"login fuera de Control", http.MethodPost, "/api/v1/auth/login", "{}", http.StatusNotFound},
		{"catálogo fuera de Control", http.MethodGet, "/api/v1/productos", "", http.StatusNotFound},
		{"admin fuera de Control", http.MethodGet, "/api/v1/admin/productos", "", http.StatusNotFound},
		{"Swagger deshabilitado", http.MethodGet, "/swagger/doc.json", "", http.StatusNotFound},
		{"trabajo inválido", http.MethodGet, "/api/v1/scraper/trabajos/invalido", "", http.StatusBadRequest},
		{"creación inválida", http.MethodPost, "/api/v1/scraper/trabajos", "{}", http.StatusBadRequest},
		{"ingesta inválida", http.MethodPost, "/api/v1/scraper/productos", "{}", http.StatusBadRequest},
		{"ejecución con ID inválido", http.MethodPost, "/api/v1/scraper/trabajos/invalido/ejecutar", "{}", http.StatusBadRequest},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			if respuesta := servir(enrutador, caso.metodo, caso.ruta, caso.cuerpo); respuesta.Code != caso.esperado {
				prueba.Fatalf("HTTP %d; esperado %d: %s", respuesta.Code, caso.esperado, respuesta.Body)
			}
		})
	}
}

// TestPanicoNoFiltraDetalle responde 500 genérico ante un fallo inesperado
// (aquí, consultar SQL sin base configurada).
func TestPanicoNoFiltraDetalle(prueba *testing.T) {
	enrutador, _ := api.Componer(configuracionPrueba(false), nil, nil)
	respuesta := servir(enrutador, http.MethodGet, "/api/v1/scraper/trabajos/"+uuid.NewString(), "")
	var cuerpo api.RespuestaError
	if errorObtenido := json.Unmarshal(respuesta.Body.Bytes(), &cuerpo); errorObtenido != nil || respuesta.Code != http.StatusInternalServerError || cuerpo.Detalle != "" || strings.Contains(respuesta.Body.String(), "nil") {
		prueba.Fatalf("HTTP %d: %s", respuesta.Code, respuesta.Body)
	}
}

// ===== Swagger =====

// TestSwaggerSoloDocumentaRutasRegistradas compara cada operación publicada con el router.
func TestSwaggerSoloDocumentaRutasRegistradas(prueba *testing.T) {
	enrutador, _ := api.Componer(configuracionPrueba(true), nil, nil)
	respuesta := servir(enrutador, http.MethodGet, "/swagger/doc.json", "")
	var documento struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if respuesta.Code != http.StatusOK || json.Unmarshal(respuesta.Body.Bytes(), &documento) != nil || documento.Paths["/api/v1/scraper/trabajos"] == nil {
		prueba.Fatalf("HTTP %d: falta el contrato de coordinación", respuesta.Code)
	}
	registradas := map[string]bool{}
	for _, ruta := range enrutador.Routes() {
		registradas[ruta.Method+" "+ruta.Path] = true
	}
	parametro := regexp.MustCompile(`\{([^}]+)\}`)
	for ruta, metodos := range documento.Paths {
		if strings.HasPrefix(ruta, "/_") {
			prueba.Errorf("Swagger público incluye una sonda privada: %s", ruta)
		}
		for metodo := range metodos {
			if clave := strings.ToUpper(metodo) + " " + parametro.ReplaceAllString(ruta, ":$1"); !registradas[clave] {
				prueba.Errorf("Swagger publica una operación no registrada: %s", clave)
			}
		}
	}
}

// ===== Sondas =====

// conectorSonda controla el resultado de Ping sin abrir una base real y
// rechaza cualquier SQL: la readiness sólo debe hacer ping.
type conectorSonda struct {
	falloPing     atomic.Bool
	consultasPing atomic.Int32
}

// Connect entrega una conexión controlada por el conector.
func (conector *conectorSonda) Connect(context.Context) (driver.Conn, error) {
	return conexionSonda{conector: conector}, nil
}

// Driver satisface driver.Connector.
func (conector *conectorSonda) Driver() driver.Driver { return driverSonda{} }

// driverSonda sólo permite conexiones a través del conector.
type driverSonda struct{}

// Open rechaza conexiones directas.
func (driverSonda) Open(string) (driver.Conn, error) { return nil, errors.New("usar el conector") }

// conexionSonda implementa lo mínimo que usa la readiness.
type conexionSonda struct{ conector *conectorSonda }

// Prepare rechaza SQL.
func (conexionSonda) Prepare(string) (driver.Stmt, error) { return nil, errors.New("SQL inesperado") }

// Close no tiene efectos.
func (conexionSonda) Close() error { return nil }

// Begin rechaza transacciones.
func (conexionSonda) Begin() (driver.Tx, error) { return nil, errors.New("transacción inesperada") }

// Ping cuenta la llamada y falla si el conector lo indica.
func (conexion conexionSonda) Ping(contexto context.Context) error {
	conexion.conector.consultasPing.Add(1)
	if errorObtenido := contexto.Err(); errorObtenido != nil {
		return errorObtenido
	}
	if conexion.conector.falloPing.Load() {
		return errors.New("diagnóstico privado de PostgreSQL")
	}
	return nil
}

// baseSonda crea una conexión GORM sobre el conector controlado.
func baseSonda(prueba *testing.T, conector *conectorSonda) *gorm.DB {
	prueba.Helper()
	conexionSQL := sql.OpenDB(conector)
	prueba.Cleanup(func() { _ = conexionSQL.Close() })
	baseDatos, errorObtenido := gorm.Open(postgres.New(postgres.Config{Conn: conexionSQL}), &gorm.Config{DisableAutomaticPing: true})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	return baseDatos
}

// TestReadinessDependeDeSQL alterna PostgreSQL sano y caído; liveness nunca consulta SQL.
func TestReadinessDependeDeSQL(prueba *testing.T) {
	conector := &conectorSonda{}
	enrutador, _ := api.Componer(configuracionPrueba(false), baseSonda(prueba, conector), nil)
	for _, caso := range []struct {
		nombre   string
		falla    bool
		esperado int
	}{{"sana", false, http.StatusOK}, {"caída", true, http.StatusServiceUnavailable}, {"recuperada", false, http.StatusOK}} {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			conector.falloPing.Store(caso.falla)
			for _, ruta := range []string{"/_scraping/ready", "/health"} {
				respuesta := servir(enrutador, http.MethodGet, ruta, "")
				var cuerpo struct {
					Estado string            `json:"status"`
					Checks map[string]string `json:"checks"`
				}
				_ = json.Unmarshal(respuesta.Body.Bytes(), &cuerpo)
				if respuesta.Code != caso.esperado || (cuerpo.Estado == "ok") != (caso.esperado == http.StatusOK) ||
					cuerpo.Checks["database"] != cuerpo.Checks["scraping_database"] || strings.Contains(respuesta.Body.String(), "privado") {
					prueba.Fatalf("%s: HTTP %d %s", ruta, respuesta.Code, respuesta.Body)
				}
			}
			consultasAntes := conector.consultasPing.Load()
			if respuesta := servir(enrutador, http.MethodGet, "/_scraping/live", ""); respuesta.Code != http.StatusOK || conector.consultasPing.Load() != consultasAntes {
				prueba.Fatal("liveness depende de PostgreSQL")
			}
		})
	}
}

// TestReadinessToleraRedisLento informa Redis como error sin retirar la API.
func TestReadinessToleraRedisLento(prueba *testing.T) {
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
			go func() { defer conexion.Close(); _, _ = io.Copy(io.Discard, conexion) }()
		}
	}()
	clienteRedis := redis.NewClient(&redis.Options{Addr: servidorTCP.Addr().String(), MaxRetries: -1, ContextTimeoutEnabled: true})
	defer clienteRedis.Close()
	enrutador, _ := api.Componer(configuracionPrueba(false), baseSonda(prueba, &conectorSonda{}), clienteRedis)
	inicio := time.Now()
	respuesta := servir(enrutador, http.MethodGet, "/_scraping/ready", "")
	if transcurrido := time.Since(inicio); transcurrido > time.Second {
		prueba.Fatalf("Redis lento bloqueó la sonda durante %v", transcurrido)
	}
	var cuerpo struct {
		Checks map[string]string `json:"checks"`
	}
	if json.Unmarshal(respuesta.Body.Bytes(), &cuerpo) != nil || respuesta.Code != http.StatusOK || cuerpo.Checks["redis"] != "error" || cuerpo.Checks["database"] != "ok" {
		prueba.Fatalf("Redis retiró la readiness con PostgreSQL sano: %d %s", respuesta.Code, respuesta.Body)
	}
}
