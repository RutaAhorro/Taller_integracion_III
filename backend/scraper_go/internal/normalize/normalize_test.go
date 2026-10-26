package normalize

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPrecioCLP(t *testing.T) {
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
		got := PrecioCLP(tc.value)
		if got == nil || *got != tc.want {
			t.Errorf("PrecioCLP(%v) = %v, want %v", tc.value, got, tc.want)
		}
	}

	for _, value := range []any{nil, true, "", "$abc", -1, math.Inf(1)} {
		if got := PrecioCLP(value); got != nil {
			t.Errorf("PrecioCLP(%v) = %v, want nil", value, *got)
		}
	}
}

func TestEAN(t *testing.T) {
	cases := []struct {
		value any
		want  string
	}{
		{" 00-7800000000001 ", "007800000000001"},
		{7800000000001, "7800000000001"},
	}
	for _, tc := range cases {
		got := EAN(tc.value)
		if got == nil || *got != tc.want {
			t.Errorf("EAN(%v) = %v, want %q", tc.value, got, tc.want)
		}
	}
	if got := EAN("EAN-unknown"); got != nil {
		t.Errorf("EAN inválido = %v, want nil", *got)
	}
}

func TestTexto(t *testing.T) {
	got := Texto("  Leche\n entera   1 L ")
	if got == nil || *got != "Leche entera 1 L" {
		t.Errorf("Texto = %v", got)
	}
	if got := Texto(" \n "); got != nil {
		t.Errorf("Texto vacío = %v, want nil", *got)
	}
}

func TestTruncar(t *testing.T) {
	if got := Truncar("Leche entera", 100); got != "Leche entera" {
		t.Errorf("Truncar corto = %q", got)
	}
	if got := Truncar("Leche entera", 5); got != "Leche" {
		t.Errorf("Truncar = %q, want %q", got, "Leche")
	}
	got := Truncar("Bebida azúcar 1 L", 8)
	if got != "Bebida a" || !utf8.ValidString(got) {
		t.Errorf("Truncar UTF-8 = %q, válido=%v", got, utf8.ValidString(got))
	}
	if got := Truncar("Leche", 0); got != "" {
		t.Errorf("Truncar con límite 0 = %q", got)
	}
	value := strings.Repeat("a", 120)
	if got := TruncarPtr(&value, 100); got == nil || len(*got) != 100 {
		t.Errorf("TruncarPtr = %v", got)
	}
	if got := TruncarPtr(nil, 100); got != nil {
		t.Errorf("TruncarPtr(nil) = %v", got)
	}
}

func TestSKUFromURL(t *testing.T) {
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
		got := SKUFromURL(tc.url)
		if tc.want == "" {
			if got != nil {
				t.Errorf("SKUFromURL(%q) = %q, want nil", tc.url, *got)
			}
			continue
		}
		if got == nil || *got != tc.want {
			t.Errorf("SKUFromURL(%q) = %v, want %q", tc.url, got, tc.want)
		}
	}
}
