// acuenta.go implementa el spider de A Cuenta y la extracción de productos
// desde flujos React Server Components (RSC) o tarjetas DOM.
package extraccion

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
)

// ===== Estructura y Constructor =====

// Acuenta ejecuta el spider de A Cuenta.
type Acuenta struct {
	spiderBase
}

// nuevoAcuenta instancia el spider específico de A Cuenta usando el catálogo central.
func nuevoAcuenta(sitio Site, obtenedor Fetcher, rutaCategorias string, maxPaginas int, registrador *slog.Logger) *Acuenta {
	return &Acuenta{spiderBase: nuevoSpiderBase(obtenedor, rutaCategorias, maxPaginas, registrador, sitio, configuracionSpider{
		extraerProductos: func(cuerpo []byte, urlRespuesta *url.URL) []dominio.Product {
			return AcuentaProducts(sitio, cuerpo, urlRespuesta)
		},
		extraerProducto: func(cuerpo []byte, urlRespuesta *url.URL, solicitado string) *dominio.Product {
			return AcuentaProduct(sitio, cuerpo, solicitado, urlRespuesta)
		},
	})}
}

// ===== Expresiones Regulares de A Cuenta =====

var (
	patronInicioReferenciaAcuenta = regexp.MustCompile(`(?:^|\\n|\n)([a-z0-9]+):`)
	patronTipoPromocionAcuenta    = regexp.MustCompile(`\\"type\\":\\"([^"\\]+)\\"`)
	patronCondicionesAcuenta      = regexp.MustCompile(`\\"conditions\\":\\"\$([a-z0-9]+)\\"`)
	patronReferenciaCondicion     = regexp.MustCompile(`\\"\$([a-z0-9]+)\\"`)
	patronClavePrecioAcuenta      = regexp.MustCompile(`\\"priceBeforeTaxes\\":(-?\d+)`)
	patronReferenciaEANAcuenta    = regexp.MustCompile(`(?:^|\\n|\n)([a-z0-9]+):\[\\?"(\d{8,14})`)
	patronSKUAcuenta              = regexp.MustCompile(`\\"sku\\":\\"(\d+)\\"`)
	patronSlugAcuenta             = regexp.MustCompile(`\\"slug\\":\\"([^"\\]+)\\"`)
	patronMarcaAcuenta            = regexp.MustCompile(`\\"brand\\":\\"([^"\\]+)\\"`)
	patronStockAcuenta            = regexp.MustCompile(`\\"stock\\":(-?\d+)`)
	patronNombreAcuenta           = regexp.MustCompile(`\\"name\\":\\"([^"\\]+)\\"`)
	patronCampoEANAcuenta         = regexp.MustCompile(`\\"ean\\":(.*?),\\"maxQty\\"`)
	patronDigitosEANAcuenta       = regexp.MustCompile(`\d{8,14}`)
	patronRefEANAcuenta           = regexp.MustCompile(`\\"\$([a-z0-9]+)\\"`)
	patronRefPromocionAcuenta     = regexp.MustCompile(`\\"promotion\\":\\"\$([a-z0-9]+)\\"`)
	patronRefFotosAcuenta         = regexp.MustCompile(`\\"photosUrl\\":\\"\$([a-z0-9]+)\\"`)
	patronPrecioMultipleAcuenta   = regexp.MustCompile(`\b(\d+)\s*[xX]\s*\$\s*([\d.,]+)`)
	patronMontoAcuenta            = regexp.MustCompile(`\$\s*([\d.,]+)`)
	patronFormatoAcuenta          = regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*(?:kg|g|gr|ml|lt|l|un(?:idad(?:es)?)?)\b`)
	patronPalabraAcuenta          = regexp.MustCompile(`[a-záéíóúñ0-9]+`)
	patronDigitosFinalesAcuenta   = regexp.MustCompile(`-\d+$`)
)

// ===== Extracción Principal =====

// AcuentaProducts usa el payload RSC y completa precios visibles; si no hay
// RSC, parsea las tarjetas del HTML.
func AcuentaProducts(sitio Site, cuerpo []byte, urlRespuesta *url.URL) []dominio.Product {
	productosRSC := productosRSCAcuenta(sitio, cuerpo, urlRespuesta)
	if len(productosRSC) > 0 {
		return fusionarPreciosVisiblesAcuenta(sitio, cuerpo, urlRespuesta, productosRSC)
	}
	return tarjetasAcuenta(sitio, cuerpo, urlRespuesta)
}

// AcuentaProduct elige la ficha solicitada dentro de los productos.
func AcuentaProduct(sitio Site, cuerpo []byte, solicitado string, urlRespuesta *url.URL) *dominio.Product {
	return SelectProductByURL(AcuentaProducts(sitio, cuerpo, urlRespuesta), solicitado, urlRespuesta)
}

// ===== Extracción HTML DOM =====

// tarjetasAcuenta extrae productos a partir de las tarjetas del catálogo HTML de Acuenta.
func tarjetasAcuenta(sitio Site, cuerpo []byte, urlRespuesta *url.URL) []dominio.Product {
	selectorEnlace := sitio.Selector("card_link")
	if selectorEnlace == "" {
		return nil
	}
	documento, errorObtenido := goquery.NewDocumentFromReader(bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return nil
	}
	var productos []dominio.Product
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

		tarjeta := tarjetaAcuentaDesdeEnlace(elemento)
		if tarjeta.Length() == 0 {
			return
		}
		nombre := dominio.Texto(todoElTexto(elemento))
		texto := dominio.Texto(todoElTexto(tarjeta))
		if nombre == nil || texto == nil {
			return
		}
		actual, normal, promocion := preciosAcuentaDesdeTexto(*texto)
		if actual == nil || normal == nil {
			return
		}

		imagen := primerAtributo(tarjeta, sitio.Selector("image"), "src")
		if imagen == "" {
			imagen = primerAtributo(tarjeta, sitio.Selector("image"), "data-src")
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
		sku := skuAcuentaDesdeURL(sitio, urlProducto)
		productos = append(productos, dominio.Product{
			Producto:          *nombre,
			Precio:            actual,
			PrecioNormal:      normal,
			PrecioOferta:      oferta,
			SKU:               sku,
			Marca:             marcaAcuentaDesdeTexto(*texto, sitio.List("brands")),
			FormatoCrudo:      FormatFromText(*nombre, patronFormatoAcuenta),
			MecanicaPromocion: promocion,
			URLProducto:       &urlProducto,
			Imagen:            urlImagen,
		})
	})
	return productos
}

// fusionarPreciosVisiblesAcuenta sincroniza precios visibles en el DOM de Acuenta.
func fusionarPreciosVisiblesAcuenta(sitio Site, cuerpo []byte, urlRespuesta *url.URL, productos []dominio.Product) []dominio.Product {
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
		tarjeta := tarjetaAcuentaDesdeEnlace(elemento)
		if tarjeta.Length() == 0 {
			return
		}
		texto := dominio.Texto(todoElTexto(tarjeta))
		if texto == nil {
			return
		}
		actual, normal, promocion := preciosAcuentaDesdeTexto(*texto)
		if actual == nil || normal == nil {
			return
		}
		producto := &productos[indice]
		if *actual < *normal {
			producto.Precio = actual
			producto.PrecioNormal = normal
			producto.PrecioOferta = actual
			if producto.MecanicaPromocion == nil {
				producto.MecanicaPromocion = dominio.StringPtr(sitio.Setting("promotion"))
			}
		}
		if promocion != nil {
			producto.MecanicaPromocion = promocion
		}
	})
	return productos
}

// tarjetaAcuentaDesdeEnlace sube desde el enlace hasta el contenedor que muestra un
// precio.
func tarjetaAcuentaDesdeEnlace(enlace *goquery.Selection) *goquery.Selection {
	tarjeta := enlace.Parent()
	for tarjeta.Length() > 0 && !strings.Contains(todoElTexto(tarjeta), "$") {
		tarjeta = tarjeta.Parent()
	}
	return tarjeta
}

// ===== Extracción React Server Components (RSC) =====

// productosRSCAcuenta procesa el flujo React Server Components (RSC) de Acuenta.
func productosRSCAcuenta(sitio Site, cuerpo []byte, urlRespuesta *url.URL) []dominio.Product {
	texto := string(cuerpo)
	preciosPromocion := preciosPromocionAcuenta(texto)
	referenciasEAN := map[string]string{}
	for _, coincidencia := range patronReferenciaEANAcuenta.FindAllStringSubmatch(texto, -1) {
		referenciasEAN[coincidencia[1]] = coincidencia[2]
	}

	var productos []dominio.Product
	skusVistos := map[string]struct{}{}
	for _, coincidencia := range patronSKUAcuenta.FindAllStringSubmatchIndex(texto, -1) {
		sku := texto[coincidencia[2]:coincidencia[3]]
		if _, valido := skusVistos[sku]; valido {
			continue
		}
		fin := coincidencia[1] + sitio.Limit("rsc_window")
		if fin > len(texto) {
			fin = len(texto)
		}
		ventana := texto[coincidencia[0]:fin]

		coincidenciaSlug := patronSlugAcuenta.FindStringSubmatch(ventana)
		coincidenciaPrecio := patronClavePrecioAcuenta.FindStringSubmatch(ventana)
		if len(coincidenciaSlug) < 2 || len(coincidenciaPrecio) < 2 {
			continue
		}
		slug := coincidenciaSlug[1]
		precioNormal := dominio.PrecioCLP(coincidenciaPrecio[1])
		if precioNormal == nil {
			continue
		}

		var nombre *string
		if coincidenciaNombre := patronNombreAcuenta.FindStringSubmatch(ventana); len(coincidenciaNombre) >= 2 {
			nombre = dominio.Texto(coincidenciaNombre[1])
		}
		if nombre == nil || !nombreCoincideConSlugAcuenta(sitio, *nombre, slug) {
			nombre = nombreDesdeSlugAcuenta(slug)
		}
		if nombre == nil {
			continue
		}

		var ean *string
		if campoEAN := patronCampoEANAcuenta.FindStringSubmatch(ventana); len(campoEAN) >= 2 {
			valoresEAN := patronDigitosEANAcuenta.FindAllString(campoEAN[1], -1)
			if len(valoresEAN) == 0 {
				if referencia := patronRefEANAcuenta.FindStringSubmatch(campoEAN[1]); len(referencia) >= 2 {
					if valor, valido := referenciasEAN[referencia[1]]; valido {
						valoresEAN = []string{valor}
					}
				}
			}
			if len(valoresEAN) > 0 {
				ean = dominio.EAN(valoresEAN[0])
			}
		}

		referenciaPromocion := ""
		if coincidenciaPromocion := patronRefPromocionAcuenta.FindStringSubmatch(ventana); len(coincidenciaPromocion) >= 2 {
			referenciaPromocion = coincidenciaPromocion[1]
		}
		precioActual := precioNormal
		if valor, valido := preciosPromocion[referenciaPromocion]; valido {
			precioActual = valor
		}

		imagen := imagenDesdeRSCAcuenta(sitio, texto, sku)
		if imagen == "" {
			imagen = resolverReferenciaImagenAcuenta(texto, ventana)
		}
		if imagen == "" {
			imagen = imagenDesdeSKUAcuenta(sitio, sku)
		}
		urlImagen := urlImagenCanonicaAcuenta(imagen)

		urlProducto := ResolveURL(urlRespuesta, "/p/"+slug)
		var existencias *bool
		if coincidenciaStock := patronStockAcuenta.FindStringSubmatch(ventana); len(coincidenciaStock) >= 2 {
			if cantidad, errorObtenido := strconv.Atoi(coincidenciaStock[1]); errorObtenido == nil {
				existencias = dominio.BoolPtr(cantidad > 0)
			}
		}
		var marca *string
		if coincidenciaMarca := patronMarcaAcuenta.FindStringSubmatch(ventana); len(coincidenciaMarca) >= 2 {
			marca = dominio.Texto(coincidenciaMarca[1])
		}
		var oferta *float64
		if *precioActual < *precioNormal {
			oferta = precioActual
		}
		var mecanicas *string
		if referenciaPromocion != "" {
			mecanicas = dominio.StringPtr(sitio.Setting("promotion"))
		}
		skusVistos[sku] = struct{}{}
		productos = append(productos, dominio.Product{
			Producto:          *nombre,
			Precio:            precioActual,
			PrecioNormal:      precioNormal,
			PrecioOferta:      oferta,
			EanGtin:           ean,
			SKU:               &sku,
			Marca:             marca,
			FormatoCrudo:      FormatFromText(*nombre, patronFormatoAcuenta),
			MecanicaPromocion: mecanicas,
			EnStock:           existencias,
			URLProducto:       &urlProducto,
			Imagen:            urlImagen,
		})
	}
	return productos
}

// preciosPromocionAcuenta extrae precios promocionales desde respuestas RSC de Acuenta.
func preciosPromocionAcuenta(texto string) map[string]*float64 {
	inicios := patronInicioReferenciaAcuenta.FindAllStringSubmatchIndex(texto, -1)
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
		coincidenciaTipo := patronTipoPromocionAcuenta.FindStringSubmatch(datos)
		if len(coincidenciaTipo) < 2 || coincidenciaTipo[1] != "specialPrice" {
			continue
		}
		coincidenciasCondiciones := patronCondicionesAcuenta.FindStringSubmatch(datos)
		if len(coincidenciasCondiciones) < 2 {
			continue
		}
		condiciones := referencias[coincidenciasCondiciones[1]]
		if condiciones == "" {
			continue
		}
		var precios []float64
		for _, referenciaCondicion := range patronReferenciaCondicion.FindAllStringSubmatch(condiciones, -1) {
			datosCondicion := referencias[referenciaCondicion[1]]
			coincidenciaPrecio := patronClavePrecioAcuenta.FindStringSubmatch(datosCondicion)
			if len(coincidenciaPrecio) < 2 {
				continue
			}
			if precio := dominio.PrecioCLP(coincidenciaPrecio[1]); precio != nil {
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

// imagenDesdeRSCAcuenta localiza la imagen del producto en el payload RSC.
func imagenDesdeRSCAcuenta(sitio Site, texto, sku string) string {
	patron := regexp.MustCompile(
		`(?i)` + regexp.QuoteMeta(sitio.Setting("image_base")) +
			regexp.QuoteMeta(sku) +
			`[^"\s]*?\.(?:jpe?g|png|webp)\](?:\\u0026|&)sink`,
	)
	coincidencia := patron.FindString(texto)
	return strings.ReplaceAll(coincidencia, `\u0026`, "&")
}

// resolverReferenciaImagenAcuenta resuelve referencias a imágenes anidadas en RSC.
func resolverReferenciaImagenAcuenta(texto, ventana string) string {
	coincidenciaReferencia := patronRefFotosAcuenta.FindStringSubmatch(ventana)
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

// imagenDesdeSKUAcuenta construye la URL canónica de imagen desde el SKU.
func imagenDesdeSKUAcuenta(sitio Site, sku string) string {
	return sitio.Setting("image_base") + sku + sitio.Setting("image_suffix")
}

// urlImagenCanonicaAcuenta normaliza y estandariza la URL de una imagen.
func urlImagenCanonicaAcuenta(valor string) *string {
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

// nombreCoincideConSlugAcuenta verifica concordancia entre el nombre y el slug de la URL.
func nombreCoincideConSlugAcuenta(sitio Site, nombre, slug string) bool {
	tokensNombre := tokensAcuenta(sitio, nombre)
	tokensSlug := tokensAcuenta(sitio, slug)
	interseccion := 0
	for token := range tokensNombre {
		if _, valido := tokensSlug[token]; valido {
			interseccion++
		}
	}
	return interseccion >= sitio.Limit("name_min_intersection")
}

// tokensAcuenta segmenta una cadena en palabras clave para comparación.
func tokensAcuenta(sitio Site, valor string) map[string]struct{} {
	fichas := map[string]struct{}{}
	for _, coincidencia := range patronPalabraAcuenta.FindAllString(valor, -1) {
		token := strings.ToLower(coincidencia)
		if len([]rune(token)) >= sitio.Limit("token_min_length") {
			fichas[token] = struct{}{}
		}
	}
	return fichas
}

// nombreDesdeSlugAcuenta infiere el nombre del producto basándose en el slug de la URL.
func nombreDesdeSlugAcuenta(slug string) *string {
	legible := patronDigitosFinalesAcuenta.ReplaceAllString(slug, "")
	return dominio.Texto(TitleCase(strings.ReplaceAll(legible, "-", " ")))
}

// ===== Extracción de Precios y Marca =====

// preciosAcuentaDesdeTexto extrae valores de precio normal y oferta desde un texto.
func preciosAcuentaDesdeTexto(texto string) (*float64, *float64, *string) {
	if multiple := patronPrecioMultipleAcuenta.FindStringSubmatchIndex(texto); multiple != nil {
		var montosPrefijo []float64
		for _, coincidencia := range patronMontoAcuenta.FindAllStringSubmatch(texto[:multiple[0]], -1) {
			if monto := dominio.PrecioCLP(coincidencia[1]); monto != nil {
				montosPrefijo = append(montosPrefijo, *monto)
			}
		}
		var actual *float64
		if len(montosPrefijo) > 0 {
			actual = &montosPrefijo[len(montosPrefijo)-1]
		} else {
			actual = dominio.PrecioCLP(texto[multiple[4]:multiple[5]])
		}
		if actual != nil {
			promocion := texto[multiple[0]:multiple[1]]
			return actual, actual, &promocion
		}
	}

	var montos []float64
	for _, coincidencia := range patronMontoAcuenta.FindAllStringSubmatch(texto, -1) {
		if monto := dominio.PrecioCLP(coincidencia[1]); monto != nil {
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

// marcaAcuentaDesdeTexto extrae la marca comercial desde la descripción o ficha.
func marcaAcuentaDesdeTexto(texto string, marcas []string) *string {
	for _, marca := range marcas {
		patron := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(marca) + `\b`)
		if coincidencia := patron.FindString(texto); coincidencia != "" {
			return dominio.Texto(coincidencia)
		}
	}
	return nil
}

// skuAcuentaDesdeURL extrae el identificador SKU numérico de la URL del producto.
func skuAcuentaDesdeURL(sitio Site, urlProducto string) *string {
	slug := ultimoSegmentoRuta(urlProducto)
	prefijo := sitio.Setting("sku_prefix")
	if coincidencia := patronDigitosFinalesAcuenta.FindString(slug); coincidencia != "" {
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
