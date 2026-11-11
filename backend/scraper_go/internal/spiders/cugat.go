package spiders

import (
	"log/slog"
	"net/url"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

type Cugat struct {
	baseSpider
}

func NewCugat(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Cugat {
	return newCugat(mustSite(CugatName), fetcher, categoryDir, maxPages, logger)
}

func newCugat(site catalog.Site, fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Cugat {
	return &Cugat{baseSpider: newBaseSpider(fetcher, categoryDir, maxPages, logger, site, spiderConfig{
		extractProducts: func(body []byte, responseURL *url.URL) []domain.Product {
			return extract.CugatProducts(site, body, responseURL)
		},
		extractProduct: func(body []byte, responseURL *url.URL, _ string) *domain.Product {
			return extract.CugatProduct(site, body, responseURL)
		},
	})}
}
