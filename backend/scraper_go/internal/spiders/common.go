package spiders

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

func formatFromText(value string, pattern *regexp.Regexp) *string {
	match := pattern.FindString(value)
	if match == "" {
		return nil
	}
	return normalize.Texto(match)
}

// pythonTitle replica str.title() de Python: cada secuencia de letras empieza
// en mayúscula y el resto se pasa a minúscula. Los dígitos y signos cortan la
// secuencia, por eso "1.5l" queda como "1.5L".
func pythonTitle(value string) string {
	var builder strings.Builder
	previousLetter := false
	for _, character := range value {
		if unicode.IsLetter(character) {
			if previousLetter {
				builder.WriteRune(unicode.ToLower(character))
			} else {
				builder.WriteRune(unicode.ToUpper(character))
			}
			previousLetter = true
			continue
		}
		builder.WriteRune(character)
		previousLetter = false
	}
	return builder.String()
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

func appendCategoryURL(file, rawURL string, validate func(string) error) error {
	if err := validate(rawURL); err != nil {
		return err
	}
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

func envCategoryURLs(name string) []string {
	var urls []string
	for _, raw := range strings.Split(os.Getenv(name), ",") {
		if value := strings.TrimSpace(raw); value != "" {
			urls = append(urls, value)
		}
	}
	return urls
}

// missingCategorySelectors distingue el fin normal de la paginación de una
// respuesta 200 que no contiene los selectores esperados. Sólo la primera
// página de una categoría se considera bloqueo: una página posterior vacía
// es el final natural de la paginación. Así un challenge que no trae ninguno
// de los marcadores conocidos no se confunde con una categoría sin stock.
func missingCategorySelectors(pageURL string, status, page int) error {
	if page != 1 {
		return nil
	}
	return &crawler.BlockedError{
		URL:              pageURL,
		Status:           status,
		Reason:           "missing_selectors",
		BlockedResponses: 1,
	}
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

func validateHTTPSHost(rawURL string, hosts map[string]struct{}, message string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("%s", message)
	}
	if _, ok := hosts[strings.ToLower(parsed.Hostname())]; !ok {
		return fmt.Errorf("%s", message)
	}
	return nil
}

func firstText(selection *goquery.Selection, selector string) string {
	return selection.Find(selector).First().Text()
}

// allText replica el " ".join(text()) de Scrapy: conserva el límite entre
// nodos de texto, que importa para detectar frases como "Available for
// Despacho" aunque el HTML las tenga en elementos separados.
func allText(selection *goquery.Selection) string {
	var parts []string
	for _, node := range selection.Nodes {
		collectText(node, &parts)
	}
	return strings.Join(parts, " ")
}

func collectText(node *html.Node, parts *[]string) {
	if node.Type == html.TextNode {
		*parts = append(*parts, node.Data)
		return
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		collectText(child, parts)
	}
}

func firstAttr(selection *goquery.Selection, selector, attribute string) string {
	value, _ := selection.Find(selector).First().Attr(attribute)
	return value
}

// noscriptImage lee el HTML crudo dentro de <noscript>. goquery puede dejar
// ese contenido como texto cuando el parser tiene scripting habilitado, así
// que se vuelve a parsear para no perder la imagen de respaldo de WooCommerce.
func noscriptImage(selection *goquery.Selection) string {
	for _, node := range selection.Find("noscript").Nodes {
		var raw strings.Builder
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.TextNode {
				raw.WriteString(child.Data)
			}
		}
		text := strings.TrimSpace(raw.String())
		if text == "" {
			continue
		}
		document, err := goquery.NewDocumentFromReader(strings.NewReader(text))
		if err != nil {
			continue
		}
		if source, ok := document.Find("img").First().Attr("src"); ok && source != "" {
			return source
		}
	}
	return ""
}

func firstNonNilString(values ...*string) *string {
	for _, value := range values {
		if value != nil && *value != "" {
			return value
		}
	}
	return nil
}

func nonEmptyStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func lastPathSegment(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.TrimRight(parsed.Path, "/"), "/")
	if len(segments) == 0 {
		return ""
	}
	return segments[len(segments)-1]
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func appendQueryParam(rawURL, key, value string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	pairs := queryPairs(parsed.RawQuery)
	filtered := pairs[:0]
	for _, pair := range pairs {
		name, _, _ := strings.Cut(pair, "=")
		if name == key {
			continue
		}
		filtered = append(filtered, pair)
	}
	filtered = append(filtered, url.QueryEscape(key)+"="+url.QueryEscape(value))
	parsed.RawQuery = strings.Join(filtered, "&")
	return parsed.String()
}

func removeQueryParam(rawURL, key string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	pairs := queryPairs(parsed.RawQuery)
	filtered := pairs[:0]
	for _, pair := range pairs {
		name, _, _ := strings.Cut(pair, "=")
		if name == key {
			continue
		}
		filtered = append(filtered, pair)
	}
	parsed.RawQuery = strings.Join(filtered, "&")
	return parsed.String()
}

func queryPairs(rawQuery string) []string {
	if rawQuery == "" {
		return nil
	}
	return strings.Split(rawQuery, "&")
}

// selectProductByURL elige el producto solicitado dentro de los candidatos que
// devuelve un parser de listado o de detalle. Compara host+path para ignorar
// query strings de tracking. Si no hay coincidencia y el documento sólo trae un
// producto, se acepta ese único candidato (es el caso normal de una ficha).
// Si hay varios y ninguno coincide, devuelve nil: nunca se emite el producto
// equivocado ni se cae al crawl masivo de categorías.
func selectProductByURL(products []domain.Product, requested string, responseURL *url.URL) *domain.Product {
	if len(products) == 0 {
		return nil
	}
	wanted := canonicalProductURL(requested)
	if wanted != "" {
		for index := range products {
			if products[index].URLProducto == nil {
				continue
			}
			candidate := resolveURL(responseURL, *products[index].URLProducto)
			if canonicalProductURL(candidate) == wanted {
				return &products[index]
			}
		}
	}
	if len(products) == 1 {
		return &products[0]
	}
	return nil
}

// extractJSONLDProduct lee la primera ficha schema.org/Product publicada como
// JSON-LD. Es el camino común de las páginas de detalle cuando el sitio no
// expone un objeto propio de producto.
func extractJSONLDProduct(body []byte, responseURL *url.URL) *domain.Product {
	products := extractJSONLDProducts(body, responseURL)
	if len(products) == 0 {
		return nil
	}
	return &products[0]
}

// extractJSONLDProducts devuelve todas las fichas schema.org/Product del
// documento. Una página de detalle puede incluir recomendaciones, por eso el
// modo product_url filtra después por URL en vez de quedarse con la primera.
func extractJSONLDProducts(body []byte, responseURL *url.URL) []domain.Product {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	var products []domain.Product
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
				productURL = resolveURL(responseURL, value)
			}
			product := domain.Product{
				Precio:       price,
				PrecioNormal: price,
				EanGtin:      normalize.EAN(eanValue),
				SKU:          firstNonNilString(normalize.Texto(firstValue(object, "sku")), normalize.SKUFromURL(productURL)),
				Marca:        normalize.Texto(brand),
				FormatoCrudo: normalize.Texto(firstValue(object, "description")),
				EnStock:      stockValue(firstValue(offers, "availability")),
				URLProducto:  &productURL,
				Imagen:       image,
			}
			if name := normalize.Texto(firstValue(object, "name")); name != nil {
				product.Producto = *name
			}
			products = append(products, product)
		}
	}
	return products
}
