package extract

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

// JumboProducts extrae las fichas schema.org/Product publicadas por Jumbo.
func JumboProducts(site catalog.Site, body []byte) []domain.Product {
	selector := site.Selector("json_ld")
	if selector == "" {
		return nil
	}
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}

	var products []domain.Product
	seen := make(map[string]struct{})
	document.Find(selector).Each(func(_ int, selection *goquery.Selection) {
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
				sku = JumboSKUFromURL(site, *productURL)
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

// JumboProduct elige la ficha solicitada dentro de los productos extraídos.
func JumboProduct(site catalog.Site, body []byte, requested string, responseURL *url.URL) *domain.Product {
	return SelectProductByURL(JumboProducts(site, body), requested, responseURL)
}

// JumboSKUFromURL deriva un SKU estable desde la URL cuando la ficha no publica
// uno. Si el resultado excede el límite, lo trunca y agrega un hash corto.
func JumboSKUFromURL(site catalog.Site, rawURL string) *string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	segments := nonEmptySegments(parsed.Path)
	trailing := site.Setting("sku_trailing_segment")
	if len(segments) > 0 && trailing != "" && strings.EqualFold(segments[len(segments)-1], trailing) {
		segments = segments[:len(segments)-1]
	}
	if len(segments) == 0 {
		return nil
	}

	prefix := site.Setting("sku_prefix")
	slug := segments[len(segments)-1]
	sku := prefix + slug
	if len([]rune(sku)) <= site.Limit("sku_max_length") {
		return &sku
	}

	sum := sha1.Sum([]byte(rawURL))
	suffix := hex.EncodeToString(sum[:])[:site.Limit("hash_length")]
	runes := []rune(slug)
	if len(runes) > site.Limit("slug_max_length") {
		runes = runes[:site.Limit("slug_max_length")]
	}
	truncated := prefix + string(runes) + "-" + suffix
	return &truncated
}
