// spiders_test.go verifica el registro de spiders, la construcción desde el
// catálogo, CompleteItem, CollectURLs y el comportamiento común de categorías.
package extraccion_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
)

// ===== Registro y Construcción de Spiders =====

// TestNewRegistryFromCatalogConstruyeCadaParser verifica que el catálogo predeterminado registre las cinco cadenas.
func TestNewRegistryFromCatalogConstruyeCadaParser(prueba *testing.T) {
	catalogo := extraccion.Default()
	registro, errorObtenido := extraccion.NewRegistryFromCatalog(catalogo, &obtenedorFalso{}, prueba.TempDir(), 100, registradorSilencioso())
	if errorObtenido != nil {
		prueba.Fatalf("NewRegistryFromCatalog() error = %v", errorObtenido)
	}
	esperados := []string{
		extraccion.JumboName,
		extraccion.SantaIsabelName,
		extraccion.CugatName,
		extraccion.AcuentaName,
		extraccion.LiderName,
	}
	if obtenidos := catalogo.Names(); !reflect.DeepEqual(obtenidos, esperados) {
		prueba.Errorf("Catalog.Names() = %v, esperado %v", obtenidos, esperados)
	}
	for _, nombre := range esperados {
		spider, valido := registro.Get(nombre)
		if !valido {
			prueba.Errorf("falta %q en el registro", nombre)
			continue
		}
		if spider.Name() != nombre {
			prueba.Errorf("Name() = %q, esperado %q", spider.Name(), nombre)
		}
	}
}

// TestBuildRechazaParserDesconocido verifica que Build falle ante un nombre de parser no soportado.
func TestBuildRechazaParserDesconocido(prueba *testing.T) {
	sitio := extraccion.Site{Name: "demo_rsc", Parser: "desconocido", Supermarket: "Demo"}
	if _, errorObtenido := extraccion.Build(sitio, &obtenedorFalso{}, prueba.TempDir(), 100, registradorSilencioso()); errorObtenido == nil {
		prueba.Fatal("Build() debió rechazar un parser desconocido")
	}
}

// TestBuildConMaxPagesNoPositivoUsaDefaultMaxPages cubre el respaldo a DefaultMaxPages cuando maxPages <= 0 (H-05).
func TestBuildConMaxPagesNoPositivoUsaDefaultMaxPages(prueba *testing.T) {
	directorio := prueba.TempDir()
	urlCategoria := "https://www.jumbo.cl/cat"
	if errorObtenido := os.WriteFile(filepath.Join(directorio, "jumbo_categories.txt"), []byte(urlCategoria+"\n"), 0o600); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	cuerpo := `<script type="application/ld+json">{"@type":"Product","name":"Producto","url":"/p/producto","offers":{"price":"100"}}</script>`
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		urlCategoria:             respuestaExitosa(urlCategoria, cuerpo),
		urlCategoria + "?page=2": respuestaExitosa(urlCategoria+"?page=2", cuerpo),
	}}
	spider := construirSpiderDePrueba(prueba, extraccion.JumboName, obtenedor, directorio, 0)
	elementos := recolectarItems(prueba, spider, dominio.Job{})
	if len(elementos) != 2 {
		prueba.Fatalf("elementos = %d, esperado 2 (maxPages=0 debe usar DefaultMaxPages y paginar)", len(elementos))
	}
}

// TestNewRegistryFromCatalogRechazaParserInvalido verifica que un catálogo con parser desconocido aborte la creación del registro.
func TestNewRegistryFromCatalogRechazaParserInvalido(prueba *testing.T) {
	catalogo := extraccion.Catalog{Sites: []extraccion.Site{{
		Name:        "demo_rsc",
		Parser:      "desconocido",
		Supermarket: "Demo",
	}}}
	if _, errorObtenido := extraccion.NewRegistryFromCatalog(catalogo, &obtenedorFalso{}, prueba.TempDir(), 100, registradorSilencioso()); errorObtenido == nil {
		prueba.Fatal("NewRegistryFromCatalog() debió rechazar el catálogo")
	}
}

// TestNewRegistryFromConfigUsaElCatalogoEmbebidoYPropagaErrores verifica NewRegistryFromConfig con catálogo embebido y archivo externo.
func TestNewRegistryFromConfigUsaElCatalogoEmbebidoYPropagaErrores(prueba *testing.T) {
	configuracion := config.Default()
	registro, errorObtenido := extraccion.NewRegistryFromConfig(configuracion, &obtenedorFalso{}, prueba.TempDir(), 100, registradorSilencioso())
	if errorObtenido != nil {
		prueba.Fatalf("NewRegistryFromConfig() error = %v", errorObtenido)
	}
	if _, valido := registro.Get(extraccion.JumboName); !valido {
		prueba.Error("el registro no contiene jumbo_rsc")
	}

	configuracion.SpidersFile = "/no/existe/extraccion.json"
	if _, errorObtenido := extraccion.NewRegistryFromConfig(configuracion, &obtenedorFalso{}, prueba.TempDir(), 100, registradorSilencioso()); errorObtenido == nil {
		prueba.Fatal("NewRegistryFromConfig() debió fallar con un catálogo inexistente")
	}
}

// ===== Normalización de Ítem Emitido (CompleteItem) =====

// TestCompleteItemDerivaSKUDesdeLaURL verifica que CompleteItem complete el SKU cuando falta en el producto.
func TestCompleteItemDerivaSKUDesdeLaURL(prueba *testing.T) {
	urlProducto := "https://www.santaisabel.cl/mas-limonada-frambuesa-con-gas-1600cc-1993238/p"
	elemento := extraccion.CompleteItem(dominio.Item{
		Product:      dominio.Product{Producto: "Agua saborizada", URLProducto: &urlProducto},
		Supermercado: "Santa Isabel",
	})
	if elemento.SKU == nil || *elemento.SKU != "1993238" {
		prueba.Errorf("sku = %v, esperado 1993238", elemento.SKU)
	}
}

// TestCompleteItemConservaElSKUExistente verifica que CompleteItem no sobrescriba un SKU ya extraído.
func TestCompleteItemConservaElSKUExistente(prueba *testing.T) {
	urlProducto := "https://cugat.cl/producto/leche-entera/"
	sku := "SKU-1"
	elemento := extraccion.CompleteItem(dominio.Item{
		Product: dominio.Product{Producto: "Leche", SKU: &sku, URLProducto: &urlProducto},
	})
	if elemento.SKU == nil || *elemento.SKU != "SKU-1" {
		prueba.Errorf("sku = %v, esperado SKU-1", elemento.SKU)
	}
}

// TestCompleteItemRecortaLasColumnasDelCatalogo verifica que CompleteItem aplique los límites de longitud del esquema.
func TestCompleteItemRecortaLasColumnasDelCatalogo(prueba *testing.T) {
	textoLargo := func(tamano int) string { return strings.Repeat("á", tamano) }
	urlProducto := "https://www.jumbo.cl/producto-con-descripcion-larga-1993238/p"
	elemento := extraccion.CompleteItem(dominio.Item{Product: dominio.Product{
		Producto:          textoLargo(300),
		SKU:               dominio.StringPtr(textoLargo(120)),
		EanGtin:           dominio.StringPtr(textoLargo(60)),
		Marca:             dominio.StringPtr(textoLargo(120)),
		Categoria:         textoLargo(120),
		FormatoCrudo:      dominio.StringPtr(textoLargo(120)),
		MecanicaPromocion: dominio.StringPtr(textoLargo(300)),
		URLProducto:       &urlProducto,
	}})

	verificaciones := []struct {
		nombre string
		valor  string
		limite int
	}{
		{"producto", elemento.Producto, dominio.MaxProductoLength},
		{"sku", *elemento.SKU, dominio.MaxCampoCortoLength},
		{"ean_gtin", *elemento.EanGtin, dominio.MaxEANLength},
		{"marca", *elemento.Marca, dominio.MaxCampoCortoLength},
		{"categoria", elemento.Categoria, dominio.MaxCampoCortoLength},
		{"formato_crudo", *elemento.FormatoCrudo, dominio.MaxCampoCortoLength},
		{"mecanica_promocion", *elemento.MecanicaPromocion, dominio.MaxProductoLength},
	}
	for _, verificacion := range verificaciones {
		if obtenido := len([]rune(verificacion.valor)); obtenido != verificacion.limite {
			prueba.Errorf("%s tiene %d caracteres, esperado %d", verificacion.nombre, obtenido, verificacion.limite)
		}
	}
}

// TestCompleteItemDerivaElSKUAntesDeRecortar verifica que la derivación de SKU ocurra antes del truncado de campos.
func TestCompleteItemDerivaElSKUAntesDeRecortar(prueba *testing.T) {
	urlProducto := "https://www.santaisabel.cl/producto-sin-sku-1993238/p"
	elemento := extraccion.CompleteItem(dominio.Item{Product: dominio.Product{
		Producto:    "Producto sin SKU",
		URLProducto: &urlProducto,
	}})
	if elemento.SKU == nil || *elemento.SKU != "1993238" {
		prueba.Errorf("sku = %v, esperado 1993238", elemento.SKU)
	}
}

// ===== Carga de URLs desde Argumentos y Archivo (CollectURLs) =====

// TestCollectURLsCombinaArgumentosYArchivo verifica que CollectURLs fusione banderas y archivo ignorando comentarios.
func TestCollectURLsCombinaArgumentosYArchivo(prueba *testing.T) {
	archivo := filepath.Join(prueba.TempDir(), "urls.txt")
	cuerpo := "# comentario\nhttps://www.jumbo.cl/uno\n\n  https://www.jumbo.cl/dos  \n"
	if errorObtenido := os.WriteFile(archivo, []byte(cuerpo), 0o644); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}

	obtenido, errorObtenido := extraccion.CollectURLs([]string{"https://www.jumbo.cl/inicial"}, archivo)
	if errorObtenido != nil {
		prueba.Fatalf("CollectURLs() error = %v", errorObtenido)
	}
	esperado := []string{
		"https://www.jumbo.cl/inicial",
		"https://www.jumbo.cl/uno",
		"https://www.jumbo.cl/dos",
	}
	if !reflect.DeepEqual(obtenido, esperado) {
		prueba.Errorf("CollectURLs() = %#v, esperado %#v", obtenido, esperado)
	}
}

// TestCollectURLsSinArchivoYConArchivoInexistente verifica los casos borde de CollectURLs.
func TestCollectURLsSinArchivoYConArchivoInexistente(prueba *testing.T) {
	obtenido, errorObtenido := extraccion.CollectURLs([]string{"https://www.jumbo.cl/uno"}, "")
	if errorObtenido != nil {
		prueba.Fatalf("CollectURLs() error = %v", errorObtenido)
	}
	if !reflect.DeepEqual(obtenido, []string{"https://www.jumbo.cl/uno"}) {
		prueba.Errorf("CollectURLs() = %#v", obtenido)
	}
	if _, errorObtenido := extraccion.CollectURLs(nil, filepath.Join(prueba.TempDir(), "no-existe.txt")); errorObtenido == nil {
		prueba.Fatal("CollectURLs() debió fallar con un archivo inexistente")
	}
}

// ===== Categorías Vacías =====

// TestEmptyCategoryDoesNotAbortChain cubre las cinco cadenas: una categoría con
// respuesta 200 y cero productos cierra esa categoría y continúa con la siguiente.
func TestEmptyCategoryDoesNotAbortChain(prueba *testing.T) {
	casos := []struct {
		nombre         string
		sitio          string
		archivo        string
		primera        string
		segunda        string
		primeraLlamada string
		segundaLlamada string
	}{
		{
			nombre:  "jumbo",
			sitio:   extraccion.JumboName,
			archivo: "jumbo_categories.txt",
			primera: "https://www.jumbo.cl/categoria-vacia",
			segunda: "https://www.jumbo.cl/frutas",
		},
		{
			nombre:  "santa_isabel",
			sitio:   extraccion.SantaIsabelName,
			archivo: "santa_isabel_categories.txt",
			primera: "https://www.santaisabel.cl/categoria-vacia",
			segunda: "https://www.santaisabel.cl/frutas",
		},
		{
			nombre:  "cugat",
			sitio:   extraccion.CugatName,
			archivo: "cugat_categories.txt",
			primera: "https://cugat.cl/categoria-producto/vacia/",
			segunda: "https://cugat.cl/categoria-producto/bebidas/",
		},
		{
			nombre:         "acuenta",
			sitio:          extraccion.AcuentaName,
			archivo:        "acuenta_categories.txt",
			primera:        "https://www.acuenta.cl/ca/vacia/01",
			segunda:        "https://www.acuenta.cl/ca/frutas/02",
			primeraLlamada: "https://www.acuenta.cl/ca/vacia/01?currentPage=1",
			segundaLlamada: "https://www.acuenta.cl/ca/frutas/02?currentPage=1",
		},
		{
			nombre:  "lider",
			sitio:   extraccion.LiderName,
			archivo: "lider_categories.txt",
			primera: "https://super.lider.cl/browse/vacia/12345678",
			segunda: "https://super.lider.cl/browse/frutas/87654321",
		},
	}

	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			directorio := prueba.TempDir()
			categorias := caso.primera + "\n" + caso.segunda + "\n"
			if errorObtenido := os.WriteFile(filepath.Join(directorio, caso.archivo), []byte(categorias), 0o600); errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
			primeraLlamada := caso.primeraLlamada
			if primeraLlamada == "" {
				primeraLlamada = caso.primera
			}
			segundaLlamada := caso.segundaLlamada
			if segundaLlamada == "" {
				segundaLlamada = caso.segunda
			}
			obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
				primeraLlamada: respuestaExitosa(primeraLlamada, `<html><body><h1>Sin productos</h1></body></html>`),
				segundaLlamada: respuestaExitosa(segundaLlamada, `<html><body><h1>Sin productos</h1></body></html>`),
			}}
			spider := construirSpiderDePrueba(prueba, caso.sitio, obtenedor, directorio, 100)
			if errorObtenido := spider.Run(context.Background(), dominio.Job{}, func(dominio.Item) error { return nil }); errorObtenido != nil {
				prueba.Fatalf("%s abortó el crawl por una categoría vacía: %v", spider.Name(), errorObtenido)
			}
			if len(obtenedor.llamadas) < 2 || obtenedor.llamadas[0] != primeraLlamada || obtenedor.llamadas[1] != segundaLlamada {
				prueba.Errorf("%s llamadas = %v, esperado la categoría vacía y luego la siguiente", spider.Name(), obtenedor.llamadas)
			}
		})
	}
}
