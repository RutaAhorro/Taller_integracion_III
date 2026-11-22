// santa_isabel_test.go verifica el parseo __renderData/JSON-LD y el recorrido de páginas de Santa Isabel.
package extraccion_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
)

// ===== Auxiliares de Prueba para Santa Isabel =====

func cuerpoRenderDataSantaIsabel(prueba *testing.T, carga any) []byte {
	prueba.Helper()
	interno, errorInterno := json.Marshal(carga)
	if errorInterno != nil {
		prueba.Fatal(errorInterno)
	}
	externo, errorExterno := json.Marshal(string(interno))
	if errorExterno != nil {
		prueba.Fatal(errorExterno)
	}
	return []byte("<script>window.__renderData = " + string(externo) + ";</script>")
}

func cargaRenderDataSantaIsabel() map[string]any {
	return map[string]any{
		"plp": map[string]any{
			"plp_products": map[string]any{
				"products": []any{
					map[string]any{
						"productId":   "product-1",
						"productName": "Cafe molido 250 g",
						"brand":       "Marca Test",
						"linkText":    "cafe-molido-250-g-product-1",
						"items": []any{
							map[string]any{
								"itemId":          "sku-1",
								"ean":             "7800000000001",
								"measurementUnit": "un",
								"unitMultiplier":  1,
								"images": []any{
									map[string]any{"imageUrl": "https://img.test/cafe.jpg"},
								},
								"sellers": []any{
									map[string]any{
										"sellerName": "santaisabel",
										"commertialOffer": map[string]any{
											"Price":                "$990",
											"ListPrice":            "1.290",
											"PriceWithoutDiscount": "1290.00",
											"AvailableQuantity":    3,
											"teasers":              []any{map[string]any{"name": "2x1"}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func cuerpoCategoriaSantaIsabel() string {
	return `<script type="application/ld+json">
{"@type":"ItemList","itemListElement":[
 {"@type":"ListItem","position":1,"item":{"@type":"Product","name":"Cafe molido 250 g",
 "url":"https://www.santaisabel.cl/cafe-molido-250-g/p",
 "brand":{"@type":"Brand","name":"Marca Test"},
 "offers":{"price":990,"availability":"https://schema.org/InStock"}}}]}
</script>`
}

// ===== Parseo de Productos de Santa Isabel =====

func TestSantaIsabelProductsDesdeRenderData(prueba *testing.T) {
	cuerpoHTML := cuerpoRenderDataSantaIsabel(prueba, cargaRenderDataSantaIsabel())
	productos := extraccion.SantaIsabelProducts(sitioDelCatalogo(prueba, "santa_isabel_rsc"), cuerpoHTML, urlDePrueba(prueba, "https://www.santaisabel.cl/despensa"))
	if len(productos) != 1 {
		prueba.Fatalf("productos = %d, esperado 1", len(productos))
	}
	producto := productos[0]
	verificarCamposProducto(prueba, producto, map[string]string{
		"producto":  "Cafe molido 250 g",
		"ean":       "7800000000001",
		"sku":       "sku-1",
		"marca":     "Marca Test",
		"formato":   "1 un",
		"promocion": "2x1",
		"url":       "https://www.santaisabel.cl/cafe-molido-250-g-product-1/p",
		"imagen":    "https://img.test/cafe.jpg",
	})
	if valor, valido := valorFlotante(producto.Precio); !valido || valor != 990 {
		prueba.Errorf("precio = %v", producto.Precio)
	}
	if valor, valido := valorFlotante(producto.PrecioNormal); !valido || valor != 1290 {
		prueba.Errorf("precio_normal = %v", producto.PrecioNormal)
	}
	if valor, valido := valorFlotante(producto.PrecioOferta); !valido || valor != 990 {
		prueba.Errorf("precio_oferta = %v", producto.PrecioOferta)
	}
	if valor, valido := valorBooleano(producto.EnStock); !valido || !valor {
		prueba.Errorf("en_stock = %v", producto.EnStock)
	}
}

func TestSantaIsabelProductsDesdeJSONLD(prueba *testing.T) {
	cuerpoHTML := []byte(`<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "ItemList",
  "numberOfItems": 2,
  "itemListElement": [
    {
      "@type": "ListItem",
      "position": 1,
      "item": {
        "@type": "Product",
        "name": "Plátano Extra Granel (1 a 2 un. Aprox)",
        "url": "https://www.santaisabel.cl/platano-granel/p",
        "image": "https://santaisabel.vteximg.com.br/platano.jpg",
        "brand": {"@type": "Brand", "name": "Frutas y Verduras Propias"},
        "offers": {
          "@type": "Offer",
          "priceCurrency": "CLP",
          "price": 745,
          "availability": "https://schema.org/InStock"
        }
      }
    },
    {
      "@type": "ListItem",
      "position": 2,
      "item": {
        "@type": "Product",
        "name": "Palta Hass Malla 1 kg",
        "url": "https://www.santaisabel.cl/palta-hass-malla-1-kg/p",
        "image": ["https://santaisabel.vteximg.com.br/palta.jpg"],
        "brand": {"@type": "Brand", "name": "Frutas y Verduras Propias"},
        "offers": {
          "@type": "Offer",
          "price": 3590,
          "availability": "https://schema.org/OutOfStock"
        }
      }
    }
  ]
}
</script>`)
	productos := extraccion.SantaIsabelProducts(sitioDelCatalogo(prueba, "santa_isabel_rsc"), cuerpoHTML, urlDePrueba(prueba, "https://www.santaisabel.cl/frutas-y-verduras"))
	if len(productos) != 2 {
		prueba.Fatalf("productos = %d, esperado 2", len(productos))
	}
	primero := productos[0]
	verificarCamposProducto(prueba, primero, map[string]string{
		"producto": "Plátano Extra Granel (1 a 2 un. Aprox)",
		"marca":    "Frutas y Verduras Propias",
		"sku":      "platano-granel",
		"url":      "https://www.santaisabel.cl/platano-granel/p",
	})
	if valor, valido := valorFlotante(primero.Precio); !valido || valor != 745 {
		prueba.Errorf("precio = %v", primero.Precio)
	}
	if valor, valido := valorFlotante(primero.PrecioNormal); !valido || valor != 745 {
		prueba.Errorf("precio_normal = %v", primero.PrecioNormal)
	}
	if primero.PrecioOferta != nil {
		prueba.Errorf("precio_oferta = %v, esperado nil", primero.PrecioOferta)
	}
	if valor, valido := valorBooleano(primero.EnStock); !valido || !valor {
		prueba.Errorf("en_stock = %v", primero.EnStock)
	}
	if valor, valido := valorBooleano(productos[1].EnStock); !valido || valor {
		prueba.Errorf("en_stock segundo = %v", productos[1].EnStock)
	}
}

func TestSantaIsabelSoloUsaAlVendedorPropio(prueba *testing.T) {
	carga := map[string]any{
		"plp": map[string]any{
			"plp_products": map[string]any{
				"products": []any{
					map[string]any{
						"productName": "Cafe",
						"items": []any{
							map[string]any{
								"itemId": "sku-1",
								"sellers": []any{
									map[string]any{
										"sellerName":      "other-seller",
										"commertialOffer": map[string]any{"Price": 990},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	cuerpoHTML := cuerpoRenderDataSantaIsabel(prueba, carga)
	if productos := extraccion.SantaIsabelProducts(sitioDelCatalogo(prueba, "santa_isabel_rsc"), cuerpoHTML, urlDePrueba(prueba, "https://www.santaisabel.cl/despensa")); len(productos) != 0 {
		prueba.Fatalf("productos = %+v, esperado 0", productos)
	}
}

func TestSantaIsabelSinDatosDevuelveVacio(prueba *testing.T) {
	if productos := extraccion.SantaIsabelProducts(sitioDelCatalogo(prueba, "santa_isabel_rsc"), []byte("<html></html>"), urlDePrueba(prueba, "https://www.santaisabel.cl/despensa")); len(productos) != 0 {
		prueba.Fatalf("productos = %+v, esperado 0", productos)
	}
}

// ===== Recorrido del Spider de Santa Isabel =====

func TestSantaIsabelPaginacionDeCategoria(prueba *testing.T) {
	directorioCategorias := prueba.TempDir()
	urlCategoria := "https://www.santaisabel.cl/despensa"
	if errorEscritura := os.WriteFile(filepath.Join(directorioCategorias, "santa_isabel_categories.txt"), []byte(urlCategoria+"\n"), 0o600); errorEscritura != nil {
		prueba.Fatal(errorEscritura)
	}
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		urlCategoria: respuestaExitosa(urlCategoria, cuerpoCategoriaSantaIsabel()),
	}}
	spider := construirSpiderDePrueba(prueba, "santa_isabel_rsc", obtenedor, directorioCategorias, 100)
	elementos := recolectarItems(prueba, spider, dominio.Job{})
	if len(elementos) != 1 {
		prueba.Fatalf("items = %d, esperado 1", len(elementos))
	}
	if elementos[0].Categoria != "despensa" || elementos[0].Supermercado != "Santa Isabel" {
		prueba.Errorf("categoría/supermercado = %q/%q", elementos[0].Categoria, elementos[0].Supermercado)
	}
	if len(obtenedor.llamadas) != 2 || obtenedor.llamadas[1] != urlCategoria+"?page=2" {
		prueba.Errorf("calls = %v", obtenedor.llamadas)
	}
}

func TestSantaIsabelAddURLCreaElArchivoDeCategorias(prueba *testing.T) {
	directorioCategorias := prueba.TempDir()
	urlAdicional := "https://www.santaisabel.cl/extra"
	spider := construirSpiderDePrueba(prueba, "santa_isabel_rsc", &obtenedorFalso{}, directorioCategorias, 100)
	if errorObtenido := spider.Run(context.Background(), dominio.Job{AddURL: urlAdicional}, func(dominio.Item) error { return nil }); errorObtenido != nil {
		prueba.Fatalf("Run() error = %v", errorObtenido)
	}
	contenido, errorLectura := os.ReadFile(filepath.Join(directorioCategorias, "santa_isabel_categories.txt"))
	if errorLectura != nil {
		prueba.Fatalf("leer categorías: %v", errorLectura)
	}
	if string(contenido) != urlAdicional+"\n" {
		prueba.Errorf("archivo = %q", contenido)
	}
}

func TestSantaIsabelRechazaCategoriaNoConfiable(prueba *testing.T) {
	directorioCategorias := prueba.TempDir()
	if errorEscritura := os.WriteFile(
		filepath.Join(directorioCategorias, "santa_isabel_categories.txt"),
		[]byte("https://example.com/despensa\n"),
		0o600,
	); errorEscritura != nil {
		prueba.Fatal(errorEscritura)
	}
	obtenedor := &obtenedorFalso{}
	spider := construirSpiderDePrueba(prueba, "santa_isabel_rsc", obtenedor, directorioCategorias, 100)
	if errorObtenido := spider.Run(context.Background(), dominio.Job{}, func(dominio.Item) error { return nil }); errorObtenido == nil {
		prueba.Fatal("Run() debió rechazar una categoría de otro dominio")
	}
	if len(obtenedor.llamadas) != 0 {
		prueba.Errorf("requests = %v", obtenedor.llamadas)
	}
}

func TestSantaIsabelOmiteCategoriaConRobotsProhibido(prueba *testing.T) {
	directorioCategorias := prueba.TempDir()
	urlCategoria := "https://www.santaisabel.cl/despensa"
	if errorEscritura := os.WriteFile(filepath.Join(directorioCategorias, "santa_isabel_categories.txt"), []byte(urlCategoria+"\n"), 0o600); errorEscritura != nil {
		prueba.Fatal(errorEscritura)
	}
	obtenedor := &obtenedorFalso{errores: map[string]error{
		urlCategoria: fmt.Errorf("%w: %s", extraccion.ErrRobotsDisallowed, urlCategoria),
	}}
	spider := construirSpiderDePrueba(prueba, "santa_isabel_rsc", obtenedor, directorioCategorias, 100)
	if errorObtenido := spider.Run(context.Background(), dominio.Job{}, func(dominio.Item) error { return nil }); errorObtenido != nil {
		prueba.Fatalf("Run() error = %v", errorObtenido)
	}
}

func TestSantaIsabelProductMode(prueba *testing.T) {
	urlProducto := "https://www.santaisabel.cl/leche-colun-1-l/p"
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		urlProducto: respuestaExitosa(urlProducto, `<script type="application/ld+json">
{"@type":"Product","name":"Leche Colún 1 L","url":"/leche-colun-1-l/p",
 "gtin13":"7800000000001","sku":"SI-1","brand":{"name":"Colún"},
 "offers":{"price":"1290","listPrice":"1590","availability":"https://schema.org/InStock"}}
</script>`),
	}}
	spider := construirSpiderDePrueba(prueba, "santa_isabel_rsc", obtenedor, prueba.TempDir(), 100)
	elementos := recolectarItems(prueba, spider, dominio.Job{ProductURL: urlProducto})
	if len(elementos) != 1 {
		prueba.Fatalf("items = %d, esperado 1", len(elementos))
	}
	elemento := elementos[0]
	if elemento.Producto != "Leche Colún 1 L" || elemento.Categoria != "producto" || elemento.Supermercado != "Santa Isabel" {
		prueba.Errorf("item = %+v", elemento)
	}
	if elemento.URLProducto == nil || *elemento.URLProducto != urlProducto {
		prueba.Errorf("url = %v", elemento.URLProducto)
	}
	if len(obtenedor.llamadas) != 1 || obtenedor.llamadas[0] != urlProducto {
		prueba.Errorf("calls = %v, esperado sólo la ficha", obtenedor.llamadas)
	}
}

func TestSantaIsabelProductModeSeleccionaLaFichaSolicitada(prueba *testing.T) {
	urlProducto := "https://www.santaisabel.cl/leche-colun-1-l/p"
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		urlProducto: respuestaExitosa(urlProducto, `<script type="application/ld+json">
{"@graph":[
 {"@type":"Product","name":"Pan","url":"https://www.santaisabel.cl/pan/p","offers":{"price":"900"}},
 {"@type":"Product","name":"Leche Colún","url":"https://www.santaisabel.cl/leche-colun-1-l/p","offers":{"price":"1290"}}]}
</script>`),
	}}
	spider := construirSpiderDePrueba(prueba, "santa_isabel_rsc", obtenedor, prueba.TempDir(), 100)
	elementos := recolectarItems(prueba, spider, dominio.Job{ProductURL: urlProducto})
	if len(elementos) != 1 || elementos[0].Producto != "Leche Colún" {
		prueba.Fatalf("items = %+v", elementos)
	}
}

func TestSantaIsabelRechazaProductURLQueNoEsFicha(prueba *testing.T) {
	obtenedor := &obtenedorFalso{}
	spider := construirSpiderDePrueba(prueba, "santa_isabel_rsc", obtenedor, prueba.TempDir(), 100)
	errorObtenido := spider.Run(context.Background(), dominio.Job{ProductURL: "https://www.santaisabel.cl/despensa"}, func(dominio.Item) error {
		return nil
	})
	if errorObtenido == nil {
		prueba.Fatal("Run() debió rechazar una categoría como product_url")
	}
	if len(obtenedor.llamadas) != 0 {
		prueba.Errorf("requests = %v", obtenedor.llamadas)
	}
}
