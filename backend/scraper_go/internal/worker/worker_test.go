package worker

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
)

// arañaDePrueba representa una araña registrada para validar el worker.
type arañaDePrueba struct {
	nombre string
}

// Name devuelve el nombre declarado en el registro de prueba.
func (araña arañaDePrueba) Name() string { return araña.nombre }

// Run no realiza extracción porque estas pruebas solo validan la selección.
func (arañaDePrueba) Run(context.Context, domain.Job, spiders.Emitter) error { return nil }

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
			configuracion := config.Default()
			mutar(&configuracion)
			if _, errorObtenido := New(configuracion, nil, nil, nil, nil); errorObtenido == nil {
				prueba.Fatal("New aceptó una configuración inválida")
			}
		})
	}
}

func TestParseJobValidaContraRegistroConfigurado(prueba *testing.T) {
	sitioJumbo, existe := catalog.Default().Get(spiders.JumboName)
	if !existe {
		prueba.Fatal("falta jumbo_rsc en el catálogo base de prueba")
	}
	sitioJumbo.Name = "tottus_rsc"
	catalogoConfigurado := catalog.Catalog{Sites: []catalog.Site{sitioJumbo}}
	registro, errorObtenido := spiders.NewRegistryFromCatalog(
		catalogoConfigurado,
		nil,
		prueba.TempDir(),
		100,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}

	trabajador := &Worker{registry: registro}
	carga := `{"trabajo_id":"550e8400-e29b-41d4-a716-446655440000","spider":"tottus_rsc"}`
	trabajo, errorObtenido := trabajador.parseJob(carga)
	if errorObtenido != nil {
		prueba.Fatalf("se rechazó un spider del registro configurado: %v", errorObtenido)
	}
	if trabajo.Spider != "tottus_rsc" {
		prueba.Errorf("Spider = %q, se esperaba tottus_rsc", trabajo.Spider)
	}

	carga = `{"trabajo_id":"550e8400-e29b-41d4-a716-446655440000","spider":"jumbo_rsc"}`
	if _, errorObtenido := trabajador.parseJob(carga); errorObtenido == nil {
		prueba.Fatal("se aceptó jumbo_rsc aunque no está en el registro configurado")
	}
}

func TestParseJobRechazaRegistroNoConfigurado(prueba *testing.T) {
	trabajador := &Worker{}
	carga := `{"trabajo_id":"550e8400-e29b-41d4-a716-446655440000","spider":"jumbo_rsc"}`
	if _, errorObtenido := trabajador.parseJob(carga); errorObtenido == nil {
		prueba.Fatal("se aceptó un trabajo sin registro de spiders")
	}
}
