// acuenta_test.go verifica el parseo de tarjetas HTML y payloads RSC de A Cuenta.
package extraccion_test

import (
	"context"
	"testing"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
)

// ===== Parseo de Productos de A Cuenta =====

func TestAcuentaProductsDesdeTarjetas(prueba *testing.T) {
	cuerpoHTML := `
	<main><div class="product-card">
	  <a href="/p/helado-chocolate-1-l-savory-1075736">Helado Chocolate Suizo 1 L Savory</a>
	  <img src="/images/helado.webp">
	  <div>$2.950 ($3.490) $2.950 por LT</div>
	</div></main>
	`
	productos := extraccion.AcuentaProducts(sitioDelCatalogo(prueba, "acuenta_rsc"), []byte(cuerpoHTML), urlDePrueba(prueba, "https://www.acuenta.cl/ca/congelados/04?currentPage=2"))
	if len(productos) != 1 {
		prueba.Fatalf("productos = %d, esperado 1", len(productos))
	}
	producto := productos[0]
	verificarCamposProducto(prueba, producto, map[string]string{
		"producto": "Helado Chocolate Suizo 1 L Savory",
		"sku":      "ACUENTA-1075736",
		"marca":    "Savory",
		"formato":  "1 L",
		"imagen":   "https://www.acuenta.cl/images/helado.webp",
	})
	if valor, valido := valorFlotante(producto.Precio); !valido || valor != 2950 {
		prueba.Errorf("precio = %v", producto.Precio)
	}
	if valor, valido := valorFlotante(producto.PrecioNormal); !valido || valor != 3490 {
		prueba.Errorf("precio_normal = %v", producto.PrecioNormal)
	}
	if valor, valido := valorFlotante(producto.PrecioOferta); !valido || valor != 2950 {
		prueba.Errorf("precio_oferta = %v", producto.PrecioOferta)
	}
}

func TestAcuentaFusionaLaOfertaVisibleEnElProductoRSC(prueba *testing.T) {
	cuerpoHTML := `<main><div class="product-card">` +
		`<a href="/p/helado-chocolate-1-l-savory-1075736">Helado Chocolate Suizo 1 L Savory</a>` +
		`<div>$2.590 ($3.490) $2.590 por LT</div>` +
		`</div></main>` +
		`<script>self.__next_f.push([1,` +
		`\"name\":\"Helado Chocolate Suizo 1 L Savory\",\"sku\":\"1075736\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"helado-chocolate-1-l-savory-1075736\",\"brand\":\"Savory\",\"stock\":1,` +
		`\"priceBeforeTaxes\":3490,\"promotion\":\"$p1\"` +
		`])</script>`
	productos := extraccion.AcuentaProducts(sitioDelCatalogo(prueba, "acuenta_rsc"), []byte(cuerpoHTML), urlDePrueba(prueba, "https://www.acuenta.cl/ca/congelados/04?currentPage=2"))
	if len(productos) != 1 {
		prueba.Fatalf("productos = %d, esperado 1", len(productos))
	}
	producto := productos[0]
	if valor, valido := valorFlotante(producto.Precio); !valido || valor != 2590 {
		prueba.Errorf("precio = %v", producto.Precio)
	}
	if valor, valido := valorFlotante(producto.PrecioNormal); !valido || valor != 3490 {
		prueba.Errorf("precio_normal = %v", producto.PrecioNormal)
	}
	if valor, valido := valorFlotante(producto.PrecioOferta); !valido || valor != 2590 {
		prueba.Errorf("precio_oferta = %v", producto.PrecioOferta)
	}
}

func TestAcuentaNoCruzaLosLimitesDeLosScriptsRSC(prueba *testing.T) {
	cuerpoHTML := `<script>self.__next_f.push([1,` +
		`\"name\":\"Producto limpio\",\"sku\":\"123\",\"ean\":[\"7800000000001\"],\"maxQty\":1,` +
		`\"slug\":\"producto-limpio-123\",\"brand\":\"Marca\",\"stock\":1,` +
		`\"priceBeforeTaxes\":990,\"promotion\":null` +
		`])</script><script>self.__next_f.push([1,` +
		`\"name\":\"otro bloque del runtime\"` +
		`])</script>`
	productos := extraccion.AcuentaProducts(sitioDelCatalogo(prueba, "acuenta_rsc"), []byte(cuerpoHTML), urlDePrueba(prueba, "https://www.acuenta.cl/ca/congelados/04"))
	if len(productos) != 1 {
		prueba.Fatalf("productos = %d, esperado 1", len(productos))
	}
	verificarCamposProducto(prueba, productos[0], map[string]string{
		"producto": "Producto Limpio",
		"ean":      "7800000000001",
	})
}

func TestAcuentaUsaLosCamposQueSiguenAlSKU(prueba *testing.T) {
	cuerpoHTML := `<script>self.__next_f.push([1,` +
		`\"name\":\"Congelados\",\"slug\":\"congelados/04\",` +
		`\"brand\":\"Categoria\",\"stock\":1,\"priceBeforeTaxes\":5000,` +
		`\"sku\":\"434645\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"salteado-chapsui-wok-congelado-500-g-frutos-del-maipo-434645\",` +
		`\"brand\":\"Frutos del Maipo\",\"stock\":25,\"priceBeforeTaxes\":2000,` +
		`\"promotion\":null` +
		`])</script>`
	productos := extraccion.AcuentaProducts(sitioDelCatalogo(prueba, "acuenta_rsc"), []byte(cuerpoHTML), urlDePrueba(prueba, "https://www.acuenta.cl/ca/congelados/04"))
	if len(productos) != 1 {
		prueba.Fatalf("productos = %d, esperado 1", len(productos))
	}
	producto := productos[0]
	verificarCamposProducto(prueba, producto, map[string]string{
		"producto": "Salteado Chapsui Wok Congelado 500 G Frutos Del Maipo",
		"url":      "https://www.acuenta.cl/p/salteado-chapsui-wok-congelado-500-g-frutos-del-maipo-434645",
	})
	if valor, valido := valorFlotante(producto.Precio); !valido || valor != 2000 {
		prueba.Errorf("precio = %v", producto.Precio)
	}
}

func TestAcuentaResuelveReferenciasDelPayloadRSC(prueba *testing.T) {
	casos := []struct {
		nombre         string
		cuerpoHTML     string
		eanEsperado    string
		precioEsperado float64
		ofertaEsperada bool
		imagenEsperada string
	}{
		{
			nombre: "ean",
			cuerpoHTML: `<script>self.__next_f.push([1,` +
				`\"name\":\"Producto referencia\",\"sku\":\"456\",\"ean\":\"$ec\",\"maxQty\":1,` +
				`\"slug\":\"producto-referencia-456\",\"brand\":\"Marca\",\"stock\":1,` +
				`\"priceBeforeTaxes\":990,\"promotion\":null` +
				`])</script><script>self.__next_f.push([1,` +
				`\nec:[\"7800000000002\"]` +
				`])</script>`,
			eanEsperado:    "7800000000002",
			precioEsperado: 990,
		},
		{
			nombre: "precio especial",
			cuerpoHTML: `<script>self.__next_f.push([1,` +
				`\"name\":\"Producto oferta\",\"sku\":\"123\",\"ean\":null,\"maxQty\":1,` +
				`\"slug\":\"producto-oferta-123\",\"brand\":\"Marca\",\"stock\":1,` +
				`\"priceBeforeTaxes\":3490,\"promotion\":\"$d7\"` +
				`])</script><script>self.__next_f.push([1,` +
				`\nd9:{\"quantity\":0,\"price\":2590,\"priceBeforeTaxes\":2590}` +
				`\nd8:[\"$d9\"]` +
				`\nd7:{\"type\":\"specialPrice\",\"conditions\":\"$d8\"}` +
				`])</script>`,
			precioEsperado: 2590,
			ofertaEsperada: true,
		},
		{
			nombre: "imagen",
			cuerpoHTML: `<script>self.__next_f.push([1,` +
				`\"name\":\"Producto imagen\",\"sku\":\"789\",\"ean\":null,\"maxQty\":1,` +
				`\"slug\":\"producto-imagen-789\",\"brand\":\"Marca\",\"stock\":1,` +
				`\"photosUrl\":\"$d5\",\"priceBeforeTaxes\":990,\"promotion\":null` +
				`])</script><script>self.__next_f.push([1,` +
				`\nd5:[\"https://images.test/producto.jpg\"]` +
				`])</script>`,
			precioEsperado: 990,
			imagenEsperada: "https://images.test/producto.jpg",
		},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			productos := extraccion.AcuentaProducts(sitioDelCatalogo(prueba, "acuenta_rsc"), []byte(caso.cuerpoHTML), urlDePrueba(prueba, "https://www.acuenta.cl/ca/congelados/04"))
			if len(productos) != 1 {
				prueba.Fatalf("productos = %d, esperado 1", len(productos))
			}
			producto := productos[0]
			if obtenido := valorCadena(producto.EanGtin); obtenido != caso.eanEsperado {
				prueba.Errorf("ean = %q, esperado %q", obtenido, caso.eanEsperado)
			}
			if valor, valido := valorFlotante(producto.Precio); !valido || valor != caso.precioEsperado {
				prueba.Errorf("precio = %v, esperado %v", producto.Precio, caso.precioEsperado)
			}
			if obtenido := producto.PrecioOferta != nil; obtenido != caso.ofertaEsperada {
				prueba.Errorf("precio_oferta = %v, esperado oferta=%t", producto.PrecioOferta, caso.ofertaEsperada)
			}
			if caso.imagenEsperada != "" {
				if obtenido := valorCadena(producto.Imagen); obtenido != caso.imagenEsperada {
					prueba.Errorf("imagen = %q, esperado %q", obtenido, caso.imagenEsperada)
				}
			}
		})
	}
}

func TestAcuentaNoUsaElPrecioMultiunidadComoOferta(prueba *testing.T) {
	cuerpoHTML := `<script>self.__next_f.push([1,` +
		`\"name\":\"Producto pack\",\"sku\":\"456\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"producto-pack-456\",\"brand\":\"Marca\",\"stock\":1,` +
		`\"priceBeforeTaxes\":3490,\"promotion\":\"$p1\"` +
		`])</script><script>self.__next_f.push([1,` +
		`\nc1:{\"quantity\":5,\"price\":2000,\"priceBeforeTaxes\":2000}` +
		`\nc0:[\"$c1\"]` +
		`\np1:{\"type\":\"nx$\",\"conditions\":\"$c0\"}` +
		`])</script>`
	productos := extraccion.AcuentaProducts(sitioDelCatalogo(prueba, "acuenta_rsc"), []byte(cuerpoHTML), urlDePrueba(prueba, "https://www.acuenta.cl/ca/congelados/04"))
	if len(productos) != 1 {
		prueba.Fatalf("productos = %d, esperado 1", len(productos))
	}
	if productos[0].PrecioOferta != nil {
		prueba.Errorf("precio_oferta = %v, esperado nil", productos[0].PrecioOferta)
	}
}

func TestAcuentaCodificaLosCorchetesDeLaImagen(prueba *testing.T) {
	cuerpoHTML := `<script>self.__next_f.push([1,` +
		`\"name\":\"Producto imagen\",\"sku\":\"4756977\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"producto-imagen-4756977\",\"brand\":\"Marca\",\"stock\":1,` +
		`\"priceBeforeTaxes\":990,\"promotion\":null` +
		`])</script>` +
		`d5:["https://images.lider.cl/wmtcl?source=url[file:/productos/4756977aa.jpg]\u0026sink"]`
	productos := extraccion.AcuentaProducts(sitioDelCatalogo(prueba, "acuenta_rsc"), []byte(cuerpoHTML), urlDePrueba(prueba, "https://www.acuenta.cl/ca/congelados/04"))
	if len(productos) != 1 {
		prueba.Fatalf("productos = %d, esperado 1", len(productos))
	}
	esperada := "https://images.lider.cl/wmtcl?source=url%5Bfile:/productos/4756977aa.jpg%5D&sink"
	verificarCamposProducto(prueba, productos[0], map[string]string{"imagen": esperada})
}

func TestAcuentaProductSeleccionaLaFichaSolicitada(prueba *testing.T) {
	cuerpoHTML := `<script>self.__next_f.push([1,` +
		`\"sku\":\"1075736\",\"name\":\"Helado Chocolate 1 L Savory\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"helado-chocolate-1-l-savory-1075736\",\"brand\":\"Savory\",\"stock\":1,` +
		`\"priceBeforeTaxes\":3490,\"promotion\":null` +
		`])</script>`
	solicitada := "https://www.acuenta.cl/p/helado-chocolate-1-l-savory-1075736"
	producto := extraccion.AcuentaProduct(sitioDelCatalogo(prueba, "acuenta_rsc"), []byte(cuerpoHTML), solicitada, urlDePrueba(prueba, solicitada))
	if producto == nil {
		prueba.Fatal("AcuentaProduct() = nil")
	}
	verificarCamposProducto(prueba, *producto, map[string]string{"producto": "Helado Chocolate 1 L Savory"})
}

// ===== Recorrido del Spider de A Cuenta =====

func TestAcuentaPaginacionDeCategoria(prueba *testing.T) {
	urlCategoria := "https://www.acuenta.cl/ca/congelados/04"
	pagina1 := urlCategoria + "?currentPage=1"
	pagina2 := urlCategoria + "?currentPage=2"
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		pagina1: respuestaExitosa(pagina1, `<div class="product-card"><a href="/p/uno-1">Uno 1 L</a><div>$1.000</div></div>`),
		pagina2: respuestaExitosa(pagina2, `<div class="product-card"><a href="/p/dos-2">Dos 1 L</a><div>$2.000</div></div>`),
	}}
	spider := construirSpiderDePrueba(prueba, "acuenta_rsc", obtenedor, prueba.TempDir(), 100, urlCategoria)
	elementos := recolectarItems(prueba, spider, dominio.Job{})
	if len(elementos) != 2 {
		prueba.Fatalf("items = %d, esperado 2", len(elementos))
	}
	for _, elemento := range elementos {
		if elemento.Categoria != "congelados" || elemento.Supermercado != "A Cuenta" {
			prueba.Errorf("item = %+v", elemento)
		}
	}
}

func TestAcuentaProductMode(prueba *testing.T) {
	urlProducto := "https://www.acuenta.cl/p/helado-chocolate-1-l-savory-1075736"
	cuerpoHTML := `<script>self.__next_f.push([1,` +
		`\"sku\":\"1075736\",\"name\":\"Helado Chocolate Suizo 1 L Savory\",\"ean\":null,\"maxQty\":1,` +
		`\"slug\":\"helado-chocolate-1-l-savory-1075736\",\"brand\":\"Savory\",\"stock\":1,` +
		`\"priceBeforeTaxes\":3490,\"promotion\":null` +
		`])</script>`
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		urlProducto: respuestaExitosa(urlProducto, cuerpoHTML),
	}}
	spider := construirSpiderDePrueba(prueba, "acuenta_rsc", obtenedor, prueba.TempDir(), 100)
	elementos := recolectarItems(prueba, spider, dominio.Job{ProductURL: urlProducto})
	if len(elementos) != 1 {
		prueba.Fatalf("items = %d, esperado 1", len(elementos))
	}
	elemento := elementos[0]
	if elemento.Producto != "Helado Chocolate 1 L Savory" || elemento.Categoria != "producto" || elemento.Supermercado != "A Cuenta" {
		prueba.Errorf("item = %+v", elemento)
	}
	if elemento.URLProducto == nil || *elemento.URLProducto != urlProducto {
		prueba.Errorf("url = %v", elemento.URLProducto)
	}
	if len(obtenedor.llamadas) != 1 || obtenedor.llamadas[0] != urlProducto {
		prueba.Errorf("calls = %v, esperado sólo la ficha", obtenedor.llamadas)
	}
}

func TestAcuentaRechazaProductURLQueNoEsFicha(prueba *testing.T) {
	obtenedor := &obtenedorFalso{}
	spider := construirSpiderDePrueba(prueba, "acuenta_rsc", obtenedor, prueba.TempDir(), 100)
	errorObtenido := spider.Run(context.Background(), dominio.Job{ProductURL: "https://www.acuenta.cl/ca/congelados/04"}, func(dominio.Item) error {
		return nil
	})
	if errorObtenido == nil {
		prueba.Fatal("Run() debió rechazar una categoría como product_url")
	}
	if len(obtenedor.llamadas) != 0 {
		prueba.Errorf("requests = %v", obtenedor.llamadas)
	}
}
