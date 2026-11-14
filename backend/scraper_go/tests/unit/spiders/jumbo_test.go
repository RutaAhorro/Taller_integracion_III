package spiders_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
)

func TestJumboProductMode(t *testing.T) {
	productURL := "https://www.jumbo.cl/p/lechuga"
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		productURL: response(productURL, `<script type="application/ld+json">
{"@type":"Product","name":"Lechuga","url":"https://www.jumbo.cl/p/lechuga","offers":{"price":"990"}}
</script>`),
	}}
	jumbo := spiders.NewJumbo(fetcher, t.TempDir(), 100, quietLogger())
	items := collect(t, jumbo, domain.Job{ProductURL: productURL})
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	item := items[0]
	if item.Producto != "Lechuga" || item.Categoria != "producto" || item.Supermercado != "Jumbo" {
		t.Errorf("item = %+v", item)
	}
	if item.URLProducto == nil || *item.URLProducto != productURL {
		t.Errorf("url = %v", item.URLProducto)
	}
}

func TestJumboProductModeSeleccionaLaFichaSolicitada(t *testing.T) {
	productURL := "https://www.jumbo.cl/p/lechuga"
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		productURL: response(productURL, `<script type="application/ld+json">
{"@graph":[
{"@type":"Product","name":"Tomate","url":"/p/tomate","offers":{"price":"1200"}},
{"@type":"Product","name":"Lechuga","url":"/p/lechuga","offers":{"price":"990"}}]}
</script>`),
	}}
	jumbo := spiders.NewJumbo(fetcher, t.TempDir(), 100, quietLogger())
	items := collect(t, jumbo, domain.Job{ProductURL: productURL})
	if len(items) != 1 || items[0].Producto != "Lechuga" {
		t.Fatalf("items = %+v", items)
	}
}

func TestJumboRechazaProductURLNoConfiable(t *testing.T) {
	fetcher := &fakeFetcher{}
	jumbo := spiders.NewJumbo(fetcher, t.TempDir(), 100, quietLogger())
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

func TestJumboPaginacionDeCategoria(t *testing.T) {
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
	jumbo := spiders.NewJumbo(fetcher, categoryDir, 100, quietLogger())
	items := collect(t, jumbo, domain.Job{})
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	if len(fetcher.calls) != 2 || fetcher.calls[1] != categoryURL+"?page=2" {
		t.Errorf("calls = %v", fetcher.calls)
	}
}

func TestJumboEnriqueceEANDesdeLaFicha(t *testing.T) {
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
	jumbo := spiders.NewJumbo(fetcher, categoryDir, 100, quietLogger())
	items := collect(t, jumbo, domain.Job{EnrichEAN: true})
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

func TestJumboOmiteCategoriaConRobotsProhibido(t *testing.T) {
	categoryDir := t.TempDir()
	categoryURL := "https://www.jumbo.cl/cat"
	if err := os.WriteFile(filepath.Join(categoryDir, "jumbo_categories.txt"), []byte(categoryURL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeFetcher{errs: map[string]error{
		categoryURL: fmt.Errorf("%w: %s", crawler.ErrRobotsDisallowed, categoryURL),
	}}
	jumbo := spiders.NewJumbo(fetcher, categoryDir, 100, quietLogger())
	if err := jumbo.Run(context.Background(), domain.Job{}, func(domain.Item) error { return nil }); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestJumboLeeElArchivoDeCategoriasConComentarios(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "jumbo_categories.txt")
	body := "# comentario\n\nhttps://www.jumbo.cl/a\nhttps://www.jumbo.cl/b\n"
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		"https://www.jumbo.cl/a": response("https://www.jumbo.cl/a", `<html></html>`),
		"https://www.jumbo.cl/b": response("https://www.jumbo.cl/b", `<html></html>`),
	}}
	spider := spiders.NewJumbo(fetcher, dir, 100, quietLogger())
	if err := spider.Run(context.Background(), domain.Job{}, func(domain.Item) error { return nil }); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(fetcher.calls) < 2 || fetcher.calls[0] != "https://www.jumbo.cl/a" || fetcher.calls[1] != "https://www.jumbo.cl/b" {
		t.Errorf("calls = %v", fetcher.calls)
	}
}
