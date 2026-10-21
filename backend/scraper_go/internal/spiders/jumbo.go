package spiders

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

var jumboDefaultCategories = []string{
	"https://www.jumbo.cl/frutas-y-verduras/verduras",
}

var jumboAllowedHosts = map[string]struct{}{
	"jumbo.cl":     {},
	"www.jumbo.cl": {},
}

type Jumbo struct {
	fetcher     Fetcher
	categoryDir string
	maxPages    int
	logger      *slog.Logger
}

func NewJumbo(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Jumbo {
	if maxPages <= 0 {
		maxPages = DefaultMaxPages
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Jumbo{
		fetcher:     fetcher,
		categoryDir: categoryDir,
		maxPages:    maxPages,
		logger:      logger,
	}
}

func (j *Jumbo) Name() string {
	return JumboName
}

func (j *Jumbo) Run(ctx context.Context, job domain.Job, emit Emitter) error {
	if job.ProductURL != "" {
		return j.runProduct(ctx, job, emit)
	}
	if job.AddURL != "" {
		if err := j.appendCategoryURL(job.AddURL); err != nil {
			return err
		}
	}
	for _, categoryURL := range j.startURLs() {
		if err := j.runCategory(ctx, categoryURL, job.EnrichEAN, emit); err != nil {
			return err
		}
	}
	return nil
}

func (j *Jumbo) runProduct(ctx context.Context, job domain.Job, emit Emitter) error {
	if err := validateProductURL(job.ProductURL); err != nil {
		return err
	}
	response, err := j.fetcher.Fetch(ctx, job.ProductURL)
	if err != nil {
		return err
	}
	if response.Status == 404 {
		j.logger.Warn("jumbo_product_not_found", "url", job.ProductURL)
		return nil
	}
	products := extractJumboProducts(response.Body)
	if len(products) == 0 {
		j.logger.Warn("jumbo_product_without_data", "url", job.ProductURL)
		return nil
	}

	requested := canonicalProductURL(job.ProductURL)
	var selected *domain.Product
	for i := range products {
		if products[i].URLProducto == nil {
			continue
		}
		candidate := resolveURL(response.URL, *products[i].URLProducto)
		if canonicalProductURL(candidate) == requested {
			selected = &products[i]
			break
		}
	}
	if selected == nil {
		if len(products) == 1 {
			selected = &products[0]
		} else {
			j.logger.Error("jumbo_product_ambiguous", "url", job.ProductURL)
			return nil
		}
	}
	return emit(toJumboItem(*selected, "producto", job.ProductURL))
}

func (j *Jumbo) runCategory(ctx context.Context, categoryURL string, enrichEAN bool, emit Emitter) error {
	category := categoryFromURL(categoryURL)
	for page := 1; page <= j.maxPages; page++ {
		pageURL := categoryURL
		if page > 1 {
			pageURL = withPage(categoryURL, page)
		}
		response, err := j.fetcher.Fetch(ctx, pageURL)
		if err != nil {
			if errors.Is(err, crawler.ErrRobotsDisallowed) {
				j.logger.Warn("jumbo_category_robots_disallowed", "url", pageURL)
				return nil
			}
			var statusErr *crawler.HTTPStatusError
			if errors.As(err, &statusErr) && statusErr.Status == 404 {
				j.logger.Info("jumbo_pagination_end", "url", pageURL, "page", page)
				return nil
			}
			return err
		}
		if response.Status == 404 {
			j.logger.Info("jumbo_pagination_end", "url", pageURL, "page", page)
			return nil
		}

		products := extractJumboProducts(response.Body)
		if len(products) == 0 {
			j.logger.Warn("jumbo_category_without_products", "url", pageURL, "page", page)
			return nil
		}
		for _, product := range products {
			item, err := j.materialize(ctx, product, category, response.URL, enrichEAN)
			if err != nil {
				return err
			}
			if err := emit(item); err != nil {
				return err
			}
		}
		if page == j.maxPages {
			j.logger.Warn("jumbo_pagination_limit", "url", categoryURL, "max_pages", j.maxPages)
		}
	}
	return nil
}

func (j *Jumbo) materialize(
	ctx context.Context,
	product domain.Product,
	category string,
	responseURL *url.URL,
	enrichEAN bool,
) (domain.Item, error) {
	pageURL := ""
	if responseURL != nil {
		pageURL = responseURL.String()
	}
	if !enrichEAN || product.EanGtin != nil || product.URLProducto == nil || *product.URLProducto == "" {
		return toJumboItem(product, category, pageURL), nil
	}

	detailURL := resolveURL(responseURL, *product.URLProducto)
	if err := validateProductURL(detailURL); err != nil {
		j.logger.Warn("jumbo_ean_url_invalid", "url", detailURL, "error", err)
		return toJumboItem(product, category, pageURL), nil
	}
	response, err := j.fetcher.Fetch(ctx, detailURL)
	if err != nil {
		j.logger.Warn("jumbo_ean_fetch_failed", "url", detailURL, "error", err)
		return toJumboItem(product, category, pageURL), nil
	}
	if response.Status == 404 {
		return toJumboItem(product, category, pageURL), nil
	}

	detailProducts := extractJumboProducts(response.Body)
	requested := canonicalProductURL(detailURL)
	var detail *domain.Product
	for i := range detailProducts {
		if detailProducts[i].URLProducto == nil {
			continue
		}
		candidate := resolveURL(response.URL, *detailProducts[i].URLProducto)
		if canonicalProductURL(candidate) == requested {
			detail = &detailProducts[i]
			break
		}
	}
	if detail == nil && len(detailProducts) == 1 {
		detail = &detailProducts[0]
	}
	enriched := product
	if detail != nil {
		if detail.EanGtin != nil && *detail.EanGtin != "" {
			enriched.EanGtin = detail.EanGtin
		}
		if detail.SKU != nil && *detail.SKU != "" {
			enriched.SKU = detail.SKU
		}
	}
	if enriched.EanGtin == nil {
		j.logger.Info("jumbo_ean_missing", "url", detailURL)
	}
	return toJumboItem(enriched, category, pageURL), nil
}

func toJumboItem(product domain.Product, category, responseURL string) domain.Item {
	if product.URLProducto == nil || *product.URLProducto == "" {
		product.URLProducto = domain.StringPtr(responseURL)
	}
	product.Categoria = category
	return domain.Item{Product: product, Supermercado: "Jumbo"}
}

func (j *Jumbo) startURLs() []string {
	urls := readCategoryURLs(filepath.Join(j.categoryDir, "jumbo_categories.txt"))
	for _, raw := range strings.Split(os.Getenv("JUMBO_CATEGORY_URLS"), ",") {
		if value := strings.TrimSpace(raw); value != "" {
			urls = append(urls, value)
		}
	}
	urls = dedupeStrings(urls)
	if len(urls) == 0 {
		return append([]string(nil), jumboDefaultCategories...)
	}
	return urls
}

func (j *Jumbo) appendCategoryURL(rawURL string) error {
	if err := validateCategoryURL(rawURL); err != nil {
		return err
	}
	file := filepath.Join(j.categoryDir, "jumbo_categories.txt")
	for _, existing := range readCategoryURLs(file) {
		if existing == rawURL {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	handle, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer handle.Close()
	_, err = handle.WriteString(rawURL + "\n")
	return err
}

func readCategoryURLs(file string) []string {
	body, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var urls []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		urls = append(urls, line)
	}
	return urls
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func validateProductURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("product_url debe ser HTTPS")
	}
	if _, ok := jumboAllowedHosts[strings.ToLower(parsed.Hostname())]; !ok {
		return fmt.Errorf("product_url debe ser una URL pública de jumbo.cl")
	}
	return nil
}

func validateCategoryURL(rawURL string) error {
	return validateProductURL(rawURL)
}

func canonicalProductURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname()) + "|" + strings.TrimRight(parsed.Path, "/")
}

func categoryFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.TrimRight(parsed.Path, "/"), "/")
	for i := len(segments) - 1; i >= 0; i-- {
		if segments[i] != "" {
			return segments[i]
		}
	}
	return ""
}

func withPage(rawURL string, page int) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	query.Set("page", strconv.Itoa(page))
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func resolveURL(base *url.URL, reference string) string {
	parsed, err := url.Parse(reference)
	if err != nil {
		return reference
	}
	if base == nil {
		return reference
	}
	return base.ResolveReference(parsed).String()
}

func extractJumboProducts(body []byte) []domain.Product {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}

	var products []domain.Product
	seen := make(map[string]struct{})
	document.Find(`script[type="application/ld+json"]`).Each(func(_ int, selection *goquery.Selection) {
		var data any
		decoder := json.NewDecoder(strings.NewReader(selection.Text()))
		// UseNumber conserva la diferencia entre 1290 y 1290.0, igual que
		// json.loads en Python. Importa para no inventar EAN/SKU.
		decoder.UseNumber()
		if err := decoder.Decode(&data); err != nil {
			return
		}
		for _, value := range walkJSON(data) {
			entry, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if typeValue, _ := entry["@type"].(string); typeValue != "Product" {
				continue
			}
			nameValue, ok := entry["name"].(string)
			if !ok {
				continue
			}
			name := normalize.Texto(nameValue)
			if name == nil {
				continue
			}
			if _, duplicate := seen[*name]; duplicate {
				continue
			}
			seen[*name] = struct{}{}

			offer := jumboOffer(entry)
			normalPrice := extractNormalPrice(entry)
			offerPrice := firstPrice(offer, "price")
			if offerPrice == nil {
				offerPrice = normalPrice
			}
			if normalPrice == nil {
				normalPrice = offerPrice
			}

			var brand any = entry["brand"]
			if brandMap, ok := brand.(map[string]any); ok {
				brand = brandMap["name"]
			}
			availability := offer["availability"]

			var image any = entry["image"]
			if imageList, ok := image.([]any); ok && len(imageList) > 0 {
				image = imageList[0]
			}

			var productURL *string
			if rawURL, ok := entry["url"].(string); ok {
				productURL = &rawURL
			}
			skuValue := firstValue(entry, "sku", "productID", "productId")
			var sku *string
			if skuValue != nil {
				value := valueString(skuValue)
				sku = &value
			} else if productURL != nil {
				sku = skuFromURL(*productURL)
			}

			var imagePtr *string
			if value, ok := image.(string); ok {
				imagePtr = &value
			}

			products = append(products, domain.Product{
				Producto:          *name,
				Precio:            offerPrice,
				PrecioNormal:      normalPrice,
				PrecioOferta:      offerPrice,
				EanGtin:           normalize.EAN(firstValue(entry, "gtin", "gtin8", "gtin12", "gtin13", "ean")),
				SKU:               sku,
				Marca:             normalize.Texto(brand),
				FormatoCrudo:      normalize.Texto(firstValue(entry, "format", "size", "description")),
				MecanicaPromocion: normalize.Texto(firstValue(offer, "promotion", "promotionMechanic", "offerDescription")),
				EnStock:           stockValue(availability),
				URLProducto:       productURL,
				Imagen:            imagePtr,
			})
		}
	})
	return products
}

func jumboOffer(entry map[string]any) map[string]any {
	switch raw := entry["offers"].(type) {
	case map[string]any:
		return raw
	case []any:
		if len(raw) > 0 {
			if offer, ok := raw[0].(map[string]any); ok {
				return offer
			}
		}
	}
	return map[string]any{}
}

func extractNormalPrice(entry map[string]any) *float64 {
	if price := firstPrice(entry, "price"); price != nil {
		return price
	}
	specification := entry["priceSpecification"]
	if list, ok := specification.([]any); ok && len(list) > 0 {
		specification = list[0]
	}
	return firstPrice(specification, "price")
}

func firstValue(data any, keys ...string) any {
	values, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	for _, key := range keys {
		value, found := values[key]
		if !found || value == nil {
			continue
		}
		switch value.(type) {
		case map[string]any, []any:
			continue
		}
		// Python convierte el escalar a texto antes de normalizarlo.
		return valueString(value)
	}
	return nil
}

func firstPrice(data any, keys ...string) *float64 {
	return normalize.PrecioCLP(firstValue(data, keys...))
}

func valueString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		text := strconv.FormatFloat(typed, 'g', -1, 64)
		if !strings.ContainsAny(text, ".eE") {
			text += ".0"
		}
		return text
	case float32:
		text := strconv.FormatFloat(float64(typed), 'g', -1, 32)
		if !strings.ContainsAny(text, ".eE") {
			text += ".0"
		}
		return text
	case bool:
		if typed {
			return "True"
		}
		return "False"
	default:
		return fmt.Sprint(value)
	}
}

func stockValue(availability any) *bool {
	text, ok := availability.(string)
	if !ok {
		return nil
	}
	parts := strings.Split(text, "/")
	inStock := strings.ToLower(parts[len(parts)-1]) != "outofstock"
	return &inStock
}

func skuFromURL(rawURL string) *string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	var segments []string
	for _, segment := range strings.Split(parsed.Path, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	if len(segments) > 0 && strings.ToLower(segments[len(segments)-1]) == "p" {
		segments = segments[:len(segments)-1]
	}
	if len(segments) == 0 {
		return nil
	}
	slug := segments[len(segments)-1]
	sku := "JUMBO-" + slug
	if len([]rune(sku)) <= 100 {
		return &sku
	}
	sum := sha1.Sum([]byte(rawURL))
	suffix := hex.EncodeToString(sum[:])[:12]
	runes := []rune(slug)
	if len(runes) > 80 {
		runes = runes[:80]
	}
	truncated := "JUMBO-" + string(runes) + "-" + suffix
	return &truncated
}

func walkJSON(value any) []any {
	var result []any
	var walk func(any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			result = append(result, typed)
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return result
}
