package extract_test

import (
	"regexp"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

func site(prueba *testing.T, name string) catalog.Site {
	prueba.Helper()
	site, ok := catalog.Default().Get(name)
	if !ok {
		prueba.Fatalf("falta el sitio %q en el catálogo", name)
	}
	return site
}

func TestPageURL(prueba *testing.T) {
	cases := []struct {
		name string
		raw  string
		page int
		want string
	}{
		{
			name: "jumbo_rsc",
			raw:  "https://www.jumbo.cl/frutas-y-verduras/verduras",
			page: 2,
			want: "https://www.jumbo.cl/frutas-y-verduras/verduras?page=2",
		},
		{
			name: "santa_isabel_rsc",
			raw:  "https://www.santaisabel.cl/despensa",
			page: 2,
			want: "https://www.santaisabel.cl/despensa?page=2",
		},
		{
			name: "cugat_rsc",
			raw:  "https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/",
			page: 2,
			want: "https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/page/2/",
		},
		{
			name: "cugat_rsc",
			raw:  "https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/page/2/",
			page: 3,
			want: "https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/page/3/",
		},
		{
			name: "acuenta_rsc",
			raw:  "https://www.acuenta.cl/ca/congelados/04?currentPage=2&_rsc=2x5uv",
			page: 3,
			want: "https://www.acuenta.cl/ca/congelados/04?currentPage=3",
		},
		{
			name: "acuenta_rsc",
			raw:  "https://www.acuenta.cl/ca/congelados/04",
			page: 1,
			want: "https://www.acuenta.cl/ca/congelados/04?currentPage=1",
		},
		{
			name: "lider_rsc",
			raw:  "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836?sort=price",
			page: 2,
			want: "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836?sort=price&page=2",
		},
		{
			name: "lider_rsc",
			raw:  "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836?page=2",
			page: 3,
			want: "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836?page=3",
		},
	}
	for _, tc := range cases {
		prueba.Run(tc.name+" "+tc.want, func(prueba *testing.T) {
			if got := extract.PageURL(site(prueba, tc.name), tc.raw, tc.page); got != tc.want {
				prueba.Errorf("PageURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStartPage(prueba *testing.T) {
	acuenta := site(prueba, "acuenta_rsc")
	if got := extract.StartPage(acuenta, "https://www.acuenta.cl/ca/congelados/04?currentPage=3"); got != 3 {
		prueba.Errorf("StartPage(acuenta) = %d, want 3", got)
	}
	if got := extract.StartPage(acuenta, "https://www.acuenta.cl/ca/congelados/04"); got != 1 {
		prueba.Errorf("StartPage(acuenta sin página) = %d, want 1", got)
	}

	lider := site(prueba, "lider_rsc")
	if got := extract.StartPage(lider, "https://super.lider.cl/browse/frutas/22884697?page=2"); got != 2 {
		prueba.Errorf("StartPage(lider) = %d, want 2", got)
	}

	jumbo := site(prueba, "jumbo_rsc")
	if got := extract.StartPage(jumbo, "https://www.jumbo.cl/frutas?page=4"); got != 1 {
		prueba.Errorf("StartPage(jumbo) = %d, want 1", got)
	}
}

func TestCategoryFromURL(prueba *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"jumbo_rsc", "https://www.jumbo.cl/frutas-y-verduras/verduras/", "verduras"},
		{"santa_isabel_rsc", "https://www.santaisabel.cl/despensa", "despensa"},
		{"cugat_rsc", "https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/page/2/", "bebidas-jugos-y-aguas"},
		{"acuenta_rsc", "https://www.acuenta.cl/ca/congelados/04", "congelados"},
		{"lider_rsc", "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836", "frutas"},
	}
	for _, tc := range cases {
		prueba.Run(tc.name, func(prueba *testing.T) {
			if got := extract.CategoryFromURL(site(prueba, tc.name), tc.raw); got != tc.want {
				prueba.Errorf("CategoryFromURL() = %q, want %q", got, tc.want)
			}
		})
	}

	cugat := site(prueba, "cugat_rsc")
	if got := extract.CategoryFromURL(cugat, "https://cugat.cl/"); got != "sin_categoria" {
		prueba.Errorf("CategoryFromURL() fallback = %q, want sin_categoria", got)
	}
}

func TestResolveURL(prueba *testing.T) {
	base := mustURL(prueba, "https://cugat.cl/categoria-producto/bebidas/page/2/")
	if got := extract.ResolveURL(base, "/producto/uno/"); got != "https://cugat.cl/producto/uno/" {
		prueba.Errorf("ResolveURL() = %q", got)
	}
	if got := extract.ResolveURL(nil, "/producto/uno/"); got != "/producto/uno/" {
		prueba.Errorf("ResolveURL(nil) = %q", got)
	}
}

func TestSelectProductByURL(prueba *testing.T) {
	tomateURL := "https://www.jumbo.cl/p/tomate"
	lechugaURL := "https://www.jumbo.cl/p/lechuga"
	products := []domain.Product{
		{Producto: "Tomate", URLProducto: &tomateURL},
		{Producto: "Lechuga", URLProducto: &lechugaURL},
	}

	selected := extract.SelectProductByURL(products, "https://www.jumbo.cl/p/lechuga?utm=1", mustURL(prueba, "https://www.jumbo.cl/cat"))
	if selected == nil || selected.Producto != "Lechuga" {
		prueba.Errorf("SelectProductByURL() = %+v", selected)
	}

	if got := extract.SelectProductByURL(products, "https://www.jumbo.cl/p/otro", mustURL(prueba, "https://www.jumbo.cl/cat")); got != nil {
		prueba.Errorf("SelectProductByURL() con varios candidatos = %+v, want nil", got)
	}

	single := []domain.Product{{Producto: "Único", URLProducto: &tomateURL}}
	if got := extract.SelectProductByURL(single, "https://www.jumbo.cl/p/otro", mustURL(prueba, "https://www.jumbo.cl/cat")); got == nil || got.Producto != "Único" {
		prueba.Errorf("SelectProductByURL() con un candidato = %+v", got)
	}
}

func TestFormatFromTextYTitleCase(prueba *testing.T) {
	pattern := regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*(?:kg|g|l)\b`)
	if got := extract.FormatFromText("Bebida 1.5 L", pattern); got == nil || *got != "1.5 L" {
		prueba.Errorf("FormatFromText() = %v", got)
	}
	if got := extract.FormatFromText("Pan", pattern); got != nil {
		prueba.Errorf("FormatFromText(sin formato) = %v", got)
	}
	if got := extract.TitleCase("coca cola 1.5l"); got != "Coca Cola 1.5L" {
		prueba.Errorf("TitleCase() = %q", got)
	}
}
