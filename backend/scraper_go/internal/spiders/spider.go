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

// IsKnown conserva la lista de spiders aceptados por el worker de Python.
// Un spider conocido pero todavía no implementado se deja pendiente en la
// cola; sólo un nombre realmente inválido se descarta como payload inválido.
func IsKnown(name string) bool {
	switch name {
	case JumboName, SantaIsabelName, CugatName, AcuentaName, LiderName:
		return true
	default:
		return false
	}
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
	if maxPages <= 0 {
		maxPages = DefaultMaxPages
	}
	if logger == nil {
		logger = slog.Default()
	}
	registry := &Registry{spiders: make(map[string]Spider)}
	registry.Register(NewJumbo(fetcher, categoryDir, maxPages, logger))
	registry.Register(NewSantaIsabel(fetcher, categoryDir, maxPages, logger))
	registry.Register(NewCugat(fetcher, categoryDir, maxPages, logger))
	registry.Register(NewAcuenta(fetcher, categoryDir, maxPages, logger))
	registry.Register(NewLider(fetcher, categoryDir, maxPages, logger))
	return registry
}

func (r *Registry) Register(spider Spider) {
	r.spiders[spider.Name()] = spider
}

func (r *Registry) Get(name string) (Spider, bool) {
	spider, ok := r.spiders[name]
	return spider, ok
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.spiders))
	for name := range r.spiders {
		names = append(names, name)
	}
	return names
}
