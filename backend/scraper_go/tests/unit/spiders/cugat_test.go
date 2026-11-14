package spiders_test

import (
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
)

func cugatCategoryBody(name, path string, price string) string {
	return `<div class="products"><div class="product-small product">
<a href="/producto/` + path + `"><img data-src="/` + path + `.webp"></a>
<p class="product-title"><a href="/producto/` + path + `">` + name + `</a></p>
<ins><span class="woocommerce-Price-amount">` + price + `</span></ins>
</div></div>`
}

func TestCugatProductMode(t *testing.T) {
	productURL := "https://cugat.cl/producto/bebida-cugat/"
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		productURL: response(productURL, `<script type="application/ld+json">
{"@type":"Product","name":"Bebida Cugat","url":"https://cugat.cl/producto/bebida-cugat/","offers":{"price":"990"}}
</script>`),
	}}
	spider := spiders.NewCugat(fetcher, t.TempDir(), 100, quietLogger())
	items := collect(t, spider, domain.Job{ProductURL: productURL})
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].Producto != "Bebida Cugat" || items[0].Categoria != "producto" || items[0].Supermercado != "Cugat" {
		t.Errorf("item = %+v", items[0])
	}
}

func TestCugatPaginacionDeCategoria(t *testing.T) {
	categoryURL := "https://cugat.cl/categoria-producto/bebidas/"
	page2 := "https://cugat.cl/categoria-producto/bebidas/page/2/"
	t.Setenv("CUGAT_CATEGORY_URLS", categoryURL)
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		categoryURL: response(categoryURL, cugatCategoryBody("Uno", "uno", "$1.000")),
		page2:       response(page2, cugatCategoryBody("Dos", "dos", "$2.000")),
	}}
	spider := spiders.NewCugat(fetcher, t.TempDir(), 100, quietLogger())
	items := collect(t, spider, domain.Job{})
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	for _, item := range items {
		if item.Categoria != "bebidas" || item.Supermercado != "Cugat" {
			t.Errorf("item = %+v", item)
		}
	}
}
