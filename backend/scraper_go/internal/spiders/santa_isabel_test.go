package spiders

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

func santaIsabelBody(t *testing.T, payload any) []byte {
	t.Helper()
	inner, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	outer, err := json.Marshal(string(inner))
	if err != nil {
		t.Fatal(err)
	}
	return []byte("<script>window.__renderData = " + string(outer) + ";</script>")
}

func santaIsabelPayload() map[string]any {
	return map[string]any{
		"plp": map[string]any{
			"plp_products": map[string]any{
				"products": []any{
					map[string]any{
						"productId":   "product-1",
						"productName": "Cafe molido 250 g",
						"brand":       "Marca Test",
						"linkText":    "cafe-molido-250-g-product-1",
						"items": []any{
							map[string]any{
								"itemId":          "sku-1",
								"ean":             "7800000000001",
								"measurementUnit": "un",
								"unitMultiplier":  1,
								"images": []any{
									map[string]any{"imageUrl": "https://img.test/cafe.jpg"},
								},
								"sellers": []any{
									map[string]any{
										"sellerName": "santaisabel",
										"commertialOffer": map[string]any{
											"Price":                "$990",
											"ListPrice":            "1.290",
											"PriceWithoutDiscount": "1290.00",
											"AvailableQuantity":    3,
											"teasers":              []any{map[string]any{"name": "2x1"}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestExtractSantaIsabelProducts(t *testing.T) {
	body := santaIsabelBody(t, santaIsabelPayload())
	products := extractSantaIsabelProducts(body, mustURL("https://www.santaisabel.cl/despensa"))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	if product.Producto != "Cafe molido 250 g" {
		t.Errorf("producto = %q", product.Producto)
	}
	if product.Precio == nil || *product.Precio != 990 {
		t.Errorf("precio = %v", product.Precio)
	}
	if product.PrecioNormal == nil || *product.PrecioNormal != 1290 {
		t.Errorf("precio_normal = %v", product.PrecioNormal)
	}
	if product.PrecioOferta == nil || *product.PrecioOferta != 990 {
		t.Errorf("precio_oferta = %v", product.PrecioOferta)
	}
	if product.EanGtin == nil || *product.EanGtin != "7800000000001" {
		t.Errorf("ean = %v", product.EanGtin)
	}
	if product.SKU == nil || *product.SKU != "sku-1" {
		t.Errorf("sku = %v", product.SKU)
	}
	if product.Marca == nil || *product.Marca != "Marca Test" {
		t.Errorf("marca = %v", product.Marca)
	}
	if product.FormatoCrudo == nil || *product.FormatoCrudo != "1 un" {
		t.Errorf("formato = %v", product.FormatoCrudo)
	}
	if product.MecanicaPromocion == nil || *product.MecanicaPromocion != "2x1" {
		t.Errorf("mecánica = %v", product.MecanicaPromocion)
	}
	if product.EnStock == nil || !*product.EnStock {
		t.Errorf("en_stock = %v", product.EnStock)
	}
	if product.URLProducto == nil || *product.URLProducto != "https://www.santaisabel.cl/cafe-molido-250-g-product-1/p" {
		t.Errorf("url = %v", product.URLProducto)
	}
	if product.Imagen == nil || *product.Imagen != "https://img.test/cafe.jpg" {
		t.Errorf("imagen = %v", product.Imagen)
	}
}

func TestSantaIsabelOnlyUsesOwnSeller(t *testing.T) {
	payload := map[string]any{
		"plp": map[string]any{
			"plp_products": map[string]any{
				"products": []any{
					map[string]any{
						"productName": "Cafe",
						"items": []any{
							map[string]any{
								"itemId": "sku-1",
								"sellers": []any{
									map[string]any{
										"sellerName":      "other-seller",
										"commertialOffer": map[string]any{"Price": 990},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	body := santaIsabelBody(t, payload)
	if products := extractSantaIsabelProducts(body, mustURL("https://www.santaisabel.cl/despensa")); len(products) != 0 {
		t.Fatalf("productos = %+v, want 0", products)
	}
}

func TestExtractSantaIsabelWithoutRenderData(t *testing.T) {
	if products := extractSantaIsabelProducts([]byte("<html></html>"), mustURL("https://www.santaisabel.cl/despensa")); len(products) != 0 {
		t.Fatalf("productos = %+v, want 0", products)
	}
}

func TestSantaIsabelCategoryPagination(t *testing.T) {
	categoryDir := t.TempDir()
	categoryURL := "https://www.santaisabel.cl/despensa"
	if err := os.WriteFile(filepath.Join(categoryDir, "santa_isabel_categories.txt"), []byte(categoryURL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	body := santaIsabelBody(t, santaIsabelPayload())
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		categoryURL: response(categoryURL, string(body)),
	}}
	spider := NewSantaIsabel(fetcher, categoryDir, 100, quietLogger())
	var items []domain.Item
	err := spider.Run(context.Background(), domain.Job{}, func(item domain.Item) error {
		items = append(items, item)
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
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

func TestSantaIsabelAddURLCreatesCategoryFile(t *testing.T) {
	categoryDir := t.TempDir()
	extraURL := "https://www.santaisabel.cl/extra"
	fetcher := &fakeFetcher{}
	spider := NewSantaIsabel(fetcher, categoryDir, 100, quietLogger())
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

func TestSantaIsabelRejectsUntrustedCategory(t *testing.T) {
	categoryDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(categoryDir, "santa_isabel_categories.txt"),
		[]byte("https://example.com/despensa\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeFetcher{}
	spider := NewSantaIsabel(fetcher, categoryDir, 100, quietLogger())
	if err := spider.Run(context.Background(), domain.Job{}, func(domain.Item) error { return nil }); err == nil {
		t.Fatal("Run() debió rechazar una categoría de otro dominio")
	}
	if len(fetcher.calls) != 0 {
		t.Errorf("requests = %v", fetcher.calls)
	}
}

func TestSantaIsabelRobotsDisallowed(t *testing.T) {
	categoryDir := t.TempDir()
	categoryURL := "https://www.santaisabel.cl/despensa"
	if err := os.WriteFile(filepath.Join(categoryDir, "santa_isabel_categories.txt"), []byte(categoryURL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeFetcher{errs: map[string]error{
		categoryURL: fmt.Errorf("%w: %s", crawler.ErrRobotsDisallowed, categoryURL),
	}}
	spider := NewSantaIsabel(fetcher, categoryDir, 100, quietLogger())
	if err := spider.Run(context.Background(), domain.Job{}, func(domain.Item) error { return nil }); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}
