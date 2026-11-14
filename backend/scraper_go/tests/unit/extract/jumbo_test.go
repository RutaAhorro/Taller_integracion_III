package extract_test

import (
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extract"
)

func TestJumboProducts(t *testing.T) {
	body := `<script type="application/ld+json">
{"@graph":[{"item":{"@type":"Product","name":"Tomate Larga Vida",
"image":["https://img.test/tomate.jpg"],"offers":{"price":"$1.290"}}}]}
</script>`
	products := extract.JumboProducts(site(t, "jumbo_rsc"), []byte(body))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	if product.Producto != "Tomate Larga Vida" {
		t.Errorf("producto = %q", product.Producto)
	}
	for name, want := range map[string]float64{
		"precio":        1290,
		"precio_normal": 1290,
		"precio_oferta": 1290,
	} {
		var got *float64
		switch name {
		case "precio":
			got = product.Precio
		case "precio_normal":
			got = product.PrecioNormal
		case "precio_oferta":
			got = product.PrecioOferta
		}
		if value, ok := floatValue(got); !ok || value != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	assertProductFields(t, product, map[string]string{
		"imagen": "https://img.test/tomate.jpg",
	})
}

func TestJumboProductsLeeCamposDeComparacion(t *testing.T) {
	body := `<script type="application/ld+json">
{"@type":"Product","name":"Lechuga","gtin13":"7800000000001","sku":"SKU-1",
"brand":{"name":"Marca Test"},"description":"1 unidad","url":"https://jumbo.cl/p/lechuga",
"offers":{"price":"990","availability":"https://schema.org/InStock","offerDescription":"2x1"}}
</script>`
	products := extract.JumboProducts(site(t, "jumbo_rsc"), []byte(body))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	assertProductFields(t, product, map[string]string{
		"ean":       "7800000000001",
		"sku":       "SKU-1",
		"marca":     "Marca Test",
		"formato":   "1 unidad",
		"promocion": "2x1",
		"url":       "https://jumbo.cl/p/lechuga",
	})
	if inStock, ok := boolValue(product.EnStock); !ok || !inStock {
		t.Errorf("en_stock = %v", product.EnStock)
	}
}

func TestJumboProductsIdentificadoresNumericos(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantEAN   string
		wantSKU   string
		wantPrice float64
	}{
		{
			name: "enteros",
			body: `<script type="application/ld+json">
{"@type":"Product","name":"Numérico","gtin13":7800000000001,"sku":12345,
"offers":{"price":1290}}
</script>`,
			wantEAN:   "7800000000001",
			wantSKU:   "12345",
			wantPrice: 1290,
		},
		{
			name: "decimal",
			body: `<script type="application/ld+json">
{"@type":"Product","name":"Float","gtin13":7800000000001.0,
"offers":{"price":1290.0}}
</script>`,
			wantPrice: 1290,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			products := extract.JumboProducts(site(t, "jumbo_rsc"), []byte(tc.body))
			if len(products) != 1 {
				t.Fatalf("productos = %d, want 1", len(products))
			}
			product := products[0]
			if got := stringValue(product.EanGtin); got != tc.wantEAN {
				t.Errorf("ean = %q, want %q", got, tc.wantEAN)
			}
			if got := stringValue(product.SKU); got != tc.wantSKU {
				t.Errorf("sku = %q, want %q", got, tc.wantSKU)
			}
			if value, ok := floatValue(product.Precio); !ok || value != tc.wantPrice {
				t.Errorf("precio = %v, want %v", product.Precio, tc.wantPrice)
			}
		})
	}
}

func TestJumboProductSeleccionaLaFichaSolicitada(t *testing.T) {
	body := `<script type="application/ld+json">
{"@graph":[
{"@type":"Product","name":"Tomate","url":"/p/tomate","offers":{"price":"1200"}},
{"@type":"Product","name":"Lechuga","url":"/p/lechuga","offers":{"price":"990"}}]}
</script>`
	product := extract.JumboProduct(
		site(t, "jumbo_rsc"),
		[]byte(body),
		"https://www.jumbo.cl/p/lechuga",
		mustURL(t, "https://www.jumbo.cl/p/lechuga"),
	)
	if product == nil || product.Producto != "Lechuga" {
		t.Fatalf("JumboProduct() = %+v", product)
	}
}

func TestJumboSKUFromURL(t *testing.T) {
	got := extract.JumboSKUFromURL(site(t, "jumbo_rsc"), "https://www.jumbo.cl/cafe/p")
	if got == nil || *got != "JUMBO-cafe" {
		t.Errorf("JumboSKUFromURL() = %v, want JUMBO-cafe", got)
	}
}
