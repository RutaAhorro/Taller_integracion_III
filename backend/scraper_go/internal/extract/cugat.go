package extract

import (
	"bytes"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

// CugatProducts extrae las tarjetas del listado de WooCommerce.
func CugatProducts(site catalog.Site, body []byte, responseURL *url.URL) []domain.Product {
	cardSelector := site.Selector("product_card")
	if cardSelector == "" {
		return nil
	}
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	var products []domain.Product
	document.Find(cardSelector).Each(func(_ int, card *goquery.Selection) {
		name := normalize.Texto(firstText(card, site.Selector("title")))
		if name == nil {
			return
		}
		offerPrice := cugatPriceFrom(card, site, site.Selector("price_offer"))
		normalPrice := cugatPriceFrom(card, site, site.Selector("price_normal"))
		if offerPrice == nil {
			offerPrice = cugatPriceFrom(card, site, site.Selector("price_fallback"))
		}
		if normalPrice == nil {
			normalPrice = offerPrice
		}

		href := firstAttr(card, site.Selector("title"), "href")
		image := firstAttr(card, site.Selector("image"), "data-src")
		if image == "" {
			image = firstAttr(card, site.Selector("image"), "data-lazy-src")
		}
		if image == "" {
			image = firstAttr(card, site.Selector("noscript")+" img", "src")
		}
		if image == "" {
			image = noscriptImage(card, site.Selector("noscript"))
		}
		if image == "" {
			image = firstAttr(card, site.Selector("image"), "src")
		}

		cart := card.Find(site.Selector("cart")).First()
		cartSKU, _ := cart.Attr("data-product_sku")
		productID, _ := cart.Attr("data-product_id")
		if productID == "" {
			cardID, _ := card.Attr("id")
			productID = strings.TrimPrefix(cardID, site.Setting("card_id_prefix"))
		}
		ean := normalize.EAN(cartSKU)
		sku := firstNonNilString(nonEmptyStringPtr(productID), ean)

		var productURL *string
		if href != "" {
			value := ResolveURL(responseURL, href)
			productURL = &value
		}
		var imageURL *string
		if image != "" {
			value := ResolveURL(responseURL, image)
			imageURL = &value
		}
		if sku == nil && productURL != nil {
			sku = normalize.SKUFromURL(*productURL)
		}

		var offer *float64
		if normalPrice != nil && offerPrice != nil && *offerPrice < *normalPrice {
			offer = offerPrice
		}
		class, _ := card.Attr("class")
		products = append(products, domain.Product{
			Producto:     *name,
			Precio:       offerPrice,
			PrecioNormal: normalPrice,
			PrecioOferta: offer,
			EanGtin:      ean,
			SKU:          sku,
			EnStock:      domain.BoolPtr(hasClass(class, site.Setting("stock_class"))),
			URLProducto:  productURL,
			Imagen:       imageURL,
		})
	})
	return products
}

// CugatProduct lee la ficha individual desde JSON-LD.
func CugatProduct(site catalog.Site, body []byte, responseURL *url.URL) *domain.Product {
	return ExtractJSONLDProduct(site, body, responseURL)
}

func cugatPriceFrom(card *goquery.Selection, site catalog.Site, selector string) *float64 {
	if selector == "" {
		return nil
	}
	amount := card.Find(selector).Find(site.Selector("price_amount")).First().Text()
	return normalize.PrecioCLP(amount)
}
