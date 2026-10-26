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
