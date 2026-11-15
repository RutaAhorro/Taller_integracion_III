package normalize_test

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

func TestPrecioCLP(prueba *testing.T) {
	cases := []struct {
		value any
		want  float64
	}{
		{"$1.290", 1290},
		{"1290", 1290},
		{"1290.00", 1290},
		{"$1.290,50", 1290.5},
		{"1,290.50", 1290.5},
		{1290, 1290},
		{1290.0, 1290},
	}
	for _, tc := range cases {
		got := normalize.PrecioCLP(tc.value)
		if got == nil || *got != tc.want {
			prueba.Errorf("PrecioCLP(%v) = %v, want %v", tc.value, got, tc.want)
		}
	}

	for _, value := range []any{nil, true, "", "$abc", -1, math.Inf(1)} {
		if got := normalize.PrecioCLP(value); got != nil {
			prueba.Errorf("PrecioCLP(%v) = %v, want nil", value, *got)
		}
	}
}

func TestEAN(prueba *testing.T) {
	cases := []struct {
		value any
		want  string
	}{
		{" 00-7800000000001 ", "007800000000001"},
		{7800000000001, "7800000000001"},
	}
	for _, tc := range cases {
		got := normalize.EAN(tc.value)
		if got == nil || *got != tc.want {
			prueba.Errorf("EAN(%v) = %v, want %q", tc.value, got, tc.want)
		}
	}
	if got := normalize.EAN("EAN-unknown"); got != nil {
		prueba.Errorf("EAN inválido = %v, want nil", *got)
	}
}

func TestTexto(prueba *testing.T) {
	got := normalize.Texto("  Leche\n entera   1 L ")
	if got == nil || *got != "Leche entera 1 L" {
		prueba.Errorf("Texto = %v", got)
	}
	if got := normalize.Texto(" \n "); got != nil {
		prueba.Errorf("Texto vacío = %v, want nil", *got)
	}
}

func TestTruncar(prueba *testing.T) {
	if got := normalize.Truncar("Leche entera", 100); got != "Leche entera" {
		prueba.Errorf("Truncar corto = %q", got)
	}
	if got := normalize.Truncar("Leche entera", 5); got != "Leche" {
		prueba.Errorf("Truncar = %q, want %q", got, "Leche")
	}
	got := normalize.Truncar("Bebida azúcar 1 L", 8)
	if got != "Bebida a" || !utf8.ValidString(got) {
		prueba.Errorf("Truncar UTF-8 = %q, válido=%v", got, utf8.ValidString(got))
	}
	if got := normalize.Truncar("Leche", 0); got != "" {
		prueba.Errorf("Truncar con límite 0 = %q", got)
	}
	value := strings.Repeat("a", 120)
	if got := normalize.TruncarPtr(&value, 100); got == nil || len(*got) != 100 {
		prueba.Errorf("TruncarPtr = %v", got)
	}
	if got := normalize.TruncarPtr(nil, 100); got != nil {
		prueba.Errorf("TruncarPtr(nil) = %v", got)
	}
}

func TestSKUFromURL(prueba *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://www.santaisabel.cl/mas-limonada-frambuesa-con-gas-1600cc-1993238/p", "1993238"},
		{"https://www.santaisabel.cl/mas-limonada-frambuesa-con-gas-1600cc-1993238/p/", "1993238"},
		{"https://cugat.cl/producto/leche-entera-pitrufquen-1lt/", "leche-entera-pitrufquen-1lt"},
		{"https://www.jumbo.cl/palta-hass-jumbo-malla-1-kg/p?foo=bar", "palta-hass-jumbo-malla-1-kg"},
		{"https://example.test/producto/12345", "12345"},
		{"", ""},
	}
	for _, tc := range cases {
		got := normalize.SKUFromURL(tc.url)
		if tc.want == "" {
			if got != nil {
				prueba.Errorf("SKUFromURL(%q) = %q, want nil", tc.url, *got)
			}
			continue
		}
		if got == nil || *got != tc.want {
			prueba.Errorf("SKUFromURL(%q) = %v, want %q", tc.url, got, tc.want)
		}
	}
}

func TestLimitarProductoRecortaColumnasYDerivaSKU(prueba *testing.T) {
	long := strings.Repeat("á", 400)
	productURL := "https://www.santaisabel.cl/producto-1993238/p"
	product := normalize.LimitarProducto(domain.Product{
		Producto:          long,
		SKU:               domain.StringPtr(strings.Repeat("s", 120)),
		EanGtin:           domain.StringPtr(strings.Repeat("9", 60)),
		Marca:             domain.StringPtr(long),
		Categoria:         long,
		FormatoCrudo:      domain.StringPtr(long),
		MecanicaPromocion: domain.StringPtr(long),
		URLProducto:       &productURL,
	})

	checks := []struct {
		name  string
		value string
		limit int
	}{
		{"producto", product.Producto, 255},
		{"sku", *product.SKU, 100},
		{"ean_gtin", *product.EanGtin, 50},
		{"marca", *product.Marca, 100},
		{"categoria", product.Categoria, 100},
		{"formato_crudo", *product.FormatoCrudo, 100},
		{"mecanica_promocion", *product.MecanicaPromocion, 255},
	}
	for _, check := range checks {
		if !utf8.ValidString(check.value) {
			prueba.Errorf("%s no es UTF-8 válido", check.name)
		}
		if got := utf8.RuneCountInString(check.value); got != check.limit {
			prueba.Errorf("%s tiene %d caracteres, want %d", check.name, got, check.limit)
		}
	}

	derived := normalize.LimitarProducto(domain.Product{
		Producto:    "Producto sin SKU",
		URLProducto: &productURL,
	})
	if derived.SKU == nil || *derived.SKU != "1993238" {
		prueba.Errorf("SKU derivado = %v, want 1993238", derived.SKU)
	}
}
