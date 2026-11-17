package spiders

import (
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
)

func TestNewBaseSpiderUsesDefaultMaxPagesForNonPositiveValues(prueba *testing.T) {
	for _, maxPaginas := range []int{0, -1} {
		base := newBaseSpider(nil, "", maxPaginas, nil, catalog.Site{}, spiderConfig{})
		if base.maxPages != config.DefaultMaxPages {
			prueba.Errorf("maxPages con entrada %d = %d, se esperaba %d", maxPaginas, base.maxPages, config.DefaultMaxPages)
		}
	}
}
