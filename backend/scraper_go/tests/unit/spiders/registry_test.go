package spiders_test

import (
	"log/slog"
	"reflect"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
)

func TestRegistryContieneTodosLosSpidersConocidos(prueba *testing.T) {
	registry := spiders.NewRegistry(&fakeFetcher{}, prueba.TempDir(), 100, quietLogger())
	for _, name := range []string{
		spiders.JumboName,
		spiders.SantaIsabelName,
		spiders.CugatName,
		spiders.AcuentaName,
		spiders.LiderName,
	} {
		if _, ok := registry.Get(name); !ok {
			prueba.Errorf("falta el spider %q en el registro", name)
		}
	}
}

func TestNamesEIsKnownSiguenElCatalogo(prueba *testing.T) {
	want := []string{
		spiders.JumboName,
		spiders.SantaIsabelName,
		spiders.CugatName,
		spiders.AcuentaName,
		spiders.LiderName,
	}
	if got := spiders.Names(); !reflect.DeepEqual(got, want) {
		prueba.Errorf("Names() = %v, want %v", got, want)
	}
	for _, name := range want {
		if !spiders.IsKnown(name) {
			prueba.Errorf("IsKnown(%q) = false", name)
		}
	}
	if spiders.IsKnown("no_existe") {
		prueba.Error("IsKnown() aceptó un nombre desconocido")
	}
}

func TestBuildRechazaParserDesconocido(prueba *testing.T) {
	site := catalog.Site{Name: "demo_rsc", Parser: "desconocido", Supermarket: "Demo"}
	if _, err := spiders.Build(site, &fakeFetcher{}, prueba.TempDir(), 100, quietLogger()); err == nil {
		prueba.Fatal("Build() debió rechazar un parser desconocido")
	}
}

func TestNewRegistryFromCatalogConstruyeCadaParser(prueba *testing.T) {
	sites := catalog.Default()
	registry, err := spiders.NewRegistryFromCatalog(sites, &fakeFetcher{}, prueba.TempDir(), 100, quietLogger())
	if err != nil {
		prueba.Fatalf("NewRegistryFromCatalog() error = %v", err)
	}
	for _, name := range sites.Names() {
		spider, ok := registry.Get(name)
		if !ok {
			prueba.Errorf("falta %q en el registro", name)
			continue
		}
		if spider.Name() != name {
			prueba.Errorf("Name() = %q, want %q", spider.Name(), name)
		}
	}
}

func TestNewRegistryFromCatalogRechazaParserInvalido(prueba *testing.T) {
	sites := catalog.Catalog{Sites: []catalog.Site{{
		Name:        "demo_rsc",
		Parser:      "desconocido",
		Supermarket: "Demo",
	}}}
	if _, err := spiders.NewRegistryFromCatalog(sites, &fakeFetcher{}, prueba.TempDir(), 100, quietLogger()); err == nil {
		prueba.Fatal("NewRegistryFromCatalog() debió rechazar el catálogo")
	}
}

func TestNewRegistryFromEnvUsaElCatalogoEmbebido(prueba *testing.T) {
	registry, err := spiders.NewRegistryFromEnv(&fakeFetcher{}, prueba.TempDir(), 100, quietLogger())
	if err != nil {
		prueba.Fatalf("NewRegistryFromEnv() error = %v", err)
	}
	if _, ok := registry.Get(spiders.JumboName); !ok {
		prueba.Error("el registro no contiene jumbo_rsc")
	}
}

func TestNewRegistryFromEnvPropagaErrorDeCatalogo(prueba *testing.T) {
	prueba.Setenv(catalog.FileEnv, "/no/existe/spiders.json")
	if _, err := spiders.NewRegistryFromEnv(&fakeFetcher{}, prueba.TempDir(), 100, slog.Default()); err == nil {
		prueba.Fatal("NewRegistryFromEnv() debió fallar con un catálogo inexistente")
	}
}
