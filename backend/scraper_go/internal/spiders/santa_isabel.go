package spiders

import (
	"log/slog"
	"net/url"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

// ===== Estructura y Constructor =====

// SantaIsabel ejecuta el spider de Santa Isabel.
type SantaIsabel struct {
	baseSpider
}

// NewSantaIsabel construye el spider de Santa Isabel con el catálogo embebido.
func NewSantaIsabel(obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) *SantaIsabel {
	return newSantaIsabel(mustSite(SantaIsabelName), obtenedor, directorioCategorias, maxPaginas, registrador)
}

// newSantaIsabel instancia el spider específico de santa_isabel usando el catálogo central.
func newSantaIsabel(sitio catalog.Site, obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) *SantaIsabel {
	return &SantaIsabel{baseSpider: newBaseSpider(obtenedor, directorioCategorias, maxPaginas, registrador, sitio, spiderConfig{
		extractProducts: func(cuerpo []byte, urlRespuesta *url.URL) []domain.Product {
			return extract.SantaIsabelProducts(sitio, cuerpo, urlRespuesta)
		},
		extractProduct: func(cuerpo []byte, urlRespuesta *url.URL, solicitado string) *domain.Product {
			return extract.SantaIsabelProduct(sitio, cuerpo, solicitado, urlRespuesta)
		},
	})}
}
