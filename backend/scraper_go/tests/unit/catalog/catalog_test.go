package catalog_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
)

func TestDefaultDeclaraTodosLosSitios(prueba *testing.T) {
	sites := catalog.Default()
	want := []string{
		"jumbo_rsc",
		"santa_isabel_rsc",
		"cugat_rsc",
		"acuenta_rsc",
		"lider_rsc",
	}
	if got := sites.Names(); !reflect.DeepEqual(got, want) {
		prueba.Fatalf("Names() = %v, want %v", got, want)
	}
	for _, name := range want {
		site, ok := sites.Get(name)
		if !ok {
			prueba.Fatalf("falta el sitio %q", name)
		}
		if site.Supermarket == "" || site.Parser == "" || site.CategoryFile == "" {
			prueba.Errorf("sitio %q incompleto: %+v", name, site)
		}
	}
}

func TestValidateCategoryURL(prueba *testing.T) {
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
		prueba.Run(tc.site, func(prueba *testing.T) {
			site, ok := sites.Get(tc.site)
			if !ok {
				prueba.Fatalf("falta el sitio %q", tc.site)
			}
			if err := site.ValidateCategoryURL(tc.valid); err != nil {
				prueba.Errorf("ValidateCategoryURL(%q) = %v", tc.valid, err)
			}
			if err := site.ValidateCategoryURL("https://example.com/categoria"); err == nil {
				prueba.Error("ValidateCategoryURL() aceptó un host ajeno")
			}
			if err := site.ValidateCategoryURL("http://" + site.AllowedHosts[0] + "/categoria"); err == nil {
				prueba.Error("ValidateCategoryURL() aceptó HTTP sin TLS")
			}
		})
	}
}

func TestValidateProductURL(prueba *testing.T) {
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
		prueba.Run(tc.site, func(prueba *testing.T) {
			site, ok := sites.Get(tc.site)
			if !ok {
				prueba.Fatalf("falta el sitio %q", tc.site)
			}
			if err := site.ValidateProductURL(tc.valid); err != nil {
				prueba.Errorf("ValidateProductURL(%q) = %v", tc.valid, err)
			}
			if err := site.ValidateProductURL("https://example.com/producto/1"); err == nil {
				prueba.Error("ValidateProductURL() aceptó un host ajeno")
			}
		})
	}
}

func TestValidateProductURLRespetaReglasDeRuta(prueba *testing.T) {
	sites := catalog.Default()

	santaIsabel, _ := sites.Get("santa_isabel_rsc")
	if err := santaIsabel.ValidateProductURL("https://www.santaisabel.cl/despensa"); err == nil {
		prueba.Error("Santa Isabel aceptó una categoría como ficha /p")
	}

	lider, _ := sites.Get("lider_rsc")
	if err := lider.ValidateProductURL("https://super.lider.cl/browse/frutas/1"); err == nil {
		prueba.Error("Lider aceptó una categoría como ficha /ip")
	}

	acuenta, _ := sites.Get("acuenta_rsc")
	if err := acuenta.ValidateProductURL("https://www.acuenta.cl/ca/congelados/04"); err == nil {
		prueba.Error("A Cuenta aceptó una categoría como ficha /p")
	}
}

func TestAccesoresDelSitio(prueba *testing.T) {
	site, ok := catalog.Default().Get("jumbo_rsc")
	if !ok {
		prueba.Fatal("falta jumbo_rsc")
	}
	if got := site.Selector("json_ld"); got == "" {
		prueba.Error("Selector(json_ld) vacío")
	}
	if got := site.Setting("sku_prefix"); got != "JUMBO-" {
		prueba.Errorf("Setting(sku_prefix) = %q", got)
	}
	if got := site.Limit("sku_max_length"); got != 100 {
		prueba.Errorf("Limit(sku_max_length) = %d", got)
	}
	if got := site.Regexp("no_existe"); got != nil {
		prueba.Errorf("Regexp(no_existe) = %v, want nil", got)
	}

	acuenta, _ := catalog.Default().Get("acuenta_rsc")
	if got := acuenta.List("brands"); len(got) == 0 {
		prueba.Error("List(brands) vacío")
	}
}

func TestFromEnvLeeElArchivoIndicado(prueba *testing.T) {
	path := filepath.Join(prueba.TempDir(), "spiders.json")
	body := `{"sites":[{
		"name":"demo_rsc","parser":"jumbo","supermarket":"Demo",
		"category_file":"demo_categories.txt",
		"allowed_hosts":["demo.cl"],
		"category_url":{"message":"categoría inválida"},
		"product_url":{"message":"producto inválido"},
		"selectors":{"json_ld":"script[type='application/ld+json']"},
		"settings":{"sku_prefix":"DEMO-","sku_trailing_segment":"p"},
		"limits":{"sku_max_length":100,"slug_max_length":80,"hash_length":12}
	}]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		prueba.Fatal(err)
	}
	prueba.Setenv(catalog.FileEnv, path)

	sites, err := catalog.FromEnv()
	if err != nil {
		prueba.Fatalf("FromEnv() error = %v", err)
	}
	if names := sites.Names(); !reflect.DeepEqual(names, []string{"demo_rsc"}) {
		prueba.Errorf("Names() = %v", names)
	}
}

// TestFromEnvRechazaCatalogoIncompletoParaElParser evita valores cero silenciosos en configuración crítica.
func TestFromEnvRechazaCatalogoIncompletoParaElParser(prueba *testing.T) {
	casos := map[string]struct {
		cuerpo string
		error  string
	}{
		"limite obligatorio ausente": {
			cuerpo: `{"sites":[{
				"name":"demo_rsc","parser":"jumbo","supermarket":"Demo",
				"category_file":"demo_categories.txt","allowed_hosts":["demo.cl"],
				"category_url":{"message":"categoría inválida"},
				"product_url":{"message":"producto inválido"},
				"selectors":{"json_ld":"script[type='application/ld+json']"},
				"settings":{"sku_prefix":"DEMO-","sku_trailing_segment":"p"},
				"limits":{"slug_max_length":80,"hash_length":12}
			}]}`,
			error: "limits.sku_max_length",
		},
		"hash excede tamaño SHA-1": {
			cuerpo: `{"sites":[{
				"name":"demo_rsc","parser":"jumbo","supermarket":"Demo",
				"category_file":"demo_categories.txt","allowed_hosts":["demo.cl"],
				"category_url":{"message":"categoría inválida"},
				"product_url":{"message":"producto inválido"},
				"selectors":{"json_ld":"script[type='application/ld+json']"},
				"settings":{"sku_prefix":"DEMO-","sku_trailing_segment":"p"},
				"limits":{"sku_max_length":100,"slug_max_length":80,"hash_length":41}
			}]}`,
			error: "limits.hash_length no puede superar 40",
		},
	}
	for nombre, caso := range casos {
		prueba.Run(nombre, func(prueba *testing.T) {
			ruta := filepath.Join(prueba.TempDir(), "spiders.json")
			if errorObtenido := os.WriteFile(ruta, []byte(caso.cuerpo), 0o600); errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
			prueba.Setenv(catalog.FileEnv, ruta)
			_, errorObtenido := catalog.FromEnv()
			if errorObtenido == nil || !strings.Contains(errorObtenido.Error(), caso.error) {
				prueba.Fatalf("FromEnv() = %v; se esperaba error que incluya %q", errorObtenido, caso.error)
			}
		})
	}
}

func TestFromEnvRechazaCatalogosInvalidos(prueba *testing.T) {
	cases := map[string]string{
		"json inválido":   `{`,
		"sin sitios":      `{"sites":[]}`,
		"sin nombre":      `{"sites":[{"parser":"jumbo"}]}`,
		"paginación rota": `{"sites":[{"name":"a","parser":"jumbo","supermarket":"A","category_file":"a.txt","allowed_hosts":["a.cl"],"category_url":{"message":"x"},"product_url":{"message":"y"},"pagination":{"mode":"desconocido"},"selectors":{"json_ld":"script[type='application/ld+json']"},"settings":{"sku_prefix":"A-","sku_trailing_segment":"p"},"limits":{"sku_max_length":100,"slug_max_length":80,"hash_length":12}}]}`,
		"regexp inválido": `{"sites":[{"name":"a","parser":"jumbo","supermarket":"A","category_file":"a.txt","allowed_hosts":["a.cl"],"category_url":{"message":"x"},"product_url":{"message":"y"},"category_from":{"id_pattern":"("},"selectors":{"json_ld":"script[type='application/ld+json']"},"settings":{"sku_prefix":"A-","sku_trailing_segment":"p"},"limits":{"sku_max_length":100,"slug_max_length":80,"hash_length":12}}]}`,
		"duplicado": `{"sites":[
			{"name":"a","parser":"jumbo","supermarket":"A","category_file":"a.txt","allowed_hosts":["a.cl"],"category_url":{"message":"x"},"product_url":{"message":"y"},"selectors":{"json_ld":"script[type='application/ld+json']"},"settings":{"sku_prefix":"A-","sku_trailing_segment":"p"},"limits":{"sku_max_length":100,"slug_max_length":80,"hash_length":12}},
			{"name":"a","parser":"jumbo","supermarket":"A","category_file":"a.txt","allowed_hosts":["a.cl"],"category_url":{"message":"x"},"product_url":{"message":"y"},"selectors":{"json_ld":"script[type='application/ld+json']"},"settings":{"sku_prefix":"A-","sku_trailing_segment":"p"},"limits":{"sku_max_length":100,"slug_max_length":80,"hash_length":12}}]}`,
	}
	for name, body := range cases {
		prueba.Run(name, func(prueba *testing.T) {
			path := filepath.Join(prueba.TempDir(), "spiders.json")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				prueba.Fatal(err)
			}
			prueba.Setenv(catalog.FileEnv, path)
			if _, err := catalog.FromEnv(); err == nil {
				prueba.Fatal("FromEnv() debió rechazar el catálogo")
			}
		})
	}
}

func TestFromEnvRechazaArchivoInexistente(prueba *testing.T) {
	prueba.Setenv(catalog.FileEnv, filepath.Join(prueba.TempDir(), "no-existe.json"))
	if _, err := catalog.FromEnv(); err == nil {
		prueba.Fatal("FromEnv() debió fallar con un archivo inexistente")
	}
}
