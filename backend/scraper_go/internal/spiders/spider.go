// Package spiders contiene el ciclo de vida de cada spider y su registro. Los
// parsers viven en internal/extract y la configuración en internal/catalog.
package spiders

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

// ===== Construcción y Catálogo =====

const (
	JumboName       = "jumbo_rsc"
	SantaIsabelName = "santa_isabel_rsc"
	CugatName       = "cugat_rsc"
	AcuentaName     = "acuenta_rsc"
	LiderName       = "lider_rsc"
)

var (
	catalogoEmbutidoUnaVez sync.Once
	catalogoEmbutido       catalog.Catalog
)

// Build construye el spider correspondiente al parser declarado en el catálogo.
func Build(sitio catalog.Site, obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) (Spider, error) {
	switch sitio.Parser {
	case "jumbo":
		return newJumbo(sitio, obtenedor, directorioCategorias, maxPaginas, registrador), nil
	case "santa_isabel":
		return newSantaIsabel(sitio, obtenedor, directorioCategorias, maxPaginas, registrador), nil
	case "cugat":
		return newCugat(sitio, obtenedor, directorioCategorias, maxPaginas, registrador), nil
	case "acuenta":
		return newAcuenta(sitio, obtenedor, directorioCategorias, maxPaginas, registrador), nil
	case "lider":
		return newLider(sitio, obtenedor, directorioCategorias, maxPaginas, registrador), nil
	default:
		return nil, fmt.Errorf("parser desconocido %q", sitio.Parser)
	}
}

// IsKnown indica si el nombre pertenece al catálogo de spiders.
func IsKnown(nombre string) bool {
	_, valido := defaultCatalog().Get(nombre)
	return valido
}

// Names devuelve los nombres del catálogo en orden estable.
func Names() []string {
	return defaultCatalog().Names()
}

// defaultCatalog parsea una vez el catálogo embebido usado por los spiders integrados.
func defaultCatalog() catalog.Catalog {
	catalogoEmbutidoUnaVez.Do(func() {
		catalogoEmbutido = catalog.Default()
	})
	return catalogoEmbutido
}

// ===== Contratos =====

// Emitter recibe cada producto ya normalizado. El worker lo conecta con el
// spool y la entrega por lotes.
type Emitter func(domain.Item) error

// Fetcher permite probar los spiders sin red y deja la puerta abierta para
// extraer el motor de fetch a otro servicio más adelante.
type Fetcher interface {
	Fetch(contexto context.Context, urlCruda string) (*crawler.Response, error)
}

// Spider es el contrato mínimo de una cadena: nombre y ejecución de un Job.
type Spider interface {
	Name() string
	Run(contexto context.Context, trabajo domain.Job, emitir Emitter) error
}

// ===== Registro =====

// Registry contiene los spiders disponibles indexados por nombre.
type Registry struct {
	spiders map[string]Spider
}

// NewRegistry construye el registro con el catálogo embebido.
func NewRegistry(obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) *Registry {
	registro, errorObtenido := NewRegistryFromCatalog(defaultCatalog(), obtenedor, directorioCategorias, maxPaginas, registrador)
	if errorObtenido != nil {
		panic(errorObtenido)
	}
	return registro
}

// NewRegistryFromEnv construye el registro con el catálogo indicado por
// SCRAPER_SPIDERS_FILE, si existe. Falla al arrancar si la configuración no es
// válida.
func NewRegistryFromEnv(obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) (*Registry, error) {
	sitios, errorObtenido := catalog.FromEnv()
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	return NewRegistryFromCatalog(sitios, obtenedor, directorioCategorias, maxPaginas, registrador)
}

// NewRegistryFromCatalog construye todos los spiders declarados en el catálogo.
func NewRegistryFromCatalog(sitios catalog.Catalog, obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) (*Registry, error) {
	registro := &Registry{spiders: make(map[string]Spider, len(sitios.Sites))}
	for _, sitio := range sitios.Sites {
		spider, errorObtenido := Build(sitio, obtenedor, directorioCategorias, maxPaginas, registrador)
		if errorObtenido != nil {
			return nil, errorObtenido
		}
		registro.Register(spider)
	}
	return registro, nil
}

// Register añade o reemplaza un spider en el registro.
func (registro *Registry) Register(spider Spider) {
	registro.spiders[spider.Name()] = spider
}

// Get devuelve el spider con ese nombre y si existe.
func (registro *Registry) Get(nombre string) (Spider, bool) {
	spider, valido := registro.spiders[nombre]
	return spider, valido
}
