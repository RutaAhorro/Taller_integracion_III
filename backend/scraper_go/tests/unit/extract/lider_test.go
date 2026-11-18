package extract_test

import (
	"encoding/json"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

func TestLiderProductsDesdeTarjetas(prueba *testing.T) {
	body := `
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
	products := extract.LiderProducts(site(prueba, "lider_rsc"), []byte(body), mustURL(prueba, "https://super.lider.cl/browse/frutas/22884697_93034836?page=2"))
	if len(products) != 2 {
		prueba.Fatalf("productos = %d, want 2", len(products))
	}
	product := products[0]
	assertProductFields(prueba, product, map[string]string{
		"ean":     "00079919219638",
		"marca":   "Marca Lider",
		"formato": "100g",
		"imagen":  "https://super.lider.cl/images/cafe.webp",
	})
	if value, ok := floatValue(product.Precio); !ok || value != 26990 {
		prueba.Errorf("precio = %v", product.Precio)
	}
	if value, ok := floatValue(product.PrecioNormal); !ok || value != 30970 {
		prueba.Errorf("precio_normal = %v", product.PrecioNormal)
	}
	if value, ok := floatValue(product.PrecioOferta); !ok || value != 26990 {
		prueba.Errorf("precio_oferta = %v", product.PrecioOferta)
	}
	if value, ok := boolValue(product.EnStock); !ok || !value {
		prueba.Errorf("en_stock = %v", product.EnStock)
	}

	second := products[1]
	if value, ok := floatValue(second.Precio); !ok || value != 13350 {
		prueba.Errorf("segundo precio = %v", second.Precio)
	}
	if value, ok := floatValue(second.PrecioNormal); !ok || value != 13350 {
		prueba.Errorf("segundo precio_normal = %v", second.PrecioNormal)
	}
	if second.PrecioOferta != nil {
		prueba.Errorf("segundo precio_oferta = %v", second.PrecioOferta)
	}
}

func TestLiderProductsPrefiereItemsHidratados(prueba *testing.T) {
	payload := map[string]any{
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
	encoded, err := json.Marshal(payload)
	if err != nil {
		prueba.Fatal(err)
	}
	body := `<div data-testid="item-stack"><div role="group" data-item-id="tile-only" data-dca-id="00000000000001">` +
		`<a href="/ip/cafe/tile-only/00000000000001"><span data-automation-id="product-title">Only card</span></a></div></div>` +
		`<script id="__NEXT_DATA__" type="application/json">` + string(encoded) + `</script>`

	products := extract.LiderProducts(site(prueba, "lider_rsc"), []byte(body), mustURL(prueba, "https://super.lider.cl/browse/frutas/22884697_93034836"))
	if len(products) != 2 {
		prueba.Fatalf("productos = %d, want 2", len(products))
	}
	product := products[0]
	if value, ok := floatValue(product.Precio); !ok || value != 26990 {
		prueba.Errorf("precio = %v", product.Precio)
	}
	if value, ok := floatValue(product.PrecioNormal); !ok || value != 30970 {
		prueba.Errorf("precio_normal = %v", product.PrecioNormal)
	}
	if value, ok := floatValue(product.PrecioOferta); !ok || value != 26990 {
		prueba.Errorf("precio_oferta = %v", product.PrecioOferta)
	}
	assertProductFields(prueba, product, map[string]string{
		"marca":  "Marca Test",
		"imagen": "https://images.test/cafe.webp",
	})
	if value, ok := boolValue(products[1].EnStock); !ok || value {
		prueba.Errorf("en_stock segundo = %v", products[1].EnStock)
	}
}

func TestLiderFormatoDelTitulo(prueba *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{name: "unidad no es formato", title: "Pan 1 unidad", want: ""},
		{name: "unidade", title: "Pan 1 unidade", want: "1 unidade"},
		{name: "litros", title: "Bebida 1.5 L", want: "1.5 L"},
	}
	for _, tc := range tests {
		prueba.Run(tc.name, func(prueba *testing.T) {
			body := `<div data-testid="item-stack"><div role="group" data-item-id="1" data-dca-id="00000000000001">` +
				`<a href="/ip/frutas/uno/00000000000001"><span data-automation-id="product-title">` + tc.title + `</span></a>` +
				`<div data-automation-id="product-price"><div aria-hidden="true">$1.000</div></div>` +
				`</div></div>`
			products := extract.LiderProducts(site(prueba, "lider_rsc"), []byte(body), mustURL(prueba, "https://super.lider.cl/browse/frutas/22884697"))
			if len(products) != 1 {
				prueba.Fatalf("productos = %d, want 1", len(products))
			}
			if got := stringValue(products[0].FormatoCrudo); got != tc.want {
				prueba.Errorf("formato = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLiderProductDesdeNextData(prueba *testing.T) {
	productURL := "https://super.lider.cl/ip/leche-entera/12345678"
	nextData := `{"props":{"pageProps":{"product":{` +
		`"name":"Leche Entera 1 L","usItemId":"12345678","price":1290,` +
		`"canonicalUrl":"/ip/leche-entera/12345678","sku":"L-1","brand":"Lider",` +
		`"isOutOfStock":false,"imageInfo":{"thumbnailUrl":"/img/leche.webp"}}}}}`
	body := `<html><body><script id="__NEXT_DATA__" type="application/json">` + nextData + `</script></body></html>`

	product := extract.LiderProduct(site(prueba, "lider_rsc"), []byte(body), productURL, mustURL(prueba, productURL))
	if product == nil || product.Producto != "Leche Entera 1 L" {
		prueba.Fatalf("LiderProduct() = %+v", product)
	}
	if value, ok := floatValue(product.Precio); !ok || value != 1290 {
		prueba.Errorf("precio = %v", product.Precio)
	}
	assertProductFields(prueba, *product, map[string]string{
		"url":    productURL,
		"imagen": "https://super.lider.cl/img/leche.webp",
	})
}
