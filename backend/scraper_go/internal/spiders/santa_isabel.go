package spiders

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

var (
	santaIsabelDefaultCategories = []string{
		"https://www.santaisabel.cl/despensa",
	}
	santaIsabelAllowedHosts = map[string]struct{}{
		"santaisabel.cl":     {},
		"www.santaisabel.cl": {},
	}
	renderDataPattern = regexp.MustCompile(`window\.__renderData\s*=\s*("(?:\\.|[^"\\])*")`)
)

type SantaIsabel struct {
	fetcher     Fetcher
	categoryDir string
	maxPages    int
	logger      *slog.Logger
}

func NewSantaIsabel(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *SantaIsabel {
	if maxPages <= 0 {
		maxPages = DefaultMaxPages
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &SantaIsabel{
		fetcher:     fetcher,
		categoryDir: categoryDir,
		maxPages:    maxPages,
		logger:      logger,
	}
}

func (s *SantaIsabel) Name() string {
	return SantaIsabelName
}

func (s *SantaIsabel) Run(ctx context.Context, job domain.Job, emit Emitter) error {
	if job.AddURL != "" {
		if err := s.appendCategoryURL(job.AddURL); err != nil {
			return err
		}
	}
	urls, err := s.startURLs()
	if err != nil {
		return err
	}
	for _, categoryURL := range urls {
		if err := s.runCategory(ctx, categoryURL, emit); err != nil {
			return err
		}
	}
	return nil
}

func (s *SantaIsabel) runCategory(ctx context.Context, categoryURL string, emit Emitter) error {
	category := categoryFromURL(categoryURL)
	for page := 1; page <= s.maxPages; page++ {
		pageURL := categoryURL
		if page > 1 {
			pageURL = withPage(categoryURL, page)
		}
		response, err := s.fetcher.Fetch(ctx, pageURL)
		if err != nil {
			if errors.Is(err, crawler.ErrRobotsDisallowed) {
				s.logger.Warn("santa_isabel_category_robots_disallowed", "url", pageURL)
				return nil
			}
			var statusErr *crawler.HTTPStatusError
			if errors.As(err, &statusErr) && statusErr.Status == 404 {
				s.logger.Info("santa_isabel_pagination_end", "url", pageURL, "page", page)
				return nil
			}
			return err
		}
		if response.Status == 404 {
			s.logger.Info("santa_isabel_pagination_end", "url", pageURL, "page", page)
			return nil
		}

		products := extractSantaIsabelProducts(response.Body, response.URL)
		if len(products) == 0 {
			s.logger.Warn("santa_isabel_category_without_products", "url", pageURL, "page", page)
			return missingCategorySelectors(pageURL, response.Status, page)
		}
		for _, product := range products {
			item := toSantaIsabelItem(product, category, urlString(response.URL))
			if err := emit(item); err != nil {
				return err
			}
		}
		if page == s.maxPages {
			s.logger.Warn("santa_isabel_pagination_limit", "url", categoryURL, "max_pages", s.maxPages)
		}
	}
	return nil
}

func (s *SantaIsabel) startURLs() ([]string, error) {
	urls := readCategoryURLs(filepath.Join(s.categoryDir, "santa_isabel_categories.txt"))
	urls = append(urls, envCategoryURLs("SANTA_ISABEL_CATEGORY_URLS")...)
	urls = dedupeStrings(urls)
	if len(urls) == 0 {
		return append([]string(nil), santaIsabelDefaultCategories...), nil
	}
	for _, rawURL := range urls {
		if err := validateSantaIsabelURL(rawURL); err != nil {
			return nil, err
		}
	}
	return urls, nil
}

func (s *SantaIsabel) appendCategoryURL(rawURL string) error {
	return appendCategoryURL(
		filepath.Join(s.categoryDir, "santa_isabel_categories.txt"),
		rawURL,
		validateSantaIsabelURL,
	)
}

func validateSantaIsabelURL(rawURL string) error {
	return validateHTTPSHost(
		rawURL,
		santaIsabelAllowedHosts,
		"La categoría debe ser una URL HTTPS pública de santaisabel.cl",
	)
}

func extractSantaIsabelProducts(body []byte, baseURL *url.URL) []domain.Product {
	// El sitio dejó de publicar window.__renderData en 2026. Se mantiene el
	// parser anterior para rollback y se usa JSON-LD (ItemList) como fallback.
	if products := extractSantaIsabelRenderData(body, baseURL); len(products) > 0 {
		return products
	}
	return extractSantaIsabelJSONLD(body, baseURL)
}

func extractSantaIsabelRenderData(body []byte, baseURL *url.URL) []domain.Product {
	match := renderDataPattern.FindSubmatch(body)
	if len(match) < 2 {
		return nil
	}
	var renderData string
	if err := json.Unmarshal(match[1], &renderData); err != nil {
		return nil
	}

	var payload map[string]any
	decoder := json.NewDecoder(strings.NewReader(renderData))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil
	}
	plp, _ := payload["plp"].(map[string]any)
	plpProducts, _ := plp["plp_products"].(map[string]any)
	rawProducts, _ := plpProducts["products"].([]any)

	var products []domain.Product
	for _, rawProduct := range rawProducts {
		product, ok := rawProduct.(map[string]any)
		if !ok {
			continue
		}
		name := normalize.Texto(product["productName"])
		item := firstMap(product["items"])
		if name == nil || item == nil {
			continue
		}
		seller := santaIsabelSeller(item["sellers"])
		if seller == nil {
			continue
		}

		offer, _ := seller["commertialOffer"].(map[string]any)
		if offer == nil {
			offer = map[string]any{}
		}
		price := normalize.PrecioCLP(offer["Price"])
		if price == nil {
			continue
		}
		normalPrice := firstNonZeroPrice(offer, "ListPrice", "PriceWithoutDiscount")
		if normalPrice == nil {
			normalPrice = price
		}
		var offerPrice *float64
		if *price < *normalPrice {
			offerPrice = price
		}

		var inStock *bool
		if quantity := numberValue(offer["AvailableQuantity"]); quantity != nil {
			value := *quantity > 0
			inStock = &value
		}

		var image *string
		images, _ := item["images"].([]any)
		if len(images) > 0 {
			if first, ok := images[0].(map[string]any); ok {
				if imageURL, ok := first["imageUrl"].(string); ok {
					image = &imageURL
				}
			}
		}

		slug, _ := product["linkText"].(string)
		reference := ""
		if slug != "" {
			reference = "/" + slug + "/p"
		}
		productURL := resolveURL(baseURL, reference)

		var promotionNames []string
		teasers, _ := offer["teasers"].([]any)
		for _, rawTeaser := range teasers {
			teaser, ok := rawTeaser.(map[string]any)
			if !ok {
				continue
			}
			if promotion, ok := teaser["name"].(string); ok && promotion != "" {
				promotionNames = append(promotionNames, promotion)
			}
		}

		format := joinValueStrings(item["unitMultiplier"], item["measurementUnit"])

		var mechanics *string
		if len(promotionNames) > 0 {
			value := strings.Join(promotionNames, ", ")
			mechanics = &value
		}

		products = append(products, domain.Product{
			Producto:          *name,
			Precio:            price,
			PrecioNormal:      normalPrice,
			PrecioOferta:      offerPrice,
			EanGtin:           normalize.EAN(item["ean"]),
			SKU:               firstTruthyString(item["itemId"], product["productId"]),
			Marca:             normalize.Texto(product["brand"]),
			FormatoCrudo:      format,
			MecanicaPromocion: mechanics,
			EnStock:           inStock,
			URLProducto:       &productURL,
			Imagen:            image,
		})
	}
	return products
}

func extractSantaIsabelJSONLD(body []byte, baseURL *url.URL) []domain.Product {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}

	var products []domain.Product
	seen := make(map[string]struct{})
	document.Find(`script[type="application/ld+json"]`).Each(func(_ int, selection *goquery.Selection) {
		var data any
		decoder := json.NewDecoder(strings.NewReader(selection.Text()))
		decoder.UseNumber()
		if err := decoder.Decode(&data); err != nil {
			return
		}
		for _, value := range walkJSON(data) {
			entry, ok := value.(map[string]any)
			if !ok {
				continue
			}
			typeValue, _ := entry["@type"].(string)
			if typeValue != "Product" {
				continue
			}
			name := normalize.Texto(entry["name"])
			if name == nil {
				continue
			}

			offer := schemaOffer(entry)
			price := firstPrice(offer, "price")
			if price == nil {
				price = firstPrice(entry, "price")
			}
			if price == nil {
				continue
			}

			normalPrice := firstPrice(offer, "listPrice", "priceWithoutDiscount")
			var offerPrice *float64
			if normalPrice != nil && *normalPrice > *price {
				offerPrice = price
			} else {
				normalPrice = price
			}

			rawURL, _ := entry["url"].(string)
			var productURL *string
			if rawURL != "" {
				resolved := resolveURL(baseURL, rawURL)
				productURL = &resolved
			}
			key := strings.ToLower(*name)
			if productURL != nil {
				key += "|" + *productURL
			}
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}

			var image *string
			switch value := entry["image"].(type) {
			case string:
				image = &value
			case []any:
				if len(value) > 0 {
					if first, ok := value[0].(string); ok {
						image = &first
					}
				}
			}
			var brand any = entry["brand"]
			if brandMap, ok := brand.(map[string]any); ok {
				brand = brandMap["name"]
			}
			sku := firstTruthyString(entry["sku"], entry["productID"], entry["productId"], entry["mpn"])
			if sku == nil {
				sku = normalize.SKUFromURL(rawURL)
			}

			products = append(products, domain.Product{
				Producto:          *name,
				Precio:            price,
				PrecioNormal:      normalPrice,
				PrecioOferta:      offerPrice,
				EanGtin:           normalize.EAN(firstValue(entry, "gtin", "gtin8", "gtin12", "gtin13", "ean")),
				SKU:               sku,
				Marca:             normalize.Texto(brand),
				EnStock:           stockValue(offer["availability"]),
				URLProducto:       productURL,
				Imagen:            image,
				MecanicaPromocion: normalize.Texto(firstValue(offer, "promotion", "promotionMechanic", "description")),
			})
		}
	})
	return products
}

func santaIsabelSeller(value any) map[string]any {
	sellers, _ := value.([]any)
	for _, rawSeller := range sellers {
		seller, ok := rawSeller.(map[string]any)
		if !ok {
			continue
		}
		name, _ := seller["sellerName"].(string)
		if strings.ToLower(name) == "santaisabel" {
			return seller
		}
	}
	return nil
}

func firstMap(value any) map[string]any {
	values, _ := value.([]any)
	for _, raw := range values {
		if mapped, ok := raw.(map[string]any); ok {
			return mapped
		}
	}
	return nil
}

func firstNonZeroPrice(data map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		if price := normalize.PrecioCLP(data[key]); price != nil && *price != 0 {
			return price
		}
	}
	return nil
}

func firstTruthyString(values ...any) *string {
	for _, value := range values {
		if !isTruthy(value) {
			continue
		}
		text := valueString(value)
		if text == "" {
			continue
		}
		return &text
	}
	return nil
}

func isTruthy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return typed != ""
	case bool:
		return typed
	case json.Number:
		number, err := typed.Float64()
		return err == nil && number != 0
	case float64:
		return typed != 0
	case float32:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	default:
		return true
	}
}

func numberValue(value any) *float64 {
	var number float64
	switch typed := value.(type) {
	case nil:
		return nil
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return nil
		}
		number = parsed
	case float64:
		number = typed
	case float32:
		number = float64(typed)
	case int:
		number = float64(typed)
	case int64:
		number = float64(typed)
	case bool:
		if typed {
			number = 1
		}
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return nil
		}
		number = parsed
	default:
		return nil
	}
	return &number
}

func joinValueStrings(values ...any) *string {
	var parts []string
	for _, value := range values {
		if value != nil {
			parts = append(parts, valueString(value))
		}
	}
	if len(parts) == 0 {
		return nil
	}
	text := strings.Join(parts, " ")
	if text == "" {
		return nil
	}
	return &text
}

func toSantaIsabelItem(product domain.Product, category, pageURL string) domain.Item {
	if product.URLProducto == nil || *product.URLProducto == "" {
		product.URLProducto = &pageURL
	}
	product.Categoria = category
	return domain.Item{Product: product, Supermercado: "Santa Isabel"}
}

func urlString(value *url.URL) string {
	if value == nil {
		return ""
	}
	return value.String()
}
