package spiders

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
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
	fetcher     Fetcher
	categoryDir string
	maxPages    int
	logger      *slog.Logger
}

func NewCugat(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Cugat {
	if maxPages <= 0 {
		maxPages = DefaultMaxPages
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Cugat{
		fetcher:     fetcher,
		categoryDir: categoryDir,
		maxPages:    maxPages,
		logger:      logger,
	}
}

func (c *Cugat) Name() string {
	return CugatName
}

func (c *Cugat) Run(ctx context.Context, job domain.Job, emit Emitter) error {
	if job.AddURL != "" {
		if err := c.appendCategoryURL(job.AddURL); err != nil {
			return err
		}
	}
	if job.ProductURL != "" {
		if err := validateCugatProductURL(job.ProductURL); err != nil {
			return err
		}
		response, err := c.fetcher.Fetch(ctx, job.ProductURL)
		if err != nil {
			return err
		}
		if response.Status == 404 {
			c.logger.Info("cugat_product_not_found", "url", job.ProductURL)
			return nil
		}
		product := extractCugatDetail(response.Body, response.URL)
		if product == nil {
			c.logger.Warn("cugat_product_without_jsonld", "url", job.ProductURL)
			return nil
		}
		return emit(toCugatItem(*product, "producto", urlString(response.URL)))
	}

	urls, err := c.startURLs()
	if err != nil {
		return err
	}
	for _, categoryURL := range urls {
		if err := c.runCategory(ctx, categoryURL, emit); err != nil {
			return err
		}
	}
	return nil
}

func (c *Cugat) runCategory(ctx context.Context, categoryURL string, emit Emitter) error {
	category := cugatCategoryFromURL(categoryURL)
	for page := 1; page <= c.maxPages; page++ {
		pageURL := categoryURL
		if page > 1 {
			pageURL = cugatPageURL(categoryURL, page)
		}
		response, err := c.fetcher.Fetch(ctx, pageURL)
		if err != nil {
			if errors.Is(err, crawler.ErrRobotsDisallowed) {
				c.logger.Warn("cugat_category_robots_disallowed", "url", pageURL)
				return nil
			}
			var statusErr *crawler.HTTPStatusError
			if errors.As(err, &statusErr) && statusErr.Status == 404 {
				c.logger.Info("cugat_pagination_end", "url", pageURL, "page", page)
				return nil
			}
			return err
		}
		if response.Status == 404 {
			c.logger.Info("cugat_pagination_end", "url", pageURL, "page", page)
			return nil
		}

		products := extractCugatProducts(response.Body, response.URL)
		if len(products) == 0 {
			c.logger.Warn("cugat_category_without_products", "url", pageURL, "page", page)
			return missingCategorySelectors(pageURL, response.Status, page)
		}
		for _, product := range products {
			item := toCugatItem(product, category, urlString(response.URL))
			if err := emit(item); err != nil {
				return err
			}
		}
	}
	c.logger.Warn("cugat_pagination_limit", "max_pages", c.maxPages)
	return nil
}

func (c *Cugat) startURLs() ([]string, error) {
	urls := readCategoryURLs(filepath.Join(c.categoryDir, "cugat_categories.txt"))
	urls = append(urls, envCategoryURLs("CUGAT_CATEGORY_URLS")...)
	urls = dedupeStrings(urls)
	if len(urls) == 0 {
		return append([]string(nil), cugatDefaultCategories...), nil
	}
	for _, rawURL := range urls {
		if err := validateCugatCategoryURL(rawURL); err != nil {
			return nil, err
		}
	}
	return urls, nil
}

func (c *Cugat) appendCategoryURL(rawURL string) error {
	return appendCategoryURL(
		filepath.Join(c.categoryDir, "cugat_categories.txt"),
		rawURL,
		validateCugatCategoryURL,
	)
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
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	scripts := document.Find(`script[type="application/ld+json"]`)
	for index := 0; index < scripts.Length(); index++ {
		var data any
		decoder := json.NewDecoder(strings.NewReader(scripts.Eq(index).Text()))
		decoder.UseNumber()
		if err := decoder.Decode(&data); err != nil {
			continue
		}
		for _, entry := range walkJSON(data) {
			object, ok := entry.(map[string]any)
			if !ok || !isProductType(object["@type"]) {
				continue
			}
			offers := firstMapValue(object["offers"])
			price := normalize.PrecioCLP(firstValue(offers, "price"))
			image := cugatImageURL(object["image"])
			brand := object["brand"]
			if brandMap, ok := brand.(map[string]any); ok {
				brand = brandMap["name"]
			}
			eanValue := firstValue(object, "gtin13")
			if eanValue == nil || eanValue == "" {
				eanValue = firstValue(object, "gtin")
			}
			productURL := urlString(responseURL)
			if value, ok := object["url"].(string); ok && value != "" {
				productURL = value
			}
			product := domain.Product{
				Precio:       price,
				PrecioNormal: price,
				EanGtin:      normalize.EAN(eanValue),
				SKU:          normalize.Texto(firstValue(object, "sku")),
				Marca:        normalize.Texto(brand),
				FormatoCrudo: normalize.Texto(firstValue(object, "description")),
				EnStock:      stockValue(firstValue(offers, "availability")),
				URLProducto:  &productURL,
				Imagen:       image,
			}
			if name := normalize.Texto(firstValue(object, "name")); name != nil {
				product.Producto = *name
			}
			return &product
		}
	}
	return nil
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

func toCugatItem(product domain.Product, category, pageURL string) domain.Item {
	if product.URLProducto == nil || *product.URLProducto == "" {
		product.URLProducto = &pageURL
	}
	product.Categoria = category
	return domain.Item{Product: product, Supermercado: "Cugat"}
}

func nonEmptySegments(path string) []string {
	var segments []string
	for _, segment := range strings.Split(path, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}

func mustParsePath(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Path
}

func hasClass(classes, wanted string) bool {
	for _, class := range strings.Fields(classes) {
		if class == wanted {
			return true
		}
	}
	return false
}
