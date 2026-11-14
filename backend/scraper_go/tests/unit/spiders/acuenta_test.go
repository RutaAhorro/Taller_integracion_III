package spiders_test

import (
	"context"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
)

func TestAcuentaPaginacionDeCategoria(t *testing.T) {
	categoryURL := "https://www.acuenta.cl/ca/congelados/04"
	t.Setenv("ACUENTA_CATEGORY_URLS", categoryURL)
	page1 := categoryURL + "?currentPage=1"
	page2 := categoryURL + "?currentPage=2"
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		page1: response(page1, `<div class="product-card"><a href="/p/uno-1">Uno 1 L</a><div>$1.000</div></div>`),
		page2: response(page2, `<div class="product-card"><a href="/p/dos-2">Dos 1 L</a><div>$2.000</div></div>`),
	}}
	spider := spiders.NewAcuenta(fetcher, t.TempDir(), 100, quietLogger())
	items := collect(t, spider, domain.Job{})
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	for _, item := range items {
		if item.Categoria != "congelados" || item.Supermercado != "A Cuenta" {
			t.Errorf("item = %+v", item)
		}
	}
}

func TestAcuentaProductMode(t *testing.T) {
	productURL := "https://www.acuenta.cl/p/helado-chocolate-1-l-savory-1075736"
	body := `<script>self.__next_f.push([1,` +
		`\"sku\":\"1075736\",\"name\":\"Helado Chocolate Suizo 1 L Savory\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"helado-chocolate-1-l-savory-1075736\",\"brand\":\"Savory\",\"stock\":1,` +
		`\"priceBeforeTaxes\":3490,\"promotion\":null` +
		`])</script>`
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		productURL: response(productURL, body),
	}}
	spider := spiders.NewAcuenta(fetcher, t.TempDir(), 100, quietLogger())
	items := collect(t, spider, domain.Job{ProductURL: productURL})
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	item := items[0]
	if item.Producto != "Helado Chocolate 1 L Savory" || item.Categoria != "producto" || item.Supermercado != "A Cuenta" {
		t.Errorf("item = %+v", item)
	}
	if item.URLProducto == nil || *item.URLProducto != productURL {
		t.Errorf("url = %v", item.URLProducto)
	}
	if len(fetcher.calls) != 1 || fetcher.calls[0] != productURL {
		t.Errorf("calls = %v, want sólo la ficha", fetcher.calls)
	}
}

func TestAcuentaRechazaProductURLQueNoEsFicha(t *testing.T) {
	fetcher := &fakeFetcher{}
	spider := spiders.NewAcuenta(fetcher, t.TempDir(), 100, quietLogger())
	err := spider.Run(context.Background(), domain.Job{ProductURL: "https://www.acuenta.cl/ca/congelados/04"}, func(domain.Item) error {
		return nil
	})
	if err == nil {
		t.Fatal("Run() debió rechazar una categoría como product_url")
	}
	if len(fetcher.calls) != 0 {
		t.Errorf("requests = %v", fetcher.calls)
	}
}
