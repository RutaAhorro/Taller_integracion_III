package extract_test

import (
	"net/url"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

func mustURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", rawURL, err)
	}
	return parsed
}

func floatPtr(value float64) *float64 {
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func floatValue(value *float64) (float64, bool) {
	if value == nil {
		return 0, false
	}
	return *value, true
}

func boolValue(value *bool) (bool, bool) {
	if value == nil {
		return false, false
	}
	return *value, true
}

func assertProductFields(t *testing.T, product domain.Product, checks map[string]string) {
	t.Helper()
	for name, want := range checks {
		var got string
		switch name {
		case "producto":
			got = product.Producto
		case "sku":
			got = stringValue(product.SKU)
		case "ean":
			got = stringValue(product.EanGtin)
		case "marca":
			got = stringValue(product.Marca)
		case "formato":
			got = stringValue(product.FormatoCrudo)
		case "promocion":
			got = stringValue(product.MecanicaPromocion)
		case "imagen":
			got = stringValue(product.Imagen)
		case "url":
			got = stringValue(product.URLProducto)
		case "categoria":
			got = product.Categoria
		}
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}
