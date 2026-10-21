package spiders

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

func TestExtractLiderProductsFromCards(t *testing.T) {
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
	products := extractLiderProducts([]byte(body), mustURL("https://super.lider.cl/browse/frutas/22884697_93034836?page=2"))
	if len(products) != 2 {
		t.Fatalf("productos = %d, want 2", len(products))
	}
	product := products[0]
	if product.Precio == nil || *product.Precio != 26990 {
		t.Errorf("precio = %v", product.Precio)
	}
	if product.PrecioNormal == nil || *product.PrecioNormal != 30970 {
		t.Errorf("precio_normal = %v", product.PrecioNormal)
	}
	if product.PrecioOferta == nil || *product.PrecioOferta != 26990 {
		t.Errorf("precio_oferta = %v", product.PrecioOferta)
	}
	if product.EanGtin == nil || *product.EanGtin != "00079919219638" {
		t.Errorf("ean = %v", product.EanGtin)
	}
	if product.Marca == nil || *product.Marca != "Marca Lider" {
		t.Errorf("marca = %v", product.Marca)
	}
	if product.FormatoCrudo == nil || *product.FormatoCrudo != "100g" {
		t.Errorf("formato = %v", product.FormatoCrudo)
	}
	if product.EnStock == nil || !*product.EnStock {
		t.Errorf("en_stock = %v", product.EnStock)
	}
	if product.Imagen == nil || *product.Imagen != "https://super.lider.cl/images/cafe.webp" {
		t.Errorf("imagen = %v", product.Imagen)
	}
	if products[1].Precio == nil || *products[1].Precio != 13350 {
		t.Errorf("segundo precio = %v", products[1].Precio)
	}
	if products[1].PrecioNormal == nil || *products[1].PrecioNormal != 13350 {
		t.Errorf("segundo precio_normal = %v", products[1].PrecioNormal)
	}
	if products[1].PrecioOferta != nil {
		t.Errorf("segundo precio_oferta = %v", products[1].PrecioOferta)
	}
}

func TestExtractLiderPrefersHydratedItems(t *testing.T) {
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
		t.Fatal(err)
	}
	body := `<div data-testid="item-stack"><div role="group" data-item-id="tile-only" data-dca-id="00000000000001">` +
		`<a href="/ip/cafe/tile-only/00000000000001"><span data-automation-id="product-title">Only card</span></a></div></div>` +
		`<script id="__NEXT_DATA__" type="application/json">` + string(encoded) + `</script>`

	products := extractLiderProducts([]byte(body), mustURL("https://super.lider.cl/browse/frutas/22884697_93034836"))
	if len(products) != 2 {
		t.Fatalf("productos = %d, want 2", len(products))
	}
	product := products[0]
	if product.Precio == nil || *product.Precio != 26990 {
		t.Errorf("precio = %v", product.Precio)
	}
	if product.PrecioNormal == nil || *product.PrecioNormal != 30970 {
		t.Errorf("precio_normal = %v", product.PrecioNormal)
	}
	if product.PrecioOferta == nil || *product.PrecioOferta != 26990 {
		t.Errorf("precio_oferta = %v", product.PrecioOferta)
	}
	if product.Marca == nil || *product.Marca != "Marca Test" {
		t.Errorf("marca = %v", product.Marca)
	}
	if product.Imagen == nil || *product.Imagen != "https://images.test/cafe.webp" {
		t.Errorf("imagen = %v", product.Imagen)
	}
	if products[1].EnStock == nil || *products[1].EnStock {
		t.Errorf("en_stock segundo = %v", products[1].EnStock)
	}
}

func TestValidateLiderURLs(t *testing.T) {
	categoryURL := "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836"
	productURL := "https://super.lider.cl/ip/frutas/00203017000000"
	if err := validateLiderCategoryURL(categoryURL); err != nil {
		t.Errorf("categoría válida: %v", err)
	}
	if err := validateLiderProductURL(productURL); err != nil {
		t.Errorf("producto válido: %v", err)
	}
	if err := validateLiderCategoryURL("https://www.lider.cl/browse/alimentacion/94975735"); err == nil {
		t.Fatal("se esperaba error para host no permitido")
	}
	if err := validateLiderCategoryURL("https://www.lider.cl/supermercado/category/frutas-y-verduras"); err == nil {
		t.Fatal("se esperaba error para ruta no permitida")
	}
	if err := validateLiderProductURL("https://example.com/ip/item/1"); err == nil {
		t.Fatal("se esperaba error para producto no confiable")
	}
}

func TestLiderPageURLAndCategory(t *testing.T) {
	categoryURL := "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836?sort=price"
	if got := liderPageURL(categoryURL, 2); got != "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836?sort=price&page=2" {
		t.Errorf("page URL = %q", got)
	}
	if got := liderPageURL("https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836?page=2", 3); got != "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836?page=3" {
		t.Errorf("page URL con página existente = %q", got)
	}
	if got := liderCategoryFromURL(categoryURL); got != "frutas" {
		t.Errorf("categoría = %q", got)
	}
	if got := liderPageNumber(categoryURL); got != 1 {
		t.Errorf("página = %d", got)
	}
}

func TestLiderRunPagination(t *testing.T) {
	categoryURL := "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836"
	t.Setenv("LIDER_CATEGORY_URLS", categoryURL)
	page2 := liderPageURL(categoryURL, 2)
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		categoryURL: response(categoryURL, `<div data-testid="item-stack"><div role="group" data-item-id="1" data-dca-id="00000000000001">
<a href="/ip/frutas/uno/00000000000001"><img data-testid="productTileImage" src="/uno.webp"></a>
<div data-automation-id="product-price"><div aria-hidden="true">$1.000</div></div>
<span data-automation-id="product-title">Uno 1 kg</span></div></div>`),
		page2: response(page2, `<div data-testid="item-stack"><div role="group" data-item-id="2" data-dca-id="00000000000002">
<a href="/ip/frutas/dos/00000000000002"><img data-testid="productTileImage" src="/dos.webp"></a>
<div data-automation-id="product-price"><div aria-hidden="true">$2.000</div></div>
<span data-automation-id="product-title">Dos 1 kg</span></div></div>`),
	}}
	spider := NewLider(fetcher, t.TempDir(), 100, quietLogger())
	var items []domain.Item
	err := spider.Run(context.Background(), domain.Job{}, func(item domain.Item) error {
		items = append(items, item)
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	for _, item := range items {
		if item.Categoria != "frutas" || item.Supermercado != "Lider" {
			t.Errorf("item = %+v", item)
		}
	}
}
