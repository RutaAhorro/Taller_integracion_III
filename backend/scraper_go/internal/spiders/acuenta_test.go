package spiders

import "testing"

func TestExtractAcuentaProductCard(t *testing.T) {
	body := `
	<main><div class="product-card">
	  <a href="/p/helado-chocolate-1-l-savory-1075736">Helado Chocolate Suizo 1 L Savory</a>
	  <img src="/images/helado.webp">
	  <div>$2.950 ($3.490) $2.950 por LT</div>
	</div></main>
	`
	products := extractAcuentaProducts([]byte(body), mustURL("https://www.acuenta.cl/ca/congelados/04?currentPage=2"))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	if product.Producto != "Helado Chocolate Suizo 1 L Savory" {
		t.Errorf("producto = %q", product.Producto)
	}
	if product.Precio == nil || *product.Precio != 2950 {
		t.Errorf("precio = %v", product.Precio)
	}
	if product.PrecioNormal == nil || *product.PrecioNormal != 3490 {
		t.Errorf("precio_normal = %v", product.PrecioNormal)
	}
	if product.PrecioOferta == nil || *product.PrecioOferta != 2950 {
		t.Errorf("precio_oferta = %v", product.PrecioOferta)
	}
	if product.SKU == nil || *product.SKU != "ACUENTA-1075736" {
		t.Errorf("sku = %v", product.SKU)
	}
	if product.Marca == nil || *product.Marca != "Savory" {
		t.Errorf("marca = %v", product.Marca)
	}
	if product.FormatoCrudo == nil || *product.FormatoCrudo != "1 L" {
		t.Errorf("formato = %v", product.FormatoCrudo)
	}
	if product.Imagen == nil || *product.Imagen != "https://www.acuenta.cl/images/helado.webp" {
		t.Errorf("imagen = %v", product.Imagen)
	}
}

func TestAcuentaMergesVisibleOfferIntoRSCProduct(t *testing.T) {
	body := `<main><div class="product-card">` +
		`<a href="/p/helado-chocolate-1-l-savory-1075736">Helado Chocolate Suizo 1 L Savory</a>` +
		`<div>$2.590 ($3.490) $2.590 por LT</div>` +
		`</div></main>` +
		`<script>self.__next_f.push([1,` +
		`\"name\":\"Helado Chocolate Suizo 1 L Savory\",\"sku\":\"1075736\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"helado-chocolate-1-l-savory-1075736\",\"brand\":\"Savory\",\"stock\":1,` +
		`\"priceBeforeTaxes\":3490,\"promotion\":\"$p1\"` +
		`])</script>`
	products := extractAcuentaProducts([]byte(body), mustURL("https://www.acuenta.cl/ca/congelados/04?currentPage=2"))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	if product.Precio == nil || *product.Precio != 2590 {
		t.Errorf("precio = %v", product.Precio)
	}
	if product.PrecioNormal == nil || *product.PrecioNormal != 3490 {
		t.Errorf("precio_normal = %v", product.PrecioNormal)
	}
	if product.PrecioOferta == nil || *product.PrecioOferta != 2590 {
		t.Errorf("precio_oferta = %v", product.PrecioOferta)
	}
}

func TestAcuentaPageURLRemovesInternalToken(t *testing.T) {
	rawURL := "https://www.acuenta.cl/ca/congelados/04?currentPage=2&_rsc=2x5uv"
	if got := acuentaPageURL(rawURL, 3); got != "https://www.acuenta.cl/ca/congelados/04?currentPage=3" {
		t.Errorf("page URL = %q", got)
	}
	if got := acuentaCategoryFromURL(rawURL); got != "congelados" {
		t.Errorf("categoría = %q", got)
	}
}

func TestAcuentaMultiunitPromotion(t *testing.T) {
	current, normal, promotion := acuentaPricesFromText("$3.090 2 X $5.700 Carne molida")
	if current == nil || *current != 3090 {
		t.Errorf("precio = %v", current)
	}
	if normal == nil || *normal != 3090 {
		t.Errorf("precio_normal = %v", normal)
	}
	if promotion == nil || *promotion != "2 X $5.700" {
		t.Errorf("promoción = %v", promotion)
	}
}

func TestAcuentaDoesNotCrossRSCScriptBoundaries(t *testing.T) {
	body := `<script>self.__next_f.push([1,` +
		`\"name\":\"Producto limpio\",\"sku\":\"123\",\"ean\":[\"7800000000001\"],\"maxQty\":1,` +
		`\"slug\":\"producto-limpio-123\",\"brand\":\"Marca\",\"stock\":1,` +
		`\"priceBeforeTaxes\":990,\"promotion\":null` +
		`])</script><script>self.__next_f.push([1,` +
		`\"name\":\"otro bloque del runtime\"` +
		`])</script>`
	products := extractAcuentaRSCProducts([]byte(body), mustURL("https://www.acuenta.cl/ca/congelados/04"))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	if products[0].Producto != "Producto Limpio" {
		t.Errorf("producto = %q", products[0].Producto)
	}
	if products[0].EanGtin == nil || *products[0].EanGtin != "7800000000001" {
		t.Errorf("ean = %v", products[0].EanGtin)
	}
}

func TestAcuentaUsesFieldsAfterSKU(t *testing.T) {
	body := `<script>self.__next_f.push([1,` +
		`\"name\":\"Congelados\",\"slug\":\"congelados/04\",` +
		`\"brand\":\"Categoria\",\"stock\":1,\"priceBeforeTaxes\":5000,` +
		`\"sku\":\"434645\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"salteado-chapsui-wok-congelado-500-g-frutos-del-maipo-434645\",` +
		`\"brand\":\"Frutos del Maipo\",\"stock\":25,\"priceBeforeTaxes\":2000,` +
		`\"promotion\":null` +
		`])</script>`
	products := extractAcuentaRSCProducts([]byte(body), mustURL("https://www.acuenta.cl/ca/congelados/04"))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	if product.Producto != "Salteado Chapsui Wok Congelado 500 G Frutos Del Maipo" {
		t.Errorf("producto = %q", product.Producto)
	}
	if product.URLProducto == nil || *product.URLProducto != "https://www.acuenta.cl/p/salteado-chapsui-wok-congelado-500-g-frutos-del-maipo-434645" {
		t.Errorf("url = %v", product.URLProducto)
	}
	if product.Precio == nil || *product.Precio != 2000 {
		t.Errorf("precio = %v", product.Precio)
	}
}

func TestAcuentaResolvesEANReference(t *testing.T) {
	body := `<script>self.__next_f.push([1,` +
		`\"name\":\"Producto referencia\",\"sku\":\"456\",\"ean\":\"$ec\",\"maxQty\":1,` +
		`\"slug\":\"producto-referencia-456\",\"brand\":\"Marca\",\"stock\":1,` +
		`\"priceBeforeTaxes\":990,\"promotion\":null` +
		`])</script><script>self.__next_f.push([1,` +
		`\nec:[\"7800000000002\"]` +
		`])</script>`
	products := extractAcuentaRSCProducts([]byte(body), mustURL("https://www.acuenta.cl/ca/congelados/04"))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	if products[0].EanGtin == nil || *products[0].EanGtin != "7800000000002" {
		t.Errorf("ean = %v", products[0].EanGtin)
	}
}

func TestAcuentaResolvesSpecialPriceChain(t *testing.T) {
	body := `<script>self.__next_f.push([1,` +
		`\"name\":\"Producto oferta\",\"sku\":\"123\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"producto-oferta-123\",\"brand\":\"Marca\",\"stock\":1,` +
		`\"priceBeforeTaxes\":3490,\"promotion\":\"$d7\"` +
		`])</script><script>self.__next_f.push([1,` +
		`\nd9:{\"quantity\":0,\"price\":2590,\"priceBeforeTaxes\":2590}` +
		`\nd8:[\"$d9\"]` +
		`\nd7:{\"type\":\"specialPrice\",\"conditions\":\"$d8\"}` +
		`])</script>`
	products := extractAcuentaRSCProducts([]byte(body), mustURL("https://www.acuenta.cl/ca/congelados/04"))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	if product.Precio == nil || *product.Precio != 2590 {
		t.Errorf("precio = %v", product.Precio)
	}
	if product.PrecioNormal == nil || *product.PrecioNormal != 3490 {
		t.Errorf("precio_normal = %v", product.PrecioNormal)
	}
	if product.PrecioOferta == nil || *product.PrecioOferta != 2590 {
		t.Errorf("precio_oferta = %v", product.PrecioOferta)
	}
}

func TestAcuentaDoesNotUseMultiunitRSCPriceAsOffer(t *testing.T) {
	body := `<script>self.__next_f.push([1,` +
		`\"name\":\"Producto pack\",\"sku\":\"456\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"producto-pack-456\",\"brand\":\"Marca\",\"stock\":1,` +
		`\"priceBeforeTaxes\":3490,\"promotion\":\"$p1\"` +
		`])</script><script>self.__next_f.push([1,` +
		`\nc1:{\"quantity\":5,\"price\":2000,\"priceBeforeTaxes\":2000}` +
		`\nc0:[\"$c1\"]` +
		`\np1:{\"type\":\"nx$\",\"conditions\":\"$c0\"}` +
		`])</script>`
	products := extractAcuentaRSCProducts([]byte(body), mustURL("https://www.acuenta.cl/ca/congelados/04"))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	if products[0].PrecioOferta != nil {
		t.Errorf("precio_oferta = %v", products[0].PrecioOferta)
	}
}

func TestAcuentaResolvesImageReference(t *testing.T) {
	body := `<script>self.__next_f.push([1,` +
		`\"name\":\"Producto imagen\",\"sku\":\"789\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"producto-imagen-789\",\"brand\":\"Marca\",\"stock\":1,` +
		`\"photosUrl\":\"$d5\",\"priceBeforeTaxes\":990,\"promotion\":null` +
		`])</script><script>self.__next_f.push([1,` +
		`\nd5:[\"https://images.test/producto.jpg\"]` +
		`])</script>`
	products := extractAcuentaRSCProducts([]byte(body), mustURL("https://www.acuenta.cl/ca/congelados/04"))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	if products[0].Imagen == nil || *products[0].Imagen != "https://images.test/producto.jpg" {
		t.Errorf("imagen = %v", products[0].Imagen)
	}
}

func TestAcuentaUsesExactImageVariantForSKU(t *testing.T) {
	text := `d5:["https://images.lider.cl/wmtcl?source=url[file:/productos/` +
		`4756977aa.jpg]\u0026sink"]`
	got := acuentaImageFromRSC(text, "4756977")
	want := "https://images.lider.cl/wmtcl?source=url[file:/productos/4756977aa.jpg]&sink"
	if got != want {
		t.Errorf("imagen = %q, want %q", got, want)
	}
}

func TestAcuentaEncodesImageProxyBrackets(t *testing.T) {
	got := acuentaCanonicalImageURL("https://images.lider.cl/wmtcl?source=url[file:/productos/4756977aa.jpg]&sink")
	if got == nil || *got != "https://images.lider.cl/wmtcl?source=url%5Bfile:/productos/4756977aa.jpg%5D&sink" {
		t.Errorf("imagen = %v", got)
	}
}

func TestValidateAcuentaCategoryURL(t *testing.T) {
	if err := validateAcuentaCategoryURL("https://example.com/ca/congelados/04"); err == nil {
		t.Fatal("se esperaba error para host no confiable")
	}
	if err := validateAcuentaCategoryURL("https://www.acuenta.cl/ca/congelados/04"); err != nil {
		t.Errorf("URL válida: %v", err)
	}
}
