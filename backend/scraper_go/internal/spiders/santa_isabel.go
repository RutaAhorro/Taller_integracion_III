package spiders

import (
	"log/slog"
	"net/url"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

type SantaIsabel struct {
	baseSpider
}

func NewSantaIsabel(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *SantaIsabel {
	return newSantaIsabel(mustSite(SantaIsabelName), fetcher, categoryDir, maxPages, logger)
}

func newSantaIsabel(site catalog.Site, fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *SantaIsabel {
	return &SantaIsabel{baseSpider: newBaseSpider(fetcher, categoryDir, maxPages, logger, site, spiderConfig{
		extractProducts: func(body []byte, responseURL *url.URL) []domain.Product {
			return extract.SantaIsabelProducts(site, body, responseURL)
		},
		extractProduct: func(body []byte, responseURL *url.URL, requested string) *domain.Product {
			return extract.SantaIsabelProduct(site, body, requested, responseURL)
		},
	})}
}
