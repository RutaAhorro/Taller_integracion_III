package extract

import (
	"bytes"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

var liderFormatPattern = regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*(?:kg|grs?|g|ml|lt|l|un(?:idades?)?)\b`)

// LiderProducts intenta __NEXT_DATA__ y cae a las tarjetas del HTML.
func LiderProducts(site catalog.Site, body []byte, responseURL *url.URL) []domain.Product {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	if products, found := liderNextDataProducts(site, document, responseURL); found {
		return products
	}
	return liderCards(site, document, responseURL)
}

// LiderProduct reconstruye una ficha /ip/. Prueba, en orden, JSON-LD, el objeto
// de producto de __NEXT_DATA__ y las tarjetas del HTML. Sólo acepta un
// candidato cuya URL coincida con la solicitada (o el único disponible).
func LiderProduct(site catalog.Site, body []byte, requested string, responseURL *url.URL) *domain.Product {
	if products := ExtractJSONLDProducts(site, body, responseURL); len(products) > 0 {
		if selected := SelectProductByURL(products, requested, responseURL); selected != nil {
			if resolved := ResolveURL(responseURL, *selected.URLProducto); site.ValidateProductURL(resolved) == nil {
				selected.URLProducto = &resolved
				return selected
			}
		}
	}

	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	if payload, ok := liderNextDataPayload(site, document); ok {
		var products []domain.Product
		for _, value := range walkJSON(payload) {
			item, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if product, ok := liderProductFromItem(site, item, responseURL); ok {
				products = append(products, product)
			}
		}
		if selected := SelectProductByURL(products, requested, responseURL); selected != nil {
			return selected
		}
	}
	return SelectProductByURL(liderCards(site, document, responseURL), requested, responseURL)
}

func liderNextDataProducts(site catalog.Site, document *goquery.Document, responseURL *url.URL) ([]domain.Product, bool) {
	payload, ok := liderNextDataPayload(site, document)
	if !ok {
		return nil, false
	}

	props, _ := payload["props"].(map[string]any)
	pageProps, _ := props["pageProps"].(map[string]any)
	initialData, _ := pageProps["initialData"].(map[string]any)
	searchResult, _ := initialData["searchResult"].(map[string]any)
	itemStacks, ok := searchResult["itemStacks"].([]any)
	if !ok {
		return nil, false
	}

	products := make([]domain.Product, 0)
	for _, rawStack := range itemStacks {
		stack, ok := rawStack.(map[string]any)
		if !ok {
			continue
		}
		items, ok := stack["items"].([]any)
		if !ok {
			continue
		}
		for _, rawItem := range items {
			item, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			if product, ok := liderProductFromItem(site, item, responseURL); ok {
				products = append(products, product)
			}
		}
	}
	return products, true
}

func liderNextDataPayload(site catalog.Site, document *goquery.Document) (map[string]any, bool) {
	selector := site.Selector("next_data")
	if selector == "" {
		return nil, false
	}
	script := document.Find(selector).First()
	if script.Length() == 0 {
		return nil, false
	}
	var payload map[string]any
	decoder := json.NewDecoder(strings.NewReader(script.Text()))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, false
	}
	return payload, true
}

func liderProductFromItem(site catalog.Site, item map[string]any, responseURL *url.URL) (domain.Product, bool) {
	name := normalize.Texto(item["name"])
	productCode := normalize.EAN(item["usItemId"])
	currentPrice := normalize.PrecioCLP(item["price"])
	if name == nil || productCode == nil || currentPrice == nil {
		return domain.Product{}, false
	}

	priceInfo, _ := item["priceInfo"].(map[string]any)
	normalPrice := normalize.PrecioCLP(firstValue(priceInfo, "wasPrice"))
	if normalPrice == nil {
		normalPrice = normalize.PrecioCLP(firstValue(priceInfo, "itemPrice"))
	}
	if normalPrice == nil {
		normalPrice = currentPrice
	}

	canonical, ok := item["canonicalUrl"].(string)
	if !ok || canonical == "" {
		return domain.Product{}, false
	}
	productURL := ResolveURL(responseURL, canonical)
	if err := site.ValidateProductURL(productURL); err != nil {
		return domain.Product{}, false
	}

	var image *string
	imageInfo, _ := item["imageInfo"].(map[string]any)
	if thumbnail, ok := imageInfo["thumbnailUrl"].(string); ok && thumbnail != "" {
		value := ResolveURL(responseURL, thumbnail)
		image = &value
	}

	var inStock *bool
	if outOfStock, ok := item["isOutOfStock"].(bool); ok {
		value := !outOfStock
		inStock = &value
	}

	sku := normalize.Texto(item["sku"])
	if sku == nil {
		sku = productCode
	}
	var offer *float64
	var mechanics *string
	if *currentPrice < *normalPrice {
		offer = currentPrice
		mechanics = domain.StringPtr(site.Setting("promotion"))
	}
	return domain.Product{
		Producto:          *name,
		Precio:            currentPrice,
		PrecioNormal:      normalPrice,
		PrecioOferta:      offer,
		EanGtin:           productCode,
		SKU:               sku,
		Marca:             normalize.Texto(item["brand"]),
		FormatoCrudo:      FormatFromText(*name, liderFormatPattern),
		MecanicaPromocion: mechanics,
		EnStock:           inStock,
		URLProducto:       &productURL,
		Imagen:            image,
	}, true
}

func liderCards(site catalog.Site, document *goquery.Document, responseURL *url.URL) []domain.Product {
	cards := document.Find(site.Selector("product_card"))
	if cards.Length() == 0 {
		cards = document.Find(site.Selector("product_card_fallback"))
	}

	var products []domain.Product
	cards.Each(func(_ int, card *goquery.Selection) {
		name := normalize.Texto(allText(card.Find(site.Selector("card_title"))))
		href := firstAttr(card, site.Selector("product_link"), "href")
		if name == nil || href == "" {
			return
		}
		productURL := ResolveURL(responseURL, href)
		if err := site.ValidateProductURL(productURL); err != nil {
			return
		}

		priceBlock := card.Find(site.Selector("price_block")).First()
		currentPrice := normalize.PrecioCLP(firstText(priceBlock, site.Selector("price_current")))
		if currentPrice == nil {
			return
		}
		normalPrice := normalize.PrecioCLP(firstText(priceBlock, site.Selector("price_normal")))
		if normalPrice == nil {
			normalPrice = currentPrice
		}

		rawCode := card.AttrOr("data-dca-id", "")
		if rawCode == "" {
			rawCode = lastPathSegment(productURL)
		}
		productCode := normalize.EAN(rawCode)
		sku := firstNonNilString(productCode, nonEmptyStringPtr(card.AttrOr("data-item-id", "")))

		image := firstAttr(card, site.Selector("image"), "src")
		if image == "" {
			image = firstAttr(card, "img", "data-src")
		}
		if image == "" {
			image = firstAttr(card, "img", "src")
		}
		var imageURL *string
		if image != "" {
			value := ResolveURL(responseURL, image)
			imageURL = &value
		}

		cardText := strings.ToLower(allText(card))
		var inStock *bool
		switch {
		case containsAny(cardText, site.List("stock_in")):
			inStock = domain.BoolPtr(true)
		case containsAny(cardText, site.List("stock_out")):
			inStock = domain.BoolPtr(false)
		}

		var offer *float64
		var mechanics *string
		if *currentPrice < *normalPrice {
			offer = currentPrice
			mechanics = domain.StringPtr(site.Setting("promotion"))
		}
		products = append(products, domain.Product{
			Producto:          *name,
			Precio:            currentPrice,
			PrecioNormal:      normalPrice,
			PrecioOferta:      offer,
			EanGtin:           productCode,
			SKU:               sku,
			Marca:             normalize.Texto(firstText(card, site.Selector("brand"))),
			FormatoCrudo:      FormatFromText(*name, liderFormatPattern),
			MecanicaPromocion: mechanics,
			EnStock:           inStock,
			URLProducto:       &productURL,
			Imagen:            imageURL,
		})
	})
	return products
}

func containsAny(text string, values []string) bool {
	for _, value := range values {
		if value != "" && strings.Contains(text, value) {
			return true
		}
	}
	return false
}
