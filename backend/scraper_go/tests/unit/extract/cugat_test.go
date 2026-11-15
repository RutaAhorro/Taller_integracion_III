package extract_test

import (
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

func TestCugatProducts(prueba *testing.T) {
	body := `
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
	products := extract.CugatProducts(site(prueba, "cugat_rsc"), []byte(body), mustURL(prueba, "https://cugat.cl/categoria-producto/bebidas/page/2/"))
	if len(products) != 1 {
		prueba.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	assertProductFields(prueba, product, map[string]string{
		"producto": "Bebida Cugat 350cc",
		"ean":      "7801620000196",
		"sku":      "75118",
		"imagen":   "https://cugat.cl/uploads/bebida.webp",
	})
	if value, ok := floatValue(product.Precio); !ok || value != 6290 {
		prueba.Errorf("precio = %v", product.Precio)
	}
	if value, ok := floatValue(product.PrecioNormal); !ok || value != 7290 {
		prueba.Errorf("precio_normal = %v", product.PrecioNormal)
	}
	if value, ok := floatValue(product.PrecioOferta); !ok || value != 6290 {
		prueba.Errorf("precio_oferta = %v", product.PrecioOferta)
	}
	if value, ok := boolValue(product.EnStock); !ok || !value {
		prueba.Errorf("en_stock = %v", product.EnStock)
	}
}

func TestCugatProductsUsaNoscriptComoRespaldoDeImagen(prueba *testing.T) {
	body := `<div class="products"><div class="product-small product">
<p class="product-title"><a href="/producto/uno/">Uno</a></p>
<noscript><img src="/uploads/uno.webp"></noscript>
<ins><span class="woocommerce-Price-amount">$1.000</span></ins>
</div></div>`
	products := extract.CugatProducts(site(prueba, "cugat_rsc"), []byte(body), mustURL(prueba, "https://cugat.cl/categoria/"))
	if len(products) != 1 {
		prueba.Fatalf("productos = %d", len(products))
	}
	assertProductFields(prueba, products[0], map[string]string{
		"imagen": "https://cugat.cl/uploads/uno.webp",
	})
}

func TestCugatProductDesdeJSONLD(prueba *testing.T) {
	body := `<script type="application/ld+json">
{"@context":"https://schema.org","@type":"Product","name":"Bebida Cugat",
"sku":"7800000000001","gtin13":"7800000000001","brand":{"@type":"Brand","name":"Marca Test"},
"image":[{"url":"https://cugat.cl/bebida.webp"}],
"offers":{"price":"1.290","availability":"https://schema.org/InStock"}}
</script>`
	product := extract.CugatProduct(site(prueba, "cugat_rsc"), []byte(body), mustURL(prueba, "https://cugat.cl/producto/bebida-cugat/"))
	if product == nil {
		prueba.Fatal("CugatProduct() = nil")
	}
	assertProductFields(prueba, *product, map[string]string{
		"ean":    "7800000000001",
		"marca":  "Marca Test",
		"imagen": "https://cugat.cl/bebida.webp",
	})
	if value, ok := floatValue(product.Precio); !ok || value != 1290 {
		prueba.Errorf("precio = %v", product.Precio)
	}
	if value, ok := boolValue(product.EnStock); !ok || !value {
		prueba.Errorf("en_stock = %v", product.EnStock)
	}
}

func TestCugatProductDerivaSKUDeLaURL(prueba *testing.T) {
	body := `<script type="application/ld+json">
{"@context":"https://schema.org","@type":"Product","name":"Bebida Cugat",
"gtin13":"7800000000001","offers":{"price":"1.290","availability":"https://schema.org/InStock"}}
</script>`
	product := extract.CugatProduct(site(prueba, "cugat_rsc"), []byte(body), mustURL(prueba, "https://cugat.cl/producto/bebida-cugat/"))
	if product == nil {
		prueba.Fatal("CugatProduct() = nil")
	}
	assertProductFields(prueba, *product, map[string]string{"sku": "bebida-cugat"})
}
