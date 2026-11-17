package spiders

import (
	"log/slog"
	"net/url"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

// ===== Estructura y Constructor =====

// Cugat ejecuta el spider de Cugat.
type Cugat struct {
	baseSpider
}

// NewCugat construye el spider de Cugat con el catálogo embebido.
func NewCugat(obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) *Cugat {
	return newCugat(mustSite(CugatName), obtenedor, directorioCategorias, maxPaginas, registrador)
}

// newCugat instancia el spider específico de cugat usando el catálogo central.
func newCugat(sitio catalog.Site, obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) *Cugat {
	return &Cugat{baseSpider: newBaseSpider(obtenedor, directorioCategorias, maxPaginas, registrador, sitio, spiderConfig{
		extractProducts: func(cuerpo []byte, urlRespuesta *url.URL) []domain.Product {
			return extract.CugatProducts(sitio, cuerpo, urlRespuesta)
		},
		extractProduct: func(cuerpo []byte, urlRespuesta *url.URL, _ string) *domain.Product {
			return extract.CugatProduct(sitio, cuerpo, urlRespuesta)
		},
	})}
}
