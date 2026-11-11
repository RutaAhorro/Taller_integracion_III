package spiders

import (
	"log/slog"
	"net/url"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

type Lider struct {
	baseSpider
}

func NewLider(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Lider {
	return newLider(mustSite(LiderName), fetcher, categoryDir, maxPages, logger)
}

func newLider(site catalog.Site, fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Lider {
	return &Lider{baseSpider: newBaseSpider(fetcher, categoryDir, maxPages, logger, site, spiderConfig{
		extractProducts: func(body []byte, responseURL *url.URL) []domain.Product {
			return extract.LiderProducts(site, body, responseURL)
		},
		extractProduct: func(body []byte, responseURL *url.URL, requested string) *domain.Product {
			return extract.LiderProduct(site, body, requested, responseURL)
		},
	})}
}
