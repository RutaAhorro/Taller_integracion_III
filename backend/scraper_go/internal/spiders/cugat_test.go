package spiders

import (
	"context"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

func TestRegistryContainsAllKnownSpiders(t *testing.T) {
	registry := NewRegistry(&fakeFetcher{}, t.TempDir(), 100, quietLogger())
	for _, name := range []string{JumboName, SantaIsabelName, CugatName, AcuentaName, LiderName} {
		if _, ok := registry.Get(name); !ok {
			t.Errorf("falta el spider %q en el registro", name)
		}
	}
}

func TestExtractCugatProducts(t *testing.T) {
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
	products := extractCugatProducts([]byte(body), mustURL("https://cugat.cl/categoria-producto/bebidas/page/2/"))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	product := products[0]
	if product.Producto != "Bebida Cugat 350cc" {
		t.Errorf("producto = %q", product.Producto)
	}
	if product.Precio == nil || *product.Precio != 6290 {
		t.Errorf("precio = %v", product.Precio)
	}
	if product.PrecioNormal == nil || *product.PrecioNormal != 7290 {
		t.Errorf("precio_normal = %v", product.PrecioNormal)
	}
	if product.PrecioOferta == nil || *product.PrecioOferta != 6290 {
		t.Errorf("precio_oferta = %v", product.PrecioOferta)
	}
	if product.EanGtin == nil || *product.EanGtin != "7801620000196" {
		t.Errorf("ean = %v", product.EanGtin)
	}
	if product.SKU == nil || *product.SKU != "75118" {
		t.Errorf("sku = %v", product.SKU)
	}
	if product.EnStock == nil || !*product.EnStock {
		t.Errorf("en_stock = %v", product.EnStock)
	}
	if product.Imagen == nil || *product.Imagen != "https://cugat.cl/uploads/bebida.webp" {
		t.Errorf("imagen = %v", product.Imagen)
	}
}

func TestCugatImageFallsBackToNoscript(t *testing.T) {
	body := `<div class="products"><div class="product-small product">
<p class="product-title"><a href="/producto/uno/">Uno</a></p>
<noscript><img src="/uploads/uno.webp"></noscript>
<ins><span class="woocommerce-Price-amount">$1.000</span></ins>
</div></div>`
	products := extractCugatProducts([]byte(body), mustURL("https://cugat.cl/categoria/"))
	if len(products) != 1 {
		t.Fatalf("productos = %d, want 1", len(products))
	}
	if products[0].Imagen == nil || *products[0].Imagen != "https://cugat.cl/uploads/uno.webp" {
		t.Errorf("imagen = %v", products[0].Imagen)
	}
}

func TestCugatPageURLAndCategory(t *testing.T) {
	categoryURL := "https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/"
	if got := cugatPageURL(categoryURL, 2); got != "https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/page/2/" {
		t.Errorf("page URL = %q", got)
	}
	if got := cugatPageURL("https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/page/2/", 3); got != "https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/page/3/" {
		t.Errorf("page URL desde página existente = %q", got)
	}
	if got := cugatCategoryFromURL("https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/page/2/"); got != "bebidas-jugos-y-aguas" {
		t.Errorf("categoría = %q", got)
	}
}

func TestExtractCugatDetail(t *testing.T) {
	body := `<script type="application/ld+json">
{"@context":"https://schema.org","@type":"Product","name":"Bebida Cugat",
"sku":"7800000000001","gtin13":"7800000000001","brand":{"@type":"Brand","name":"Marca Test"},
"image":[{"url":"https://cugat.cl/bebida.webp"}],
"offers":{"price":"1.290","availability":"https://schema.org/InStock"}}
</script>`
	product := extractCugatDetail([]byte(body), mustURL("https://cugat.cl/producto/bebida-cugat/"))
	if product == nil {
		t.Fatal("producto nil")
	}
	if product.Precio == nil || *product.Precio != 1290 {
		t.Errorf("precio = %v", product.Precio)
	}
	if product.EanGtin == nil || *product.EanGtin != "7800000000001" {
		t.Errorf("ean = %v", product.EanGtin)
	}
	if product.Marca == nil || *product.Marca != "Marca Test" {
		t.Errorf("marca = %v", product.Marca)
	}
	if product.EnStock == nil || !*product.EnStock {
		t.Errorf("en_stock = %v", product.EnStock)
	}
	if product.Imagen == nil || *product.Imagen != "https://cugat.cl/bebida.webp" {
		t.Errorf("imagen = %v", product.Imagen)
	}
}

func TestValidateCugatURLs(t *testing.T) {
	if err := validateCugatCategoryURL("https://example.com/categoria/"); err == nil {
		t.Fatal("se esperaba error para host no confiable")
	}
	if err := validateCugatProductURL("https://www.cugat.cl/producto/bebida/"); err != nil {
		t.Errorf("URL válida: %v", err)
	}
}

func TestCugatProductMode(t *testing.T) {
	productURL := "https://cugat.cl/producto/bebida-cugat/"
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		productURL: response(productURL, `<script type="application/ld+json">
{"@type":"Product","name":"Bebida Cugat","url":"https://cugat.cl/producto/bebida-cugat/","offers":{"price":"990"}}
</script>`),
	}}
	spider := NewCugat(fetcher, t.TempDir(), 100, quietLogger())
	var items []domain.Item
	err := spider.Run(context.Background(), domain.Job{ProductURL: productURL}, func(item domain.Item) error {
		items = append(items, item)
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].Producto != "Bebida Cugat" || items[0].Categoria != "producto" || items[0].Supermercado != "Cugat" {
		t.Errorf("item = %+v", items[0])
	}
}

func TestCugatRunPagination(t *testing.T) {
	categoryURL := "https://cugat.cl/categoria-producto/bebidas/"
	page2 := "https://cugat.cl/categoria-producto/bebidas/page/2/"
	t.Setenv("CUGAT_CATEGORY_URLS", categoryURL)
	fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
		categoryURL: response(categoryURL, `<div class="products"><div class="product-small product">
<a href="/producto/uno/"><img data-src="/uno.webp"></a>
<p class="product-title"><a href="/producto/uno/">Uno</a></p>
<ins><span class="woocommerce-Price-amount">$1.000</span></ins>
</div></div>`),
		page2: response(page2, `<div class="products"><div class="product-small product">
<a href="/producto/dos/"><img data-src="/dos.webp"></a>
<p class="product-title"><a href="/producto/dos/">Dos</a></p>
<ins><span class="woocommerce-Price-amount">$2.000</span></ins>
</div></div>`),
	}}
	spider := NewCugat(fetcher, t.TempDir(), 100, quietLogger())
	var items []domain.Item
	err := spider.Run(context.Background(), domain.Job{}, func(item domain.Item) error {
		items = append(items, item)
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].Categoria != "bebidas" || items[1].Categoria != "bebidas" {
		t.Errorf("categorías = %q, %q", items[0].Categoria, items[1].Categoria)
	}
}
