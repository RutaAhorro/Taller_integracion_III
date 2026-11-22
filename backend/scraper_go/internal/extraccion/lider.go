// lider.go implementa el spider de Lider y sus parsers de __NEXT_DATA__,
// JSON-LD y tarjetas HTML de respaldo.
package extraccion

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
)

// ===== Estructura y Constructor =====

// Lider ejecuta el spider de Lider.
type Lider struct {
	spiderBase
}

// nuevoLider instancia el spider específico de Lider usando el catálogo central.
func nuevoLider(sitio Site, obtenedor Fetcher, rutaCategorias string, maxPaginas int, registrador *slog.Logger) *Lider {
	return &Lider{spiderBase: nuevoSpiderBase(obtenedor, rutaCategorias, maxPaginas, registrador, sitio, configuracionSpider{
		extraerProductos: func(cuerpo []byte, urlRespuesta *url.URL) []dominio.Product {
			return LiderProducts(sitio, cuerpo, urlRespuesta)
		},
		extraerProducto: func(cuerpo []byte, urlRespuesta *url.URL, solicitado string) *dominio.Product {
			return LiderProduct(sitio, cuerpo, solicitado, urlRespuesta)
		},
	})}
}

// ===== Extracción Principal =====

var patronFormatoLider = regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*(?:kg|grs?|g|ml|lt|l|un(?:idades?)?)\b`)

// LiderProducts intenta __NEXT_DATA__ y cae a las tarjetas del HTML.
func LiderProducts(sitio Site, cuerpo []byte, urlRespuesta *url.URL) []dominio.Product {
	documento, errorObtenido := goquery.NewDocumentFromReader(bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return nil
	}
	if productos, encontrado := productosNextDataLider(sitio, documento, urlRespuesta); encontrado {
		return productos
	}
	return tarjetasLider(sitio, documento, urlRespuesta)
}

// LiderProduct reconstruye una ficha /ip/. Prueba, en orden, JSON-LD, el objeto
// de producto de __NEXT_DATA__ y las tarjetas del HTML. Sólo acepta un
// candidato cuya URL coincida con la solicitada (o el único disponible).
func LiderProduct(sitio Site, cuerpo []byte, solicitado string, urlRespuesta *url.URL) *dominio.Product {
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
	if carga, valido := cargaNextDataLider(sitio, documento); valido {
		var productos []dominio.Product
		for _, valor := range recorrerJSON(carga) {
			elemento, valido := valor.(map[string]any)
			if !valido {
				continue
			}
			if producto, valido := productoDesdeElementoLider(sitio, elemento, urlRespuesta); valido {
				productos = append(productos, producto)
			}
		}
		if seleccionado := SelectProductByURL(productos, solicitado, urlRespuesta); seleccionado != nil {
			return seleccionado
		}
	}
	return SelectProductByURL(tarjetasLider(sitio, documento, urlRespuesta), solicitado, urlRespuesta)
}

// ===== Extracción __NEXT_DATA__ =====

// productosNextDataLider extrae productos desde el bloque __NEXT_DATA__ en Lider.
func productosNextDataLider(sitio Site, documento *goquery.Document, urlRespuesta *url.URL) ([]dominio.Product, bool) {
	carga, valido := cargaNextDataLider(sitio, documento)
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

	productos := make([]dominio.Product, 0)
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
			if producto, valido := productoDesdeElementoLider(sitio, elemento, urlRespuesta); valido {
				productos = append(productos, producto)
			}
		}
	}
	return productos, true
}

// cargaNextDataLider parsea el contenido JSON de __NEXT_DATA__.
func cargaNextDataLider(sitio Site, documento *goquery.Document) (map[string]any, bool) {
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

// productoDesdeElementoLider mapea un ítem del JSON de Lider a dominio.Product.
func productoDesdeElementoLider(sitio Site, elemento map[string]any, urlRespuesta *url.URL) (dominio.Product, bool) {
	nombre := dominio.Texto(elemento["name"])
	codigoProducto := dominio.EAN(elemento["usItemId"])
	precioActual := dominio.PrecioCLP(elemento["price"])
	if nombre == nil || codigoProducto == nil || precioActual == nil {
		return dominio.Product{}, false
	}

	informacionPrecio, _ := elemento["priceInfo"].(map[string]any)
	precioNormal := dominio.PrecioCLP(primerValor(informacionPrecio, "wasPrice"))
	if precioNormal == nil {
		precioNormal = dominio.PrecioCLP(primerValor(informacionPrecio, "itemPrice"))
	}
	if precioNormal == nil {
		precioNormal = precioActual
	}

	canonica, valido := elemento["canonicalUrl"].(string)
	if !valido || canonica == "" {
		return dominio.Product{}, false
	}
	urlProducto := ResolveURL(urlRespuesta, canonica)
	if errorObtenido := sitio.ValidateProductURL(urlProducto); errorObtenido != nil {
		return dominio.Product{}, false
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

	sku := dominio.Texto(elemento["sku"])
	if sku == nil {
		sku = codigoProducto
	}
	var oferta *float64
	var mecanicas *string
	if *precioActual < *precioNormal {
		oferta = precioActual
		mecanicas = dominio.StringPtr(sitio.Setting("promotion"))
	}
	return dominio.Product{
		Producto:          *nombre,
		Precio:            precioActual,
		PrecioNormal:      precioNormal,
		PrecioOferta:      oferta,
		EanGtin:           codigoProducto,
		SKU:               sku,
		Marca:             dominio.Texto(elemento["brand"]),
		FormatoCrudo:      FormatFromText(*nombre, patronFormatoLider),
		MecanicaPromocion: mecanicas,
		EnStock:           conStock,
		URLProducto:       &urlProducto,
		Imagen:            imagen,
	}, true
}

// ===== Extracción Tarjetas HTML =====

// tarjetasLider extrae productos a partir de las tarjetas del catálogo HTML de Lider.
func tarjetasLider(sitio Site, documento *goquery.Document, urlRespuesta *url.URL) []dominio.Product {
	tarjetas := documento.Find(sitio.Selector("product_card"))
	if tarjetas.Length() == 0 {
		tarjetas = documento.Find(sitio.Selector("product_card_fallback"))
	}

	var productos []dominio.Product
	tarjetas.Each(func(_ int, tarjeta *goquery.Selection) {
		nombre := dominio.Texto(todoElTexto(tarjeta.Find(sitio.Selector("card_title"))))
		enlace := primerAtributo(tarjeta, sitio.Selector("product_link"), "href")
		if nombre == nil || enlace == "" {
			return
		}
		urlProducto := ResolveURL(urlRespuesta, enlace)
		if errorObtenido := sitio.ValidateProductURL(urlProducto); errorObtenido != nil {
			return
		}

		bloquePrecio := tarjeta.Find(sitio.Selector("price_block")).First()
		precioActual := dominio.PrecioCLP(primerTexto(bloquePrecio, sitio.Selector("price_current")))
		if precioActual == nil {
			return
		}
		precioNormal := dominio.PrecioCLP(primerTexto(bloquePrecio, sitio.Selector("price_normal")))
		if precioNormal == nil {
			precioNormal = precioActual
		}

		codigoCrudo := tarjeta.AttrOr("data-dca-id", "")
		if codigoCrudo == "" {
			codigoCrudo = ultimoSegmentoRuta(urlProducto)
		}
		codigoProducto := dominio.EAN(codigoCrudo)
		sku := primerPunteroCadenaNoNulo(codigoProducto, punteroCadenaNoVacia(tarjeta.AttrOr("data-item-id", "")))

		imagen := primerAtributo(tarjeta, sitio.Selector("image"), "src")
		if imagen == "" {
			imagen = primerAtributo(tarjeta, "img", "data-src")
		}
		if imagen == "" {
			imagen = primerAtributo(tarjeta, "img", "src")
		}
		var urlImagen *string
		if imagen != "" {
			valor := ResolveURL(urlRespuesta, imagen)
			urlImagen = &valor
		}

		textoTarjeta := strings.ToLower(todoElTexto(tarjeta))
		var conStock *bool
		switch {
		case contieneAlguno(textoTarjeta, sitio.List("stock_in")):
			conStock = dominio.BoolPtr(true)
		case contieneAlguno(textoTarjeta, sitio.List("stock_out")):
			conStock = dominio.BoolPtr(false)
		}

		var oferta *float64
		var mecanicas *string
		if *precioActual < *precioNormal {
			oferta = precioActual
			mecanicas = dominio.StringPtr(sitio.Setting("promotion"))
		}
		productos = append(productos, dominio.Product{
			Producto:          *nombre,
			Precio:            precioActual,
			PrecioNormal:      precioNormal,
			PrecioOferta:      oferta,
			EanGtin:           codigoProducto,
			SKU:               sku,
			Marca:             dominio.Texto(primerTexto(tarjeta, sitio.Selector("brand"))),
			FormatoCrudo:      FormatFromText(*nombre, patronFormatoLider),
			MecanicaPromocion: mecanicas,
			EnStock:           conStock,
			URLProducto:       &urlProducto,
			Imagen:            urlImagen,
		})
	})
	return productos
}

// ===== Utilidades =====

// contieneAlguno determina si una cadena contiene alguna de las subcadenas indicadas.
func contieneAlguno(texto string, valores []string) bool {
	for _, valor := range valores {
		if valor != "" && strings.Contains(texto, valor) {
			return true
		}
	}
	return false
}
