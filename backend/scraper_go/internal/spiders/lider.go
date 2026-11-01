package spiders

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

var (
	liderDefaultCategories = []string{
		"https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836",
	}
	liderAllowedHosts = map[string]struct{}{
		"super.lider.cl": {},
	}
	liderCategoryIDPattern = regexp.MustCompile(`^\d+(?:_\d+)*$`)
	liderFormatPattern     = regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*(?:kg|grs?|g|ml|lt|l|un(?:idades?)?)\b`)
)

type Lider struct {
	baseSpider
}

func NewLider(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Lider {
	return &Lider{baseSpider: newBaseSpider(fetcher, categoryDir, maxPages, logger, spiderConfig{
		name:              LiderName,
		supermarket:       "Lider",
		categoryFile:      "lider_categories.txt",
		categoryEnv:       "LIDER_CATEGORY_URLS",
		defaultCategories: liderDefaultCategories,
		validateCategory:  validateLiderCategoryURL,
		validateProduct:   validateLiderProductURL,
		extractProducts:   extractLiderProducts,
		extractProduct: func(body []byte, responseURL *url.URL, requested string) *domain.Product {
			return extractLiderProduct(body, responseURL, requested)
		},
		pageURL:         liderPageURL,
		startPage:       liderPageNumber,
		categoryFromURL: liderCategoryFromURL,
	})}
}

func validateLiderCategoryURL(rawURL string) error {
	message := "La categoría debe ser una URL HTTPS /browse/ de super.lider.cl"
	if err := validateHTTPSHost(rawURL, liderAllowedHosts, message); err != nil {
		return err
	}
	parsed, _ := url.Parse(rawURL)
	if !strings.HasPrefix(parsed.Path, "/browse/") {
		return errors.New(message)
	}
	return nil
}

func validateLiderProductURL(rawURL string) error {
	message := "product_url debe ser una URL HTTPS /ip/ de super.lider.cl"
	if err := validateHTTPSHost(rawURL, liderAllowedHosts, message); err != nil {
		return err
	}
	parsed, _ := url.Parse(rawURL)
	if !strings.HasPrefix(parsed.Path, "/ip/") {
		return errors.New(message)
	}
	return nil
}

func liderPageNumber(rawURL string) int {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 1
	}
	value := parsed.Query().Get("page")
	page, err := strconv.Atoi(value)
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func liderPageURL(rawURL string, page int) string {
	if page <= 1 {
		return removeQueryParam(rawURL, "page")
	}
	return appendQueryParam(rawURL, "page", strconv.Itoa(page))
}

func liderCategoryFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "sin_categoria"
	}
	segments := nonEmptySegments(parsed.Path)
	if len(segments) > 0 && segments[0] == "browse" {
		segments = segments[1:]
	}
	if len(segments) > 0 && liderCategoryIDPattern.MatchString(segments[len(segments)-1]) {
		segments = segments[:len(segments)-1]
	}
	if len(segments) == 0 {
		return "sin_categoria"
	}
	return segments[len(segments)-1]
}

func extractLiderProducts(body []byte, responseURL *url.URL) []domain.Product {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	if products, found := extractLiderNextData(document, responseURL); found {
		return products
	}
	return extractLiderCards(document, responseURL)
}

func extractLiderNextData(document *goquery.Document, responseURL *url.URL) ([]domain.Product, bool) {
	payload, ok := liderNextDataPayload(document)
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
			if product, ok := liderProductFromItem(item, responseURL); ok {
				products = append(products, product)
			}
		}
	}
	return products, true
}

func liderNextDataPayload(document *goquery.Document) (map[string]any, bool) {
	script := document.Find("script#__NEXT_DATA__").First()
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

// extractLiderProduct reconstruye una ficha /ip/. Prueba, en orden, JSON-LD,
// el objeto de producto de __NEXT_DATA__ y las tarjetas del HTML. Sólo acepta
// un candidato cuya URL coincida con la solicitada (o el único disponible).
func extractLiderProduct(body []byte, responseURL *url.URL, requested string) *domain.Product {
	if products := extractJSONLDProducts(body, responseURL); len(products) > 0 {
		if selected := selectProductByURL(products, requested, responseURL); selected != nil {
			if resolved := resolveURL(responseURL, *selected.URLProducto); validateLiderProductURL(resolved) == nil {
				selected.URLProducto = &resolved
				return selected
			}
		}
	}

	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	if payload, ok := liderNextDataPayload(document); ok {
		var products []domain.Product
		for _, value := range walkJSON(payload) {
			item, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if product, ok := liderProductFromItem(item, responseURL); ok {
				products = append(products, product)
			}
		}
		if selected := selectProductByURL(products, requested, responseURL); selected != nil {
			return selected
		}
	}
	return selectProductByURL(extractLiderCards(document, responseURL), requested, responseURL)
}

func liderProductFromItem(item map[string]any, responseURL *url.URL) (domain.Product, bool) {
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
	productURL := resolveURL(responseURL, canonical)
	if err := validateLiderProductURL(productURL); err != nil {
		return domain.Product{}, false
	}

	var image *string
	imageInfo, _ := item["imageInfo"].(map[string]any)
	if thumbnail, ok := imageInfo["thumbnailUrl"].(string); ok && thumbnail != "" {
		value := resolveURL(responseURL, thumbnail)
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
		mechanics = domain.StringPtr("promoción")
	}
	return domain.Product{
		Producto:          *name,
		Precio:            currentPrice,
		PrecioNormal:      normalPrice,
		PrecioOferta:      offer,
		EanGtin:           productCode,
		SKU:               sku,
		Marca:             normalize.Texto(item["brand"]),
		FormatoCrudo:      formatFromText(*name, liderFormatPattern),
		MecanicaPromocion: mechanics,
		EnStock:           inStock,
		URLProducto:       &productURL,
		Imagen:            image,
	}, true
}

func extractLiderCards(document *goquery.Document, responseURL *url.URL) []domain.Product {
	cards := document.Find(`[data-testid='item-stack'] div[role='group'][data-item-id]`)
	if cards.Length() == 0 {
		cards = document.Find(`div[role='group'][data-dca-id]`)
	}

	var products []domain.Product
	cards.Each(func(_ int, card *goquery.Selection) {
		name := normalize.Texto(allText(card.Find(`[data-automation-id='product-title']`)))
		href := firstAttr(card, `a[href*="/ip/"]`, "href")
		if name == nil || href == "" {
			return
		}
		productURL := resolveURL(responseURL, href)
		if err := validateLiderProductURL(productURL); err != nil {
			return
		}

		priceBlock := card.Find(`[data-automation-id='product-price']`).First()
		currentPrice := normalize.PrecioCLP(firstText(priceBlock, `div[aria-hidden='true']`))
		if currentPrice == nil {
			return
		}
		normalPrice := normalize.PrecioCLP(firstText(priceBlock, "span.strike"))
		if normalPrice == nil {
			normalPrice = currentPrice
		}

		rawCode := card.AttrOr("data-dca-id", "")
		if rawCode == "" {
			rawCode = lastPathSegment(productURL)
		}
		productCode := normalize.EAN(rawCode)
		sku := firstNonNilString(productCode, nonEmptyStringPtr(card.AttrOr("data-item-id", "")))

		image := firstAttr(card, `img[data-testid='productTileImage']`, "src")
		if image == "" {
			image = firstAttr(card, "img", "data-src")
		}
		if image == "" {
			image = firstAttr(card, "img", "src")
		}
		var imageURL *string
		if image != "" {
			value := resolveURL(responseURL, image)
			imageURL = &value
		}

		cardText := strings.ToLower(allText(card))
		var inStock *bool
		switch {
		case strings.Contains(cardText, "available for despacho"),
			strings.Contains(cardText, "available for pickup"):
			inStock = domain.BoolPtr(true)
		case strings.Contains(cardText, "agotado"),
			strings.Contains(cardText, "no disponible"):
			inStock = domain.BoolPtr(false)
		}

		var offer *float64
		var mechanics *string
		if *currentPrice < *normalPrice {
			offer = currentPrice
			mechanics = domain.StringPtr("promoción")
		}
		products = append(products, domain.Product{
			Producto:          *name,
			Precio:            currentPrice,
			PrecioNormal:      normalPrice,
			PrecioOferta:      offer,
			EanGtin:           productCode,
			SKU:               sku,
			Marca:             normalize.Texto(firstText(card, "div.mb1.mt2.b")),
			FormatoCrudo:      formatFromText(*name, liderFormatPattern),
			MecanicaPromocion: mechanics,
			EnStock:           inStock,
			URLProducto:       &productURL,
			Imagen:            imageURL,
		})
	})
	return products
}
