package spiders

import (
	"context"
	"log/slog"
	"net/url"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

type Jumbo struct {
	baseSpider
}

func NewJumbo(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Jumbo {
	return newJumbo(mustSite(JumboName), fetcher, categoryDir, maxPages, logger)
}

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

// enrich consulta la ficha del producto cuando el listado no publica EAN/GTIN.
func (j *Jumbo) enrich(
	ctx context.Context,
	product domain.Product,
	category string,
	responseURL *url.URL,
	enrichEAN bool,
) (domain.Item, error) {
	pageURL := urlString(responseURL)
	if !enrichEAN || product.EanGtin != nil || product.URLProducto == nil || *product.URLProducto == "" {
		return j.toItem(product, category, pageURL), nil
	}

	detailURL := extract.ResolveURL(responseURL, *product.URLProducto)
	if err := j.site.ValidateProductURL(detailURL); err != nil {
		j.logger.Warn("jumbo_ean_url_invalid", "url", detailURL, "error", err)
		return j.toItem(product, category, pageURL), nil
	}
	response, err := j.fetcher.Fetch(ctx, detailURL)
	if err != nil {
		j.logger.Warn("jumbo_ean_fetch_failed", "url", detailURL, "error", err)
		return j.toItem(product, category, pageURL), nil
	}
	if response.Status == 404 {
		return j.toItem(product, category, pageURL), nil
	}

	enriched := product
	if detail := extract.JumboProduct(j.site, response.Body, detailURL, response.URL); detail != nil {
		if detail.EanGtin != nil && *detail.EanGtin != "" {
			enriched.EanGtin = detail.EanGtin
		}
		if detail.SKU != nil && *detail.SKU != "" {
			enriched.SKU = detail.SKU
		}
	}
	if enriched.EanGtin == nil {
		j.logger.Info("jumbo_ean_missing", "url", detailURL)
	}
	return j.toItem(enriched, category, pageURL), nil
}
