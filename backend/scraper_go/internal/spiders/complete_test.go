package spiders

import (
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

func TestCompleteItemDerivesSKUFromProductURL(t *testing.T) {
	productURL := "https://www.santaisabel.cl/mas-limonada-frambuesa-con-gas-1600cc-1993238/p"
	item := CompleteItem(domain.Item{
		Product:      domain.Product{Producto: "Agua saborizada", URLProducto: &productURL},
		Supermercado: "Santa Isabel",
	})
	if item.SKU == nil || *item.SKU != "1993238" {
		t.Errorf("sku = %v, want 1993238", item.SKU)
	}
}

func TestCompleteItemKeepsExistingSKU(t *testing.T) {
	productURL := "https://cugat.cl/producto/leche-entera/"
	sku := "SKU-1"
	item := CompleteItem(domain.Item{
		Product: domain.Product{Producto: "Leche", SKU: &sku, URLProducto: &productURL},
	})
	if item.SKU == nil || *item.SKU != "SKU-1" {
		t.Errorf("sku = %v, want SKU-1", item.SKU)
	}
}

func TestCompleteItemTruncatesSKU(t *testing.T) {
	productURL := "https://cugat.cl/producto/leche-entera/"
	sku := strings.Repeat("s", 120)
	item := CompleteItem(domain.Item{
		Product: domain.Product{Producto: "Leche", SKU: &sku, URLProducto: &productURL},
	})
	if item.SKU == nil || len(*item.SKU) != 100 {
		t.Errorf("sku = %v", item.SKU)
	}
}

func TestCompleteItemTruncatesAllCatalogFields(t *testing.T) {
	productURL := "https://www.jumbo.cl/producto-con-descripcion-larga-1993238/p"
	long := func(size int) string { return strings.Repeat("á", size) }
	item := CompleteItem(domain.Item{Product: domain.Product{
		Producto:          long(300),
		SKU:               domain.StringPtr(long(120)),
		EanGtin:           domain.StringPtr(long(60)),
		Marca:             domain.StringPtr(long(120)),
		Categoria:         long(120),
		FormatoCrudo:      domain.StringPtr(long(120)),
		MecanicaPromocion: domain.StringPtr(long(300)),
		URLProducto:       &productURL,
	}})

	checks := []struct {
		name  string
		value string
		limit int
	}{
		{"producto", item.Producto, 255},
		{"sku", *item.SKU, 100},
		{"ean_gtin", *item.EanGtin, 50},
		{"marca", *item.Marca, 100},
		{"categoria", item.Categoria, 100},
		{"formato_crudo", *item.FormatoCrudo, 100},
		{"mecanica_promocion", *item.MecanicaPromocion, 255},
	}
	for _, check := range checks {
		if len([]rune(check.value)) != check.limit {
			t.Errorf("%s = %d runas, want %d", check.name, len([]rune(check.value)), check.limit)
		}
	}
}

func TestCompleteItemDerivesSKUBeforeTruncating(t *testing.T) {
	productURL := "https://www.santaisabel.cl/producto-sin-sku-1993238/p"
	item := CompleteItem(domain.Item{Product: domain.Product{
		Producto:    "Producto sin SKU",
		URLProducto: &productURL,
	}})
	if item.SKU == nil || *item.SKU != "1993238" {
		t.Errorf("sku = %v, want 1993238", item.SKU)
	}
}
