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

func santaIsabelCategoryBody() string {
	return `<script type="application/ld+json">
{"@type":"ItemList","itemListElement":[
 {"@type":"ListItem","position":1,"item":{"@type":"Product","name":"Cafe molido 250 g",
 "url":"https://www.santaisabel.cl/cafe-molido-250-g/p",
 "brand":{"@type":"Brand","name":"Marca Test"},
 "offers":{"price":990,"availability":"https://schema.org/InStock"}}}]}
</script>`
}

func TestSantaIsabelPaginacionDeCategoria(t *testing.T) {
	categoryDir := t.TempDir()
	categoryURL := "https://www.santaisabel.cl/despensa"
	if err := os.WriteFile(filepath.Join(categoryDir, "santa_isabel_categories.txt"), []byte(categoryURL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		categoryURL: response(categoryURL, santaIsabelCategoryBody()),
	}}
	spider := spiders.NewSantaIsabel(fetcher, categoryDir, 100, quietLogger())
	items := collect(t, spider, domain.Job{})
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Categoria != "despensa" || items[0].Supermercado != "Santa Isabel" {
		t.Errorf("categoría/supermercado = %q/%q", items[0].Categoria, items[0].Supermercado)
	}
	if len(fetcher.calls) != 2 || fetcher.calls[1] != categoryURL+"?page=2" {
		t.Errorf("calls = %v", fetcher.calls)
	}
}

func TestSantaIsabelAddURLCreaElArchivoDeCategorias(t *testing.T) {
	categoryDir := t.TempDir()
	extraURL := "https://www.santaisabel.cl/extra"
	spider := spiders.NewSantaIsabel(&fakeFetcher{}, categoryDir, 100, quietLogger())
	if err := spider.Run(context.Background(), domain.Job{AddURL: extraURL}, func(domain.Item) error { return nil }); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	body, err := os.ReadFile(filepath.Join(categoryDir, "santa_isabel_categories.txt"))
	if err != nil {
		t.Fatalf("leer categorías: %v", err)
	}
	if string(body) != extraURL+"\n" {
		t.Errorf("archivo = %q", body)
	}
}

func TestSantaIsabelRechazaCategoriaNoConfiable(t *testing.T) {
	categoryDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(categoryDir, "santa_isabel_categories.txt"),
		[]byte("https://example.com/despensa\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeFetcher{}
	spider := spiders.NewSantaIsabel(fetcher, categoryDir, 100, quietLogger())
	if err := spider.Run(context.Background(), domain.Job{}, func(domain.Item) error { return nil }); err == nil {
		t.Fatal("Run() debió rechazar una categoría de otro dominio")
	}
	if len(fetcher.calls) != 0 {
		t.Errorf("requests = %v", fetcher.calls)
	}
}

func TestSantaIsabelOmiteCategoriaConRobotsProhibido(t *testing.T) {
	categoryDir := t.TempDir()
	categoryURL := "https://www.santaisabel.cl/despensa"
	if err := os.WriteFile(filepath.Join(categoryDir, "santa_isabel_categories.txt"), []byte(categoryURL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeFetcher{errs: map[string]error{
		categoryURL: fmt.Errorf("%w: %s", crawler.ErrRobotsDisallowed, categoryURL),
	}}
	spider := spiders.NewSantaIsabel(fetcher, categoryDir, 100, quietLogger())
	if err := spider.Run(context.Background(), domain.Job{}, func(domain.Item) error { return nil }); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestSantaIsabelProductMode(t *testing.T) {
	productURL := "https://www.santaisabel.cl/leche-colun-1-l/p"
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		productURL: response(productURL, `<script type="application/ld+json">
{"@type":"Product","name":"Leche Colún 1 L","url":"/leche-colun-1-l/p",
 "gtin13":"7800000000001","sku":"SI-1","brand":{"name":"Colún"},
 "offers":{"price":"1290","listPrice":"1590","availability":"https://schema.org/InStock"}}
</script>`),
	}}
	spider := spiders.NewSantaIsabel(fetcher, t.TempDir(), 100, quietLogger())
	items := collect(t, spider, domain.Job{ProductURL: productURL})
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	item := items[0]
	if item.Producto != "Leche Colún 1 L" || item.Categoria != "producto" || item.Supermercado != "Santa Isabel" {
		t.Errorf("item = %+v", item)
	}
	if item.URLProducto == nil || *item.URLProducto != productURL {
		t.Errorf("url = %v", item.URLProducto)
	}
	if len(fetcher.calls) != 1 || fetcher.calls[0] != productURL {
		t.Errorf("calls = %v, want sólo la ficha", fetcher.calls)
	}
}

func TestSantaIsabelProductModeSeleccionaLaFichaSolicitada(t *testing.T) {
	productURL := "https://www.santaisabel.cl/leche-colun-1-l/p"
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		productURL: response(productURL, `<script type="application/ld+json">
{"@graph":[
 {"@type":"Product","name":"Pan","url":"https://www.santaisabel.cl/pan/p","offers":{"price":"900"}},
 {"@type":"Product","name":"Leche Colún","url":"https://www.santaisabel.cl/leche-colun-1-l/p","offers":{"price":"1290"}}]}
</script>`),
	}}
	spider := spiders.NewSantaIsabel(fetcher, t.TempDir(), 100, quietLogger())
	items := collect(t, spider, domain.Job{ProductURL: productURL})
	if len(items) != 1 || items[0].Producto != "Leche Colún" {
		t.Fatalf("items = %+v", items)
	}
}

func TestSantaIsabelRechazaProductURLQueNoEsFicha(t *testing.T) {
	fetcher := &fakeFetcher{}
	spider := spiders.NewSantaIsabel(fetcher, t.TempDir(), 100, quietLogger())
	err := spider.Run(context.Background(), domain.Job{ProductURL: "https://www.santaisabel.cl/despensa"}, func(domain.Item) error {
		return nil
	})
	if err == nil {
		t.Fatal("Run() debió rechazar una categoría como product_url")
	}
	if len(fetcher.calls) != 0 {
		t.Errorf("requests = %v", fetcher.calls)
	}
}
