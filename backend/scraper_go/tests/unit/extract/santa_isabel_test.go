package extract_test

import (
	"encoding/json"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

func santaIsabelBody(t *testing.T, payload any) []byte {
	t.Helper()
	inner, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	outer, err := json.Marshal(string(inner))
	if err != nil {
		t.Fatal(err)
	}
	return []byte("<script>window.__renderData = " + string(outer) + ";</script>")
}

func santaIsabelPayload() map[string]any {
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

func TestSantaIsabelProductsDesdeRenderData(t *testing.T) {
	body := santaIsabelBody(t, santaIsabelPayload())
	products := extract.SantaIsabelProducts(site(t, "santa_isabel_rsc"), body, mustURL(t, "https://www.santaisabel.cl/despensa"))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	assertProductFields(t, product, map[string]string{
		"producto":  "Cafe molido 250 g",
		"ean":       "7800000000001",
		"sku":       "sku-1",
		"marca":     "Marca Test",
		"formato":   "1 un",
		"promocion": "2x1",
		"url":       "https://www.santaisabel.cl/cafe-molido-250-g-product-1/p",
		"imagen":    "https://img.test/cafe.jpg",
	})
	if value, ok := floatValue(product.Precio); !ok || value != 990 {
		t.Errorf("precio = %v", product.Precio)
	}
	if value, ok := floatValue(product.PrecioNormal); !ok || value != 1290 {
		t.Errorf("precio_normal = %v", product.PrecioNormal)
	}
	if value, ok := floatValue(product.PrecioOferta); !ok || value != 990 {
		t.Errorf("precio_oferta = %v", product.PrecioOferta)
	}
	if value, ok := boolValue(product.EnStock); !ok || !value {
		t.Errorf("en_stock = %v", product.EnStock)
	}
}

func TestSantaIsabelProductsDesdeJSONLD(t *testing.T) {
	body := []byte(`<script type="application/ld+json">
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
	products := extract.SantaIsabelProducts(site(t, "santa_isabel_rsc"), body, mustURL(t, "https://www.santaisabel.cl/frutas-y-verduras"))
	if len(products) != 2 {
		t.Fatalf("productos = %d, want 2", len(products))
	}
	first := products[0]
	assertProductFields(t, first, map[string]string{
		"producto": "Plátano Extra Granel (1 a 2 un. Aprox)",
		"marca":    "Frutas y Verduras Propias",
		"sku":      "platano-granel",
		"url":      "https://www.santaisabel.cl/platano-granel/p",
	})
	if value, ok := floatValue(first.Precio); !ok || value != 745 {
		t.Errorf("precio = %v", first.Precio)
	}
	if value, ok := floatValue(first.PrecioNormal); !ok || value != 745 {
		t.Errorf("precio_normal = %v", first.PrecioNormal)
	}
	if first.PrecioOferta != nil {
		t.Errorf("precio_oferta = %v, want nil", first.PrecioOferta)
	}
	if value, ok := boolValue(first.EnStock); !ok || !value {
		t.Errorf("en_stock = %v", first.EnStock)
	}
	if value, ok := boolValue(products[1].EnStock); !ok || value {
		t.Errorf("en_stock segundo = %v", products[1].EnStock)
	}
}

func TestSantaIsabelSoloUsaAlVendedorPropio(t *testing.T) {
	payload := map[string]any{
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
	body := santaIsabelBody(t, payload)
	if products := extract.SantaIsabelProducts(site(t, "santa_isabel_rsc"), body, mustURL(t, "https://www.santaisabel.cl/despensa")); len(products) != 0 {
		t.Fatalf("productos = %+v, want 0", products)
	}
}

func TestSantaIsabelSinDatosDevuelveVacio(t *testing.T) {
	if products := extract.SantaIsabelProducts(site(t, "santa_isabel_rsc"), []byte("<html></html>"), mustURL(t, "https://www.santaisabel.cl/despensa")); len(products) != 0 {
		t.Fatalf("productos = %+v, want 0", products)
	}
}
