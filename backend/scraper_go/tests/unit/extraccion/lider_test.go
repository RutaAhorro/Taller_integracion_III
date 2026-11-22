// lider_test.go verifica el parseo de tarjetas/NextData y el recorrido de páginas de Lider.
package extraccion_test

import (
	"context"
	"encoding/json"
	"testing"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
)

// ===== Parseo de Productos de Lider =====

func TestLiderProductsDesdeTarjetas(prueba *testing.T) {
	cuerpoHTML := `
	<div data-testid="item-stack">
	  <div role="group" data-item-id="item-1" data-dca-id="00079919219638">
		<a href="/ip/cafe-te-y-hierbas/pack-cafe/00079919219638">
		  <img data-testid="productTileImage" src="/images/cafe.webp">
		</a>
		<div data-automation-id="product-price">
		  <div aria-hidden="true">$26.990</div>
		  <span>precio actual $26.990, Costaba $30.970</span>
		  <div><span class="strike">$30.970</span></div>
		</div>
		<div class="mb1 mt2 b f6 black mr1 lh-copy">Marca Lider</div>
		<span data-automation-id="product-title">Pack Café Molido 100g</span>
		<div>Available for Despacho</div>
	  </div>
	  <div role="group" data-item-id="item-2" data-dca-id="00040005049502">
		<a href="/ip/cafe-te-y-hierbas/cafe-veracruz/00040005049502">
		  <img data-testid="productTileImage" src="/images/veracruz.webp">
		</a>
		<div data-automation-id="product-price">
		  <div aria-hidden="true">$13.350</div>
		  <span>precio actual $13.350</span>
		</div>
		<span data-automation-id="product-title">Café Veracruz</span>
	  </div>
	</div>
	`
	productos := extraccion.LiderProducts(sitioDelCatalogo(prueba, "lider_rsc"), []byte(cuerpoHTML), urlDePrueba(prueba, "https://super.lider.cl/browse/frutas/22884697_93034836?page=2"))
	if len(productos) != 2 {
		prueba.Fatalf("productos = %d, esperado 2", len(productos))
	}
	producto := productos[0]
	verificarCamposProducto(prueba, producto, map[string]string{
		"ean":     "00079919219638",
		"marca":   "Marca Lider",
		"formato": "100g",
		"imagen":  "https://super.lider.cl/images/cafe.webp",
	})
	if valor, valido := valorFlotante(producto.Precio); !valido || valor != 26990 {
		prueba.Errorf("precio = %v", producto.Precio)
	}
	if valor, valido := valorFlotante(producto.PrecioNormal); !valido || valor != 30970 {
		prueba.Errorf("precio_normal = %v", producto.PrecioNormal)
	}
	if valor, valido := valorFlotante(producto.PrecioOferta); !valido || valor != 26990 {
		prueba.Errorf("precio_oferta = %v", producto.PrecioOferta)
	}
	if valor, valido := valorBooleano(producto.EnStock); !valido || !valor {
		prueba.Errorf("en_stock = %v", producto.EnStock)
	}

	segundo := productos[1]
	if valor, valido := valorFlotante(segundo.Precio); !valido || valor != 13350 {
		prueba.Errorf("segundo precio = %v", segundo.Precio)
	}
	if valor, valido := valorFlotante(segundo.PrecioNormal); !valido || valor != 13350 {
		prueba.Errorf("segundo precio_normal = %v", segundo.PrecioNormal)
	}
	if segundo.PrecioOferta != nil {
		prueba.Errorf("segundo precio_oferta = %v", segundo.PrecioOferta)
	}
}

func TestLiderProductsPrefiereItemsHidratados(prueba *testing.T) {
	carga := map[string]any{
		"props": map[string]any{
			"pageProps": map[string]any{
				"initialData": map[string]any{
					"searchResult": map[string]any{
						"itemStacks": []any{
							map[string]any{
								"items": []any{
									map[string]any{
										"usItemId":     "00079919219638",
										"name":         "Pack Café Descafeinado 100g",
										"brand":        "Marca Test",
										"price":        26990,
										"priceInfo":    map[string]any{"wasPrice": "$30.970"},
										"canonicalUrl": "/ip/cafe/pack-cafe/00079919219638",
										"imageInfo":    map[string]any{"thumbnailUrl": "https://images.test/cafe.webp"},
										"isOutOfStock": false,
									},
									map[string]any{
										"usItemId":     "00040005049502",
										"name":         "Café Veracruz",
										"price":        13350,
										"priceInfo":    map[string]any{"wasPrice": ""},
										"canonicalUrl": "/ip/cafe/veracruz/00040005049502",
										"imageInfo":    map[string]any{},
										"isOutOfStock": true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	codificado, errorCodificacion := json.Marshal(carga)
	if errorCodificacion != nil {
		prueba.Fatal(errorCodificacion)
	}
	cuerpoHTML := `<div data-testid="item-stack"><div role="group" data-item-id="tile-only" data-dca-id="00000000000001">` +
		`<a href="/ip/cafe/tile-only/00000000000001"><span data-automation-id="product-title">Only card</span></a></div></div>` +
		`<script id="__NEXT_DATA__" type="application/json">` + string(codificado) + `</script>`

	productos := extraccion.LiderProducts(sitioDelCatalogo(prueba, "lider_rsc"), []byte(cuerpoHTML), urlDePrueba(prueba, "https://super.lider.cl/browse/frutas/22884697_93034836"))
	if len(productos) != 2 {
		prueba.Fatalf("productos = %d, esperado 2", len(productos))
	}
	producto := productos[0]
	if valor, valido := valorFlotante(producto.Precio); !valido || valor != 26990 {
		prueba.Errorf("precio = %v", producto.Precio)
	}
	if valor, valido := valorFlotante(producto.PrecioNormal); !valido || valor != 30970 {
		prueba.Errorf("precio_normal = %v", producto.PrecioNormal)
	}
	if valor, valido := valorFlotante(producto.PrecioOferta); !valido || valor != 26990 {
		prueba.Errorf("precio_oferta = %v", producto.PrecioOferta)
	}
	verificarCamposProducto(prueba, producto, map[string]string{
		"marca":  "Marca Test",
		"imagen": "https://images.test/cafe.webp",
	})
	if valor, valido := valorBooleano(productos[1].EnStock); !valido || valor {
		prueba.Errorf("en_stock segundo = %v", productos[1].EnStock)
	}
}

func TestLiderFormatoDelTitulo(prueba *testing.T) {
	casos := []struct {
		nombre   string
		titulo   string
		esperado string
	}{
		{nombre: "unidad no es formato", titulo: "Pan 1 unidad", esperado: ""},
		{nombre: "unidade", titulo: "Pan 1 unidade", esperado: "1 unidade"},
		{nombre: "litros", titulo: "Bebida 1.5 L", esperado: "1.5 L"},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			cuerpoHTML := `<div data-testid="item-stack"><div role="group" data-item-id="1" data-dca-id="00000000000001">` +
				`<a href="/ip/frutas/uno/00000000000001"><span data-automation-id="product-title">` + caso.titulo + `</span></a>` +
				`<div data-automation-id="product-price"><div aria-hidden="true">$1.000</div></div>` +
				`</div></div>`
			productos := extraccion.LiderProducts(sitioDelCatalogo(prueba, "lider_rsc"), []byte(cuerpoHTML), urlDePrueba(prueba, "https://super.lider.cl/browse/frutas/22884697"))
			if len(productos) != 1 {
				prueba.Fatalf("productos = %d, esperado 1", len(productos))
			}
			if obtenido := valorCadena(productos[0].FormatoCrudo); obtenido != caso.esperado {
				prueba.Errorf("formato = %q, esperado %q", obtenido, caso.esperado)
			}
		})
	}
}

func TestLiderProductDesdeNextData(prueba *testing.T) {
	urlProducto := "https://super.lider.cl/ip/leche-entera/12345678"
	datosNext := `{"props":{"pageProps":{"product":{` +
		`"name":"Leche Entera 1 L","usItemId":"12345678","price":1290,` +
		`"canonicalUrl":"/ip/leche-entera/12345678","sku":"L-1","brand":"Lider",` +
		`"isOutOfStock":false,"imageInfo":{"thumbnailUrl":"/img/leche.webp"}}}}}`
	cuerpoHTML := `<html><body><script id="__NEXT_DATA__" type="application/json">` + datosNext + `</script></body></html>`

	producto := extraccion.LiderProduct(sitioDelCatalogo(prueba, "lider_rsc"), []byte(cuerpoHTML), urlProducto, urlDePrueba(prueba, urlProducto))
	if producto == nil || producto.Producto != "Leche Entera 1 L" {
		prueba.Fatalf("LiderProduct() = %+v", producto)
	}
	if valor, valido := valorFlotante(producto.Precio); !valido || valor != 1290 {
		prueba.Errorf("precio = %v", producto.Precio)
	}
	verificarCamposProducto(prueba, *producto, map[string]string{
		"url":    urlProducto,
		"imagen": "https://super.lider.cl/img/leche.webp",
	})
}

// ===== Recorrido del Spider de Lider =====

func cuerpoCategoriaLider(identificador, nombre, precio string) string {
	return `<div data-testid="item-stack"><div role="group" data-item-id="` + identificador + `" data-dca-id="` + identificador + `">
<a href="/ip/frutas/` + identificador + `"><img data-testid="productTileImage" src="/` + identificador + `.webp"></a>
<div data-automation-id="product-price"><div aria-hidden="true">` + precio + `</div></div>
<span data-automation-id="product-title">` + nombre + `</span></div></div>`
}

func TestLiderPaginacionDeCategoria(prueba *testing.T) {
	urlCategoria := "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836"
	pagina2 := urlCategoria + "?page=2"
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		urlCategoria: respuestaExitosa(urlCategoria, cuerpoCategoriaLider("00000000000001", "Uno 1 kg", "$1.000")),
		pagina2:      respuestaExitosa(pagina2, cuerpoCategoriaLider("00000000000002", "Dos 1 kg", "$2.000")),
	}}
	spider := construirSpiderDePrueba(prueba, "lider_rsc", obtenedor, prueba.TempDir(), 100, urlCategoria)
	elementos := recolectarItems(prueba, spider, dominio.Job{})
	if len(elementos) != 2 {
		prueba.Fatalf("items = %d, esperado 2", len(elementos))
	}
	for _, elemento := range elementos {
		if elemento.Categoria != "frutas" || elemento.Supermercado != "Lider" {
			prueba.Errorf("item = %+v", elemento)
		}
	}
}

func TestLiderProductMode(prueba *testing.T) {
	urlProducto := "https://super.lider.cl/ip/leche-entera/12345678"
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		urlProducto: respuestaExitosa(urlProducto, `<script type="application/ld+json">
{"@type":"Product","name":"Leche Entera 1 L","url":"/ip/leche-entera/12345678",
 "gtin13":"7800000000001","sku":"L-1","brand":{"name":"Lider"},
 "offers":{"price":"1290","availability":"https://schema.org/InStock"}}
</script>`),
	}}
	spider := construirSpiderDePrueba(prueba, "lider_rsc", obtenedor, prueba.TempDir(), 100)
	elementos := recolectarItems(prueba, spider, dominio.Job{ProductURL: urlProducto})
	if len(elementos) != 1 {
		prueba.Fatalf("items = %d, esperado 1", len(elementos))
	}
	elemento := elementos[0]
	if elemento.Producto != "Leche Entera 1 L" || elemento.Categoria != "producto" || elemento.Supermercado != "Lider" {
		prueba.Errorf("item = %+v", elemento)
	}
	if elemento.URLProducto == nil || *elemento.URLProducto != urlProducto {
		prueba.Errorf("url = %v", elemento.URLProducto)
	}
	if len(obtenedor.llamadas) != 1 || obtenedor.llamadas[0] != urlProducto {
		prueba.Errorf("calls = %v, esperado sólo la ficha", obtenedor.llamadas)
	}
}

func TestLiderProductModeLeeNextData(prueba *testing.T) {
	urlProducto := "https://super.lider.cl/ip/leche-entera/12345678"
	datosNext := `{"props":{"pageProps":{"product":{` +
		`"name":"Leche Entera 1 L","usItemId":"12345678","price":1290,` +
		`"canonicalUrl":"/ip/leche-entera/12345678","sku":"L-1","brand":"Lider",` +
		`"isOutOfStock":false,"imageInfo":{"thumbnailUrl":"/img/leche.webp"}}}}}`
	cuerpoHTML := `<html><body><script id="__NEXT_DATA__" type="application/json">` + datosNext + `</script></body></html>`
	obtenedor := &obtenedorFalso{respuestas: map[string]*extraccion.Response{
		urlProducto: respuestaExitosa(urlProducto, cuerpoHTML),
	}}
	spider := construirSpiderDePrueba(prueba, "lider_rsc", obtenedor, prueba.TempDir(), 100)
	elementos := recolectarItems(prueba, spider, dominio.Job{ProductURL: urlProducto})
	if len(elementos) != 1 || elementos[0].Producto != "Leche Entera 1 L" {
		prueba.Fatalf("items = %+v", elementos)
	}
	if elementos[0].Precio == nil || *elementos[0].Precio != 1290 {
		prueba.Errorf("precio = %v", elementos[0].Precio)
	}
	if elementos[0].URLProducto == nil || *elementos[0].URLProducto != urlProducto {
		prueba.Errorf("url = %v", elementos[0].URLProducto)
	}
}

func TestLiderRechazaProductURLQueNoEsFicha(prueba *testing.T) {
	obtenedor := &obtenedorFalso{}
	spider := construirSpiderDePrueba(prueba, "lider_rsc", obtenedor, prueba.TempDir(), 100)
	errorObtenido := spider.Run(context.Background(), dominio.Job{ProductURL: "https://super.lider.cl/browse/frutas/22884697_93034836"}, func(dominio.Item) error {
		return nil
	})
	if errorObtenido == nil {
		prueba.Fatal("Run() debió rechazar una categoría como product_url")
	}
	if len(obtenedor.llamadas) != 0 {
		prueba.Errorf("requests = %v", obtenedor.llamadas)
	}
}
