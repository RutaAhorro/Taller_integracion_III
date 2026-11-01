package spiders

import (
	"bytes"
	"fmt"
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
	acuentaDefaultCategories = []string{
		"https://www.acuenta.cl/ca/congelados/04",
	}
	acuentaAllowedHosts = map[string]struct{}{
		"acuenta.cl":     {},
		"www.acuenta.cl": {},
	}

	acuentaReferenceStart = regexp.MustCompile(`(?:^|\\n|\n)([a-z0-9]+):`)
	acuentaPromotionType  = regexp.MustCompile(`\\"type\\":\\"([^"\\]+)\\"`)
	acuentaConditions     = regexp.MustCompile(`\\"conditions\\":\\"\$([a-z0-9]+)\\"`)
	acuentaConditionRef   = regexp.MustCompile(`\\"\$([a-z0-9]+)\\"`)
	acuentaPriceKey       = regexp.MustCompile(`\\"priceBeforeTaxes\\":(-?\d+)`)
	acuentaEANReference   = regexp.MustCompile(`(?:^|\\n|\n)([a-z0-9]+):\[\\?"(\d{8,14})`)
	acuentaSKUPattern     = regexp.MustCompile(`\\"sku\\":\\"(\d+)\\"`)
	acuentaSlugPattern    = regexp.MustCompile(`\\"slug\\":\\"([^"\\]+)\\"`)
	acuentaBrandPattern   = regexp.MustCompile(`\\"brand\\":\\"([^"\\]+)\\"`)
	acuentaStockPattern   = regexp.MustCompile(`\\"stock\\":(-?\d+)`)
	acuentaNamePattern    = regexp.MustCompile(`\\"name\\":\\"([^"\\]+)\\"`)
	acuentaEANField       = regexp.MustCompile(`\\"ean\\":(.*?),\\"maxQty\\"`)
	acuentaDigits         = regexp.MustCompile(`\d{8,14}`)
	acuentaEANRefPattern  = regexp.MustCompile(`\\"\$([a-z0-9]+)\\"`)
	acuentaPromotionRef   = regexp.MustCompile(`\\"promotion\\":\\"\$([a-z0-9]+)\\"`)
	acuentaPhotosRef      = regexp.MustCompile(`\\"photosUrl\\":\\"\$([a-z0-9]+)\\"`)
	acuentaMultiPrice     = regexp.MustCompile(`\b(\d+)\s*[xX]\s*\$\s*([\d.,]+)`)
	acuentaAmountPattern  = regexp.MustCompile(`\$\s*([\d.,]+)`)
	acuentaBrandText      = regexp.MustCompile(`(?i)\b(Lider|Super Cerdo|La Crianza|Savory|Mega|PF)\b`)
	acuentaFormatPattern  = regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*(?:kg|g|gr|ml|lt|l|un(?:idad(?:es)?)?)\b`)
	acuentaWordPattern    = regexp.MustCompile(`[a-záéíóúñ0-9]+`)
	acuentaTrailingDigits = regexp.MustCompile(`-\d+$`)
)

type Acuenta struct {
	baseSpider
}

func NewAcuenta(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Acuenta {
	return &Acuenta{baseSpider: newBaseSpider(fetcher, categoryDir, maxPages, logger, spiderConfig{
		name:              AcuentaName,
		supermarket:       "A Cuenta",
		categoryFile:      "acuenta_categories.txt",
		categoryEnv:       "ACUENTA_CATEGORY_URLS",
		defaultCategories: acuentaDefaultCategories,
		validateCategory:  validateAcuentaCategoryURL,
		validateProduct:   validateAcuentaProductURL,
		extractProducts:   extractAcuentaProducts,
		extractProduct: func(body []byte, responseURL *url.URL, requested string) *domain.Product {
			return selectProductByURL(extractAcuentaProducts(body, responseURL), requested, responseURL)
		},
		pageURL:         acuentaPageURL,
		startPage:       acuentaPageNumber,
		categoryFromURL: acuentaCategoryFromURL,
	})}
}

var (
	validateAcuentaCategoryURL = validateSiteURL(
		acuentaAllowedHosts,
		"/ca/",
		"",
		"La categoría de A Cuenta debe usar una URL HTTPS /ca/ de acuenta.cl",
		0,
	)
	validateAcuentaProductURL = validateSiteURL(
		acuentaAllowedHosts,
		"/p/",
		"",
		"product_url debe ser una URL HTTPS /p/ de acuenta.cl",
		0,
	)
)

func acuentaCategoryFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "sin_categoria"
	}
	segments := nonEmptySegments(parsed.Path)
	if len(segments) == 0 {
		return "sin_categoria"
	}
	if isDigits(segments[len(segments)-1]) && len(segments) >= 2 {
		return segments[len(segments)-2]
	}
	return segments[len(segments)-1]
}

func acuentaPageNumber(rawURL string) int {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 1
	}
	value := parsed.Query().Get("currentPage")
	page, err := strconv.Atoi(value)
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func acuentaPageURL(rawURL string, page int) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := url.Values{}
	query.Set("currentPage", strconv.Itoa(page))
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func extractAcuentaProducts(body []byte, responseURL *url.URL) []domain.Product {
	rscProducts := extractAcuentaRSCProducts(body, responseURL)
	if len(rscProducts) > 0 {
		return mergeAcuentaVisiblePrices(body, responseURL, rscProducts)
	}
	return extractAcuentaCards(body, responseURL)
}

func extractAcuentaCards(body []byte, responseURL *url.URL) []domain.Product {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	var products []domain.Product
	seenURLs := map[string]struct{}{}
	document.Find(`a[href^="/p/"]`).Each(func(_ int, link *goquery.Selection) {
		href, _ := link.Attr("href")
		if href == "" {
			return
		}
		productURL := resolveURL(responseURL, href)
		if _, ok := seenURLs[productURL]; ok {
			return
		}
		seenURLs[productURL] = struct{}{}

		card := link.Parent()
		for card.Length() > 0 && !strings.Contains(allText(card), "$") {
			card = card.Parent()
		}
		if card.Length() == 0 {
			return
		}
		name := normalize.Texto(allText(link))
		text := normalize.Texto(allText(card))
		if name == nil || text == nil {
			return
		}
		current, normal, promotion := acuentaPricesFromText(*text)
		if current == nil || normal == nil {
			return
		}

		image := firstAttr(card, "img", "src")
		if image == "" {
			image = firstAttr(card, "img", "data-src")
		}
		var imageURL *string
		if image != "" {
			value := resolveURL(responseURL, image)
			imageURL = &value
		}
		var offer *float64
		if *current < *normal {
			offer = current
		}
		sku := acuentaSKUFromURL(productURL)
		products = append(products, domain.Product{
			Producto:          *name,
			Precio:            current,
			PrecioNormal:      normal,
			PrecioOferta:      offer,
			SKU:               sku,
			Marca:             acuentaBrandFromText(*text),
			FormatoCrudo:      formatFromText(*name, acuentaFormatPattern),
			MecanicaPromocion: promotion,
			URLProducto:       &productURL,
			Imagen:            imageURL,
		})
	})
	return products
}

func mergeAcuentaVisiblePrices(body []byte, responseURL *url.URL, products []domain.Product) []domain.Product {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return products
	}
	byURL := make(map[string]int, len(products))
	for index, product := range products {
		if product.URLProducto != nil {
			byURL[*product.URLProducto] = index
		}
	}
	document.Find(`a[href^="/p/"]`).Each(func(_ int, link *goquery.Selection) {
		href, _ := link.Attr("href")
		if href == "" {
			return
		}
		productURL := resolveURL(responseURL, href)
		index, ok := byURL[productURL]
		if !ok {
			return
		}
		card := link.Parent()
		for card.Length() > 0 && !strings.Contains(allText(card), "$") {
			card = card.Parent()
		}
		if card.Length() == 0 {
			return
		}
		text := normalize.Texto(allText(card))
		if text == nil {
			return
		}
		current, normal, promotion := acuentaPricesFromText(*text)
		if current == nil || normal == nil {
			return
		}
		product := &products[index]
		if *current < *normal {
			product.Precio = current
			product.PrecioNormal = normal
			product.PrecioOferta = current
			if product.MecanicaPromocion == nil {
				product.MecanicaPromocion = domain.StringPtr("promoción")
			}
		}
		if promotion != nil {
			product.MecanicaPromocion = promotion
		}
	})
	return products
}

func extractAcuentaRSCProducts(body []byte, responseURL *url.URL) []domain.Product {
	text := string(body)
	promotionPrices := acuentaPromotionPrices(text)
	eanReferences := map[string]string{}
	for _, match := range acuentaEANReference.FindAllStringSubmatch(text, -1) {
		eanReferences[match[1]] = match[2]
	}

	var products []domain.Product
	seenSKUs := map[string]struct{}{}
	for _, match := range acuentaSKUPattern.FindAllStringSubmatchIndex(text, -1) {
		sku := text[match[2]:match[3]]
		if _, ok := seenSKUs[sku]; ok {
			continue
		}
		end := match[1] + 1800
		if end > len(text) {
			end = len(text)
		}
		window := text[match[0]:end]

		slugMatch := acuentaSlugPattern.FindStringSubmatch(window)
		priceMatch := acuentaPriceKey.FindStringSubmatch(window)
		if len(slugMatch) < 2 || len(priceMatch) < 2 {
			continue
		}
		slug := slugMatch[1]
		normalPrice := normalize.PrecioCLP(priceMatch[1])
		if normalPrice == nil {
			continue
		}

		var name *string
		if nameMatch := acuentaNamePattern.FindStringSubmatch(window); len(nameMatch) >= 2 {
			name = normalize.Texto(nameMatch[1])
		}
		if name == nil || !acuentaNameMatchesSlug(*name, slug) {
			name = acuentaNameFromSlug(slug)
		}
		if name == nil {
			continue
		}

		var ean *string
		if eanField := acuentaEANField.FindStringSubmatch(window); len(eanField) >= 2 {
			eanValues := acuentaDigits.FindAllString(eanField[1], -1)
			if len(eanValues) == 0 {
				if reference := acuentaEANRefPattern.FindStringSubmatch(eanField[1]); len(reference) >= 2 {
					if value, ok := eanReferences[reference[1]]; ok {
						eanValues = []string{value}
					}
				}
			}
			if len(eanValues) > 0 {
				ean = normalize.EAN(eanValues[0])
			}
		}

		promotionRef := ""
		if promotionMatch := acuentaPromotionRef.FindStringSubmatch(window); len(promotionMatch) >= 2 {
			promotionRef = promotionMatch[1]
		}
		currentPrice := normalPrice
		if value, ok := promotionPrices[promotionRef]; ok {
			currentPrice = value
		}

		image := acuentaImageFromRSC(text, sku)
		if image == "" {
			image = acuentaResolveImageReference(text, window)
		}
		if image == "" {
			image = acuentaImageFromSKU(sku)
		}
		imageURL := acuentaCanonicalImageURL(image)

		productURL := resolveURL(responseURL, "/p/"+slug)
		var stock *bool
		if stockMatch := acuentaStockPattern.FindStringSubmatch(window); len(stockMatch) >= 2 {
			if quantity, err := strconv.Atoi(stockMatch[1]); err == nil {
				stock = domain.BoolPtr(quantity > 0)
			}
		}
		var brand *string
		if brandMatch := acuentaBrandPattern.FindStringSubmatch(window); len(brandMatch) >= 2 {
			brand = normalize.Texto(brandMatch[1])
		}
		var offer *float64
		if *currentPrice < *normalPrice {
			offer = currentPrice
		}
		var mechanics *string
		if promotionRef != "" {
			mechanics = domain.StringPtr("promoción")
		}
		seenSKUs[sku] = struct{}{}
		products = append(products, domain.Product{
			Producto:          *name,
			Precio:            currentPrice,
			PrecioNormal:      normalPrice,
			PrecioOferta:      offer,
			EanGtin:           ean,
			SKU:               &sku,
			Marca:             brand,
			FormatoCrudo:      formatFromText(*name, acuentaFormatPattern),
			MecanicaPromocion: mechanics,
			EnStock:           stock,
			URLProducto:       &productURL,
			Imagen:            imageURL,
		})
	}
	return products
}

func acuentaPromotionPrices(text string) map[string]*float64 {
	starts := acuentaReferenceStart.FindAllStringSubmatchIndex(text, -1)
	references := map[string]string{}
	for index, match := range starts {
		key := text[match[2]:match[3]]
		end := len(text)
		if index+1 < len(starts) {
			end = starts[index+1][0]
		}
		references[key] = text[match[1]:end]
	}

	promotionPrices := map[string]*float64{}
	for reference, data := range references {
		typeMatch := acuentaPromotionType.FindStringSubmatch(data)
		if len(typeMatch) < 2 || typeMatch[1] != "specialPrice" {
			continue
		}
		conditionsMatch := acuentaConditions.FindStringSubmatch(data)
		if len(conditionsMatch) < 2 {
			continue
		}
		conditions := references[conditionsMatch[1]]
		if conditions == "" {
			continue
		}
		var prices []float64
		for _, conditionRef := range acuentaConditionRef.FindAllStringSubmatch(conditions, -1) {
			conditionData := references[conditionRef[1]]
			priceMatch := acuentaPriceKey.FindStringSubmatch(conditionData)
			if len(priceMatch) < 2 {
				continue
			}
			if price := normalize.PrecioCLP(priceMatch[1]); price != nil {
				prices = append(prices, *price)
			}
		}
		if len(prices) == 0 {
			continue
		}
		minimum := prices[0]
		for _, price := range prices[1:] {
			if price < minimum {
				minimum = price
			}
		}
		value := minimum
		promotionPrices[reference] = &value
	}
	return promotionPrices
}

func acuentaImageFromRSC(text, sku string) string {
	pattern := regexp.MustCompile(
		`(?i)https://images\.lider\.cl/wmtcl\?source=url\[file:/productos/` +
			regexp.QuoteMeta(sku) +
			`[^"\s]*?\.(?:jpe?g|png|webp)\](?:\\u0026|&)sink`,
	)
	match := pattern.FindString(text)
	return strings.ReplaceAll(match, `\u0026`, "&")
}

func acuentaResolveImageReference(text, window string) string {
	referenceMatch := acuentaPhotosRef.FindStringSubmatch(window)
	if len(referenceMatch) < 2 {
		return ""
	}
	pattern := regexp.MustCompile(
		regexp.QuoteMeta(referenceMatch[1]) + `:\[\\?"(https?://[^"\\]+)`,
	)
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.ReplaceAll(match[1], `\u0026`, "&")
}

func acuentaImageFromSKU(sku string) string {
	return "https://images.lider.cl/wmtcl?source=url[file:/productos/" + sku + "a.jpg]&sink"
}

func acuentaCanonicalImageURL(value string) *string {
	if value == "" {
		return nil
	}
	value = strings.ReplaceAll(value, `\u0026`, "&")
	var builder strings.Builder
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune(":/?=&.%_-~", rune(character)) {
			builder.WriteByte(character)
			continue
		}
		fmt.Fprintf(&builder, "%%%02X", character)
	}
	result := builder.String()
	return &result
}

func acuentaNameMatchesSlug(name, slug string) bool {
	nameTokens := acuentaTokens(name)
	slugTokens := acuentaTokens(slug)
	intersection := 0
	for token := range nameTokens {
		if _, ok := slugTokens[token]; ok {
			intersection++
		}
	}
	return intersection >= 2
}

func acuentaTokens(value string) map[string]struct{} {
	tokens := map[string]struct{}{}
	for _, match := range acuentaWordPattern.FindAllString(value, -1) {
		token := strings.ToLower(match)
		if len([]rune(token)) > 2 {
			tokens[token] = struct{}{}
		}
	}
	return tokens
}

func acuentaNameFromSlug(slug string) *string {
	readable := acuentaTrailingDigits.ReplaceAllString(slug, "")
	return normalize.Texto(pythonTitle(strings.ReplaceAll(readable, "-", " ")))
}

func acuentaPricesFromText(text string) (*float64, *float64, *string) {
	if multi := acuentaMultiPrice.FindStringSubmatchIndex(text); multi != nil {
		var prefixAmounts []float64
		for _, match := range acuentaAmountPattern.FindAllStringSubmatch(text[:multi[0]], -1) {
			if amount := normalize.PrecioCLP(match[1]); amount != nil {
				prefixAmounts = append(prefixAmounts, *amount)
			}
		}
		var current *float64
		if len(prefixAmounts) > 0 {
			current = &prefixAmounts[len(prefixAmounts)-1]
		} else {
			current = normalize.PrecioCLP(text[multi[4]:multi[5]])
		}
		if current != nil {
			promotion := text[multi[0]:multi[1]]
			return current, current, &promotion
		}
	}

	var amounts []float64
	for _, match := range acuentaAmountPattern.FindAllStringSubmatch(text, -1) {
		if amount := normalize.PrecioCLP(match[1]); amount != nil {
			amounts = append(amounts, *amount)
		}
	}
	if len(amounts) == 0 {
		return nil, nil, nil
	}
	current := amounts[0]
	normal := current
	if len(amounts) > 1 {
		normal = amounts[1]
	}
	if normal < current {
		normal = current
	}
	return &current, &normal, nil
}

func acuentaBrandFromText(text string) *string {
	match := acuentaBrandText.FindString(text)
	if match == "" {
		return nil
	}
	return normalize.Texto(match)
}

func acuentaSKUFromURL(productURL string) *string {
	slug := lastPathSegment(productURL)
	if match := acuentaTrailingDigits.FindString(slug); match != "" {
		value := "ACUENTA-" + strings.TrimPrefix(match, "-")
		return &value
	}
	value := "ACUENTA-" + slug
	runes := []rune(value)
	if len(runes) > 100 {
		value = string(runes[:100])
	}
	return &value
}
