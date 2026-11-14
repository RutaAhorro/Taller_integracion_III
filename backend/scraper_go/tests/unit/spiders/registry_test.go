package spiders_test

import (
	"log/slog"
	"reflect"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
)

func TestRegistryContieneTodosLosSpidersConocidos(t *testing.T) {
	registry := spiders.NewRegistry(&fakeFetcher{}, t.TempDir(), 100, quietLogger())
	for _, name := range []string{
		spiders.JumboName,
		spiders.SantaIsabelName,
		spiders.CugatName,
		spiders.AcuentaName,
		spiders.LiderName,
	} {
		if _, ok := registry.Get(name); !ok {
			t.Errorf("falta el spider %q en el registro", name)
		}
	}
}

func TestNamesEIsKnownSiguenElCatalogo(t *testing.T) {
	want := []string{
		spiders.JumboName,
		spiders.SantaIsabelName,
		spiders.CugatName,
		spiders.AcuentaName,
		spiders.LiderName,
	}
	if got := spiders.Names(); !reflect.DeepEqual(got, want) {
		t.Errorf("Names() = %v, want %v", got, want)
	}
	for _, name := range want {
		if !spiders.IsKnown(name) {
			t.Errorf("IsKnown(%q) = false", name)
		}
	}
	if spiders.IsKnown("no_existe") {
		t.Error("IsKnown() aceptó un nombre desconocido")
	}
}

func TestBuildRechazaParserDesconocido(t *testing.T) {
	site := catalog.Site{Name: "demo_rsc", Parser: "desconocido", Supermarket: "Demo"}
	if _, err := spiders.Build(site, &fakeFetcher{}, t.TempDir(), 100, quietLogger()); err == nil {
		t.Fatal("Build() debió rechazar un parser desconocido")
	}
}

func TestNewRegistryFromCatalogConstruyeCadaParser(t *testing.T) {
	sites := catalog.Default()
	registry, err := spiders.NewRegistryFromCatalog(sites, &fakeFetcher{}, t.TempDir(), 100, quietLogger())
	if err != nil {
		t.Fatalf("NewRegistryFromCatalog() error = %v", err)
	}
	for _, name := range sites.Names() {
		spider, ok := registry.Get(name)
		if !ok {
			t.Errorf("falta %q en el registro", name)
			continue
		}
		if spider.Name() != name {
			t.Errorf("Name() = %q, want %q", spider.Name(), name)
		}
	}
}

func TestNewRegistryFromCatalogRechazaParserInvalido(t *testing.T) {
	sites := catalog.Catalog{Sites: []catalog.Site{{
		Name:        "demo_rsc",
		Parser:      "desconocido",
		Supermarket: "Demo",
	}}}
	if _, err := spiders.NewRegistryFromCatalog(sites, &fakeFetcher{}, t.TempDir(), 100, quietLogger()); err == nil {
		t.Fatal("NewRegistryFromCatalog() debió rechazar el catálogo")
	}
}

func TestNewRegistryFromEnvUsaElCatalogoEmbebido(t *testing.T) {
	registry, err := spiders.NewRegistryFromEnv(&fakeFetcher{}, t.TempDir(), 100, quietLogger())
	if err != nil {
		t.Fatalf("NewRegistryFromEnv() error = %v", err)
	}
	if _, ok := registry.Get(spiders.JumboName); !ok {
		t.Error("el registro no contiene jumbo_rsc")
	}
}

func TestNewRegistryFromEnvPropagaErrorDeCatalogo(t *testing.T) {
	t.Setenv(catalog.FileEnv, "/no/existe/spiders.json")
	if _, err := spiders.NewRegistryFromEnv(&fakeFetcher{}, t.TempDir(), 100, slog.Default()); err == nil {
		t.Fatal("NewRegistryFromEnv() debió fallar con un catálogo inexistente")
	}
}
