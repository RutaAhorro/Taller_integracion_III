package spiders

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"path/filepath"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
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
	fetcher Fetcher,
	categoryDir string,
	maxPages int,
	logger *slog.Logger,
	site catalog.Site,
	cfg spiderConfig,
) baseSpider {
	if maxPages <= 0 {
		maxPages = 1
	}
	if logger == nil {
		logger = slog.Default()
	}
	return baseSpider{
		fetcher:     fetcher,
		categoryDir: categoryDir,
		maxPages:    maxPages,
		logger:      logger,
		site:        site,
		cfg:         cfg,
	}
}

// Name devuelve el identificador oficial del spider.
func (base *baseSpider) Name() string {
	return base.site.Name
}

// Run ejecuta la rutina de extracción del spider para el trabajo indicado.
func (base *baseSpider) Run(ctx context.Context, job domain.Job, emit Emitter) error {
	if job.ProductURL != "" {
		return base.runProduct(ctx, job, emit)
	}
	if job.AddURL != "" {
		if err := base.appendCategoryURL(job.AddURL); err != nil {
			return err
		}
	}
	urls, err := base.startURLs(job.CategoryURLs)
	if err != nil {
		return err
	}
	for _, categoryURL := range urls {
		if err := base.runCategory(ctx, categoryURL, job.EnrichEAN, emit); err != nil {
			return err
		}
	}
	return nil
}

// runProduct atiende un scraping puntual. Nunca cae al crawl de categorías.
func (base *baseSpider) runProduct(ctx context.Context, job domain.Job, emit Emitter) error {
	if err := base.site.ValidateProductURL(job.ProductURL); err != nil {
		return err
	}
	response, err := base.fetcher.Fetch(ctx, job.ProductURL)
	if err != nil {
		return err
	}
	if response.Status == 404 {
		base.logger.Warn(base.site.Name+"_product_not_found", "url", job.ProductURL)
		return nil
	}
	product := base.cfg.extractProduct(response.Body, response.URL, job.ProductURL)
	if product == nil {
		base.logger.Warn(base.site.Name+"_product_without_data", "url", job.ProductURL)
		return nil
	}
	return emit(base.toItem(*product, "producto", job.ProductURL))
}

// runCategory procesa la secuencia de páginas pertenecientes a una categoría.
func (base *baseSpider) runCategory(ctx context.Context, categoryURL string, enrichEAN bool, emit Emitter) error {
	category := extract.CategoryFromURL(base.site, categoryURL)
	for page := extract.StartPage(base.site, categoryURL); page <= base.maxPages; page++ {
		pageURL := extract.PageURL(base.site, categoryURL, page)
		response, err := base.fetcher.Fetch(ctx, pageURL)
		if err != nil {
			if errors.Is(err, crawler.ErrRobotsDisallowed) {
				base.logger.Warn(base.site.Name+"_category_robots_disallowed", "url", pageURL)
				return nil
			}
			var statusErr *crawler.HTTPStatusError
			if errors.As(err, &statusErr) && statusErr.Status == 404 {
				base.logger.Info(base.site.Name+"_pagination_end", "url", pageURL, "page", page)
				return nil
			}
			return err
		}
		if response.Status == 404 {
			base.logger.Info(base.site.Name+"_pagination_end", "url", pageURL, "page", page)
			return nil
		}

		products := base.cfg.extractProducts(response.Body, response.URL)
		if len(products) == 0 {
			base.logger.Warn(base.site.Name+"_category_without_products", "url", pageURL, "page", page)
			return nil
		}
		for _, product := range products {
			item, err := base.item(ctx, product, category, response.URL, enrichEAN)
			if err != nil {
				return err
			}
			if err := emit(item); err != nil {
				return err
			}
		}
		if page == base.maxPages {
			base.logger.Warn(base.site.Name+"_pagination_limit", "url", categoryURL, "max_pages", base.maxPages)
		}
	}
	return nil
}

// item empaqueta un producto asociándole su supermercado de origen.
func (base *baseSpider) item(
	ctx context.Context,
	product domain.Product,
	category string,
	responseURL *url.URL,
	enrichEAN bool,
) (domain.Item, error) {
	if base.materialize != nil {
		return base.materialize(ctx, product, category, responseURL, enrichEAN)
	}
	return base.toItem(product, category, urlString(responseURL)), nil
}

// startURLs determina las URLs iniciales de extracción a partir del trabajo.
func (base *baseSpider) startURLs(overrides []string) ([]string, error) {
	urls := append([]string(nil), overrides...)
	if len(urls) == 0 {
		urls = readCategoryURLs(filepath.Join(base.categoryDir, base.site.CategoryFile))
		urls = append(urls, envCategoryURLs(base.site.CategoryEnv)...)
	}
	urls = dedupeStrings(urls)
	if len(urls) == 0 {
		return append([]string(nil), base.site.DefaultCategories...), nil
	}
	for _, rawURL := range urls {
		if err := base.site.ValidateCategoryURL(rawURL); err != nil {
			return nil, err
		}
	}
	return urls, nil
}

// appendCategoryURL agrega una categoría validando formato y unicidad.
func (base *baseSpider) appendCategoryURL(rawURL string) error {
	return appendCategoryURL(
		filepath.Join(base.categoryDir, base.site.CategoryFile),
		rawURL,
		base.site.ValidateCategoryURL,
	)
}

// toItem convierte un producto en elemento listo para ser emitido.
func (base *baseSpider) toItem(product domain.Product, category, responseURL string) domain.Item {
	if product.URLProducto == nil || *product.URLProducto == "" {
		product.URLProducto = domain.StringPtr(responseURL)
	}
	product.Categoria = category
	return domain.Item{Product: product, Supermercado: base.site.Supermarket}
}

// urlString normaliza la representación textual de una URL.
func urlString(value *url.URL) string {
	if value == nil {
		return ""
	}
	return value.String()
}

// mustSite devuelve el sitio embebido o entra en pánico si el catálogo está
// roto (error de build, no de ejecución).
func mustSite(name string) catalog.Site {
	site, ok := catalog.Default().Get(name)
	if !ok {
		panic("catálogo sin sitio " + name)
	}
	return site
}
