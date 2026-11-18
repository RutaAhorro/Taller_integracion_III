package extract

import (
	"bytes"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

var liderFormatPattern = regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*(?:kg|grs?|g|ml|lt|l|un(?:idades?)?)\b`)

// ===== Extracción Principal =====

// LiderProducts intenta __NEXT_DATA__ y cae a las tarjetas del HTML.
func LiderProducts(sitio catalog.Site, cuerpo []byte, urlRespuesta *url.URL) []domain.Product {
	documento, errorObtenido := goquery.NewDocumentFromReader(bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return nil
	}
	if productos, encontrado := liderNextDataProducts(sitio, documento, urlRespuesta); encontrado {
		return productos
	}
	return liderCards(sitio, documento, urlRespuesta)
}

// LiderProduct reconstruye una ficha /ip/. Prueba, en orden, JSON-LD, el objeto
// de producto de __NEXT_DATA__ y las tarjetas del HTML. Sólo acepta un
// candidato cuya URL coincida con la solicitada (o el único disponible).
func LiderProduct(sitio catalog.Site, cuerpo []byte, solicitado string, urlRespuesta *url.URL) *domain.Product {
	if productos := ExtractJSONLDProducts(sitio, cuerpo, urlRespuesta); len(productos) > 0 {
		if seleccionado := SelectProductByURL(productos, solicitado, urlRespuesta); seleccionado != nil {
			if resuelto := ResolveURL(urlRespuesta, *seleccionado.URLProducto); sitio.ValidateProductURL(resuelto) == nil {
				seleccionado.URLProducto = &resuelto
				return seleccionado
			}
		}
	}

	documento, errorObtenido := goquery.NewDocumentFromReader(bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return nil
	}
	if carga, valido := liderNextDataPayload(sitio, documento); valido {
		var productos []domain.Product
		for _, valor := range walkJSON(carga) {
			elemento, valido := valor.(map[string]any)
			if !valido {
				continue
			}
			if producto, valido := liderProductFromItem(sitio, elemento, urlRespuesta); valido {
				productos = append(productos, producto)
			}
		}
		if seleccionado := SelectProductByURL(productos, solicitado, urlRespuesta); seleccionado != nil {
			return seleccionado
		}
	}
	return SelectProductByURL(liderCards(sitio, documento, urlRespuesta), solicitado, urlRespuesta)
}

// ===== Extracción __NEXT_DATA__ =====

// liderNextDataProducts extrae productos desde el bloque __NEXT_DATA__ en Lider.
func liderNextDataProducts(sitio catalog.Site, documento *goquery.Document, urlRespuesta *url.URL) ([]domain.Product, bool) {
	carga, valido := liderNextDataPayload(sitio, documento)
	if !valido {
		return nil, false
	}

	propiedades, _ := carga["props"].(map[string]any)
	propiedadesPagina, _ := propiedades["pageProps"].(map[string]any)
	datosIniciales, _ := propiedadesPagina["initialData"].(map[string]any)
	resultadoBusqueda, _ := datosIniciales["searchResult"].(map[string]any)
	pilasElementos, valido := resultadoBusqueda["itemStacks"].([]any)
	if !valido {
		return nil, false
	}

	productos := make([]domain.Product, 0)
	for _, pilaCruda := range pilasElementos {
		pila, valido := pilaCruda.(map[string]any)
		if !valido {
			continue
		}
		elementos, valido := pila["items"].([]any)
		if !valido {
			continue
		}
		for _, elementoCrudo := range elementos {
			elemento, valido := elementoCrudo.(map[string]any)
			if !valido {
				continue
			}
			if producto, valido := liderProductFromItem(sitio, elemento, urlRespuesta); valido {
				productos = append(productos, producto)
			}
		}
	}
	return productos, true
}

// liderNextDataPayload parsea el contenido JSON de __NEXT_DATA__.
func liderNextDataPayload(sitio catalog.Site, documento *goquery.Document) (map[string]any, bool) {
	selector := sitio.Selector("next_data")
	if selector == "" {
		return nil, false
	}
	guion := documento.Find(selector).First()
	if guion.Length() == 0 {
		return nil, false
	}
	var carga map[string]any
	decodificador := json.NewDecoder(strings.NewReader(guion.Text()))
	decodificador.UseNumber()
	if errorObtenido := decodificador.Decode(&carga); errorObtenido != nil {
		return nil, false
	}
	return carga, true
}

// liderProductFromItem mapea un ítem del JSON de Lider a domain.Product.
func liderProductFromItem(sitio catalog.Site, elemento map[string]any, urlRespuesta *url.URL) (domain.Product, bool) {
	nombre := normalize.Texto(elemento["name"])
	codigoProducto := normalize.EAN(elemento["usItemId"])
	precioActual := normalize.PrecioCLP(elemento["price"])
	if nombre == nil || codigoProducto == nil || precioActual == nil {
		return domain.Product{}, false
	}

	informacionPrecio, _ := elemento["priceInfo"].(map[string]any)
	precioNormal := normalize.PrecioCLP(firstValue(informacionPrecio, "wasPrice"))
	if precioNormal == nil {
		precioNormal = normalize.PrecioCLP(firstValue(informacionPrecio, "itemPrice"))
	}
	if precioNormal == nil {
		precioNormal = precioActual
	}

	canonica, valido := elemento["canonicalUrl"].(string)
	if !valido || canonica == "" {
		return domain.Product{}, false
	}
	urlProducto := ResolveURL(urlRespuesta, canonica)
	if errorObtenido := sitio.ValidateProductURL(urlProducto); errorObtenido != nil {
		return domain.Product{}, false
	}

	var imagen *string
	informacionImagen, _ := elemento["imageInfo"].(map[string]any)
	if miniatura, valido := informacionImagen["thumbnailUrl"].(string); valido && miniatura != "" {
		valor := ResolveURL(urlRespuesta, miniatura)
		imagen = &valor
	}

	var conStock *bool
	if sinStock, valido := elemento["isOutOfStock"].(bool); valido {
		valor := !sinStock
		conStock = &valor
	}

	sku := normalize.Texto(elemento["sku"])
	if sku == nil {
		sku = codigoProducto
	}
	var oferta *float64
	var mecanicas *string
	if *precioActual < *precioNormal {
		oferta = precioActual
		mecanicas = domain.StringPtr(sitio.Setting("promotion"))
	}
	return domain.Product{
		Producto:          *nombre,
		Precio:            precioActual,
		PrecioNormal:      precioNormal,
		PrecioOferta:      oferta,
		EanGtin:           codigoProducto,
		SKU:               sku,
		Marca:             normalize.Texto(elemento["brand"]),
		FormatoCrudo:      FormatFromText(*nombre, liderFormatPattern),
		MecanicaPromocion: mecanicas,
		EnStock:           conStock,
		URLProducto:       &urlProducto,
		Imagen:            imagen,
	}, true
}

// ===== Extracción Tarjetas HTML =====

// liderCards extrae productos a partir de las tarjetas del catálogo HTML de Lider.
func liderCards(sitio catalog.Site, documento *goquery.Document, urlRespuesta *url.URL) []domain.Product {
	tarjetas := documento.Find(sitio.Selector("product_card"))
	if tarjetas.Length() == 0 {
		tarjetas = documento.Find(sitio.Selector("product_card_fallback"))
	}

	var productos []domain.Product
	tarjetas.Each(func(_ int, tarjeta *goquery.Selection) {
		nombre := normalize.Texto(allText(tarjeta.Find(sitio.Selector("card_title"))))
		enlace := firstAttr(tarjeta, sitio.Selector("product_link"), "href")
		if nombre == nil || enlace == "" {
			return
		}
		urlProducto := ResolveURL(urlRespuesta, enlace)
		if errorObtenido := sitio.ValidateProductURL(urlProducto); errorObtenido != nil {
			return
		}

		bloquePrecio := tarjeta.Find(sitio.Selector("price_block")).First()
		precioActual := normalize.PrecioCLP(firstText(bloquePrecio, sitio.Selector("price_current")))
		if precioActual == nil {
			return
		}
		precioNormal := normalize.PrecioCLP(firstText(bloquePrecio, sitio.Selector("price_normal")))
		if precioNormal == nil {
			precioNormal = precioActual
		}

		codigoCrudo := tarjeta.AttrOr("data-dca-id", "")
		if codigoCrudo == "" {
			codigoCrudo = lastPathSegment(urlProducto)
		}
		codigoProducto := normalize.EAN(codigoCrudo)
		sku := firstNonNilString(codigoProducto, nonEmptyStringPtr(tarjeta.AttrOr("data-item-id", "")))

		imagen := firstAttr(tarjeta, sitio.Selector("image"), "src")
		if imagen == "" {
			imagen = firstAttr(tarjeta, "img", "data-src")
		}
		if imagen == "" {
			imagen = firstAttr(tarjeta, "img", "src")
		}
		var urlImagen *string
		if imagen != "" {
			valor := ResolveURL(urlRespuesta, imagen)
			urlImagen = &valor
		}

		textoTarjeta := strings.ToLower(allText(tarjeta))
		var conStock *bool
		switch {
		case containsAny(textoTarjeta, sitio.List("stock_in")):
			conStock = domain.BoolPtr(true)
		case containsAny(textoTarjeta, sitio.List("stock_out")):
			conStock = domain.BoolPtr(false)
		}

		var oferta *float64
		var mecanicas *string
		if *precioActual < *precioNormal {
			oferta = precioActual
			mecanicas = domain.StringPtr(sitio.Setting("promotion"))
		}
		productos = append(productos, domain.Product{
			Producto:          *nombre,
			Precio:            precioActual,
			PrecioNormal:      precioNormal,
			PrecioOferta:      oferta,
			EanGtin:           codigoProducto,
			SKU:               sku,
			Marca:             normalize.Texto(firstText(tarjeta, sitio.Selector("brand"))),
			FormatoCrudo:      FormatFromText(*nombre, liderFormatPattern),
			MecanicaPromocion: mecanicas,
			EnStock:           conStock,
			URLProducto:       &urlProducto,
			Imagen:            urlImagen,
		})
	})
	return productos
}

// ===== Utilidades =====

// containsAny determina si una cadena contiene alguna de las subcadenas indicadas.
func containsAny(texto string, valores []string) bool {
	for _, valor := range valores {
		if valor != "" && strings.Contains(texto, valor) {
			return true
		}
	}
	return false
}
