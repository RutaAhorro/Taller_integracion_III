// worker_test.go verifica las guardas de construcción de worker.New ante
// configuraciones inválidas o dependencias obligatorias ausentes.
package worker_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	cola "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/cola"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/worker"
)

// ===== Dobles de Prueba =====

type obtenedorNulo struct{}

// Fetch implementa extraccion.Fetcher sin realizar llamadas HTTP.
func (obtenedorNulo) Fetch(_ context.Context, _ string) (*extraccion.Response, error) {
	return nil, nil
}

// construirDependenciasValidas prepara instancias válidas de cliente API, cola y registro para pruebas unitarias.
func construirDependenciasValidas(prueba *testing.T) (config.Config, *cola.Client, *cola.Queue, *extraccion.Registry, *slog.Logger) {
	prueba.Helper()
	configuracion := config.Default()
	configuracion.Token = "token-secreto"
	registrador := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := cola.NewControlClient(configuracion.APIURL, configuracion.Token, time.Second)
	clienteRedis := redis.NewClient(&redis.Options{Addr: "localhost:0"})
	prueba.Cleanup(func() { _ = clienteRedis.Close() })
	clienteCola, errorObtenido := cola.NewQueue(clienteRedis, cola.Options{Lease: 10 * time.Second})
	if errorObtenido != nil {
		prueba.Fatalf("NewQueue() error = %v", errorObtenido)
	}
	registro, errorObtenido := extraccion.NewRegistryFromCatalog(extraccion.Default(), obtenedorNulo{}, prueba.TempDir(), 100, registrador)
	if errorObtenido != nil {
		prueba.Fatalf("NewRegistryFromCatalog() error = %v", errorObtenido)
	}
	return configuracion, api, clienteCola, registro, registrador
}

// ===== Pruebas de Guardas en worker.New =====

// TestNewRechazaConfiguracionInvalida verifica que worker.New valide la configuración antes de construir el worker.
func TestNewRechazaConfiguracionInvalida(prueba *testing.T) {
	casos := map[string]func(*config.Config){
		"configuración vacía": func(configuracion *config.Config) {
			*configuracion = config.Config{}
		},
		"sin intentos de crawl": func(configuracion *config.Config) {
			configuracion.CrawlAttempts = 0
		},
		"sin intervalo de consulta": func(configuracion *config.Config) {
			configuracion.PollInterval = 0
		},
		"sin intervalo de reintento": func(configuracion *config.Config) {
			configuracion.RetryInterval = 0
		},
	}
	for nombre, mutar := range casos {
		prueba.Run(nombre, func(prueba *testing.T) {
			configuracion, api, clienteCola, registro, registrador := construirDependenciasValidas(prueba)
			mutar(&configuracion)
			if _, errorObtenido := worker.New(configuracion, api, clienteCola, registro, registrador); errorObtenido == nil {
				prueba.Fatal("New aceptó una configuración inválida")
			}
		})
	}
}

// TestNewRechazaDependenciasObligatoriasNulas verifica las guardas H-06 y H-12 en worker.New.
func TestNewRechazaDependenciasObligatoriasNulas(prueba *testing.T) {
	configuracionBase, apiValida, colaValida, registroValido, registrador := construirDependenciasValidas(prueba)

	casos := []struct {
		nombre        string
		configuracion config.Config
		api           *cola.Client
		clienteCola   *cola.Queue
		registro      *extraccion.Registry
		mensaje       string
	}{
		{
			nombre: "sin token de servicio",
			configuracion: func() config.Config {
				copia := configuracionBase
				copia.Token = "   "
				return copia
			}(),
			api:         apiValida,
			clienteCola: colaValida,
			registro:    registroValido,
			mensaje:     "SCRAPING_SERVICE_TOKEN",
		},
		{
			nombre:        "sin cliente API",
			configuracion: configuracionBase,
			api:           nil,
			clienteCola:   colaValida,
			registro:      registroValido,
			mensaje:       "Control de Scraping",
		},
		{
			nombre:        "sin cola Redis",
			configuracion: configuracionBase,
			api:           apiValida,
			clienteCola:   nil,
			registro:      registroValido,
			mensaje:       "cola de trabajos",
		},
		{
			nombre:        "sin registro de spiders",
			configuracion: configuracionBase,
			api:           apiValida,
			clienteCola:   colaValida,
			registro:      nil,
			mensaje:       "registro de spiders",
		},
	}

	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			_, errorObtenido := worker.New(caso.configuracion, caso.api, caso.clienteCola, caso.registro, registrador)
			if errorObtenido == nil || !strings.Contains(errorObtenido.Error(), caso.mensaje) {
				prueba.Fatalf("worker.New() error = %v, se esperaba que contuviera %q", errorObtenido, caso.mensaje)
			}
		})
	}

	if trabajador, errorObtenido := worker.New(configuracionBase, apiValida, colaValida, registroValido, nil); errorObtenido != nil || trabajador == nil {
		prueba.Fatalf("worker.New() con dependencias válidas falló: %v", errorObtenido)
	}
}
