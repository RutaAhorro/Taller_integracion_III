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
func PageURL(sitio catalog.Site, urlCruda string, pagina int) string {
	paginacion := sitio.Pagination
	switch paginacion.Mode {
	case "path":
		if pagina <= 1 && !paginacion.IncludeFirst {
			return urlCruda
		}
		return pagePathURL(urlCruda, paginacion.PathSegment, pagina)
	case "query":
		if pagina <= 1 && !paginacion.IncludeFirst {
			return removeQueryParam(urlCruda, paginacion.Param)
		}
		return setQueryPage(urlCruda, paginacion.Param, pagina, paginacion.ReplaceQuery)
	default:
		return urlCruda
	}
}

// StartPage lee la página inicial de la propia URL cuando el sitio lo admite.
func StartPage(sitio catalog.Site, urlCruda string) int {
	paginacion := sitio.Pagination
	if !paginacion.StartFromURL || paginacion.Mode != "query" {
		return 1
	}
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return 1
	}
	pagina, errorObtenido := strconv.Atoi(analizada.Query().Get(paginacion.Param))
	if errorObtenido != nil || pagina < 1 {
		return 1
	}
	return pagina
}

// CategoryFromURL deriva el nombre de categoría que acompaña al producto.
func CategoryFromURL(sitio catalog.Site, urlCruda string) string {
	regla := sitio.CategoryFrom
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return regla.Fallback
	}
	segmentos := nonEmptySegments(analizada.Path)

	switch regla.Mode {
	case "cugat":
		if len(segmentos) >= 2 && segmentos[len(segmentos)-2] == regla.PageSegment {
			segmentos = segmentos[:len(segmentos)-2]
		}
	case "lider":
		if regla.StripPrefix != "" && len(segmentos) > 0 && segmentos[0] == regla.StripPrefix {
			segmentos = segmentos[1:]
		}
		if patron := sitio.Regexp("category_id"); patron != nil && len(segmentos) > 0 && patron.MatchString(segmentos[len(segmentos)-1]) {
			segmentos = segmentos[:len(segmentos)-1]
		}
	case "acuenta":
		if len(segmentos) >= 2 && isDigits(segmentos[len(segmentos)-1]) {
			return segmentos[len(segmentos)-2]
		}
	}

	for indice := len(segmentos) - 1; indice >= 0; indice-- {
		if segmentos[indice] != "" {
			return segmentos[indice]
		}
	}
	return regla.Fallback
}

// ResolveURL resuelve una referencia relativa contra la URL de la respuesta.
func ResolveURL(base *url.URL, referencia string) string {
	analizada, errorObtenido := url.Parse(referencia)
	if errorObtenido != nil {
		return referencia
	}
	if base == nil {
		return referencia
	}
	return base.ResolveReference(analizada).String()
}

// SelectProductByURL elige el producto solicitado dentro de los candidatos.
// Compara host+path para ignorar query strings de tracking. Si no hay
// coincidencia y el documento sólo trae un producto, se acepta ese único
// candidato; si hay varios y ninguno coincide, devuelve nil para no emitir el
// producto equivocado.
func SelectProductByURL(productos []domain.Product, solicitado string, urlRespuesta *url.URL) *domain.Product {
	if len(productos) == 0 {
		return nil
	}
	deseado := canonicalProductURL(solicitado)
	if deseado != "" {
		for indice := range productos {
			if productos[indice].URLProducto == nil {
				continue
			}
			candidato := ResolveURL(urlRespuesta, *productos[indice].URLProducto)
			if canonicalProductURL(candidato) == deseado {
				return &productos[indice]
			}
		}
	}
	if len(productos) == 1 {
		return &productos[0]
	}
	return nil
}

// ExtractJSONLDProduct lee la primera ficha schema.org/Product publicada como
// JSON-LD.
func ExtractJSONLDProduct(sitio catalog.Site, cuerpo []byte, urlRespuesta *url.URL) *domain.Product {
	productos := ExtractJSONLDProducts(sitio, cuerpo, urlRespuesta)
	if len(productos) == 0 {
		return nil
	}
	return &productos[0]
}

// ExtractJSONLDProducts devuelve todas las fichas schema.org/Product del
// documento. Una página de detalle puede incluir recomendaciones, por eso el
// modo product_url filtra después por URL en vez de quedarse con la primera.
func ExtractJSONLDProducts(sitio catalog.Site, cuerpo []byte, urlRespuesta *url.URL) []domain.Product {
	selector := sitio.Selector("json_ld")
	if selector == "" {
		return nil
	}
	documento, errorObtenido := goquery.NewDocumentFromReader(bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return nil
	}
	var productos []domain.Product
	guiones := documento.Find(selector)
	for indice := 0; indice < guiones.Length(); indice++ {
		var datos any
		decodificador := json.NewDecoder(strings.NewReader(guiones.Eq(indice).Text()))
		decodificador.UseNumber()
		if errorObtenido := decodificador.Decode(&datos); errorObtenido != nil {
			continue
		}
		for _, entrada := range walkJSON(datos) {
			objeto, valido := entrada.(map[string]any)
			if !valido || !isProductType(objeto["@type"]) {
				continue
			}
			ofertas := firstMapValue(objeto["offers"])
			precio := normalize.PrecioCLP(firstValue(ofertas, "price"))
			imagen := imageURL(objeto["image"])
			marca := objeto["brand"]
			if mapaMarcas, valido := marca.(map[string]any); valido {
				marca = mapaMarcas["name"]
			}
			valorEAN := firstValue(objeto, "gtin13")
			if valorEAN == nil || valorEAN == "" {
				valorEAN = firstValue(objeto, "gtin")
			}
			urlProducto := urlString(urlRespuesta)
			if valor, valido := objeto["url"].(string); valido && valor != "" {
				urlProducto = ResolveURL(urlRespuesta, valor)
			}
			producto := domain.Product{
				Precio:       precio,
				PrecioNormal: precio,
				EanGtin:      normalize.EAN(valorEAN),
				SKU:          firstNonNilString(normalize.Texto(firstValue(objeto, "sku")), normalize.SKUFromURL(urlProducto)),
				Marca:        normalize.Texto(marca),
				FormatoCrudo: normalize.Texto(firstValue(objeto, "description")),
				EnStock:      stockValue(firstValue(ofertas, "availability")),
				URLProducto:  &urlProducto,
				Imagen:       imagen,
			}
			if nombre := normalize.Texto(firstValue(objeto, "name")); nombre != nil {
				producto.Producto = *nombre
			}
			productos = append(productos, producto)
		}
	}
	return productos
}

// FormatFromText extrae el formato (peso o volumen) del nombre del producto.
func FormatFromText(valor string, patron *regexp.Regexp) *string {
	coincidencia := patron.FindString(valor)
	if coincidencia == "" {
		return nil
	}
	return normalize.Texto(coincidencia)
}

// TitleCase pasa a mayúscula la primera letra de cada secuencia de letras y el
// resto a minúscula. Los dígitos y signos cortan la secuencia (1.5l -> 1.5L).
func TitleCase(valor string) string {
	var constructor strings.Builder
	letraPrevia := false
	for _, caracter := range valor {
		if unicode.IsLetter(caracter) {
			if letraPrevia {
				constructor.WriteRune(unicode.ToLower(caracter))
			} else {
				constructor.WriteRune(unicode.ToUpper(caracter))
			}
			letraPrevia = true
			continue
		}
		constructor.WriteRune(caracter)
		letraPrevia = false
	}
	return constructor.String()
}

// ===== Paginación y Rutas =====

// pagePathURL construye una URL de paginación incorporando el número de página a la ruta.
func pagePathURL(urlCruda, segmento string, pagina int) string {
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return urlCruda
	}
	segmentos := nonEmptySegments(analizada.Path)
	if len(segmentos) >= 2 && segmentos[len(segmentos)-2] == segmento && isDigits(segmentos[len(segmentos)-1]) {
		segmentos = segmentos[:len(segmentos)-2]
	}
	analizada.Path = "/" + strings.Join(segmentos, "/") + "/" + segmento + "/" + strconv.Itoa(pagina) + "/"
	analizada.RawPath = ""
	return analizada.String()
}

// setQueryPage actualiza o añade el parámetro de página en la consulta de la URL.
func setQueryPage(urlCruda, parametro string, pagina int, reemplazar bool) string {
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return urlCruda
	}
	if reemplazar {
		consulta := url.Values{}
		consulta.Set(parametro, strconv.Itoa(pagina))
		analizada.RawQuery = consulta.Encode()
		return analizada.String()
	}
	return appendQueryParam(urlCruda, parametro, strconv.Itoa(pagina), analizada)
}

// appendQueryParam concatena un nuevo parámetro a la query de la URL.
func appendQueryParam(urlCruda, clave, valor string, analizada *url.URL) string {
	pares := queryPairs(analizada.RawQuery)
	filtrados := pares[:0]
	for _, par := range pares {
		nombreClave, _, _ := strings.Cut(par, "=")
		if nombreClave == clave {
			continue
		}
		filtrados = append(filtrados, par)
	}
	filtrados = append(filtrados, url.QueryEscape(clave)+"="+url.QueryEscape(valor))
	analizada.RawQuery = strings.Join(filtrados, "&")
	return analizada.String()
}

// removeQueryParam elimina un parámetro de la cadena de consulta de la URL.
func removeQueryParam(urlCruda, clave string) string {
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return urlCruda
	}
	pares := queryPairs(analizada.RawQuery)
	filtrados := pares[:0]
	for _, par := range pares {
		nombreClave, _, _ := strings.Cut(par, "=")
		if nombreClave == clave {
			continue
		}
		filtrados = append(filtrados, par)
	}
	analizada.RawQuery = strings.Join(filtrados, "&")
	return analizada.String()
}

// queryPairs desglosa los parámetros clave-valor preservando su orden original.
func queryPairs(consultaCruda string) []string {
	if consultaCruda == "" {
		return nil
	}
	return strings.Split(consultaCruda, "&")
}

// ===== Normalización de URLs =====

// canonicalProductURL normaliza la URL de un producto removiendo parámetros de rastreo.
func canonicalProductURL(urlCruda string) string {
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return ""
	}
	return strings.ToLower(analizada.Hostname()) + "|" + strings.TrimRight(analizada.Path, "/")
}

// ===== Extracción de Texto y Atributos =====

// firstText retorna el texto del primer selector que coincida en la selección.
func firstText(seleccion *goquery.Selection, selector string) string {
	return seleccion.Find(selector).First().Text()
}

// allText replica el " ".join(text()) de Scrapy: conserva el límite entre nodos
// de texto, que importa para detectar frases como "Available for Despacho"
// aunque el HTML las tenga en elementos separados.
func allText(seleccion *goquery.Selection) string {
	var partes []string
	for _, nodo := range seleccion.Nodes {
		collectText(nodo, &partes)
	}
	return strings.Join(partes, " ")
}

// collectText acumula todo el texto visible limpiando saltos y espacios.
func collectText(nodo *html.Node, partes *[]string) {
	if nodo.Type == html.TextNode {
		*partes = append(*partes, nodo.Data)
		return
	}
	for hijo := nodo.FirstChild; hijo != nil; hijo = hijo.NextSibling {
		collectText(hijo, partes)
	}
}

// firstAttr retorna el valor del atributo solicitado para el primer selector coincidente.
func firstAttr(seleccion *goquery.Selection, selector, atributo string) string {
	valor, _ := seleccion.Find(selector).First().Attr(atributo)
	return valor
}

// noscriptImage lee el HTML crudo dentro de <noscript>. goquery puede dejar ese
// contenido como texto cuando el parser tiene scripting habilitado, así que se
// vuelve a parsear para no perder la imagen de respaldo de WooCommerce.
func noscriptImage(seleccion *goquery.Selection, selector string) string {
	for _, nodo := range seleccion.Find(selector).Nodes {
		var crudo strings.Builder
		for hijo := nodo.FirstChild; hijo != nil; hijo = hijo.NextSibling {
			if hijo.Type == html.TextNode {
				crudo.WriteString(hijo.Data)
			}
		}
		texto := strings.TrimSpace(crudo.String())
		if texto == "" {
			continue
		}
		documento, errorObtenido := goquery.NewDocumentFromReader(strings.NewReader(texto))
		if errorObtenido != nil {
			continue
		}
		if origen, valido := documento.Find("img").First().Attr("src"); valido && origen != "" {
			return origen
		}
	}
	return ""
}

// firstNonNilString retorna el primer puntero a cadena no nulo de la lista.
func firstNonNilString(valores ...*string) *string {
	for _, valor := range valores {
		if valor != nil && *valor != "" {
			return valor
		}
	}
	return nil
}

// nonEmptyStringPtr convierte una cadena no vacía en un puntero a string.
func nonEmptyStringPtr(valor string) *string {
	if valor == "" {
		return nil
	}
	return &valor
}

// lastPathSegment extrae el último componente de la ruta de una URL.
func lastPathSegment(urlCruda string) string {
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return ""
	}
	segmentos := nonEmptySegments(analizada.Path)
	if len(segmentos) == 0 {
		return ""
	}
	return segmentos[len(segmentos)-1]
}

// isDigits comprueba si todos los caracteres de la cadena son dígitos numéricos.
func isDigits(valor string) bool {
	if valor == "" {
		return false
	}
	for _, caracter := range valor {
		if caracter < '0' || caracter > '9' {
			return false
		}
	}
	return true
}

// nonEmptySegments divide una ruta en segmentos descartando barras intermedias vacías.
func nonEmptySegments(ruta string) []string {
	var segmentos []string
	for _, segmento := range strings.Split(ruta, "/") {
		if segmento != "" {
			segmentos = append(segmentos, segmento)
		}
	}
	return segmentos
}

// hasClass evalúa si un atributo de clase HTML contiene una clase específica.
func hasClass(clases, deseado string) bool {
	for _, clase := range strings.Fields(clases) {
		if clase == deseado {
			return true
		}
	}
	return false
}

// urlString normaliza la representación textual de una URL.
func urlString(valor *url.URL) string {
	if valor == nil {
		return ""
	}
	return valor.String()
}

// ===== Microdatos Schema.org =====

// schemaOffer extrae el nodo de oferta Schema.org de un producto.
func schemaOffer(entrada map[string]any) map[string]any {
	switch crudo := entrada["offers"].(type) {
	case map[string]any:
		return crudo
	case []any:
		if len(crudo) > 0 {
			if oferta, valido := crudo[0].(map[string]any); valido {
				return oferta
			}
		}
	}
	return map[string]any{}
}

// extractNormalPrice obtiene el precio de lista o normal desde un nodo de producto.
func extractNormalPrice(entrada map[string]any) *float64 {
	if precioObtenido := firstPrice(entrada, "price"); precioObtenido != nil {
		return precioObtenido
	}
	especificacion := entrada["priceSpecification"]
	if lista, valido := especificacion.([]any); valido && len(lista) > 0 {
		especificacion = lista[0]
	}
	return firstPrice(especificacion, "price")
}

// firstValue retorna el primer valor hallado para una lista de claves candidatas.
func firstValue(datos any, claves ...string) any {
	valores, valido := datos.(map[string]any)
	if !valido {
		return nil
	}
	for _, clave := range claves {
		valor, encontrado := valores[clave]
		if !encontrado || valor == nil {
			continue
		}
		switch valor.(type) {
		case map[string]any, []any:
			continue
		}
		return valueString(valor)
	}
	return nil
}

// firstPrice busca y transforma a float64 el primer precio válido entre varias claves.
func firstPrice(datos any, claves ...string) *float64 {
	return normalize.PrecioCLP(firstValue(datos, claves...))
}

// valueString formatea cualquier tipo de dato primitivo en su representación string.
func valueString(valor any) string {
	switch valorTipado := valor.(type) {
	case string:
		return valorTipado
	case json.Number:
		return valorTipado.String()
	case float64:
		textoFormateado := strconv.FormatFloat(valorTipado, 'g', -1, 64)
		if !strings.ContainsAny(textoFormateado, ".eE") {
			textoFormateado += ".0"
		}
		return textoFormateado
	case float32:
		textoFormateado := strconv.FormatFloat(float64(valorTipado), 'g', -1, 32)
		if !strings.ContainsAny(textoFormateado, ".eE") {
			textoFormateado += ".0"
		}
		return textoFormateado
	case bool:
		if valorTipado {
			return "True"
		}
		return "False"
	default:
		return fmt.Sprint(valor)
	}
}

// stockValue infiere la disponibilidad de stock a partir de indicadores del producto.
func stockValue(disponibilidad any) *bool {
	texto, valido := disponibilidad.(string)
	if !valido {
		return nil
	}
	partes := strings.Split(texto, "/")
	conStock := strings.ToLower(partes[len(partes)-1]) != "outofstock"
	return &conStock
}

// ===== Recorrido y Utilidades JSON =====

// walkJSON visita recursivamente cada objeto JSON invocando la función callback.
func walkJSON(valor any) []any {
	var resultado []any
	var recorrido func(any)
	recorrido = func(nodo any) {
		switch tipoNodo := nodo.(type) {
		case map[string]any:
			resultado = append(resultado, tipoNodo)
			for _, hijo := range tipoNodo {
				recorrido(hijo)
			}
		case []any:
			for _, hijo := range tipoNodo {
				recorrido(hijo)
			}
		}
	}
	recorrido(valor)
	return resultado
}

// imageURL extrae la URL de la imagen principal del producto.
func imageURL(valor any) *string {
	if lista, valido := valor.([]any); valido {
		if len(lista) == 0 {
			return nil
		}
		valor = lista[0]
	}
	if mapeado, valido := valor.(map[string]any); valido {
		valor = mapeado["url"]
	}
	if texto, valido := valor.(string); valido && texto != "" {
		return &texto
	}
	return nil
}

// isProductType comprueba si el tipo Schema.org corresponde a una entidad Producto.
func isProductType(valor any) bool {
	switch valorTipado := valor.(type) {
	case string:
		return valorTipado == "Product"
	case []any:
		for _, elemento := range valorTipado {
			if texto, valido := elemento.(string); valido && texto == "Product" {
				return true
			}
		}
	}
	return false
}

// firstMapValue obtiene el primer mapa asociado a alguna de las claves especificadas.
func firstMapValue(valor any) any {
	if lista, valido := valor.([]any); valido {
		if len(lista) == 0 {
			return nil
		}
		return lista[0]
	}
	return valor
}
