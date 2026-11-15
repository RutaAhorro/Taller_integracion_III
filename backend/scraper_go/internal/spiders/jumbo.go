package spiders

import (
	"context"
	"log/slog"
	"net/url"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

// ===== Estructura y Constructor =====

// Jumbo ejecuta el spider de Jumbo.
type Jumbo struct {
	baseSpider
}

// NewJumbo construye el spider de Jumbo con el catálogo embebido.
func NewJumbo(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Jumbo {
	return newJumbo(mustSite(JumboName), fetcher, categoryDir, maxPages, logger)
}

// newJumbo instancia el spider específico de jumbo usando el catálogo central.
func newJumbo(site catalog.Site, fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Jumbo {
	spider := &Jumbo{baseSpider: newBaseSpider(fetcher, categoryDir, maxPages, logger, site, spiderConfig{
		extractProducts: func(body []byte, _ *url.URL) []domain.Product {
			return extract.JumboProducts(site, body)
		},
		extractProduct: func(body []byte, responseURL *url.URL, requested string) *domain.Product {
			return extract.JumboProduct(site, body, requested, responseURL)
		},
	})}
	spider.baseSpider.materialize = spider.enrich
	return spider
}

// ===== Enriquecimiento =====

// enrich consulta la ficha del producto cuando el listado no publica EAN/GTIN.
func (jumbo *Jumbo) enrich(
	ctx context.Context,
	product domain.Product,
	category string,
	responseURL *url.URL,
	enrichEAN bool,
) (domain.Item, error) {
	pageURL := urlString(responseURL)
	if !enrichEAN || product.EanGtin != nil || product.URLProducto == nil || *product.URLProducto == "" {
		return jumbo.toItem(product, category, pageURL), nil
	}

	detailURL := extract.ResolveURL(responseURL, *product.URLProducto)
	if err := jumbo.site.ValidateProductURL(detailURL); err != nil {
		jumbo.logger.Warn("jumbo_ean_url_invalid", "url", detailURL, "error", err)
		return jumbo.toItem(product, category, pageURL), nil
	}
	response, err := jumbo.fetcher.Fetch(ctx, detailURL)
	if err != nil {
		jumbo.logger.Warn("jumbo_ean_fetch_failed", "url", detailURL, "error", err)
		return jumbo.toItem(product, category, pageURL), nil
	}
	if response.Status == 404 {
		return jumbo.toItem(product, category, pageURL), nil
	}

	enriched := product
	if detail := extract.JumboProduct(jumbo.site, response.Body, detailURL, response.URL); detail != nil {
		if detail.EanGtin != nil && *detail.EanGtin != "" {
			enriched.EanGtin = detail.EanGtin
		}
		if detail.SKU != nil && *detail.SKU != "" {
			enriched.SKU = detail.SKU
		}
	}
	if enriched.EanGtin == nil {
		jumbo.logger.Info("jumbo_ean_missing", "url", detailURL)
	}
	return jumbo.toItem(enriched, category, pageURL), nil
}
