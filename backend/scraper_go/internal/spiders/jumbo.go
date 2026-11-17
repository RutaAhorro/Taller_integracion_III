package spiders

import (
	"context"
	"log/slog"
	"net/url"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

// ===== Estructura y Constructor =====

// Jumbo ejecuta el spider de Jumbo.
type Jumbo struct {
	baseSpider
}

// NewJumbo construye el spider de Jumbo con el catálogo embebido.
func NewJumbo(obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) *Jumbo {
	return newJumbo(mustSite(JumboName), obtenedor, directorioCategorias, maxPaginas, registrador)
}

// newJumbo instancia el spider específico de jumbo usando el catálogo central.
func newJumbo(sitio catalog.Site, obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) *Jumbo {
	spider := &Jumbo{baseSpider: newBaseSpider(obtenedor, directorioCategorias, maxPaginas, registrador, sitio, spiderConfig{
		extractProducts: func(cuerpo []byte, _ *url.URL) []domain.Product {
			return extract.JumboProducts(sitio, cuerpo)
		},
		extractProduct: func(cuerpo []byte, urlRespuesta *url.URL, solicitado string) *domain.Product {
			return extract.JumboProduct(sitio, cuerpo, solicitado, urlRespuesta)
		},
	})}
	spider.baseSpider.materialize = spider.enrich
	return spider
}

// ===== Enriquecimiento =====

// enrich consulta la ficha del producto cuando el listado no publica EAN/GTIN.
func (jumbo *Jumbo) enrich(
	contexto context.Context,
	producto domain.Product,
	categoria string,
	urlRespuesta *url.URL,
	enriquecerEAN bool,
) (domain.Item, error) {
	urlPagina := urlString(urlRespuesta)
	if !enriquecerEAN || producto.EanGtin != nil || producto.URLProducto == nil || *producto.URLProducto == "" {
		return jumbo.toItem(producto, categoria, urlPagina), nil
	}

	urlDetalle := extract.ResolveURL(urlRespuesta, *producto.URLProducto)
	if errorObtenido := jumbo.site.ValidateProductURL(urlDetalle); errorObtenido != nil {
		jumbo.logger.Warn("jumbo_ean_url_invalid", "url", urlDetalle, "error", errorObtenido)
		return jumbo.toItem(producto, categoria, urlPagina), nil
	}
	respuesta, errorObtenido := jumbo.fetcher.Fetch(contexto, urlDetalle)
	if errorObtenido != nil {
		jumbo.logger.Warn("jumbo_ean_fetch_failed", "url", urlDetalle, "error", errorObtenido)
		return jumbo.toItem(producto, categoria, urlPagina), nil
	}
	if respuesta.Status == 404 {
		return jumbo.toItem(producto, categoria, urlPagina), nil
	}

	enriquecido := producto
	if detalle := extract.JumboProduct(jumbo.site, respuesta.Body, urlDetalle, respuesta.URL); detalle != nil {
		if detalle.EanGtin != nil && *detalle.EanGtin != "" {
			enriquecido.EanGtin = detalle.EanGtin
		}
		if detalle.SKU != nil && *detalle.SKU != "" {
			enriquecido.SKU = detalle.SKU
		}
	}
	if enriquecido.EanGtin == nil {
		jumbo.logger.Info("jumbo_ean_missing", "url", urlDetalle)
	}
	return jumbo.toItem(enriquecido, categoria, urlPagina), nil
}
