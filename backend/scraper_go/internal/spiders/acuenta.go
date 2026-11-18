package spiders

import (
	"log/slog"
	"net/url"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

// ===== Estructura y Constructor =====

// Acuenta ejecuta el spider de A Cuenta.
type Acuenta struct {
	baseSpider
}

// NewAcuenta construye el spider de A Cuenta con el catálogo embebido.
func NewAcuenta(obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) *Acuenta {
	return newAcuenta(mustSite(AcuentaName), obtenedor, directorioCategorias, maxPaginas, registrador)
}

// newAcuenta instancia el spider específico de acuenta usando el catálogo central.
func newAcuenta(sitio catalog.Site, obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) *Acuenta {
	return &Acuenta{baseSpider: newBaseSpider(obtenedor, directorioCategorias, maxPaginas, registrador, sitio, spiderConfig{
		extractProducts: func(cuerpo []byte, urlRespuesta *url.URL) []domain.Product {
			return extract.AcuentaProducts(sitio, cuerpo, urlRespuesta)
		},
		extractProduct: func(cuerpo []byte, urlRespuesta *url.URL, solicitado string) *domain.Product {
			return extract.AcuentaProduct(sitio, cuerpo, solicitado, urlRespuesta)
		},
	})}
}
