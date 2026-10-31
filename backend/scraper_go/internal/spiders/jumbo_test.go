package spiders

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

type fakeFetcher struct {
	responses map[string]*crawler.Response
	errs      map[string]error
	calls     []string
}

func (f *fakeFetcher) Fetch(_ context.Context, rawURL string) (*crawler.Response, error) {
	f.calls = append(f.calls, rawURL)
	if err, ok := f.errs[rawURL]; ok {
		return nil, err
	}
	if response, ok := f.responses[rawURL]; ok {
		return response, nil
	}
	return &crawler.Response{URL: mustURL(rawURL), Status: 404}, nil
}

func response(rawURL, body string) *crawler.Response {
	return &crawler.Response{URL: mustURL(rawURL), Status: 200, Body: []byte(body)}
}

func mustURL(rawURL string) *url.URL {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return parsed
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestJumboEmptyCategoryDoesNotAbortRemainingCategories(t *testing.T) {
	categoryDir := t.TempDir()
	emptyURL := "https://www.jumbo.cl/categoria-vacia"
	listedURL := "https://www.jumbo.cl/frutas"
	if err := os.WriteFile(
		filepath.Join(categoryDir, "jumbo_categories.txt"),
		[]byte(emptyURL+"\n"+listedURL+"\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		emptyURL: response(emptyURL, `<html><body><h1>Sin productos esta temporada</h1></body></html>`),
		listedURL: response(listedURL, `<script type="application/ld+json">
{"@type":"Product","name":"Tomate","url":"/p/tomate","offers":{"price":"1290"}}
</script>`),
	}}
	spider := NewJumbo(fetcher, categoryDir, 100, quietLogger())
	var items []domain.Item
	err := spider.Run(context.Background(), domain.Job{}, func(item domain.Item) error {
		items = append(items, item)
		return nil
	})
	if err != nil {
		t.Fatalf("una categoría vacía no debe abortar el trabajo: %v", err)
	}
	if len(items) != 1 || items[0].Producto != "Tomate" {
		t.Fatalf("items = %+v", items)
	}
	if len(fetcher.calls) < 2 || fetcher.calls[0] != emptyURL || fetcher.calls[1] != listedURL {
		t.Errorf("calls = %v, want la categoría vacía y luego la siguiente", fetcher.calls)
	}
}

func TestExtractJumboProducts(t *testing.T) {
	body := `<script type="application/ld+json">
{"@graph":[{"item":{"@type":"Product","name":"Tomate Larga Vida",
"image":["https://img.test/tomate.jpg"],"offers":{"price":"$1.290"}}}]}
</script>`
	products := extractJumboProducts([]byte(body))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	if product.Producto != "Tomate Larga Vida" {
		t.Errorf("producto = %q", product.Producto)
	}
	if product.Precio == nil || *product.Precio != 1290 {
		t.Errorf("precio = %v", product.Precio)
	}
	if product.PrecioNormal == nil || *product.PrecioNormal != 1290 {
		t.Errorf("precio_normal = %v", product.PrecioNormal)
	}
	if product.PrecioOferta == nil || *product.PrecioOferta != 1290 {
		t.Errorf("precio_oferta = %v", product.PrecioOferta)
	}
	if product.Imagen == nil || *product.Imagen != "https://img.test/tomate.jpg" {
		t.Errorf("imagen = %v", product.Imagen)
	}
}

func TestExtractJumboComparisonFields(t *testing.T) {
	body := `<script type="application/ld+json">
{"@type":"Product","name":"Lechuga","gtin13":"7800000000001","sku":"SKU-1",
"brand":{"name":"Marca Test"},"description":"1 unidad","url":"https://jumbo.cl/p/lechuga",
"offers":{"price":"990","availability":"https://schema.org/InStock","offerDescription":"2x1"}}
</script>`
	products := extractJumboProducts([]byte(body))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	if product.EanGtin == nil || *product.EanGtin != "7800000000001" {
		t.Errorf("ean = %v", product.EanGtin)
	}
	if product.SKU == nil || *product.SKU != "SKU-1" {
		t.Errorf("sku = %v", product.SKU)
	}
	if product.Marca == nil || *product.Marca != "Marca Test" {
		t.Errorf("marca = %v", product.Marca)
	}
	if product.FormatoCrudo == nil || *product.FormatoCrudo != "1 unidad" {
		t.Errorf("formato = %v", product.FormatoCrudo)
	}
	if product.MecanicaPromocion == nil || *product.MecanicaPromocion != "2x1" {
		t.Errorf("mecánica = %v", product.MecanicaPromocion)
	}
	if product.EnStock == nil || !*product.EnStock {
		t.Errorf("en_stock = %v", product.EnStock)
	}
	if product.URLProducto == nil || *product.URLProducto != "https://jumbo.cl/p/lechuga" {
		t.Errorf("url = %v", product.URLProducto)
	}
}

func TestExtractJumboNumericIdentifiers(t *testing.T) {
	body := `<script type="application/ld+json">
{"@type":"Product","name":"Numérico","gtin13":7800000000001,"sku":12345,
"offers":{"price":1290}}
</script>`
	products := extractJumboProducts([]byte(body))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	if products[0].EanGtin == nil || *products[0].EanGtin != "7800000000001" {
		t.Errorf("ean = %v", products[0].EanGtin)
	}
	if products[0].SKU == nil || *products[0].SKU != "12345" {
		t.Errorf("sku = %v", products[0].SKU)
	}

	floatBody := `<script type="application/ld+json">
{"@type":"Product","name":"Float","gtin13":7800000000001.0,
"offers":{"price":1290.0}}
</script>`
	products = extractJumboProducts([]byte(floatBody))
	if len(products) != 1 {
		t.Fatalf("productos float = %d, want 1", len(products))
	}
	if products[0].EanGtin != nil {
		t.Errorf("un EAN con decimales no debe aceptarse: %v", *products[0].EanGtin)
	}
	if products[0].Precio == nil || *products[0].Precio != 1290 {
		t.Errorf("precio = %v", products[0].Precio)
	}
}

func TestJumboProductMode(t *testing.T) {
	productURL := "https://www.jumbo.cl/p/lechuga"
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		productURL: response(productURL, `<script type="application/ld+json">
{"@type":"Product","name":"Lechuga","url":"https://www.jumbo.cl/p/lechuga","offers":{"price":"990"}}
</script>`),
	}}
	jumbo := NewJumbo(fetcher, t.TempDir(), 100, quietLogger())
	var items []domain.Item
	err := jumbo.Run(context.Background(), domain.Job{ProductURL: productURL}, func(item domain.Item) error {
		items = append(items, item)
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].Producto != "Lechuga" || items[0].Categoria != "producto" || items[0].Supermercado != "Jumbo" {
		t.Errorf("item = %+v", items[0])
	}
	if items[0].URLProducto == nil || *items[0].URLProducto != productURL {
		t.Errorf("url = %v", items[0].URLProducto)
	}
}

func TestJumboProductModeSelectsRequestedProduct(t *testing.T) {
	productURL := "https://www.jumbo.cl/p/lechuga"
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		productURL: response(productURL, `<script type="application/ld+json">
{"@graph":[
{"@type":"Product","name":"Tomate","url":"/p/tomate","offers":{"price":"1200"}},
{"@type":"Product","name":"Lechuga","url":"/p/lechuga","offers":{"price":"990"}}]}
</script>`),
	}}
	jumbo := NewJumbo(fetcher, t.TempDir(), 100, quietLogger())
	var items []domain.Item
	err := jumbo.Run(context.Background(), domain.Job{ProductURL: productURL}, func(item domain.Item) error {
		items = append(items, item)
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(items) != 1 || items[0].Producto != "Lechuga" {
		t.Fatalf("items = %+v", items)
	}
}

func TestJumboRejectsUntrustedProductURL(t *testing.T) {
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{}}
	jumbo := NewJumbo(fetcher, t.TempDir(), 100, quietLogger())
	err := jumbo.Run(context.Background(), domain.Job{ProductURL: "https://example.com/p/1"}, func(domain.Item) error {
		return nil
	})
	if err == nil {
		t.Fatal("Run() debió rechazar un dominio no confiable")
	}
	if len(fetcher.calls) != 0 {
		t.Errorf("se hicieron %d requests", len(fetcher.calls))
	}
}

func TestJumboCategoryPagination(t *testing.T) {
	categoryDir := t.TempDir()
	categoryURL := "https://www.jumbo.cl/cat"
	if err := os.WriteFile(filepath.Join(categoryDir, "jumbo_categories.txt"), []byte(categoryURL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		categoryURL: response(categoryURL, `<script type="application/ld+json">
{"@type":"Product","name":"Producto","url":"/p/producto","offers":{"price":"100"}}
</script>`),
	}}
	jumbo := NewJumbo(fetcher, categoryDir, 100, quietLogger())
	var items []domain.Item
	err := jumbo.Run(context.Background(), domain.Job{}, func(item domain.Item) error {
		items = append(items, item)
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	if len(fetcher.calls) != 2 || fetcher.calls[1] != categoryURL+"?page=2" {
		t.Errorf("calls = %v", fetcher.calls)
	}
}

func TestJumboEnrichesEANFromDetail(t *testing.T) {
	categoryDir := t.TempDir()
	categoryURL := "https://www.jumbo.cl/cat"
	if err := os.WriteFile(filepath.Join(categoryDir, "jumbo_categories.txt"), []byte(categoryURL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	detailURL := "https://www.jumbo.cl/cafe/p"
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		categoryURL: response(categoryURL, `<script type="application/ld+json">
{"@type":"Product","name":"Cafe","url":"https://www.jumbo.cl/cafe/p","offers":{"price":"990"}}
</script>`),
		detailURL: response(detailURL, `<script type="application/ld+json">
{"@type":"Product","name":"Cafe","url":"https://www.jumbo.cl/cafe/p",
"gtin13":"7800000000001","sku":"sku-detail","offers":{"price":"1200"}}
</script>`),
	}}
	jumbo := NewJumbo(fetcher, categoryDir, 100, quietLogger())
	var items []domain.Item
	err := jumbo.Run(context.Background(), domain.Job{EnrichEAN: true}, func(item domain.Item) error {
		items = append(items, item)
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	item := items[0]
	if item.EanGtin == nil || *item.EanGtin != "7800000000001" {
		t.Errorf("ean = %v", item.EanGtin)
	}
	if item.SKU == nil || *item.SKU != "sku-detail" {
		t.Errorf("sku = %v", item.SKU)
	}
	if item.Precio == nil || *item.Precio != 990 {
		t.Errorf("precio = %v", item.Precio)
	}
	if item.Categoria != "cat" || item.Supermercado != "Jumbo" {
		t.Errorf("categoría/supermercado = %q/%q", item.Categoria, item.Supermercado)
	}
}

func TestJumboRobotsDisallowedSkipsCategory(t *testing.T) {
	categoryDir := t.TempDir()
	categoryURL := "https://www.jumbo.cl/cat"
	if err := os.WriteFile(filepath.Join(categoryDir, "jumbo_categories.txt"), []byte(categoryURL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeFetcher{errs: map[string]error{
		categoryURL: fmt.Errorf("%w: %s", crawler.ErrRobotsDisallowed, categoryURL),
	}}
	jumbo := NewJumbo(fetcher, categoryDir, 100, quietLogger())
	err := jumbo.Run(context.Background(), domain.Job{}, func(domain.Item) error { return nil })
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestWithPage(t *testing.T) {
	got := withPage("https://www.jumbo.cl/frutas-y-verduras/verduras", 2)
	if got != "https://www.jumbo.cl/frutas-y-verduras/verduras?page=2" {
		t.Errorf("withPage() = %q", got)
	}
}

func TestCategoryFromURL(t *testing.T) {
	if got := categoryFromURL("https://www.jumbo.cl/frutas-y-verduras/verduras/"); got != "verduras" {
		t.Errorf("categoryFromURL() = %q", got)
	}
}

func TestValidateProductURL(t *testing.T) {
	if err := validateProductURL("https://www.jumbo.cl/p/lechuga"); err != nil {
		t.Errorf("validateProductURL() error = %v", err)
	}
	if err := validateProductURL("https://example.com/p/lechuga"); err == nil {
		t.Error("validateProductURL() aceptó un dominio ajeno")
	}
}

func TestSkuFromURL(t *testing.T) {
	got := skuFromURL("https://www.jumbo.cl/cafe/p")
	if got == nil || *got != "JUMBO-cafe" {
		t.Errorf("skuFromURL() = %v", got)
	}
}

func TestJumboCategoryFileReadsComments(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "jumbo_categories.txt")
	body := "# comentario\n\nhttps://www.jumbo.cl/a\nhttps://www.jumbo.cl/b\n"
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	urls := readCategoryURLs(file)
	if len(urls) != 2 || urls[0] != "https://www.jumbo.cl/a" || urls[1] != "https://www.jumbo.cl/b" {
		t.Errorf("urls = %v", urls)
	}
	if !strings.Contains(strings.Join(urls, ","), "jumbo") {
		t.Errorf("urls = %v", urls)
	}
}
