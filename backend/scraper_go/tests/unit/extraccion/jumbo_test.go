// jumbo_test.go verifica el parseo JSON-LD y el recorrido de páginas de Jumbo.
package extraccion_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
)

// ===== Parseo de Productos de Jumbo =====

func TestJumboProducts(prueba *testing.T) {
	cuerpoHTML := `<script type="application/ld+json">
{"@graph":[{"item":{"@type":"Product","name":"Tomate Larga Vida",
"image":["https://img.test/tomate.jpg"],"offers":{"price":"$1.290"}}}]}
</script>`
	productos := extraccion.JumboProducts(sitioDelCatalogo(prueba, "jumbo_rsc"), []byte(cuerpoHTML))
	if len(productos) != 1 {
		prueba.Fatalf("productos = %d, esperado 1", len(productos))
	}
	producto := productos[0]
	if producto.Producto != "Tomate Larga Vida" {
		prueba.Errorf("producto = %q", producto.Producto)
	}
	for nombreCampo, esperado := range map[string]float64{
		"precio":        1290,
		"precio_normal": 1290,
		"precio_oferta": 1290,
	} {
		var obtenido *float64
		switch nombreCampo {
		case "precio":
			obtenido = producto.Precio
		case "precio_normal":
			obtenido = producto.PrecioNormal
		case "precio_oferta":
			obtenido = producto.PrecioOferta
		}
		if valor, valido := valorFlotante(obtenido); !valido || valor != esperado {
			prueba.Errorf("%s = %v, esperado %v", nombreCampo, obtenido, esperado)
		}
	}
	verificarCamposProducto(prueba, producto, map[string]string{
		"imagen": "https://img.test/tomate.jpg",
	})
}

func TestJumboProductsLeeCamposDeComparacion(prueba *testing.T) {
	cuerpoHTML := `<script type="application/ld+json">
{"@type":"Product","name":"Lechuga","gtin13":"7800000000001","sku":"SKU-1",
"brand":{"name":"Marca Test"},"description":"1 unidad","url":"https://jumbo.cl/p/lechuga",
"offers":{"price":"990","availability":"https://schema.org/InStock","offerDescription":"2x1"}}
</script>`
	productos := extraccion.JumboProducts(sitioDelCatalogo(prueba, "jumbo_rsc"), []byte(cuerpoHTML))
	if len(productos) != 1 {
		prueba.Fatalf("productos = %d, esperado 1", len(productos))
	}
	producto := productos[0]
	verificarCamposProducto(prueba, producto, map[string]string{
		"ean":       "7800000000001",
		"sku":       "SKU-1",
		"marca":     "Marca Test",
		"formato":   "1 unidad",
		"promocion": "2x1",
		"url":       "https://jumbo.cl/p/lechuga",
	})
	if enStock, valido := valorBooleano(producto.EnStock); !valido || !enStock {
		prueba.Errorf("en_stock = %v", producto.EnStock)
	}
}

func TestJumboProductsIdentificadoresNumericos(prueba *testing.T) {
	casos := []struct {
		nombre         string
		cuerpoHTML     string
		eanEsperado    string
		skuEsperado    string
		precioEsperado float64
	}{
		{
			nombre: "enteros",
			cuerpoHTML: `<script type="application/ld+json">
{"@type":"Product","name":"Numérico","gtin13":7800000000001,"sku":12345,
"offers":{"price":1290}}
</script>`,
			eanEsperado:    "7800000000001",
			skuEsperado:    "12345",
			precioEsperado: 1290,
		},
		{
			nombre: "decimal",
			cuerpoHTML: `<script type="application/ld+json">
{"@type":"Product","name":"Float","gtin13":7800000000001.0,
"offers":{"price":1290.0}}
</script>`,
			precioEsperado: 1290,
		},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			productos := extraccion.JumboProducts(sitioDelCatalogo(prueba, "jumbo_rsc"), []byte(caso.cuerpoHTML))
			if len(productos) != 1 {
				prueba.Fatalf("productos = %d, esperado 1", len(productos))
			}
			producto := productos[0]
			if obtenido := valorCadena(producto.EanGtin); obtenido != caso.eanEsperado {
				prueba.Errorf("ean = %q, esperado %q", obtenido, caso.eanEsperado)
			}
			if obtenido := valorCadena(producto.SKU); obtenido != caso.skuEsperado {
				prueba.Errorf("sku = %q, esperado %q", obtenido, caso.skuEsperado)
			}
			if valor, valido := valorFlotante(producto.Precio); !valido || valor != caso.precioEsperado {
				prueba.Errorf("precio = %v, esperado %v", producto.Precio, caso.precioEsperado)
			}
		})
	}
}

func TestJumboProductSeleccionaLaFichaSolicitada(prueba *testing.T) {
	cuerpoHTML := `<script type="application/ld+json">
{"@graph":[
{"@type":"Product","name":"Tomate","url":"/p/tomate","offers":{"price":"1200"}},
{"@type":"Product","name":"Lechuga","url":"/p/lechuga","offers":{"price":"990"}}]}
</script>`
	producto := extraccion.JumboProduct(
		sitioDelCatalogo(prueba, "jumbo_rsc"),
		[]byte(cuerpoHTML),
		"https://www.jumbo.cl/p/lechuga",
		urlDePrueba(prueba, "https://www.jumbo.cl/p/lechuga"),
	)
	if producto == nil || producto.Producto != "Lechuga" {
		prueba.Fatalf("JumboProduct() = %+v", producto)
	}
}

func TestJumboSKUFromURL(prueba *testing.T) {
	obtenido := extraccion.JumboSKUFromURL(sitioDelCatalogo(prueba, "jumbo_rsc"), "https://www.jumbo.cl/cafe/p")
	if obtenido == nil || *obtenido != "JUMBO-cafe" {
		prueba.Errorf("JumboSKUFromURL() = %v, esperado JUMBO-cafe", obtenido)
	}
}

// ===== Recorrido del Spider de Jumbo =====

func TestJumboProductMode(prueba *testing.T) {
	urlProducto := "https://www.jumbo.cl/p/lechuga"
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		urlProducto: respuestaExitosa(urlProducto, `<script type="application/ld+json">
{"@type":"Product","name":"Lechuga","url":"https://www.jumbo.cl/p/lechuga","offers":{"price":"990"}}
</script>`),
	}}
	spider := construirSpiderDePrueba(prueba, "jumbo_rsc", obtenedor, prueba.TempDir(), 100)
	elementos := recolectarItems(prueba, spider, dominio.Job{ProductURL: urlProducto})
	if len(elementos) != 1 {
		prueba.Fatalf("items = %d", len(elementos))
	}
	elemento := elementos[0]
	if elemento.Producto != "Lechuga" || elemento.Categoria != "producto" || elemento.Supermercado != "Jumbo" {
		prueba.Errorf("item = %+v", elemento)
	}
	if elemento.URLProducto == nil || *elemento.URLProducto != urlProducto {
		prueba.Errorf("url = %v", elemento.URLProducto)
	}
}

func TestJumboProductModeSeleccionaLaFichaSolicitada(prueba *testing.T) {
	urlProducto := "https://www.jumbo.cl/p/lechuga"
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		urlProducto: respuestaExitosa(urlProducto, `<script type="application/ld+json">
{"@graph":[
{"@type":"Product","name":"Tomate","url":"/p/tomate","offers":{"price":"1200"}},
{"@type":"Product","name":"Lechuga","url":"/p/lechuga","offers":{"price":"990"}}]}
</script>`),
	}}
	spider := construirSpiderDePrueba(prueba, "jumbo_rsc", obtenedor, prueba.TempDir(), 100)
	elementos := recolectarItems(prueba, spider, dominio.Job{ProductURL: urlProducto})
	if len(elementos) != 1 || elementos[0].Producto != "Lechuga" {
		prueba.Fatalf("items = %+v", elementos)
	}
}

func TestJumboRechazaProductURLNoConfiable(prueba *testing.T) {
	obtenedor := &obtenedorFalso{}
	spider := construirSpiderDePrueba(prueba, "jumbo_rsc", obtenedor, prueba.TempDir(), 100)
	errorObtenido := spider.Run(context.Background(), dominio.Job{ProductURL: "https://example.com/p/1"}, func(dominio.Item) error {
		return nil
	})
	if errorObtenido == nil {
		prueba.Fatal("Run() debió rechazar un dominio no confiable")
	}
	if len(obtenedor.llamadas) != 0 {
		prueba.Errorf("se hicieron %d requests", len(obtenedor.llamadas))
	}
}

func TestJumboPaginacionDeCategoria(prueba *testing.T) {
	directorioCategorias := prueba.TempDir()
	urlCategoria := "https://www.jumbo.cl/cat"
	if errorEscritura := os.WriteFile(filepath.Join(directorioCategorias, "jumbo_categories.txt"), []byte(urlCategoria+"\n"), 0o600); errorEscritura != nil {
		prueba.Fatal(errorEscritura)
	}
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		urlCategoria: respuestaExitosa(urlCategoria, `<script type="application/ld+json">
{"@type":"Product","name":"Producto","url":"/p/producto","offers":{"price":"100"}}
</script>`),
	}}
	spider := construirSpiderDePrueba(prueba, "jumbo_rsc", obtenedor, directorioCategorias, 100)
	elementos := recolectarItems(prueba, spider, dominio.Job{})
	if len(elementos) != 1 {
		prueba.Fatalf("items = %d", len(elementos))
	}
	if len(obtenedor.llamadas) != 2 || obtenedor.llamadas[1] != urlCategoria+"?page=2" {
		prueba.Errorf("calls = %v", obtenedor.llamadas)
	}
}

func TestJumboEnriqueceEANDesdeLaFicha(prueba *testing.T) {
	directorioCategorias := prueba.TempDir()
	urlCategoria := "https://www.jumbo.cl/cat"
	if errorEscritura := os.WriteFile(filepath.Join(directorioCategorias, "jumbo_categories.txt"), []byte(urlCategoria+"\n"), 0o600); errorEscritura != nil {
		prueba.Fatal(errorEscritura)
	}
	urlDetalle := "https://www.jumbo.cl/cafe/p"
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		urlCategoria: respuestaExitosa(urlCategoria, `<script type="application/ld+json">
{"@type":"Product","name":"Cafe","url":"https://www.jumbo.cl/cafe/p","offers":{"price":"990"}}
</script>`),
		urlDetalle: respuestaExitosa(urlDetalle, `<script type="application/ld+json">
{"@type":"Product","name":"Cafe","url":"https://www.jumbo.cl/cafe/p",
"gtin13":"7800000000001","sku":"sku-detail","offers":{"price":"1200"}}
</script>`),
	}}
	spider := construirSpiderDePrueba(prueba, "jumbo_rsc", obtenedor, directorioCategorias, 100)
	elementos := recolectarItems(prueba, spider, dominio.Job{EnrichEAN: true})
	if len(elementos) != 1 {
		prueba.Fatalf("items = %d", len(elementos))
	}
	elemento := elementos[0]
	if elemento.EanGtin == nil || *elemento.EanGtin != "7800000000001" {
		prueba.Errorf("ean = %v", elemento.EanGtin)
	}
	if elemento.SKU == nil || *elemento.SKU != "sku-detail" {
		prueba.Errorf("sku = %v", elemento.SKU)
	}
	if elemento.Precio == nil || *elemento.Precio != 990 {
		prueba.Errorf("precio = %v", elemento.Precio)
	}
	if elemento.Categoria != "cat" || elemento.Supermercado != "Jumbo" {
		prueba.Errorf("categoría/supermercado = %q/%q", elemento.Categoria, elemento.Supermercado)
	}
}

func TestJumboOmiteCategoriaConRobotsProhibido(prueba *testing.T) {
	directorioCategorias := prueba.TempDir()
	urlCategoria := "https://www.jumbo.cl/cat"
	if errorEscritura := os.WriteFile(filepath.Join(directorioCategorias, "jumbo_categories.txt"), []byte(urlCategoria+"\n"), 0o600); errorEscritura != nil {
		prueba.Fatal(errorEscritura)
	}
	obtenedor := &obtenedorFalso{errores: map[string]error{
		urlCategoria: fmt.Errorf("%w: %s", extraccion.ErrRobotsDisallowed, urlCategoria),
	}}
	spider := construirSpiderDePrueba(prueba, "jumbo_rsc", obtenedor, directorioCategorias, 100)
	if errorObtenido := spider.Run(context.Background(), dominio.Job{}, func(dominio.Item) error { return nil }); errorObtenido != nil {
		prueba.Fatalf("Run() error = %v", errorObtenido)
	}
}

func TestJumboLeeElArchivoDeCategoriasConComentarios(prueba *testing.T) {
	directorio := prueba.TempDir()
	archivo := filepath.Join(directorio, "jumbo_categories.txt")
	contenido := "# comentario\n\nhttps://www.jumbo.cl/a\nhttps://www.jumbo.cl/b\n"
	if errorEscritura := os.WriteFile(archivo, []byte(contenido), 0o600); errorEscritura != nil {
		prueba.Fatal(errorEscritura)
	}
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		"https://www.jumbo.cl/a": respuestaExitosa("https://www.jumbo.cl/a", `<html></html>`),
		"https://www.jumbo.cl/b": respuestaExitosa("https://www.jumbo.cl/b", `<html></html>`),
	}}
	spider := construirSpiderDePrueba(prueba, "jumbo_rsc", obtenedor, directorio, 100)
	if errorObtenido := spider.Run(context.Background(), dominio.Job{}, func(dominio.Item) error { return nil }); errorObtenido != nil {
		prueba.Fatalf("Run() error = %v", errorObtenido)
	}
	if len(obtenedor.llamadas) < 2 || obtenedor.llamadas[0] != "https://www.jumbo.cl/a" || obtenedor.llamadas[1] != "https://www.jumbo.cl/b" {
		prueba.Errorf("calls = %v", obtenedor.llamadas)
	}
}
