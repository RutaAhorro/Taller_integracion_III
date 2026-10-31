package repositories

import "testing"

func TestEscaparPatronLike(t *testing.T) {
	casos := []struct {
		entrada  string
		esperado string
	}{
		{"normal", "normal"},
		{"100%", "100!%"},
		{"a_b", "a!_b"},
		{"x!y", "x!!y"},
		{"%_!", "!%!_!!"},
	}

	for _, caso := range casos {
		t.Run(caso.entrada, func(t *testing.T) {
			if obtenido := escaparPatronLike(caso.entrada); obtenido != caso.esperado {
				t.Fatalf("escaparPatronLike(%q) = %q, se esperaba %q", caso.entrada, obtenido, caso.esperado)
			}
		})
	}
}
