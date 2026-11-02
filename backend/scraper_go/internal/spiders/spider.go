// Package spiders contiene los spiders y su registro. Cada spider conoce un
// sitio; el worker sólo ve las interfaces Fetcher, Emitter y Spider.
package spiders

import (
	"context"
	"log/slog"

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

// spiderFactory construye un spider con las mismas dependencias que recibe el
// registry.
type spiderFactory func(Fetcher, string, int, *slog.Logger) Spider

// asFactory adapta un constructor concreto a spiderFactory. Evita repetir el
// mismo envoltorio por cada spider.
func asFactory[T Spider](newSpider func(Fetcher, string, int, *slog.Logger) T) spiderFactory {
	return func(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) Spider {
		return newSpider(fetcher, categoryDir, maxPages, logger)
	}
}

// spiderCatalog es la única fuente de nombres y constructores. De aquí salen
// IsKnown, Names y NewRegistry, de modo que no haya listas paralelas.
var spiderCatalog = []struct {
	name  string
	build spiderFactory
}{
	{JumboName, asFactory(NewJumbo)},
	{SantaIsabelName, asFactory(NewSantaIsabel)},
	{CugatName, asFactory(NewCugat)},
	{AcuentaName, asFactory(NewAcuenta)},
	{LiderName, asFactory(NewLider)},
}

// IsKnown indica si el nombre pertenece al catálogo de spiders.
func IsKnown(name string) bool {
	for _, entry := range spiderCatalog {
		if entry.name == name {
			return true
		}
	}
	return false
}

// Names devuelve los nombres del catálogo en orden estable.
func Names() []string {
	names := make([]string, 0, len(spiderCatalog))
	for _, entry := range spiderCatalog {
		names = append(names, entry.name)
	}
	return names
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

func NewRegistry(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Registry {
	registry := &Registry{spiders: make(map[string]Spider, len(spiderCatalog))}
	for _, entry := range spiderCatalog {
		registry.Register(entry.build(fetcher, categoryDir, maxPages, logger))
	}
	return registry
}

func (r *Registry) Register(spider Spider) {
	r.spiders[spider.Name()] = spider
}

func (r *Registry) Get(name string) (Spider, bool) {
	spider, ok := r.spiders[name]
	return spider, ok
}
