package spiders

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"path/filepath"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

// spiderConfig declara sólo las funciones de parsing. El resto de la
// configuración sale de catalog.Site.
type spiderConfig struct {
	extractProducts func([]byte, *url.URL) []domain.Product
	extractProduct  func([]byte, *url.URL, string) *domain.Product
}

// baseSpider concentra el comportamiento común de los spiders de supermercados.
type baseSpider struct {
	fetcher     Fetcher
	categoryDir string
	maxPages    int
	logger      *slog.Logger
	site        catalog.Site
	cfg         spiderConfig

	// materialize sólo lo implementa Jumbo para enriquecer EAN desde la ficha.
	materialize func(context.Context, domain.Product, string, *url.URL, bool) (domain.Item, error)
}

// ===== Constructor =====

// newBaseSpider crea una instancia base compartida con configuración del catálogo.
func newBaseSpider(
	obtenedor Fetcher,
	directorioCategorias string,
	maxPaginas int,
	registrador *slog.Logger,
	sitio catalog.Site,
	configuracion spiderConfig,
) baseSpider {
	if maxPaginas <= 0 {
		maxPaginas = config.DefaultMaxPages
	}
	if registrador == nil {
		registrador = slog.Default()
	}
	return baseSpider{
		fetcher:     obtenedor,
		categoryDir: directorioCategorias,
		maxPages:    maxPaginas,
		logger:      registrador,
		site:        sitio,
		cfg:         configuracion,
	}
}

// Name devuelve el identificador oficial del spider.
func (base *baseSpider) Name() string {
	return base.site.Name
}

// Run ejecuta la rutina de extracción del spider para el trabajo indicado.
func (base *baseSpider) Run(contexto context.Context, trabajo domain.Job, emitir Emitter) error {
	if trabajo.ProductURL != "" {
		return base.runProduct(contexto, trabajo, emitir)
	}
	if trabajo.AddURL != "" {
		if errorObtenido := base.appendCategoryURL(trabajo.AddURL); errorObtenido != nil {
			return errorObtenido
		}
	}
	urls, errorObtenido := base.startURLs(trabajo.CategoryURLs)
	if errorObtenido != nil {
		return errorObtenido
	}
	for _, urlCategoria := range urls {
		if errorObtenido := base.runCategory(contexto, urlCategoria, trabajo.EnrichEAN, emitir); errorObtenido != nil {
			return errorObtenido
		}
	}
	return nil
}

// runProduct atiende un scraping puntual. Nunca cae al crawl de categorías.
func (base *baseSpider) runProduct(contexto context.Context, trabajo domain.Job, emitir Emitter) error {
	if errorObtenido := base.site.ValidateProductURL(trabajo.ProductURL); errorObtenido != nil {
		return errorObtenido
	}
	respuesta, errorObtenido := base.fetcher.Fetch(contexto, trabajo.ProductURL)
	if errorObtenido != nil {
		return errorObtenido
	}
	if respuesta.Status == 404 {
		base.logger.Warn(base.site.Name+"_product_not_found", "url", trabajo.ProductURL)
		return nil
	}
	producto := base.cfg.extractProduct(respuesta.Body, respuesta.URL, trabajo.ProductURL)
	if producto == nil {
		base.logger.Warn(base.site.Name+"_product_without_data", "url", trabajo.ProductURL)
		return nil
	}
	return emitir(base.toItem(*producto, "producto", trabajo.ProductURL))
}

// runCategory procesa la secuencia de páginas pertenecientes a una categoría.
func (base *baseSpider) runCategory(contexto context.Context, urlCategoria string, enriquecerEAN bool, emitir Emitter) error {
	categoria := extract.CategoryFromURL(base.site, urlCategoria)
	for pagina := extract.StartPage(base.site, urlCategoria); pagina <= base.maxPages; pagina++ {
		urlPagina := extract.PageURL(base.site, urlCategoria, pagina)
		respuesta, errorObtenido := base.fetcher.Fetch(contexto, urlPagina)
		if errorObtenido != nil {
			if errors.Is(errorObtenido, crawler.ErrRobotsDisallowed) {
				base.logger.Warn(base.site.Name+"_category_robots_disallowed", "url", urlPagina)
				return nil
			}
			var errorEstado *crawler.HTTPStatusError
			if errors.As(errorObtenido, &errorEstado) && errorEstado.Status == 404 {
				base.logger.Info(base.site.Name+"_pagination_end", "url", urlPagina, "page", pagina)
				return nil
			}
			return errorObtenido
		}
		if respuesta.Status == 404 {
			base.logger.Info(base.site.Name+"_pagination_end", "url", urlPagina, "page", pagina)
			return nil
		}

		productos := base.cfg.extractProducts(respuesta.Body, respuesta.URL)
		if len(productos) == 0 {
			base.logger.Warn(base.site.Name+"_category_without_products", "url", urlPagina, "page", pagina)
			return nil
		}
		for _, producto := range productos {
			elemento, errorObtenido := base.item(contexto, producto, categoria, respuesta.URL, enriquecerEAN)
			if errorObtenido != nil {
				return errorObtenido
			}
			if errorObtenido := emitir(elemento); errorObtenido != nil {
				return errorObtenido
			}
		}
		if pagina == base.maxPages {
			base.logger.Warn(base.site.Name+"_pagination_limit", "url", urlCategoria, "max_pages", base.maxPages)
		}
	}
	return nil
}

// item empaqueta un producto asociándole su supermercado de origen.
func (base *baseSpider) item(
	contexto context.Context,
	producto domain.Product,
	categoria string,
	urlRespuesta *url.URL,
	enriquecerEAN bool,
) (domain.Item, error) {
	if base.materialize != nil {
		return base.materialize(contexto, producto, categoria, urlRespuesta, enriquecerEAN)
	}
	return base.toItem(producto, categoria, urlString(urlRespuesta)), nil
}

// startURLs determina las URLs iniciales de extracción a partir del trabajo.
func (base *baseSpider) startURLs(sobrescrituras []string) ([]string, error) {
	urls := append([]string(nil), sobrescrituras...)
	if len(urls) == 0 {
		urls = readCategoryURLs(filepath.Join(base.categoryDir, base.site.CategoryFile))
		urls = append(urls, envCategoryURLs(base.site.CategoryEnv)...)
	}
	urls = dedupeStrings(urls)
	if len(urls) == 0 {
		return append([]string(nil), base.site.DefaultCategories...), nil
	}
	for _, urlCruda := range urls {
		if errorObtenido := base.site.ValidateCategoryURL(urlCruda); errorObtenido != nil {
			return nil, errorObtenido
		}
	}
	return urls, nil
}

// appendCategoryURL agrega una categoría validando formato y unicidad.
func (base *baseSpider) appendCategoryURL(urlCruda string) error {
	return appendCategoryURL(
		filepath.Join(base.categoryDir, base.site.CategoryFile),
		urlCruda,
		base.site.ValidateCategoryURL,
	)
}

// toItem convierte un producto en elemento listo para ser emitido.
func (base *baseSpider) toItem(producto domain.Product, categoria, urlRespuesta string) domain.Item {
	if producto.URLProducto == nil || *producto.URLProducto == "" {
		producto.URLProducto = domain.StringPtr(urlRespuesta)
	}
	producto.Categoria = categoria
	return domain.Item{Product: producto, Supermercado: base.site.Supermarket}
}

// urlString normaliza la representación textual de una URL.
func urlString(valor *url.URL) string {
	if valor == nil {
		return ""
	}
	return valor.String()
}

// mustSite devuelve el sitio embebido o entra en pánico si el catálogo está
// roto (error de build, no de ejecución).
func mustSite(nombre string) catalog.Site {
	sitio, valido := defaultCatalog().Get(nombre)
	if !valido {
		panic("catálogo sin sitio " + nombre)
	}
	return sitio
}
