package catalog_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
)

func TestDefaultDeclaraTodosLosSitios(t *testing.T) {
	sites := catalog.Default()
	want := []string{
		"jumbo_rsc",
		"santa_isabel_rsc",
		"cugat_rsc",
		"acuenta_rsc",
		"lider_rsc",
	}
	if got := sites.Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
	for _, name := range want {
		site, ok := sites.Get(name)
		if !ok {
			t.Fatalf("falta el sitio %q", name)
		}
		if site.Supermarket == "" || site.Parser == "" || site.CategoryFile == "" {
			t.Errorf("sitio %q incompleto: %+v", name, site)
		}
	}
}

func TestValidateCategoryURL(t *testing.T) {
	cases := []struct {
		site  string
		valid string
	}{
		{"jumbo_rsc", "https://www.jumbo.cl/frutas"},
		{"santa_isabel_rsc", "https://www.santaisabel.cl/despensa"},
		{"cugat_rsc", "https://cugat.cl/categoria-producto/bebidas/"},
		{"acuenta_rsc", "https://www.acuenta.cl/ca/congelados/04"},
		{"lider_rsc", "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836"},
	}
	sites := catalog.Default()
	for _, tc := range cases {
		t.Run(tc.site, func(t *testing.T) {
			site, ok := sites.Get(tc.site)
			if !ok {
				t.Fatalf("falta el sitio %q", tc.site)
			}
			if err := site.ValidateCategoryURL(tc.valid); err != nil {
				t.Errorf("ValidateCategoryURL(%q) = %v", tc.valid, err)
			}
			if err := site.ValidateCategoryURL("https://example.com/categoria"); err == nil {
				t.Error("ValidateCategoryURL() aceptó un host ajeno")
			}
			if err := site.ValidateCategoryURL("http://" + site.AllowedHosts[0] + "/categoria"); err == nil {
				t.Error("ValidateCategoryURL() aceptó HTTP sin TLS")
			}
		})
	}
}

func TestValidateProductURL(t *testing.T) {
	cases := []struct {
		site  string
		valid string
	}{
		{"jumbo_rsc", "https://www.jumbo.cl/p/lechuga"},
		{"santa_isabel_rsc", "https://www.santaisabel.cl/leche-colun-1-l/p"},
		{"cugat_rsc", "https://cugat.cl/producto/bebida-cugat/"},
		{"acuenta_rsc", "https://www.acuenta.cl/p/helado-1075736"},
		{"lider_rsc", "https://super.lider.cl/ip/leche-entera/12345678"},
	}
	sites := catalog.Default()
	for _, tc := range cases {
		t.Run(tc.site, func(t *testing.T) {
			site, ok := sites.Get(tc.site)
			if !ok {
				t.Fatalf("falta el sitio %q", tc.site)
			}
			if err := site.ValidateProductURL(tc.valid); err != nil {
				t.Errorf("ValidateProductURL(%q) = %v", tc.valid, err)
			}
			if err := site.ValidateProductURL("https://example.com/producto/1"); err == nil {
				t.Error("ValidateProductURL() aceptó un host ajeno")
			}
		})
	}
}

func TestValidateProductURLRespetaReglasDeRuta(t *testing.T) {
	sites := catalog.Default()

	santaIsabel, _ := sites.Get("santa_isabel_rsc")
	if err := santaIsabel.ValidateProductURL("https://www.santaisabel.cl/despensa"); err == nil {
		t.Error("Santa Isabel aceptó una categoría como ficha /p")
	}

	lider, _ := sites.Get("lider_rsc")
	if err := lider.ValidateProductURL("https://super.lider.cl/browse/frutas/1"); err == nil {
		t.Error("Lider aceptó una categoría como ficha /ip")
	}

	acuenta, _ := sites.Get("acuenta_rsc")
	if err := acuenta.ValidateProductURL("https://www.acuenta.cl/ca/congelados/04"); err == nil {
		t.Error("A Cuenta aceptó una categoría como ficha /p")
	}
}

func TestAccesoresDelSitio(t *testing.T) {
	site, ok := catalog.Default().Get("jumbo_rsc")
	if !ok {
		t.Fatal("falta jumbo_rsc")
	}
	if got := site.Selector("json_ld"); got == "" {
		t.Error("Selector(json_ld) vacío")
	}
	if got := site.Setting("sku_prefix"); got != "JUMBO-" {
		t.Errorf("Setting(sku_prefix) = %q", got)
	}
	if got := site.Limit("sku_max_length"); got != 100 {
		t.Errorf("Limit(sku_max_length) = %d", got)
	}
	if got := site.Regexp("no_existe"); got != nil {
		t.Errorf("Regexp(no_existe) = %v, want nil", got)
	}

	acuenta, _ := catalog.Default().Get("acuenta_rsc")
	if got := acuenta.List("brands"); len(got) == 0 {
		t.Error("List(brands) vacío")
	}
}

func TestFromEnvLeeElArchivoIndicado(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spiders.json")
	body := `{"sites":[{
		"name":"demo_rsc","parser":"jumbo","supermarket":"Demo",
		"category_file":"demo_categories.txt",
		"allowed_hosts":["demo.cl"],
		"category_url":{"message":"categoría inválida"},
		"product_url":{"message":"producto inválido"}
	}]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(catalog.FileEnv, path)

	sites, err := catalog.FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if names := sites.Names(); !reflect.DeepEqual(names, []string{"demo_rsc"}) {
		t.Errorf("Names() = %v", names)
	}
}

func TestFromEnvRechazaCatalogosInvalidos(t *testing.T) {
	cases := map[string]string{
		"json inválido":   `{`,
		"sin sitios":      `{"sites":[]}`,
		"sin nombre":      `{"sites":[{"parser":"jumbo"}]}`,
		"paginación rota": `{"sites":[{"name":"a","parser":"jumbo","supermarket":"A","category_file":"a.txt","allowed_hosts":["a.cl"],"category_url":{"message":"x"},"product_url":{"message":"y"},"pagination":{"mode":"desconocido"}}]}`,
		"regexp inválido": `{"sites":[{"name":"a","parser":"jumbo","supermarket":"A","category_file":"a.txt","allowed_hosts":["a.cl"],"category_url":{"message":"x"},"product_url":{"message":"y"},"patterns":{"roto":"("}}]}`,
		"duplicado": `{"sites":[
			{"name":"a","parser":"jumbo","supermarket":"A","category_file":"a.txt","allowed_hosts":["a.cl"],"category_url":{"message":"x"},"product_url":{"message":"y"}},
			{"name":"a","parser":"jumbo","supermarket":"A","category_file":"a.txt","allowed_hosts":["a.cl"],"category_url":{"message":"x"},"product_url":{"message":"y"}}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "spiders.json")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv(catalog.FileEnv, path)
			if _, err := catalog.FromEnv(); err == nil {
				t.Fatal("FromEnv() debió rechazar el catálogo")
			}
		})
	}
}

func TestFromEnvRechazaArchivoInexistente(t *testing.T) {
	t.Setenv(catalog.FileEnv, filepath.Join(t.TempDir(), "no-existe.json"))
	if _, err := catalog.FromEnv(); err == nil {
		t.Fatal("FromEnv() debió fallar con un archivo inexistente")
	}
}
