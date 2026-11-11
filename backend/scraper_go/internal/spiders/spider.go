// Package spiders contiene el ciclo de vida de cada spider y su registro. Los
// parsers viven en internal/extract y la configuración en internal/catalog.
package spiders

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

const DefaultMaxPages = 100

const (
	JumboName       = "jumbo_rsc"
	SantaIsabelName = "santa_isabel_rsc"
	CugatName       = "cugat_rsc"
	AcuentaName     = "acuenta_rsc"
	LiderName       = "lider_rsc"
)

// Build construye el spider correspondiente al parser declarado en el catálogo.
func Build(site catalog.Site, fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) (Spider, error) {
	switch site.Parser {
	case "jumbo":
		return newJumbo(site, fetcher, categoryDir, maxPages, logger), nil
	case "santa_isabel":
		return newSantaIsabel(site, fetcher, categoryDir, maxPages, logger), nil
	case "cugat":
		return newCugat(site, fetcher, categoryDir, maxPages, logger), nil
	case "acuenta":
		return newAcuenta(site, fetcher, categoryDir, maxPages, logger), nil
	case "lider":
		return newLider(site, fetcher, categoryDir, maxPages, logger), nil
	default:
		return nil, fmt.Errorf("parser desconocido %q", site.Parser)
	}
}

// IsKnown indica si el nombre pertenece al catálogo de spiders.
func IsKnown(name string) bool {
	_, ok := catalog.Default().Get(name)
	return ok
}

// Names devuelve los nombres del catálogo en orden estable.
func Names() []string {
	return catalog.Default().Names()
}

// Emitter recibe cada producto ya normalizado. El worker lo conecta con el
// spool y la entrega por lotes.
type Emitter func(domain.Item) error

// Fetcher permite probar los spiders sin red y deja la puerta abierta para
// extraer el motor de fetch a otro servicio más adelante.
type Fetcher interface {
	Fetch(ctx context.Context, rawURL string) (*crawler.Response, error)
}

type Spider interface {
	Name() string
	Run(ctx context.Context, job domain.Job, emit Emitter) error
}

type Registry struct {
	spiders map[string]Spider
}

// NewRegistry construye el registro con el catálogo embebido.
func NewRegistry(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Registry {
	registry, err := NewRegistryFromCatalog(catalog.Default(), fetcher, categoryDir, maxPages, logger)
	if err != nil {
		panic(err)
	}
	return registry
}

// NewRegistryFromEnv construye el registro con el catálogo indicado por
// SCRAPER_SPIDERS_FILE, si existe. Falla al arrancar si la configuración no es
// válida.
func NewRegistryFromEnv(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) (*Registry, error) {
	sites, err := catalog.FromEnv()
	if err != nil {
		return nil, err
	}
	return NewRegistryFromCatalog(sites, fetcher, categoryDir, maxPages, logger)
}

// NewRegistryFromCatalog construye todos los spiders declarados en el catálogo.
func NewRegistryFromCatalog(sites catalog.Catalog, fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) (*Registry, error) {
	registry := &Registry{spiders: make(map[string]Spider, len(sites.Sites))}
	for _, site := range sites.Sites {
		spider, err := Build(site, fetcher, categoryDir, maxPages, logger)
		if err != nil {
			return nil, err
		}
		registry.Register(spider)
	}
	return registry, nil
}

func (r *Registry) Register(spider Spider) {
	r.spiders[spider.Name()] = spider
}

func (r *Registry) Get(name string) (Spider, bool) {
	spider, ok := r.spiders[name]
	return spider, ok
}
