package spiders

import (
	"log/slog"
	"net/url"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

// Acuenta ejecuta el spider de A Cuenta.
type Acuenta struct {
	baseSpider
}

// NewAcuenta construye el spider de A Cuenta con el catálogo embebido.
func NewAcuenta(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Acuenta {
	return newAcuenta(mustSite(AcuentaName), fetcher, categoryDir, maxPages, logger)
}

// newAcuenta instancia el spider específico de acuenta usando el catálogo central.
func newAcuenta(site catalog.Site, fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Acuenta {
	return &Acuenta{baseSpider: newBaseSpider(fetcher, categoryDir, maxPages, logger, site, spiderConfig{
		extractProducts: func(body []byte, responseURL *url.URL) []domain.Product {
			return extract.AcuentaProducts(site, body, responseURL)
		},
		extractProduct: func(body []byte, responseURL *url.URL, requested string) *domain.Product {
			return extract.AcuentaProduct(site, body, requested, responseURL)
		},
	})}
}
