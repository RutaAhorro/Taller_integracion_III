// catalogo_test.go verifica la carga, validación de reglas de URL y límites
// del catálogo declarativo de spiders.
package extraccion_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
)

// ===== Catálogo Embebido y Reglas de URL =====

// TestDefaultDeclaraTodosLosSitios verifica que el catálogo embebido incluya las cinco cadenas soportadas.
func TestDefaultDeclaraTodosLosSitios(prueba *testing.T) {
	catalogo := extraccion.Default()
	esperados := []string{
		"jumbo_rsc",
		"santa_isabel_rsc",
		"cugat_rsc",
		"acuenta_rsc",
		"lider_rsc",
	}
	if obtenidos := catalogo.Names(); !reflect.DeepEqual(obtenidos, esperados) {
		prueba.Fatalf("Names() = %v, esperado %v", obtenidos, esperados)
	}
	for _, nombre := range esperados {
		sitio, valido := catalogo.Get(nombre)
		if !valido {
			prueba.Fatalf("falta el sitio %q", nombre)
		}
		if sitio.Supermarket == "" || sitio.Parser == "" || sitio.CategoryFile == "" {
			prueba.Errorf("sitio %q incompleto: %+v", nombre, sitio)
		}
	}
}

// TestValidateCategoryURL verifica el rechazo de dominios ajenos y esquemas sin TLS en URLs de categoría.
func TestValidateCategoryURL(prueba *testing.T) {
	casos := []struct {
		sitio  string
		valida string
	}{
		{"jumbo_rsc", "https://www.jumbo.cl/frutas"},
		{"santa_isabel_rsc", "https://www.santaisabel.cl/despensa"},
		{"cugat_rsc", "https://cugat.cl/categoria-producto/bebidas/"},
		{"acuenta_rsc", "https://www.acuenta.cl/ca/congelados/04"},
		{"lider_rsc", "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836"},
	}
	catalogo := extraccion.Default()
	for _, caso := range casos {
		prueba.Run(caso.sitio, func(prueba *testing.T) {
			sitio, valido := catalogo.Get(caso.sitio)
			if !valido {
				prueba.Fatalf("falta el sitio %q", caso.sitio)
			}
			if errorObtenido := sitio.ValidateCategoryURL(caso.valida); errorObtenido != nil {
				prueba.Errorf("ValidateCategoryURL(%q) = %v", caso.valida, errorObtenido)
			}
			if errorObtenido := sitio.ValidateCategoryURL("https://example.com/categoria"); errorObtenido == nil {
				prueba.Error("ValidateCategoryURL() aceptó un host ajeno")
			}
			if errorObtenido := sitio.ValidateCategoryURL("http://" + sitio.AllowedHosts[0] + "/categoria"); errorObtenido == nil {
				prueba.Error("ValidateCategoryURL() aceptó HTTP sin TLS")
			}
		})
	}
}

// TestValidateProductURL verifica las reglas de dominio de las fichas individuales.
func TestValidateProductURL(prueba *testing.T) {
	casos := []struct {
		sitio  string
		valida string
	}{
		{"jumbo_rsc", "https://www.jumbo.cl/p/lechuga"},
		{"santa_isabel_rsc", "https://www.santaisabel.cl/leche-colun-1-l/p"},
		{"cugat_rsc", "https://cugat.cl/producto/bebida-cugat/"},
		{"acuenta_rsc", "https://www.acuenta.cl/p/helado-1075736"},
		{"lider_rsc", "https://super.lider.cl/ip/leche-entera/12345678"},
	}
	catalogo := extraccion.Default()
	for _, caso := range casos {
		prueba.Run(caso.sitio, func(prueba *testing.T) {
			sitio, valido := catalogo.Get(caso.sitio)
			if !valido {
				prueba.Fatalf("falta el sitio %q", caso.sitio)
			}
			if errorObtenido := sitio.ValidateProductURL(caso.valida); errorObtenido != nil {
				prueba.Errorf("ValidateProductURL(%q) = %v", caso.valida, errorObtenido)
			}
			if errorObtenido := sitio.ValidateProductURL("https://example.com/producto/1"); errorObtenido == nil {
				prueba.Error("ValidateProductURL() aceptó un host ajeno")
			}
		})
	}
}

// TestValidateProductURLRespetaReglasDeRuta verifica que una URL de categoría no sea aceptada como ficha de producto.
func TestValidateProductURLRespetaReglasDeRuta(prueba *testing.T) {
	catalogo := extraccion.Default()

	santaIsabel, _ := catalogo.Get("santa_isabel_rsc")
	if errorObtenido := santaIsabel.ValidateProductURL("https://www.santaisabel.cl/despensa"); errorObtenido == nil {
		prueba.Error("Santa Isabel aceptó una categoría como ficha /p")
	}

	lider, _ := catalogo.Get("lider_rsc")
	if errorObtenido := lider.ValidateProductURL("https://super.lider.cl/browse/frutas/1"); errorObtenido == nil {
		prueba.Error("Lider aceptó una categoría como ficha /ip")
	}

	acuenta, _ := catalogo.Get("acuenta_rsc")
	if errorObtenido := acuenta.ValidateProductURL("https://www.acuenta.cl/ca/congelados/04"); errorObtenido == nil {
		prueba.Error("A Cuenta aceptó una categoría como ficha /p")
	}
}

// TestAccesoresDelSitio verifica los métodos Selector, Setting, Limit, Regexp y List de Site.
func TestAccesoresDelSitio(prueba *testing.T) {
	sitio, valido := extraccion.Default().Get("jumbo_rsc")
	if !valido {
		prueba.Fatal("falta jumbo_rsc")
	}
	if obtenido := sitio.Selector("json_ld"); obtenido == "" {
		prueba.Error("Selector(json_ld) vacío")
	}
	if obtenido := sitio.Setting("sku_prefix"); obtenido != "JUMBO-" {
		prueba.Errorf("Setting(sku_prefix) = %q", obtenido)
	}
	if obtenido := sitio.Limit("sku_max_length"); obtenido != 100 {
		prueba.Errorf("Limit(sku_max_length) = %d", obtenido)
	}
	if obtenido := sitio.Regexp("no_existe"); obtenido != nil {
		prueba.Errorf("Regexp(no_existe) = %v, esperado nil", obtenido)
	}

	acuenta, _ := extraccion.Default().Get("acuenta_rsc")
	if obtenido := acuenta.List("brands"); len(obtenido) == 0 {
		prueba.Error("List(brands) vacío")
	}
}

// ===== Carga desde Archivo =====

// TestLoadLeeElArchivoIndicado verifica la carga de un archivo JSON personalizado de catálogo.
func TestLoadLeeElArchivoIndicado(prueba *testing.T) {
	ruta := filepath.Join(prueba.TempDir(), "spiders.json")
	cuerpo := `{"sites":[{
		"name":"demo_rsc","parser":"jumbo","supermarket":"Demo",
		"category_file":"demo_categories.txt",
		"allowed_hosts":["demo.cl"],
		"category_url":{"message":"categoría inválida"},
		"product_url":{"message":"producto inválido"},
		"selectors":{"json_ld":"script[type='application/ld+json']"},
		"settings":{"sku_prefix":"DEMO-","sku_trailing_segment":"p"},
		"limits":{"sku_max_length":100,"slug_max_length":80,"hash_length":12}
	}]}`
	if errorObtenido := os.WriteFile(ruta, []byte(cuerpo), 0o600); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	catalogo, errorObtenido := extraccion.Load(ruta)
	if errorObtenido != nil {
		prueba.Fatalf("Load() error = %v", errorObtenido)
	}
	if nombres := catalogo.Names(); !reflect.DeepEqual(nombres, []string{"demo_rsc"}) {
		prueba.Errorf("Names() = %v", nombres)
	}
}

// TestLoadRechazaCatalogoIncompletoParaElParser evita valores cero silenciosos en configuración crítica.
func TestLoadRechazaCatalogoIncompletoParaElParser(prueba *testing.T) {
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
			_, errorObtenido := extraccion.Load(ruta)
			if errorObtenido == nil || !strings.Contains(errorObtenido.Error(), caso.error) {
				prueba.Fatalf("Load() = %v; se esperaba error que incluya %q", errorObtenido, caso.error)
			}
		})
	}
}

// TestLoadRechazaCatalogosInvalidos verifica el rechazo de JSON inválido, sitios vacíos o duplicados.
func TestLoadRechazaCatalogosInvalidos(prueba *testing.T) {
	casos := map[string]string{
		"json inválido":   `{`,
		"sin sitios":      `{"sites":[]}`,
		"sin nombre":      `{"sites":[{"parser":"jumbo"}]}`,
		"paginación rota": `{"sites":[{"name":"a","parser":"jumbo","supermarket":"A","category_file":"a.txt","allowed_hosts":["a.cl"],"category_url":{"message":"x"},"product_url":{"message":"y"},"pagination":{"mode":"desconocido"},"selectors":{"json_ld":"script[type='application/ld+json']"},"settings":{"sku_prefix":"A-","sku_trailing_segment":"p"},"limits":{"sku_max_length":100,"slug_max_length":80,"hash_length":12}}]}`,
		"regexp inválido": `{"sites":[{"name":"a","parser":"jumbo","supermarket":"A","category_file":"a.txt","allowed_hosts":["a.cl"],"category_url":{"message":"x"},"product_url":{"message":"y"},"category_from":{"id_pattern":"("},"selectors":{"json_ld":"script[type='application/ld+json']"},"settings":{"sku_prefix":"A-","sku_trailing_segment":"p"},"limits":{"sku_max_length":100,"slug_max_length":80,"hash_length":12}}]}`,
		"duplicado": `{"sites":[
			{"name":"a","parser":"jumbo","supermarket":"A","category_file":"a.txt","allowed_hosts":["a.cl"],"category_url":{"message":"x"},"product_url":{"message":"y"},"selectors":{"json_ld":"script[type='application/ld+json']"},"settings":{"sku_prefix":"A-","sku_trailing_segment":"p"},"limits":{"sku_max_length":100,"slug_max_length":80,"hash_length":12}},
			{"name":"a","parser":"jumbo","supermarket":"A","category_file":"a.txt","allowed_hosts":["a.cl"],"category_url":{"message":"x"},"product_url":{"message":"y"},"selectors":{"json_ld":"script[type='application/ld+json']"},"settings":{"sku_prefix":"A-","sku_trailing_segment":"p"},"limits":{"sku_max_length":100,"slug_max_length":80,"hash_length":12}}]}`,
	}
	for nombre, cuerpo := range casos {
		prueba.Run(nombre, func(prueba *testing.T) {
			ruta := filepath.Join(prueba.TempDir(), "spiders.json")
			if errorObtenido := os.WriteFile(ruta, []byte(cuerpo), 0o600); errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
			if _, errorObtenido := extraccion.Load(ruta); errorObtenido == nil {
				prueba.Fatal("Load() debió rechazar el catálogo")
			}
		})
	}
}

// TestLoadRechazaArchivoInexistente comprueba que Load falle cuando la ruta indicada no existe.
func TestLoadRechazaArchivoInexistente(prueba *testing.T) {
	if _, errorObtenido := extraccion.Load(filepath.Join(prueba.TempDir(), "no-existe.json")); errorObtenido == nil {
		prueba.Fatal("Load() debió fallar con un archivo inexistente")
	}
}
