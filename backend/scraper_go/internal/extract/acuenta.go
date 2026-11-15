package extract

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

var (
	acuentaReferenceStart = regexp.MustCompile(`(?:^|\\n|\n)([a-z0-9]+):`)
	acuentaPromotionType  = regexp.MustCompile(`\\"type\\":\\"([^"\\]+)\\"`)
	acuentaConditions     = regexp.MustCompile(`\\"conditions\\":\\"\$([a-z0-9]+)\\"`)
	acuentaConditionRef   = regexp.MustCompile(`\\"\$([a-z0-9]+)\\"`)
	acuentaPriceKey       = regexp.MustCompile(`\\"priceBeforeTaxes\\":(-?\d+)`)
	acuentaEANReference   = regexp.MustCompile(`(?:^|\\n|\n)([a-z0-9]+):\[\\?"(\d{8,14})`)
	acuentaSKUPattern     = regexp.MustCompile(`\\"sku\\":\\"(\d+)\\"`)
	acuentaSlugPattern    = regexp.MustCompile(`\\"slug\\":\\"([^"\\]+)\\"`)
	acuentaBrandPattern   = regexp.MustCompile(`\\"brand\\":\\"([^"\\]+)\\"`)
	acuentaStockPattern   = regexp.MustCompile(`\\"stock\\":(-?\d+)`)
	acuentaNamePattern    = regexp.MustCompile(`\\"name\\":\\"([^"\\]+)\\"`)
	acuentaEANField       = regexp.MustCompile(`\\"ean\\":(.*?),\\"maxQty\\"`)
	acuentaDigits         = regexp.MustCompile(`\d{8,14}`)
	acuentaEANRefPattern  = regexp.MustCompile(`\\"\$([a-z0-9]+)\\"`)
	acuentaPromotionRef   = regexp.MustCompile(`\\"promotion\\":\\"\$([a-z0-9]+)\\"`)
	acuentaPhotosRef      = regexp.MustCompile(`\\"photosUrl\\":\\"\$([a-z0-9]+)\\"`)
	acuentaMultiPrice     = regexp.MustCompile(`\b(\d+)\s*[xX]\s*\$\s*([\d.,]+)`)
	acuentaAmountPattern  = regexp.MustCompile(`\$\s*([\d.,]+)`)
	acuentaFormatPattern  = regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*(?:kg|g|gr|ml|lt|l|un(?:idad(?:es)?)?)\b`)
	acuentaWordPattern    = regexp.MustCompile(`[a-záéíóúñ0-9]+`)
	acuentaTrailingDigits = regexp.MustCompile(`-\d+$`)
)

// ===== Extracción Principal =====

// AcuentaProducts usa el payload RSC y completa precios visibles; si no hay
// RSC, parsea las tarjetas del HTML.
func AcuentaProducts(sitio catalog.Site, cuerpo []byte, urlRespuesta *url.URL) []domain.Product {
	productosRSC := acuentaRSCProducts(sitio, cuerpo, urlRespuesta)
	if len(productosRSC) > 0 {
		return mergeAcuentaVisiblePrices(sitio, cuerpo, urlRespuesta, productosRSC)
	}
	return acuentaCards(sitio, cuerpo, urlRespuesta)
}

// AcuentaProduct elige la ficha solicitada dentro de los productos.
func AcuentaProduct(sitio catalog.Site, cuerpo []byte, solicitado string, urlRespuesta *url.URL) *domain.Product {
	return SelectProductByURL(AcuentaProducts(sitio, cuerpo, urlRespuesta), solicitado, urlRespuesta)
}

// ===== Extracción HTML DOM =====

// acuentaCards extrae productos a partir de las tarjetas del catálogo HTML de Acuenta.
func acuentaCards(sitio catalog.Site, cuerpo []byte, urlRespuesta *url.URL) []domain.Product {
	selectorEnlace := sitio.Selector("card_link")
	if selectorEnlace == "" {
		return nil
	}
	documento, errorObtenido := goquery.NewDocumentFromReader(bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return nil
	}
	var productos []domain.Product
	urlsVistas := map[string]struct{}{}
	documento.Find(selectorEnlace).Each(func(_ int, elemento *goquery.Selection) {
		enlace, _ := elemento.Attr("href")
		if enlace == "" {
			return
		}
		urlProducto := ResolveURL(urlRespuesta, enlace)
		if _, valido := urlsVistas[urlProducto]; valido {
			return
		}
		urlsVistas[urlProducto] = struct{}{}

		tarjeta := acuentaCardFromLink(elemento)
		if tarjeta.Length() == 0 {
			return
		}
		nombre := normalize.Texto(allText(elemento))
		texto := normalize.Texto(allText(tarjeta))
		if nombre == nil || texto == nil {
			return
		}
		actual, normal, promocion := acuentaPricesFromText(*texto)
		if actual == nil || normal == nil {
			return
		}

		imagen := firstAttr(tarjeta, sitio.Selector("image"), "src")
		if imagen == "" {
			imagen = firstAttr(tarjeta, sitio.Selector("image"), "data-src")
		}
		var urlImagen *string
		if imagen != "" {
			valor := ResolveURL(urlRespuesta, imagen)
			urlImagen = &valor
		}
		var oferta *float64
		if *actual < *normal {
			oferta = actual
		}
		sku := acuentaSKUFromURL(sitio, urlProducto)
		productos = append(productos, domain.Product{
			Producto:          *nombre,
			Precio:            actual,
			PrecioNormal:      normal,
			PrecioOferta:      oferta,
			SKU:               sku,
			Marca:             acuentaBrandFromText(*texto, sitio.List("brands")),
			FormatoCrudo:      FormatFromText(*nombre, acuentaFormatPattern),
			MecanicaPromocion: promocion,
			URLProducto:       &urlProducto,
			Imagen:            urlImagen,
		})
	})
	return productos
}

// mergeAcuentaVisiblePrices sincroniza precios visibles en el DOM de Acuenta.
func mergeAcuentaVisiblePrices(sitio catalog.Site, cuerpo []byte, urlRespuesta *url.URL, productos []domain.Product) []domain.Product {
	selectorEnlace := sitio.Selector("card_link")
	if selectorEnlace == "" {
		return productos
	}
	documento, errorObtenido := goquery.NewDocumentFromReader(bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return productos
	}
	porURL := make(map[string]int, len(productos))
	for indice, producto := range productos {
		if producto.URLProducto != nil {
			porURL[*producto.URLProducto] = indice
		}
	}
	documento.Find(selectorEnlace).Each(func(_ int, elemento *goquery.Selection) {
		enlace, _ := elemento.Attr("href")
		if enlace == "" {
			return
		}
		urlProducto := ResolveURL(urlRespuesta, enlace)
		indice, valido := porURL[urlProducto]
		if !valido {
			return
		}
		tarjeta := acuentaCardFromLink(elemento)
		if tarjeta.Length() == 0 {
			return
		}
		texto := normalize.Texto(allText(tarjeta))
		if texto == nil {
			return
		}
		actual, normal, promocion := acuentaPricesFromText(*texto)
		if actual == nil || normal == nil {
			return
		}
		producto := &productos[indice]
		if *actual < *normal {
			producto.Precio = actual
			producto.PrecioNormal = normal
			producto.PrecioOferta = actual
			if producto.MecanicaPromocion == nil {
				producto.MecanicaPromocion = domain.StringPtr(sitio.Setting("promotion"))
			}
		}
		if promocion != nil {
			producto.MecanicaPromocion = promocion
		}
	})
	return productos
}

// acuentaCardFromLink sube desde el enlace hasta el contenedor que muestra un
// precio.
func acuentaCardFromLink(enlace *goquery.Selection) *goquery.Selection {
	tarjeta := enlace.Parent()
	for tarjeta.Length() > 0 && !strings.Contains(allText(tarjeta), "$") {
		tarjeta = tarjeta.Parent()
	}
	return tarjeta
}

// ===== Extracción React Server Components (RSC) =====

// acuentaRSCProducts procesa el flujo React Server Components (RSC) de Acuenta.
func acuentaRSCProducts(sitio catalog.Site, cuerpo []byte, urlRespuesta *url.URL) []domain.Product {
	texto := string(cuerpo)
	preciosPromocion := acuentaPromotionPrices(texto)
	referenciasEAN := map[string]string{}
	for _, coincidencia := range acuentaEANReference.FindAllStringSubmatch(texto, -1) {
		referenciasEAN[coincidencia[1]] = coincidencia[2]
	}

	var productos []domain.Product
	skusVistos := map[string]struct{}{}
	for _, coincidencia := range acuentaSKUPattern.FindAllStringSubmatchIndex(texto, -1) {
		sku := texto[coincidencia[2]:coincidencia[3]]
		if _, valido := skusVistos[sku]; valido {
			continue
		}
		fin := coincidencia[1] + sitio.Limit("rsc_window")
		if fin > len(texto) {
			fin = len(texto)
		}
		ventana := texto[coincidencia[0]:fin]

		coincidenciaSlug := acuentaSlugPattern.FindStringSubmatch(ventana)
		coincidenciaPrecio := acuentaPriceKey.FindStringSubmatch(ventana)
		if len(coincidenciaSlug) < 2 || len(coincidenciaPrecio) < 2 {
			continue
		}
		slug := coincidenciaSlug[1]
		precioNormal := normalize.PrecioCLP(coincidenciaPrecio[1])
		if precioNormal == nil {
			continue
		}

		var nombre *string
		if coincidenciaNombre := acuentaNamePattern.FindStringSubmatch(ventana); len(coincidenciaNombre) >= 2 {
			nombre = normalize.Texto(coincidenciaNombre[1])
		}
		if nombre == nil || !acuentaNameMatchesSlug(sitio, *nombre, slug) {
			nombre = acuentaNameFromSlug(slug)
		}
		if nombre == nil {
			continue
		}

		var ean *string
		if campoEAN := acuentaEANField.FindStringSubmatch(ventana); len(campoEAN) >= 2 {
			valoresEAN := acuentaDigits.FindAllString(campoEAN[1], -1)
			if len(valoresEAN) == 0 {
				if referencia := acuentaEANRefPattern.FindStringSubmatch(campoEAN[1]); len(referencia) >= 2 {
					if valor, valido := referenciasEAN[referencia[1]]; valido {
						valoresEAN = []string{valor}
					}
				}
			}
			if len(valoresEAN) > 0 {
				ean = normalize.EAN(valoresEAN[0])
			}
		}

		referenciaPromocion := ""
		if coincidenciaPromocion := acuentaPromotionRef.FindStringSubmatch(ventana); len(coincidenciaPromocion) >= 2 {
			referenciaPromocion = coincidenciaPromocion[1]
		}
		precioActual := precioNormal
		if valor, valido := preciosPromocion[referenciaPromocion]; valido {
			precioActual = valor
		}

		imagen := acuentaImageFromRSC(sitio, texto, sku)
		if imagen == "" {
			imagen = acuentaResolveImageReference(texto, ventana)
		}
		if imagen == "" {
			imagen = acuentaImageFromSKU(sitio, sku)
		}
		urlImagen := acuentaCanonicalImageURL(imagen)

		urlProducto := ResolveURL(urlRespuesta, "/p/"+slug)
		var existencias *bool
		if coincidenciaStock := acuentaStockPattern.FindStringSubmatch(ventana); len(coincidenciaStock) >= 2 {
			if cantidad, errorObtenido := strconv.Atoi(coincidenciaStock[1]); errorObtenido == nil {
				existencias = domain.BoolPtr(cantidad > 0)
			}
		}
		var marca *string
		if coincidenciaMarca := acuentaBrandPattern.FindStringSubmatch(ventana); len(coincidenciaMarca) >= 2 {
			marca = normalize.Texto(coincidenciaMarca[1])
		}
		var oferta *float64
		if *precioActual < *precioNormal {
			oferta = precioActual
		}
		var mecanicas *string
		if referenciaPromocion != "" {
			mecanicas = domain.StringPtr(sitio.Setting("promotion"))
		}
		skusVistos[sku] = struct{}{}
		productos = append(productos, domain.Product{
			Producto:          *nombre,
			Precio:            precioActual,
			PrecioNormal:      precioNormal,
			PrecioOferta:      oferta,
			EanGtin:           ean,
			SKU:               &sku,
			Marca:             marca,
			FormatoCrudo:      FormatFromText(*nombre, acuentaFormatPattern),
			MecanicaPromocion: mecanicas,
			EnStock:           existencias,
			URLProducto:       &urlProducto,
			Imagen:            urlImagen,
		})
	}
	return productos
}

// acuentaPromotionPrices extrae precios promocionales desde respuestas RSC de Acuenta.
func acuentaPromotionPrices(texto string) map[string]*float64 {
	inicios := acuentaReferenceStart.FindAllStringSubmatchIndex(texto, -1)
	referencias := map[string]string{}
	for indice, coincidencia := range inicios {
		clave := texto[coincidencia[2]:coincidencia[3]]
		fin := len(texto)
		if indice+1 < len(inicios) {
			fin = inicios[indice+1][0]
		}
		referencias[clave] = texto[coincidencia[1]:fin]
	}

	preciosPromocion := map[string]*float64{}
	for referencia, datos := range referencias {
		coincidenciaTipo := acuentaPromotionType.FindStringSubmatch(datos)
		if len(coincidenciaTipo) < 2 || coincidenciaTipo[1] != "specialPrice" {
			continue
		}
		coincidenciasCondiciones := acuentaConditions.FindStringSubmatch(datos)
		if len(coincidenciasCondiciones) < 2 {
			continue
		}
		condiciones := referencias[coincidenciasCondiciones[1]]
		if condiciones == "" {
			continue
		}
		var precios []float64
		for _, referenciaCondicion := range acuentaConditionRef.FindAllStringSubmatch(condiciones, -1) {
			datosCondicion := referencias[referenciaCondicion[1]]
			coincidenciaPrecio := acuentaPriceKey.FindStringSubmatch(datosCondicion)
			if len(coincidenciaPrecio) < 2 {
				continue
			}
			if precio := normalize.PrecioCLP(coincidenciaPrecio[1]); precio != nil {
				precios = append(precios, *precio)
			}
		}
		if len(precios) == 0 {
			continue
		}
		minimo := precios[0]
		for _, precio := range precios[1:] {
			if precio < minimo {
				minimo = precio
			}
		}
		valor := minimo
		preciosPromocion[referencia] = &valor
	}
	return preciosPromocion
}

// ===== Extracción de Imágenes =====

// acuentaImageFromRSC localiza la imagen del producto en el payload RSC.
func acuentaImageFromRSC(sitio catalog.Site, texto, sku string) string {
	patron := regexp.MustCompile(
		`(?i)` + regexp.QuoteMeta(sitio.Setting("image_base")) +
			regexp.QuoteMeta(sku) +
			`[^"\s]*?\.(?:jpe?g|png|webp)\](?:\\u0026|&)sink`,
	)
	coincidencia := patron.FindString(texto)
	return strings.ReplaceAll(coincidencia, `\u0026`, "&")
}

// acuentaResolveImageReference resuelve referencias a imágenes anidadas en RSC.
func acuentaResolveImageReference(texto, ventana string) string {
	coincidenciaReferencia := acuentaPhotosRef.FindStringSubmatch(ventana)
	if len(coincidenciaReferencia) < 2 {
		return ""
	}
	patron := regexp.MustCompile(
		regexp.QuoteMeta(coincidenciaReferencia[1]) + `:\[\\?"(https?://[^"\\]+)`,
	)
	coincidencia := patron.FindStringSubmatch(texto)
	if len(coincidencia) < 2 {
		return ""
	}
	return strings.ReplaceAll(coincidencia[1], `\u0026`, "&")
}

// acuentaImageFromSKU construye la URL canónica de imagen desde el SKU.
func acuentaImageFromSKU(sitio catalog.Site, sku string) string {
	return sitio.Setting("image_base") + sku + sitio.Setting("image_suffix")
}

// acuentaCanonicalImageURL normaliza y estandariza la URL de una imagen.
func acuentaCanonicalImageURL(valor string) *string {
	if valor == "" {
		return nil
	}
	valor = strings.ReplaceAll(valor, `\u0026`, "&")
	var constructor strings.Builder
	for indice := 0; indice < len(valor); indice++ {
		caracter := valor[indice]
		if (caracter >= 'a' && caracter <= 'z') ||
			(caracter >= 'A' && caracter <= 'Z') ||
			(caracter >= '0' && caracter <= '9') ||
			strings.ContainsRune(":/?=&.%_-~", rune(caracter)) {
			constructor.WriteByte(caracter)
			continue
		}
		fmt.Fprintf(&constructor, "%%%02X", caracter)
	}
	resultado := constructor.String()
	return &resultado
}

// ===== Normalización y Slugs =====

// acuentaNameMatchesSlug verifica concordancia entre el nombre y el slug de la URL.
func acuentaNameMatchesSlug(sitio catalog.Site, nombre, slug string) bool {
	tokensNombre := acuentaTokens(sitio, nombre)
	tokensSlug := acuentaTokens(sitio, slug)
	interseccion := 0
	for token := range tokensNombre {
		if _, valido := tokensSlug[token]; valido {
			interseccion++
		}
	}
	return interseccion >= sitio.Limit("name_min_intersection")
}

// acuentaTokens segmenta una cadena en palabras clave para comparación.
func acuentaTokens(sitio catalog.Site, valor string) map[string]struct{} {
	fichas := map[string]struct{}{}
	for _, coincidencia := range acuentaWordPattern.FindAllString(valor, -1) {
		token := strings.ToLower(coincidencia)
		if len([]rune(token)) >= sitio.Limit("token_min_length") {
			fichas[token] = struct{}{}
		}
	}
	return fichas
}

// acuentaNameFromSlug infiere el nombre del producto basándose en el slug de la URL.
func acuentaNameFromSlug(slug string) *string {
	legible := acuentaTrailingDigits.ReplaceAllString(slug, "")
	return normalize.Texto(TitleCase(strings.ReplaceAll(legible, "-", " ")))
}

// ===== Extracción de Precios y Marca =====

// acuentaPricesFromText extrae valores de precio normal y oferta desde un texto.
func acuentaPricesFromText(texto string) (*float64, *float64, *string) {
	if multiple := acuentaMultiPrice.FindStringSubmatchIndex(texto); multiple != nil {
		var montosPrefijo []float64
		for _, coincidencia := range acuentaAmountPattern.FindAllStringSubmatch(texto[:multiple[0]], -1) {
			if monto := normalize.PrecioCLP(coincidencia[1]); monto != nil {
				montosPrefijo = append(montosPrefijo, *monto)
			}
		}
		var actual *float64
		if len(montosPrefijo) > 0 {
			actual = &montosPrefijo[len(montosPrefijo)-1]
		} else {
			actual = normalize.PrecioCLP(texto[multiple[4]:multiple[5]])
		}
		if actual != nil {
			promocion := texto[multiple[0]:multiple[1]]
			return actual, actual, &promocion
		}
	}

	var montos []float64
	for _, coincidencia := range acuentaAmountPattern.FindAllStringSubmatch(texto, -1) {
		if monto := normalize.PrecioCLP(coincidencia[1]); monto != nil {
			montos = append(montos, *monto)
		}
	}
	if len(montos) == 0 {
		return nil, nil, nil
	}
	actual := montos[0]
	normal := actual
	if len(montos) > 1 {
		normal = montos[1]
	}
	if normal < actual {
		normal = actual
	}
	return &actual, &normal, nil
}

// acuentaBrandFromText extrae la marca comercial desde la descripción o ficha.
func acuentaBrandFromText(texto string, marcas []string) *string {
	for _, marca := range marcas {
		patron := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(marca) + `\b`)
		if coincidencia := patron.FindString(texto); coincidencia != "" {
			return normalize.Texto(coincidencia)
		}
	}
	return nil
}

// acuentaSKUFromURL extrae el identificador SKU numérico de la URL del producto.
func acuentaSKUFromURL(sitio catalog.Site, urlProducto string) *string {
	slug := lastPathSegment(urlProducto)
	prefijo := sitio.Setting("sku_prefix")
	if coincidencia := acuentaTrailingDigits.FindString(slug); coincidencia != "" {
		valor := prefijo + strings.TrimPrefix(coincidencia, "-")
		return &valor
	}
	valor := prefijo + slug
	runas := []rune(valor)
	if len(runas) > sitio.Limit("sku_max_length") {
		valor = string(runas[:sitio.Limit("sku_max_length")])
	}
	return &valor
}
