// cugat_test.go verifica el parseo HTML/JSON-LD y la paginación de Cugat.
package extraccion_test

import (
	"testing"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
)

// ===== Parseo de Productos de Cugat =====

func TestCugatProducts(prueba *testing.T) {
	cuerpoHTML := `
	<div class="products"><div class="product-small col product type-product post-75118 instock"
		data-product_id="75118" data-product_sku="7801620000196">
	  <a href="/producto/bebida-cugat/"><img data-src="/uploads/bebida.webp"></a>
	  <p class="name product-title woocommerce-loop-product__title">
		<a href="/producto/bebida-cugat/">Bebida Cugat 350cc</a>
	  </p>
	  <span class="price"><del><span class="woocommerce-Price-amount amount">$7.290</span></del>
		<ins><span class="woocommerce-Price-amount amount">$6.290</span></ins></span>
	  <a class="add_to_cart_button" data-product_id="75118" data-product_sku="7801620000196"></a>
	</div></div>
	`
	productos := extraccion.CugatProducts(sitioDelCatalogo(prueba, "cugat_rsc"), []byte(cuerpoHTML), urlDePrueba(prueba, "https://cugat.cl/categoria-producto/bebidas/page/2/"))
	if len(productos) != 1 {
		prueba.Fatalf("productos = %d, esperado 1", len(productos))
	}
	producto := productos[0]
	verificarCamposProducto(prueba, producto, map[string]string{
		"producto": "Bebida Cugat 350cc",
		"ean":      "7801620000196",
		"sku":      "75118",
		"imagen":   "https://cugat.cl/uploads/bebida.webp",
	})
	if valor, valido := valorFlotante(producto.Precio); !valido || valor != 6290 {
		prueba.Errorf("precio = %v", producto.Precio)
	}
	if valor, valido := valorFlotante(producto.PrecioNormal); !valido || valor != 7290 {
		prueba.Errorf("precio_normal = %v", producto.PrecioNormal)
	}
	if valor, valido := valorFlotante(producto.PrecioOferta); !valido || valor != 6290 {
		prueba.Errorf("precio_oferta = %v", producto.PrecioOferta)
	}
	if valor, valido := valorBooleano(producto.EnStock); !valido || !valor {
		prueba.Errorf("en_stock = %v", producto.EnStock)
	}
}

func TestCugatProductsUsaNoscriptComoRespaldoDeImagen(prueba *testing.T) {
	cuerpoHTML := `<div class="products"><div class="product-small product">
<p class="product-title"><a href="/producto/uno/">Uno</a></p>
<noscript><img src="/uploads/uno.webp"></noscript>
<ins><span class="woocommerce-Price-amount">$1.000</span></ins>
</div></div>`
	productos := extraccion.CugatProducts(sitioDelCatalogo(prueba, "cugat_rsc"), []byte(cuerpoHTML), urlDePrueba(prueba, "https://cugat.cl/categoria/"))
	if len(productos) != 1 {
		prueba.Fatalf("productos = %d", len(productos))
	}
	verificarCamposProducto(prueba, productos[0], map[string]string{
		"imagen": "https://cugat.cl/uploads/uno.webp",
	})
}

func TestCugatProductDesdeJSONLD(prueba *testing.T) {
	cuerpoHTML := `<script type="application/ld+json">
{"@context":"https://schema.org","@type":"Product","name":"Bebida Cugat",
"sku":"7800000000001","gtin13":"7800000000001","brand":{"@type":"Brand","name":"Marca Test"},
"image":[{"url":"https://cugat.cl/bebida.webp"}],
"offers":{"price":"1.290","availability":"https://schema.org/InStock"}}
</script>`
	producto := extraccion.CugatProduct(sitioDelCatalogo(prueba, "cugat_rsc"), []byte(cuerpoHTML), urlDePrueba(prueba, "https://cugat.cl/producto/bebida-cugat/"))
	if producto == nil {
		prueba.Fatal("CugatProduct() = nil")
	}
	verificarCamposProducto(prueba, *producto, map[string]string{
		"ean":    "7800000000001",
		"marca":  "Marca Test",
		"imagen": "https://cugat.cl/bebida.webp",
	})
	if valor, valido := valorFlotante(producto.Precio); !valido || valor != 1290 {
		prueba.Errorf("precio = %v", producto.Precio)
	}
	if valor, valido := valorBooleano(producto.EnStock); !valido || !valor {
		prueba.Errorf("en_stock = %v", producto.EnStock)
	}
}

func TestCugatProductDerivaSKUDeLaURL(prueba *testing.T) {
	cuerpoHTML := `<script type="application/ld+json">
{"@context":"https://schema.org","@type":"Product","name":"Bebida Cugat",
"gtin13":"7800000000001","offers":{"price":"1.290","availability":"https://schema.org/InStock"}}
</script>`
	producto := extraccion.CugatProduct(sitioDelCatalogo(prueba, "cugat_rsc"), []byte(cuerpoHTML), urlDePrueba(prueba, "https://cugat.cl/producto/bebida-cugat/"))
	if producto == nil {
		prueba.Fatal("CugatProduct() = nil")
	}
	verificarCamposProducto(prueba, *producto, map[string]string{"sku": "bebida-cugat"})
}

// ===== Recorrido del Spider de Cugat =====

func cuerpoCategoriaCugat(nombre, ruta, precio string) string {
	return `<div class="products"><div class="product-small product">
<a href="/producto/` + ruta + `"><img data-src="/` + ruta + `.webp"></a>
<p class="product-title"><a href="/producto/` + ruta + `">` + nombre + `</a></p>
<ins><span class="woocommerce-Price-amount">` + precio + `</span></ins>
</div></div>`
}

func TestCugatProductMode(prueba *testing.T) {
	urlProducto := "https://cugat.cl/producto/bebida-cugat/"
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		urlProducto: respuestaExitosa(urlProducto, `<script type="application/ld+json">
{"@type":"Product","name":"Bebida Cugat","url":"https://cugat.cl/producto/bebida-cugat/","offers":{"price":"990"}}
</script>`),
	}}
	spider := construirSpiderDePrueba(prueba, "cugat_rsc", obtenedor, prueba.TempDir(), 100)
	elementos := recolectarItems(prueba, spider, dominio.Job{ProductURL: urlProducto})
	if len(elementos) != 1 {
		prueba.Fatalf("items = %d", len(elementos))
	}
	if elementos[0].Producto != "Bebida Cugat" || elementos[0].Categoria != "producto" || elementos[0].Supermercado != "Cugat" {
		prueba.Errorf("item = %+v", elementos[0])
	}
}

func TestCugatPaginacionDeCategoria(prueba *testing.T) {
	urlCategoria := "https://cugat.cl/categoria-producto/bebidas/"
	pagina2 := "https://cugat.cl/categoria-producto/bebidas/page/2/"
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		urlCategoria: respuestaExitosa(urlCategoria, cuerpoCategoriaCugat("Uno", "uno", "$1.000")),
		pagina2:      respuestaExitosa(pagina2, cuerpoCategoriaCugat("Dos", "dos", "$2.000")),
	}}
	spider := construirSpiderDePrueba(prueba, "cugat_rsc", obtenedor, prueba.TempDir(), 100, urlCategoria)
	elementos := recolectarItems(prueba, spider, dominio.Job{})
	if len(elementos) != 2 {
		prueba.Fatalf("items = %d", len(elementos))
	}
	for _, elemento := range elementos {
		if elemento.Categoria != "bebidas" || elemento.Supermercado != "Cugat" {
			prueba.Errorf("item = %+v", elemento)
		}
	}
}
