package spiders

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"path/filepath"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

// spiderConfig declara lo que cambia entre cadenas. El resto del ciclo de vida
// (categorías, paginación, scraping puntual y armado del item) vive en
// baseSpider.
type spiderConfig struct {
	name              string
	supermarket       string
	categoryFile      string
	categoryEnv       string
	defaultCategories []string

	validateCategory func(string) error
	validateProduct  func(string) error

	extractProducts func([]byte, *url.URL) []domain.Product
	extractProduct  func([]byte, *url.URL, string) *domain.Product

	categoryFromURL func(string) string
	pageURL         func(string, int) string
	startPage       func(string) int
}

// baseSpider concentra el comportamiento común de los spiders de supermercados.
type baseSpider struct {
	fetcher     Fetcher
	categoryDir string
	maxPages    int
	logger      *slog.Logger
	cfg         spiderConfig

	// materialize sólo lo implementa Jumbo para enriquecer EAN desde la ficha.
	materialize func(context.Context, domain.Product, string, *url.URL, bool) (domain.Item, error)
}

func newBaseSpider(
	fetcher Fetcher,
	categoryDir string,
	maxPages int,
	logger *slog.Logger,
	cfg spiderConfig,
) baseSpider {
	if maxPages <= 0 {
		maxPages = DefaultMaxPages
	}
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.categoryFromURL == nil {
		cfg.categoryFromURL = categoryFromURL
	}
	if cfg.startPage == nil {
		cfg.startPage = func(string) int { return 1 }
	}
	return baseSpider{
		fetcher:     fetcher,
		categoryDir: categoryDir,
		maxPages:    maxPages,
		logger:      logger,
		cfg:         cfg,
	}
}

func (b *baseSpider) Name() string {
	return b.cfg.name
}

func (b *baseSpider) Run(ctx context.Context, job domain.Job, emit Emitter) error {
	if job.ProductURL != "" {
		return b.runProduct(ctx, job, emit)
	}
	if job.AddURL != "" {
		if err := b.appendCategoryURL(job.AddURL); err != nil {
			return err
		}
	}
	urls, err := b.startURLs(job.CategoryURLs)
	if err != nil {
		return err
	}
	for _, categoryURL := range urls {
		if err := b.runCategory(ctx, categoryURL, job.EnrichEAN, emit); err != nil {
			return err
		}
	}
	return nil
}

// runProduct atiende un scraping puntual. Nunca cae al crawl de categorías.
func (b *baseSpider) runProduct(ctx context.Context, job domain.Job, emit Emitter) error {
	if err := b.cfg.validateProduct(job.ProductURL); err != nil {
		return err
	}
	response, err := b.fetcher.Fetch(ctx, job.ProductURL)
	if err != nil {
		return err
	}
	if response.Status == 404 {
		b.logger.Warn(b.cfg.name+"_product_not_found", "url", job.ProductURL)
		return nil
	}
	product := b.cfg.extractProduct(response.Body, response.URL, job.ProductURL)
	if product == nil {
		b.logger.Warn(b.cfg.name+"_product_without_data", "url", job.ProductURL)
		return nil
	}
	return emit(b.toItem(*product, "producto", job.ProductURL))
}

func (b *baseSpider) runCategory(ctx context.Context, categoryURL string, enrichEAN bool, emit Emitter) error {
	category := b.cfg.categoryFromURL(categoryURL)
	for page := b.cfg.startPage(categoryURL); page <= b.maxPages; page++ {
		pageURL := b.cfg.pageURL(categoryURL, page)
		response, err := b.fetcher.Fetch(ctx, pageURL)
		if err != nil {
			if errors.Is(err, crawler.ErrRobotsDisallowed) {
				b.logger.Warn(b.cfg.name+"_category_robots_disallowed", "url", pageURL)
				return nil
			}
			var statusErr *crawler.HTTPStatusError
			if errors.As(err, &statusErr) && statusErr.Status == 404 {
				b.logger.Info(b.cfg.name+"_pagination_end", "url", pageURL, "page", page)
				return nil
			}
			return err
		}
		if response.Status == 404 {
			b.logger.Info(b.cfg.name+"_pagination_end", "url", pageURL, "page", page)
			return nil
		}

		products := b.cfg.extractProducts(response.Body, response.URL)
		if len(products) == 0 {
			b.logger.Warn(b.cfg.name+"_category_without_products", "url", pageURL, "page", page)
			return nil
		}
		for _, product := range products {
			item, err := b.item(ctx, product, category, response.URL, enrichEAN)
			if err != nil {
				return err
			}
			if err := emit(item); err != nil {
				return err
			}
		}
		if page == b.maxPages {
			b.logger.Warn(b.cfg.name+"_pagination_limit", "url", categoryURL, "max_pages", b.maxPages)
		}
	}
	return nil
}

func (b *baseSpider) item(
	ctx context.Context,
	product domain.Product,
	category string,
	responseURL *url.URL,
	enrichEAN bool,
) (domain.Item, error) {
	if b.materialize != nil {
		return b.materialize(ctx, product, category, responseURL, enrichEAN)
	}
	return b.toItem(product, category, urlString(responseURL)), nil
}

func (b *baseSpider) startURLs(overrides []string) ([]string, error) {
	urls := append([]string(nil), overrides...)
	if len(urls) == 0 {
		urls = readCategoryURLs(filepath.Join(b.categoryDir, b.cfg.categoryFile))
		urls = append(urls, envCategoryURLs(b.cfg.categoryEnv)...)
	}
	urls = dedupeStrings(urls)
	if len(urls) == 0 {
		return append([]string(nil), b.cfg.defaultCategories...), nil
	}
	for _, rawURL := range urls {
		if err := b.cfg.validateCategory(rawURL); err != nil {
			return nil, err
		}
	}
	return urls, nil
}

func (b *baseSpider) appendCategoryURL(rawURL string) error {
	return appendCategoryURL(
		filepath.Join(b.categoryDir, b.cfg.categoryFile),
		rawURL,
		b.cfg.validateCategory,
	)
}

func (b *baseSpider) toItem(product domain.Product, category, responseURL string) domain.Item {
	if product.URLProducto == nil || *product.URLProducto == "" {
		product.URLProducto = domain.StringPtr(responseURL)
	}
	product.Categoria = category
	return domain.Item{Product: product, Supermercado: b.cfg.supermarket}
}

// pageURLOrSelf conserva la URL original en la primera página y agrega el
// parámetro page desde la segunda. Es el patrón de Jumbo, Santa Isabel y Cugat.
func pageURLOrSelf(rawURL string, page int) string {
	if page <= 1 {
		return rawURL
	}
	return withPage(rawURL, page)
}

func urlString(value *url.URL) string {
	if value == nil {
		return ""
	}
	return value.String()
}
