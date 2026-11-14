package spiders_test

import (
	"context"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
)

func liderCategoryBody(id, name, price string) string {
	return `<div data-testid="item-stack"><div role="group" data-item-id="` + id + `" data-dca-id="` + id + `">
<a href="/ip/frutas/` + id + `"><img data-testid="productTileImage" src="/` + id + `.webp"></a>
<div data-automation-id="product-price"><div aria-hidden="true">` + price + `</div></div>
<span data-automation-id="product-title">` + name + `</span></div></div>`
}

func TestLiderPaginacionDeCategoria(t *testing.T) {
	categoryURL := "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836"
	t.Setenv("LIDER_CATEGORY_URLS", categoryURL)
	page2 := categoryURL + "?page=2"
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		categoryURL: response(categoryURL, liderCategoryBody("00000000000001", "Uno 1 kg", "$1.000")),
		page2:       response(page2, liderCategoryBody("00000000000002", "Dos 1 kg", "$2.000")),
	}}
	spider := spiders.NewLider(fetcher, t.TempDir(), 100, quietLogger())
	items := collect(t, spider, domain.Job{})
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	for _, item := range items {
		if item.Categoria != "frutas" || item.Supermercado != "Lider" {
			t.Errorf("item = %+v", item)
		}
	}
}

func TestLiderProductMode(t *testing.T) {
	productURL := "https://super.lider.cl/ip/leche-entera/12345678"
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		productURL: response(productURL, `<script type="application/ld+json">
{"@type":"Product","name":"Leche Entera 1 L","url":"/ip/leche-entera/12345678",
 "gtin13":"7800000000001","sku":"L-1","brand":{"name":"Lider"},
 "offers":{"price":"1290","availability":"https://schema.org/InStock"}}
</script>`),
	}}
	spider := spiders.NewLider(fetcher, t.TempDir(), 100, quietLogger())
	items := collect(t, spider, domain.Job{ProductURL: productURL})
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	item := items[0]
	if item.Producto != "Leche Entera 1 L" || item.Categoria != "producto" || item.Supermercado != "Lider" {
		t.Errorf("item = %+v", item)
	}
	if item.URLProducto == nil || *item.URLProducto != productURL {
		t.Errorf("url = %v", item.URLProducto)
	}
	if len(fetcher.calls) != 1 || fetcher.calls[0] != productURL {
		t.Errorf("calls = %v, want sólo la ficha", fetcher.calls)
	}
}

func TestLiderProductModeLeeNextData(t *testing.T) {
	productURL := "https://super.lider.cl/ip/leche-entera/12345678"
	nextData := `{"props":{"pageProps":{"product":{` +
		`"name":"Leche Entera 1 L","usItemId":"12345678","price":1290,` +
		`"canonicalUrl":"/ip/leche-entera/12345678","sku":"L-1","brand":"Lider",` +
		`"isOutOfStock":false,"imageInfo":{"thumbnailUrl":"/img/leche.webp"}}}}}`
	body := `<html><body><script id="__NEXT_DATA__" type="application/json">` + nextData + `</script></body></html>`
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		productURL: response(productURL, body),
	}}
	spider := spiders.NewLider(fetcher, t.TempDir(), 100, quietLogger())
	items := collect(t, spider, domain.Job{ProductURL: productURL})
	if len(items) != 1 || items[0].Producto != "Leche Entera 1 L" {
		t.Fatalf("items = %+v", items)
	}
	if items[0].Precio == nil || *items[0].Precio != 1290 {
		t.Errorf("precio = %v", items[0].Precio)
	}
	if items[0].URLProducto == nil || *items[0].URLProducto != productURL {
		t.Errorf("url = %v", items[0].URLProducto)
	}
}

func TestLiderRechazaProductURLQueNoEsFicha(t *testing.T) {
	fetcher := &fakeFetcher{}
	spider := spiders.NewLider(fetcher, t.TempDir(), 100, quietLogger())
	err := spider.Run(context.Background(), domain.Job{ProductURL: "https://super.lider.cl/browse/frutas/22884697_93034836"}, func(domain.Item) error {
		return nil
	})
	if err == nil {
		t.Fatal("Run() debió rechazar una categoría como product_url")
	}
	if len(fetcher.calls) != 0 {
		t.Errorf("requests = %v", fetcher.calls)
	}
}
