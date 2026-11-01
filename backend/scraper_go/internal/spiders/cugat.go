package spiders

import (
	"bytes"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

var (
	cugatDefaultCategories = []string{
		"https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/",
	}
	cugatAllowedHosts = map[string]struct{}{
		"cugat.cl":     {},
		"www.cugat.cl": {},
	}
)

type Cugat struct {
	baseSpider
}

func NewCugat(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Cugat {
	return &Cugat{baseSpider: newBaseSpider(fetcher, categoryDir, maxPages, logger, spiderConfig{
		name:              CugatName,
		supermarket:       "Cugat",
		categoryFile:      "cugat_categories.txt",
		categoryEnv:       "CUGAT_CATEGORY_URLS",
		defaultCategories: cugatDefaultCategories,
		validateCategory:  validateCugatCategoryURL,
		validateProduct:   validateCugatProductURL,
		extractProducts:   extractCugatProducts,
		extractProduct: func(body []byte, responseURL *url.URL, _ string) *domain.Product {
			return extractCugatDetail(body, responseURL)
		},
		pageURL: func(rawURL string, page int) string {
			if page <= 1 {
				return rawURL
			}
			return cugatPageURL(rawURL, page)
		},
		categoryFromURL: cugatCategoryFromURL,
	})}
}

func validateCugatCategoryURL(rawURL string) error {
	return validateHTTPSHost(
		rawURL,
		cugatAllowedHosts,
		"La categoría debe ser una URL HTTPS pública de cugat.cl",
	)
}

func validateCugatProductURL(rawURL string) error {
	return validateHTTPSHost(
		rawURL,
		cugatAllowedHosts,
		"product_url debe ser una URL HTTPS pública de cugat.cl",
	)
}

func cugatPageURL(rawURL string, page int) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	segments := nonEmptySegments(parsed.Path)
	if len(segments) >= 2 && segments[len(segments)-2] == "page" && isDigits(segments[len(segments)-1]) {
		segments = segments[:len(segments)-2]
	}
	parsed.Path = "/" + strings.Join(segments, "/") + "/page/" + strconv.Itoa(page) + "/"
	parsed.RawPath = ""
	return parsed.String()
}

func cugatCategoryFromURL(rawURL string) string {
	segments := nonEmptySegments(mustParsePath(rawURL))
	if len(segments) >= 2 && segments[len(segments)-2] == "page" {
		segments = segments[:len(segments)-2]
	}
	if len(segments) == 0 {
		return "sin_categoria"
	}
	return segments[len(segments)-1]
}

func extractCugatProducts(body []byte, responseURL *url.URL) []domain.Product {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	var products []domain.Product
	document.Find("div.products div.product-small.product").Each(func(_ int, card *goquery.Selection) {
		name := normalize.Texto(firstText(card, "p.product-title a"))
		if name == nil {
			return
		}
		offerPrice := cugatPriceFrom(card, "ins")
		normalPrice := cugatPriceFrom(card, "del")
		if offerPrice == nil {
			offerPrice = cugatPriceFrom(card, "span.price")
		}
		if normalPrice == nil {
			normalPrice = offerPrice
		}

		href := firstAttr(card, "p.product-title a", "href")
		image := firstAttr(card, "img", "data-src")
		if image == "" {
			image = firstAttr(card, "img", "data-lazy-src")
		}
		if image == "" {
			image = firstAttr(card, "noscript img", "src")
		}
		if image == "" {
			image = noscriptImage(card)
		}
		if image == "" {
			image = firstAttr(card, "img", "src")
		}

		cart := card.Find("a.add_to_cart_button").First()
		cartSKU, _ := cart.Attr("data-product_sku")
		productID, _ := cart.Attr("data-product_id")
		if productID == "" {
			cardID, _ := card.Attr("id")
			productID = strings.TrimPrefix(cardID, "product-")
		}
		ean := normalize.EAN(cartSKU)
		sku := firstNonNilString(nonEmptyStringPtr(productID), ean)

		var productURL *string
		if href != "" {
			value := resolveURL(responseURL, href)
			productURL = &value
		}
		var imageURL *string
		if image != "" {
			value := resolveURL(responseURL, image)
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
			EnStock:      domain.BoolPtr(hasClass(class, "instock")),
			URLProducto:  productURL,
			Imagen:       imageURL,
		})
	})
	return products
}

func cugatPriceFrom(card *goquery.Selection, selector string) *float64 {
	amount := card.Find(selector).Find("span.woocommerce-Price-amount").First().Text()
	return normalize.PrecioCLP(amount)
}

func extractCugatDetail(body []byte, responseURL *url.URL) *domain.Product {
	return extractJSONLDProduct(body, responseURL)
}

func cugatImageURL(value any) *string {
	if list, ok := value.([]any); ok {
		if len(list) == 0 {
			return nil
		}
		value = list[0]
	}
	if mapped, ok := value.(map[string]any); ok {
		value = mapped["url"]
	}
	if text, ok := value.(string); ok && text != "" {
		return &text
	}
	return nil
}

func isProductType(value any) bool {
	switch typed := value.(type) {
	case string:
		return typed == "Product"
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok && text == "Product" {
				return true
			}
		}
	}
	return false
}

func firstMapValue(value any) any {
	if list, ok := value.([]any); ok {
		if len(list) == 0 {
			return nil
		}
		return list[0]
	}
	return value
}
