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
	fetcher     Fetcher
	categoryDir string
	maxPages    int
	logger      *slog.Logger
}

func NewLider(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Lider {
	if maxPages <= 0 {
		maxPages = DefaultMaxPages
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Lider{
		fetcher:     fetcher,
		categoryDir: categoryDir,
		maxPages:    maxPages,
		logger:      logger,
	}
}

func (l *Lider) Name() string {
	return LiderName
}

func (l *Lider) Run(ctx context.Context, job domain.Job, emit Emitter) error {
	if job.AddURL != "" {
		if err := l.appendCategoryURL(job.AddURL); err != nil {
			return err
		}
	}
	urls, err := l.startURLs()
	if err != nil {
		return err
	}
	for _, categoryURL := range urls {
		if err := l.runCategory(ctx, categoryURL, emit); err != nil {
			return err
		}
	}
	return nil
}

func (l *Lider) runCategory(ctx context.Context, categoryURL string, emit Emitter) error {
	category := liderCategoryFromURL(categoryURL)
	startPage := liderPageNumber(categoryURL)
	for page := startPage; page <= l.maxPages; page++ {
		pageURL := liderPageURL(categoryURL, page)
		response, err := l.fetcher.Fetch(ctx, pageURL)
		if err != nil {
			if errors.Is(err, crawler.ErrRobotsDisallowed) {
				l.logger.Warn("lider_category_robots_disallowed", "url", pageURL)
				return nil
			}
			var statusErr *crawler.HTTPStatusError
			if errors.As(err, &statusErr) && statusErr.Status == 404 {
				l.logger.Info("lider_pagination_end", "url", pageURL, "page", page)
				return nil
			}
			return err
		}
		if response.Status == 404 {
			l.logger.Info("lider_pagination_end", "url", pageURL, "page", page)
			return nil
		}

		products := extractLiderProducts(response.Body, response.URL)
		if len(products) == 0 {
			l.logger.Warn("lider_category_without_products", "url", pageURL, "page", page)
			return missingCategorySelectors(pageURL, response.Status, page)
		}
		for _, product := range products {
			item := toLiderItem(product, category, urlString(response.URL))
			if err := emit(item); err != nil {
				return err
			}
		}
		if page == l.maxPages {
			l.logger.Warn("lider_pagination_limit", "url", categoryURL, "max_pages", l.maxPages)
		}
	}
	return nil
}

func (l *Lider) startURLs() ([]string, error) {
	urls := readCategoryURLs(filepath.Join(l.categoryDir, "lider_categories.txt"))
	urls = append(urls, envCategoryURLs("LIDER_CATEGORY_URLS")...)
	urls = dedupeStrings(urls)
	if len(urls) == 0 {
		return append([]string(nil), liderDefaultCategories...), nil
	}
	for _, rawURL := range urls {
		if err := validateLiderCategoryURL(rawURL); err != nil {
			return nil, err
		}
	}
	return urls, nil
}

func (l *Lider) appendCategoryURL(rawURL string) error {
	return appendCategoryURL(
		filepath.Join(l.categoryDir, "lider_categories.txt"),
		rawURL,
		validateLiderCategoryURL,
	)
}

func validateLiderCategoryURL(rawURL string) error {
	if err := validateHTTPSHost(
		rawURL,
		liderAllowedHosts,
		"La categoría debe ser una URL HTTPS /browse/ de super.lider.cl",
	); err != nil {
		return err
	}
	parsed, _ := url.Parse(rawURL)
	if !strings.HasPrefix(parsed.Path, "/browse/") {
		return errors.New("La categoría debe ser una URL HTTPS /browse/ de super.lider.cl")
	}
	return nil
}

func validateLiderProductURL(rawURL string) error {
	if err := validateHTTPSHost(
		rawURL,
		liderAllowedHosts,
		"product_url debe ser una URL HTTPS /ip/ de super.lider.cl",
	); err != nil {
		return err
	}
	parsed, _ := url.Parse(rawURL)
	if !strings.HasPrefix(parsed.Path, "/ip/") {
		return errors.New("product_url debe ser una URL HTTPS /ip/ de super.lider.cl")
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
			name := normalize.Texto(item["name"])
			productCode := normalize.EAN(item["usItemId"])
			currentPrice := normalize.PrecioCLP(item["price"])
			if name == nil || productCode == nil || currentPrice == nil {
				continue
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
				continue
			}
			productURL := resolveURL(responseURL, canonical)
			if err := validateLiderProductURL(productURL); err != nil {
				continue
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
			products = append(products, domain.Product{
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
			})
		}
	}
	return products, true
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

func toLiderItem(product domain.Product, category, pageURL string) domain.Item {
	if product.URLProducto == nil || *product.URLProducto == "" {
		product.URLProducto = &pageURL
	}
	product.Categoria = category
	return domain.Item{Product: product, Supermercado: "Lider"}
}
