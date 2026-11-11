// Package extract contiene los parsers de HTML de cada tienda. Cada función
// recibe el sitio del catálogo y devuelve productos del dominio; el ciclo de
// vida del spider queda en internal/spiders.
package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

// PageURL construye la URL de una página según la configuración del sitio.
func PageURL(site catalog.Site, rawURL string, page int) string {
	pagination := site.Pagination
	switch pagination.Mode {
	case "path":
		if page <= 1 && !pagination.IncludeFirst {
			return rawURL
		}
		return pagePathURL(rawURL, pagination.PathSegment, page)
	case "query":
		if page <= 1 && !pagination.IncludeFirst {
			return removeQueryParam(rawURL, pagination.Param)
		}
		return setQueryPage(rawURL, pagination.Param, page, pagination.ReplaceQuery)
	default:
		return rawURL
	}
}

// StartPage lee la página inicial de la propia URL cuando el sitio lo admite.
func StartPage(site catalog.Site, rawURL string) int {
	pagination := site.Pagination
	if !pagination.StartFromURL || pagination.Mode != "query" {
		return 1
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 1
	}
	page, err := strconv.Atoi(parsed.Query().Get(pagination.Param))
	if err != nil || page < 1 {
		return 1
	}
	return page
}

// CategoryFromURL deriva el nombre de categoría que acompaña al producto.
func CategoryFromURL(site catalog.Site, rawURL string) string {
	rule := site.CategoryFrom
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rule.Fallback
	}
	segments := nonEmptySegments(parsed.Path)

	switch rule.Mode {
	case "cugat":
		if len(segments) >= 2 && segments[len(segments)-2] == rule.PageSegment {
			segments = segments[:len(segments)-2]
		}
	case "lider":
		if rule.StripPrefix != "" && len(segments) > 0 && segments[0] == rule.StripPrefix {
			segments = segments[1:]
		}
		if pattern := site.Regexp("category_id"); pattern != nil && len(segments) > 0 && pattern.MatchString(segments[len(segments)-1]) {
			segments = segments[:len(segments)-1]
		}
	case "acuenta":
		if len(segments) >= 2 && isDigits(segments[len(segments)-1]) {
			return segments[len(segments)-2]
		}
	}

	for index := len(segments) - 1; index >= 0; index-- {
		if segments[index] != "" {
			return segments[index]
		}
	}
	return rule.Fallback
}

// ResolveURL resuelve una referencia relativa contra la URL de la respuesta.
func ResolveURL(base *url.URL, reference string) string {
	parsed, err := url.Parse(reference)
	if err != nil {
		return reference
	}
	if base == nil {
		return reference
	}
	return base.ResolveReference(parsed).String()
}

// SelectProductByURL elige el producto solicitado dentro de los candidatos.
// Compara host+path para ignorar query strings de tracking. Si no hay
// coincidencia y el documento sólo trae un producto, se acepta ese único
// candidato; si hay varios y ninguno coincide, devuelve nil para no emitir el
// producto equivocado.
func SelectProductByURL(products []domain.Product, requested string, responseURL *url.URL) *domain.Product {
	if len(products) == 0 {
		return nil
	}
	wanted := canonicalProductURL(requested)
	if wanted != "" {
		for index := range products {
			if products[index].URLProducto == nil {
				continue
			}
			candidate := ResolveURL(responseURL, *products[index].URLProducto)
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

// ExtractJSONLDProduct lee la primera ficha schema.org/Product publicada como
// JSON-LD.
func ExtractJSONLDProduct(site catalog.Site, body []byte, responseURL *url.URL) *domain.Product {
	products := ExtractJSONLDProducts(site, body, responseURL)
	if len(products) == 0 {
		return nil
	}
	return &products[0]
}

// ExtractJSONLDProducts devuelve todas las fichas schema.org/Product del
// documento. Una página de detalle puede incluir recomendaciones, por eso el
// modo product_url filtra después por URL en vez de quedarse con la primera.
func ExtractJSONLDProducts(site catalog.Site, body []byte, responseURL *url.URL) []domain.Product {
	selector := site.Selector("json_ld")
	if selector == "" {
		return nil
	}
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	var products []domain.Product
	scripts := document.Find(selector)
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
			image := imageURL(object["image"])
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
				productURL = ResolveURL(responseURL, value)
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

// FormatFromText extrae el formato (peso o volumen) del nombre del producto.
func FormatFromText(value string, pattern *regexp.Regexp) *string {
	match := pattern.FindString(value)
	if match == "" {
		return nil
	}
	return normalize.Texto(match)
}

// TitleCase pasa a mayúscula la primera letra de cada secuencia de letras y el
// resto a minúscula. Los dígitos y signos cortan la secuencia (1.5l -> 1.5L).
func TitleCase(value string) string {
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

func pagePathURL(rawURL, segment string, page int) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	segments := nonEmptySegments(parsed.Path)
	if len(segments) >= 2 && segments[len(segments)-2] == segment && isDigits(segments[len(segments)-1]) {
		segments = segments[:len(segments)-2]
	}
	parsed.Path = "/" + strings.Join(segments, "/") + "/" + segment + "/" + strconv.Itoa(page) + "/"
	parsed.RawPath = ""
	return parsed.String()
}

func setQueryPage(rawURL, param string, page int, replace bool) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if replace {
		query := url.Values{}
		query.Set(param, strconv.Itoa(page))
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return appendQueryParam(rawURL, param, strconv.Itoa(page), parsed)
}

func appendQueryParam(rawURL, key, value string, parsed *url.URL) string {
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

func canonicalProductURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname()) + "|" + strings.TrimRight(parsed.Path, "/")
}

func firstText(selection *goquery.Selection, selector string) string {
	return selection.Find(selector).First().Text()
}

// allText replica el " ".join(text()) de Scrapy: conserva el límite entre nodos
// de texto, que importa para detectar frases como "Available for Despacho"
// aunque el HTML las tenga en elementos separados.
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

// noscriptImage lee el HTML crudo dentro de <noscript>. goquery puede dejar ese
// contenido como texto cuando el parser tiene scripting habilitado, así que se
// vuelve a parsear para no perder la imagen de respaldo de WooCommerce.
func noscriptImage(selection *goquery.Selection, selector string) string {
	for _, node := range selection.Find(selector).Nodes {
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
	segments := nonEmptySegments(parsed.Path)
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

func nonEmptySegments(path string) []string {
	var segments []string
	for _, segment := range strings.Split(path, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}

func hasClass(classes, wanted string) bool {
	for _, class := range strings.Fields(classes) {
		if class == wanted {
			return true
		}
	}
	return false
}

func urlString(value *url.URL) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func schemaOffer(entry map[string]any) map[string]any {
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

func imageURL(value any) *string {
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
