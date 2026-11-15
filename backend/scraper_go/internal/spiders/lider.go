package spiders

import (
	"log/slog"
	"net/url"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

// ===== Estructura y Constructor =====

// Lider ejecuta el spider de Lider.
type Lider struct {
	baseSpider
}

// NewLider construye el spider de Lider con el catálogo embebido.
func NewLider(obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) *Lider {
	return newLider(mustSite(LiderName), obtenedor, directorioCategorias, maxPaginas, registrador)
}

// newLider instancia el spider específico de lider usando el catálogo central.
func newLider(sitio catalog.Site, obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) *Lider {
	return &Lider{baseSpider: newBaseSpider(obtenedor, directorioCategorias, maxPaginas, registrador, sitio, spiderConfig{
		extractProducts: func(cuerpo []byte, urlRespuesta *url.URL) []domain.Product {
			return extract.LiderProducts(sitio, cuerpo, urlRespuesta)
		},
		extractProduct: func(cuerpo []byte, urlRespuesta *url.URL, solicitado string) *domain.Product {
			return extract.LiderProduct(sitio, cuerpo, solicitado, urlRespuesta)
		},
	})}
}
