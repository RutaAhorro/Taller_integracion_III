package normalize

import (
	"math"
	"testing"
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
