package spiders

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

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
	baseSpider
}

func NewJumbo(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Jumbo {
	spider := &Jumbo{baseSpider: newBaseSpider(fetcher, categoryDir, maxPages, logger, spiderConfig{
		name:              JumboName,
		supermarket:       "Jumbo",
		categoryFile:      "jumbo_categories.txt",
		categoryEnv:       "JUMBO_CATEGORY_URLS",
		defaultCategories: jumboDefaultCategories,
		validateCategory:  validateCategoryURL,
		validateProduct:   validateProductURL,
		extractProducts: func(body []byte, _ *url.URL) []domain.Product {
			return extractJumboProducts(body)
		},
		extractProduct: func(body []byte, responseURL *url.URL, requested string) *domain.Product {
			return selectJumboProduct(extractJumboProducts(body), requested, responseURL)
		},
		pageURL: pageURLOrSelf,
	})}
	spider.baseSpider.materialize = spider.enrich
	return spider
}

// enrich consulta la ficha del producto cuando el listado no publica EAN/GTIN.
func (j *Jumbo) enrich(
	ctx context.Context,
	product domain.Product,
	category string,
	responseURL *url.URL,
	enrichEAN bool,
) (domain.Item, error) {
	pageURL := urlString(responseURL)
	if !enrichEAN || product.EanGtin != nil || product.URLProducto == nil || *product.URLProducto == "" {
		return j.toItem(product, category, pageURL), nil
	}

	detailURL := resolveURL(responseURL, *product.URLProducto)
	if err := validateProductURL(detailURL); err != nil {
		j.logger.Warn("jumbo_ean_url_invalid", "url", detailURL, "error", err)
		return j.toItem(product, category, pageURL), nil
	}
	response, err := j.fetcher.Fetch(ctx, detailURL)
	if err != nil {
		j.logger.Warn("jumbo_ean_fetch_failed", "url", detailURL, "error", err)
		return j.toItem(product, category, pageURL), nil
	}
	if response.Status == 404 {
		return j.toItem(product, category, pageURL), nil
	}

	enriched := product
	if detail := selectJumboProduct(extractJumboProducts(response.Body), detailURL, response.URL); detail != nil {
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
	return j.toItem(enriched, category, pageURL), nil
}

func selectJumboProduct(products []domain.Product, requested string, responseURL *url.URL) *domain.Product {
	wanted := canonicalProductURL(requested)
	for i := range products {
		if products[i].URLProducto == nil {
			continue
		}
		candidate := resolveURL(responseURL, *products[i].URLProducto)
		if canonicalProductURL(candidate) == wanted {
			return &products[i]
		}
	}
	if len(products) == 1 {
		return &products[0]
	}
	return nil
}

var (
	validateProductURL = validateSiteURL(
		jumboAllowedHosts,
		"",
		"",
		"product_url debe ser una URL HTTPS pública de jumbo.cl",
		0,
	)
	validateCategoryURL = validateSiteURL(
		jumboAllowedHosts,
		"",
		"",
		"La categoría debe ser una URL HTTPS pública de jumbo.cl",
		0,
	)
)

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
		// evitar desbordamientos o parsing incorrecto. Importa para no inventar EAN/SKU.
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

			offer := schemaOffer(entry)
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
