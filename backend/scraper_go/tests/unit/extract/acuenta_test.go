package extract_test

import (
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

func acuentaSite(prueba *testing.T) string {
	prueba.Helper()
	return "acuenta_rsc"
}

func TestAcuentaProductsDesdeTarjetas(prueba *testing.T) {
	body := `
	<main><div class="product-card">
	  <a href="/p/helado-chocolate-1-l-savory-1075736">Helado Chocolate Suizo 1 L Savory</a>
	  <img src="/images/helado.webp">
	  <div>$2.950 ($3.490) $2.950 por LT</div>
	</div></main>
	`
	products := extract.AcuentaProducts(site(prueba, acuentaSite(prueba)), []byte(body), mustURL(prueba, "https://www.acuenta.cl/ca/congelados/04?currentPage=2"))
	if len(products) != 1 {
		prueba.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	assertProductFields(prueba, product, map[string]string{
		"producto": "Helado Chocolate Suizo 1 L Savory",
		"sku":      "ACUENTA-1075736",
		"marca":    "Savory",
		"formato":  "1 L",
		"imagen":   "https://www.acuenta.cl/images/helado.webp",
	})
	if value, ok := floatValue(product.Precio); !ok || value != 2950 {
		prueba.Errorf("precio = %v", product.Precio)
	}
	if value, ok := floatValue(product.PrecioNormal); !ok || value != 3490 {
		prueba.Errorf("precio_normal = %v", product.PrecioNormal)
	}
	if value, ok := floatValue(product.PrecioOferta); !ok || value != 2950 {
		prueba.Errorf("precio_oferta = %v", product.PrecioOferta)
	}
}

func TestAcuentaFusionaLaOfertaVisibleEnElProductoRSC(prueba *testing.T) {
	body := `<main><div class="product-card">` +
		`<a href="/p/helado-chocolate-1-l-savory-1075736">Helado Chocolate Suizo 1 L Savory</a>` +
		`<div>$2.590 ($3.490) $2.590 por LT</div>` +
		`</div></main>` +
		`<script>self.__next_f.push([1,` +
		`\"name\":\"Helado Chocolate Suizo 1 L Savory\",\"sku\":\"1075736\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"helado-chocolate-1-l-savory-1075736\",\"brand\":\"Savory\",\"stock\":1,` +
		`\"priceBeforeTaxes\":3490,\"promotion\":\"$p1\"` +
		`])</script>`
	products := extract.AcuentaProducts(site(prueba, acuentaSite(prueba)), []byte(body), mustURL(prueba, "https://www.acuenta.cl/ca/congelados/04?currentPage=2"))
	if len(products) != 1 {
		prueba.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	if value, ok := floatValue(product.Precio); !ok || value != 2590 {
		prueba.Errorf("precio = %v", product.Precio)
	}
	if value, ok := floatValue(product.PrecioNormal); !ok || value != 3490 {
		prueba.Errorf("precio_normal = %v", product.PrecioNormal)
	}
	if value, ok := floatValue(product.PrecioOferta); !ok || value != 2590 {
		prueba.Errorf("precio_oferta = %v", product.PrecioOferta)
	}
}

func TestAcuentaNoCruzaLosLimitesDeLosScriptsRSC(prueba *testing.T) {
	body := `<script>self.__next_f.push([1,` +
		`\"name\":\"Producto limpio\",\"sku\":\"123\",\"ean\":[\"7800000000001\"],\"maxQty\":1,` +
		`\"slug\":\"producto-limpio-123\",\"brand\":\"Marca\",\"stock\":1,` +
		`\"priceBeforeTaxes\":990,\"promotion\":null` +
		`])</script><script>self.__next_f.push([1,` +
		`\"name\":\"otro bloque del runtime\"` +
		`])</script>`
	products := extract.AcuentaProducts(site(prueba, acuentaSite(prueba)), []byte(body), mustURL(prueba, "https://www.acuenta.cl/ca/congelados/04"))
	if len(products) != 1 {
		prueba.Fatalf("productos = %d, want 1", len(products))
	}
	assertProductFields(prueba, products[0], map[string]string{
		"producto": "Producto Limpio",
		"ean":      "7800000000001",
	})
}

func TestAcuentaUsaLosCamposQueSiguenAlSKU(prueba *testing.T) {
	body := `<script>self.__next_f.push([1,` +
		`\"name\":\"Congelados\",\"slug\":\"congelados/04\",` +
		`\"brand\":\"Categoria\",\"stock\":1,\"priceBeforeTaxes\":5000,` +
		`\"sku\":\"434645\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"salteado-chapsui-wok-congelado-500-g-frutos-del-maipo-434645\",` +
		`\"brand\":\"Frutos del Maipo\",\"stock\":25,\"priceBeforeTaxes\":2000,` +
		`\"promotion\":null` +
		`])</script>`
	products := extract.AcuentaProducts(site(prueba, acuentaSite(prueba)), []byte(body), mustURL(prueba, "https://www.acuenta.cl/ca/congelados/04"))
	if len(products) != 1 {
		prueba.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	assertProductFields(prueba, product, map[string]string{
		"producto": "Salteado Chapsui Wok Congelado 500 G Frutos Del Maipo",
		"url":      "https://www.acuenta.cl/p/salteado-chapsui-wok-congelado-500-g-frutos-del-maipo-434645",
	})
	if value, ok := floatValue(product.Precio); !ok || value != 2000 {
		prueba.Errorf("precio = %v", product.Precio)
	}
}

func TestAcuentaResuelveReferenciasDelPayloadRSC(prueba *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantEAN   string
		wantPrice float64
		wantOffer bool
		wantImage string
	}{
		{
			name: "ean",
			body: `<script>self.__next_f.push([1,` +
				`\"name\":\"Producto referencia\",\"sku\":\"456\",\"ean\":\"$ec\",\"maxQty\":1,` +
				`\"slug\":\"producto-referencia-456\",\"brand\":\"Marca\",\"stock\":1,` +
				`\"priceBeforeTaxes\":990,\"promotion\":null` +
				`])</script><script>self.__next_f.push([1,` +
				`\nec:[\"7800000000002\"]` +
				`])</script>`,
			wantEAN:   "7800000000002",
			wantPrice: 990,
		},
		{
			name: "precio especial",
			body: `<script>self.__next_f.push([1,` +
				`\"name\":\"Producto oferta\",\"sku\":\"123\",\"ean\":null,\"maxQty\":1,` +
				`\"slug\":\"producto-oferta-123\",\"brand\":\"Marca\",\"stock\":1,` +
				`\"priceBeforeTaxes\":3490,\"promotion\":\"$d7\"` +
				`])</script><script>self.__next_f.push([1,` +
				`\nd9:{\"quantity\":0,\"price\":2590,\"priceBeforeTaxes\":2590}` +
				`\nd8:[\"$d9\"]` +
				`\nd7:{\"type\":\"specialPrice\",\"conditions\":\"$d8\"}` +
				`])</script>`,
			wantPrice: 2590,
			wantOffer: true,
		},
		{
			name: "imagen",
			body: `<script>self.__next_f.push([1,` +
				`\"name\":\"Producto imagen\",\"sku\":\"789\",\"ean\":null,\"maxQty\":1,` +
				`\"slug\":\"producto-imagen-789\",\"brand\":\"Marca\",\"stock\":1,` +
				`\"photosUrl\":\"$d5\",\"priceBeforeTaxes\":990,\"promotion\":null` +
				`])</script><script>self.__next_f.push([1,` +
				`\nd5:[\"https://images.test/producto.jpg\"]` +
				`])</script>`,
			wantPrice: 990,
			wantImage: "https://images.test/producto.jpg",
		},
	}
	for _, tc := range cases {
		prueba.Run(tc.name, func(prueba *testing.T) {
			products := extract.AcuentaProducts(site(prueba, acuentaSite(prueba)), []byte(tc.body), mustURL(prueba, "https://www.acuenta.cl/ca/congelados/04"))
			if len(products) != 1 {
				prueba.Fatalf("productos = %d, want 1", len(products))
			}
			product := products[0]
			if got := stringValue(product.EanGtin); got != tc.wantEAN {
				prueba.Errorf("ean = %q, want %q", got, tc.wantEAN)
			}
			if value, ok := floatValue(product.Precio); !ok || value != tc.wantPrice {
				prueba.Errorf("precio = %v, want %v", product.Precio, tc.wantPrice)
			}
			if got := product.PrecioOferta != nil; got != tc.wantOffer {
				prueba.Errorf("precio_oferta = %v, want oferta=%t", product.PrecioOferta, tc.wantOffer)
			}
			if tc.wantImage != "" {
				if got := stringValue(product.Imagen); got != tc.wantImage {
					prueba.Errorf("imagen = %q, want %q", got, tc.wantImage)
				}
			}
		})
	}
}

func TestAcuentaNoUsaElPrecioMultiunidadComoOferta(prueba *testing.T) {
	body := `<script>self.__next_f.push([1,` +
		`\"name\":\"Producto pack\",\"sku\":\"456\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"producto-pack-456\",\"brand\":\"Marca\",\"stock\":1,` +
		`\"priceBeforeTaxes\":3490,\"promotion\":\"$p1\"` +
		`])</script><script>self.__next_f.push([1,` +
		`\nc1:{\"quantity\":5,\"price\":2000,\"priceBeforeTaxes\":2000}` +
		`\nc0:[\"$c1\"]` +
		`\np1:{\"type\":\"nx$\",\"conditions\":\"$c0\"}` +
		`])</script>`
	products := extract.AcuentaProducts(site(prueba, acuentaSite(prueba)), []byte(body), mustURL(prueba, "https://www.acuenta.cl/ca/congelados/04"))
	if len(products) != 1 {
		prueba.Fatalf("productos = %d, want 1", len(products))
	}
	if products[0].PrecioOferta != nil {
		prueba.Errorf("precio_oferta = %v, want nil", products[0].PrecioOferta)
	}
}

func TestAcuentaCodificaLosCorchetesDeLaImagen(prueba *testing.T) {
	body := `<script>self.__next_f.push([1,` +
		`\"name\":\"Producto imagen\",\"sku\":\"4756977\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"producto-imagen-4756977\",\"brand\":\"Marca\",\"stock\":1,` +
		`\"priceBeforeTaxes\":990,\"promotion\":null` +
		`])</script>` +
		`d5:["https://images.lider.cl/wmtcl?source=url[file:/productos/4756977aa.jpg]\u0026sink"]`
	products := extract.AcuentaProducts(site(prueba, acuentaSite(prueba)), []byte(body), mustURL(prueba, "https://www.acuenta.cl/ca/congelados/04"))
	if len(products) != 1 {
		prueba.Fatalf("productos = %d, want 1", len(products))
	}
	want := "https://images.lider.cl/wmtcl?source=url%5Bfile:/productos/4756977aa.jpg%5D&sink"
	assertProductFields(prueba, products[0], map[string]string{"imagen": want})
}

func TestAcuentaProductSeleccionaLaFichaSolicitada(prueba *testing.T) {
	body := `<script>self.__next_f.push([1,` +
		`\"sku\":\"1075736\",\"name\":\"Helado Chocolate 1 L Savory\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"helado-chocolate-1-l-savory-1075736\",\"brand\":\"Savory\",\"stock\":1,` +
		`\"priceBeforeTaxes\":3490,\"promotion\":null` +
		`])</script>`
	requested := "https://www.acuenta.cl/p/helado-chocolate-1-l-savory-1075736"
	product := extract.AcuentaProduct(site(prueba, acuentaSite(prueba)), []byte(body), requested, mustURL(prueba, requested))
	if product == nil {
		prueba.Fatal("AcuentaProduct() = nil")
	}
	assertProductFields(prueba, *product, map[string]string{"producto": "Helado Chocolate 1 L Savory"})
}
