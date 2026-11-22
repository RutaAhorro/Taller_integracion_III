// parseo_comun.go agrupa las funciones compartidas de paginación, resolución
// de URLs, parseo de JSON-LD y extracción de texto o atributos DOM.
package extraccion

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

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
)

// ===== Paginación y Categorías desde URL =====

// PageURL construye la URL de una página según la configuración del sitio.
func PageURL(sitio Site, urlCruda string, pagina int) string {
	paginacion := sitio.Pagination
	switch paginacion.Mode {
	case "path":
		if pagina <= 1 && !paginacion.IncludeFirst {
			return urlCruda
		}
		return urlPaginacionRuta(urlCruda, paginacion.PathSegment, pagina)
	case "query":
		if pagina <= 1 && !paginacion.IncludeFirst {
			return eliminarParametroConsulta(urlCruda, paginacion.Param)
		}
		return fijarPaginaConsulta(urlCruda, paginacion.Param, pagina, paginacion.ReplaceQuery)
	default:
		return urlCruda
	}
}

// StartPage lee la página inicial de la propia URL cuando el sitio lo admite.
func StartPage(sitio Site, urlCruda string) int {
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
func CategoryFromURL(sitio Site, urlCruda string) string {
	regla := sitio.CategoryFrom
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return regla.Fallback
	}
	segmentos := dominio.SegmentosNoVacios(analizada.Path)

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
		if len(segmentos) >= 2 && dominio.EsSoloDigitos(segmentos[len(segmentos)-1]) {
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

// ===== Selección y JSON-LD Común =====

// SelectProductByURL elige el producto solicitado dentro de los candidatos.
// Compara host+path para ignorar query strings de tracking. Si no hay
// coincidencia y el documento sólo trae un producto, se acepta ese único
// candidato; si hay varios y ninguno coincide, devuelve nil para no emitir el
// producto equivocado.
func SelectProductByURL(productos []dominio.Product, solicitado string, urlRespuesta *url.URL) *dominio.Product {
	if len(productos) == 0 {
		return nil
	}
	deseado := urlProductoCanonica(solicitado)
	if deseado != "" {
		for indice := range productos {
			if productos[indice].URLProducto == nil {
				continue
			}
			candidato := ResolveURL(urlRespuesta, *productos[indice].URLProducto)
			if urlProductoCanonica(candidato) == deseado {
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
func ExtractJSONLDProduct(sitio Site, cuerpo []byte, urlRespuesta *url.URL) *dominio.Product {
	productos := ExtractJSONLDProducts(sitio, cuerpo, urlRespuesta)
	if len(productos) == 0 {
		return nil
	}
	return &productos[0]
}

// ExtractJSONLDProducts devuelve todas las fichas schema.org/Product del
// documento. Una página de detalle puede incluir recomendaciones, por eso el
// modo product_url filtra después por URL en vez de quedarse con la primera.
func ExtractJSONLDProducts(sitio Site, cuerpo []byte, urlRespuesta *url.URL) []dominio.Product {
	selector := sitio.Selector("json_ld")
	if selector == "" {
		return nil
	}
	documento, errorObtenido := goquery.NewDocumentFromReader(bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return nil
	}
	var productos []dominio.Product
	guiones := documento.Find(selector)
	for indice := 0; indice < guiones.Length(); indice++ {
		var datos any
		decodificador := json.NewDecoder(strings.NewReader(guiones.Eq(indice).Text()))
		decodificador.UseNumber()
		if errorObtenido := decodificador.Decode(&datos); errorObtenido != nil {
			continue
		}
		for _, entrada := range recorrerJSON(datos) {
			objeto, valido := entrada.(map[string]any)
			if !valido || !esTipoProducto(objeto["@type"]) {
				continue
			}
			ofertas := primerValorMapa(objeto["offers"])
			precio := dominio.PrecioCLP(primerValor(ofertas, "price"))
			imagen := urlImagen(objeto["image"])
			marca := objeto["brand"]
			if mapaMarcas, valido := marca.(map[string]any); valido {
				marca = mapaMarcas["name"]
			}
			valorEAN := primerValor(objeto, "gtin13")
			if valorEAN == nil || valorEAN == "" {
				valorEAN = primerValor(objeto, "gtin")
			}
			urlProducto := textoURL(urlRespuesta)
			if valor, valido := objeto["url"].(string); valido && valor != "" {
				urlProducto = ResolveURL(urlRespuesta, valor)
			}
			producto := dominio.Product{
				Precio:       precio,
				PrecioNormal: precio,
				EanGtin:      dominio.EAN(valorEAN),
				SKU:          primerPunteroCadenaNoNulo(dominio.Texto(primerValor(objeto, "sku")), dominio.SKUFromURL(urlProducto)),
				Marca:        dominio.Texto(marca),
				FormatoCrudo: dominio.Texto(primerValor(objeto, "description")),
				EnStock:      valorDisponibilidad(primerValor(ofertas, "availability")),
				URLProducto:  &urlProducto,
				Imagen:       imagen,
			}
			if nombre := dominio.Texto(primerValor(objeto, "name")); nombre != nil {
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
	return dominio.Texto(coincidencia)
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

// urlPaginacionRuta construye una URL de paginación incorporando el número de página a la ruta.
func urlPaginacionRuta(urlCruda, segmento string, pagina int) string {
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return urlCruda
	}
	segmentos := dominio.SegmentosNoVacios(analizada.Path)
	if len(segmentos) >= 2 && segmentos[len(segmentos)-2] == segmento && dominio.EsSoloDigitos(segmentos[len(segmentos)-1]) {
		segmentos = segmentos[:len(segmentos)-2]
	}
	analizada.Path = "/" + strings.Join(segmentos, "/") + "/" + segmento + "/" + strconv.Itoa(pagina) + "/"
	analizada.RawPath = ""
	return analizada.String()
}

// fijarPaginaConsulta actualiza o añade el parámetro de página en la consulta de la URL.
func fijarPaginaConsulta(urlCruda, parametro string, pagina int, reemplazar bool) string {
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
	return agregarParametroConsulta(parametro, strconv.Itoa(pagina), analizada)
}

// agregarParametroConsulta concatena un nuevo parámetro a la query de la URL.
func agregarParametroConsulta(clave, valor string, analizada *url.URL) string {
	pares := paresConsulta(analizada.RawQuery)
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

// eliminarParametroConsulta elimina un parámetro de la cadena de consulta de la URL.
func eliminarParametroConsulta(urlCruda, clave string) string {
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return urlCruda
	}
	pares := paresConsulta(analizada.RawQuery)
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

// paresConsulta desglosa los parámetros clave-valor preservando su orden original.
func paresConsulta(consultaCruda string) []string {
	if consultaCruda == "" {
		return nil
	}
	return strings.Split(consultaCruda, "&")
}

// ===== Normalización de URLs =====

// urlProductoCanonica normaliza la URL de un producto removiendo parámetros de rastreo.
func urlProductoCanonica(urlCruda string) string {
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return ""
	}
	return strings.ToLower(analizada.Hostname()) + "|" + strings.TrimRight(analizada.Path, "/")
}

// ===== Extracción de Texto y Atributos =====

// primerTexto retorna el texto del primer selector que coincida en la selección.
func primerTexto(seleccion *goquery.Selection, selector string) string {
	return seleccion.Find(selector).First().Text()
}

// todoElTexto replica el " ".join(text()) de Scrapy: conserva el límite entre nodos
// de texto, que importa para detectar frases como "Available for Despacho"
// aunque el HTML las tenga en elementos separados.
func todoElTexto(seleccion *goquery.Selection) string {
	var partes []string
	for _, nodo := range seleccion.Nodes {
		acumularTexto(nodo, &partes)
	}
	return strings.Join(partes, " ")
}

// acumularTexto acumula todo el texto visible limpiando saltos y espacios.
func acumularTexto(nodo *html.Node, partes *[]string) {
	if nodo.Type == html.TextNode {
		*partes = append(*partes, nodo.Data)
		return
	}
	for hijo := nodo.FirstChild; hijo != nil; hijo = hijo.NextSibling {
		acumularTexto(hijo, partes)
	}
}

// primerAtributo retorna el valor del atributo solicitado para el primer selector coincidente.
func primerAtributo(seleccion *goquery.Selection, selector, atributo string) string {
	valor, _ := seleccion.Find(selector).First().Attr(atributo)
	return valor
}

// imagenNoscript lee el HTML crudo dentro de <noscript>. goquery puede dejar ese
// contenido como texto cuando el parser tiene scripting habilitado, así que se
// vuelve a parsear para no perder la imagen de respaldo de WooCommerce.
func imagenNoscript(seleccion *goquery.Selection, selector string) string {
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

// primerPunteroCadenaNoNulo retorna el primer puntero a cadena no nulo de la lista.
func primerPunteroCadenaNoNulo(valores ...*string) *string {
	for _, valor := range valores {
		if valor != nil && *valor != "" {
			return valor
		}
	}
	return nil
}

// punteroCadenaNoVacia convierte una cadena no vacía en un puntero a string.
func punteroCadenaNoVacia(valor string) *string {
	if valor == "" {
		return nil
	}
	return &valor
}

// ultimoSegmentoRuta extrae el último componente de la ruta de una URL.
func ultimoSegmentoRuta(urlCruda string) string {
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return ""
	}
	segmentos := dominio.SegmentosNoVacios(analizada.Path)
	if len(segmentos) == 0 {
		return ""
	}
	return segmentos[len(segmentos)-1]
}

// contieneClase evalúa si un atributo de clase HTML contiene una clase específica.
func contieneClase(clases, deseado string) bool {
	for _, clase := range strings.Fields(clases) {
		if clase == deseado {
			return true
		}
	}
	return false
}

// textoURL normaliza la representación textual de una URL.
func textoURL(valor *url.URL) string {
	if valor == nil {
		return ""
	}
	return valor.String()
}

// ===== Microdatos Schema.org =====

// ofertaSchema extrae el nodo de oferta Schema.org de un producto.
func ofertaSchema(entrada map[string]any) map[string]any {
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

// extraerPrecioNormal obtiene el precio de lista o normal desde un nodo de producto.
func extraerPrecioNormal(entrada map[string]any) *float64 {
	if precioObtenido := primerPrecio(entrada, "price"); precioObtenido != nil {
		return precioObtenido
	}
	especificacion := entrada["priceSpecification"]
	if lista, valido := especificacion.([]any); valido && len(lista) > 0 {
		especificacion = lista[0]
	}
	return primerPrecio(especificacion, "price")
}

// primerValor retorna el primer valor hallado para una lista de claves candidatas.
func primerValor(datos any, claves ...string) any {
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
		return valorComoTexto(valor)
	}
	return nil
}

// primerPrecio busca y transforma a float64 el primer precio válido entre varias claves.
func primerPrecio(datos any, claves ...string) *float64 {
	return dominio.PrecioCLP(primerValor(datos, claves...))
}

// valorComoTexto formatea cualquier tipo de dato primitivo en su representación string.
func valorComoTexto(valor any) string {
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

// valorDisponibilidad infiere la disponibilidad de stock a partir de indicadores del producto.
func valorDisponibilidad(disponibilidad any) *bool {
	texto, valido := disponibilidad.(string)
	if !valido {
		return nil
	}
	partes := strings.Split(texto, "/")
	conStock := strings.ToLower(partes[len(partes)-1]) != "outofstock"
	return &conStock
}

// ===== Recorrido y Utilidades JSON =====

// recorrerJSON visita recursivamente cada objeto JSON acumulando los mapas encontrados.
func recorrerJSON(valor any) []any {
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

// urlImagen extrae la URL de la imagen principal del producto.
func urlImagen(valor any) *string {
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

// esTipoProducto comprueba si el tipo Schema.org corresponde a una entidad Producto.
func esTipoProducto(valor any) bool {
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

// primerValorMapa obtiene el primer mapa asociado a una entrada o lista de entradas.
func primerValorMapa(valor any) any {
	if lista, valido := valor.([]any); valido {
		if len(lista) == 0 {
			return nil
		}
		return lista[0]
	}
	return valor
}
