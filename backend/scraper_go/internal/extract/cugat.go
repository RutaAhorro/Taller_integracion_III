package extract

import (
	"bytes"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

// ===== Extracción Principal =====

// CugatProducts extrae las tarjetas del listado de WooCommerce.
func CugatProducts(sitio catalog.Site, cuerpo []byte, urlRespuesta *url.URL) []domain.Product {
	selectorTarjeta := sitio.Selector("product_card")
	if selectorTarjeta == "" {
		return nil
	}
	documento, errorObtenido := goquery.NewDocumentFromReader(bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return nil
	}
	var productos []domain.Product
	documento.Find(selectorTarjeta).Each(func(_ int, tarjeta *goquery.Selection) {
		nombre := normalize.Texto(firstText(tarjeta, sitio.Selector("title")))
		if nombre == nil {
			return
		}
		precioOferta := cugatPriceFrom(tarjeta, sitio, sitio.Selector("price_offer"))
		precioNormal := cugatPriceFrom(tarjeta, sitio, sitio.Selector("price_normal"))
		if precioOferta == nil {
			precioOferta = cugatPriceFrom(tarjeta, sitio, sitio.Selector("price_fallback"))
		}
		if precioNormal == nil {
			precioNormal = precioOferta
		}

		enlace := firstAttr(tarjeta, sitio.Selector("title"), "href")
		imagen := firstAttr(tarjeta, sitio.Selector("image"), "data-src")
		if imagen == "" {
			imagen = firstAttr(tarjeta, sitio.Selector("image"), "data-lazy-src")
		}
		if imagen == "" {
			imagen = firstAttr(tarjeta, sitio.Selector("noscript")+" img", "src")
		}
		if imagen == "" {
			imagen = noscriptImage(tarjeta, sitio.Selector("noscript"))
		}
		if imagen == "" {
			imagen = firstAttr(tarjeta, sitio.Selector("image"), "src")
		}

		carrito := tarjeta.Find(sitio.Selector("cart")).First()
		skuCarrito, _ := carrito.Attr("data-product_sku")
		productoID, _ := carrito.Attr("data-product_id")
		if productoID == "" {
			tarjetaID, _ := tarjeta.Attr("id")
			productoID = strings.TrimPrefix(tarjetaID, sitio.Setting("card_id_prefix"))
		}
		ean := normalize.EAN(skuCarrito)
		sku := firstNonNilString(nonEmptyStringPtr(productoID), ean)

		var urlProducto *string
		if enlace != "" {
			valor := ResolveURL(urlRespuesta, enlace)
			urlProducto = &valor
		}
		var urlImagen *string
		if imagen != "" {
			valor := ResolveURL(urlRespuesta, imagen)
			urlImagen = &valor
		}
		if sku == nil && urlProducto != nil {
			sku = normalize.SKUFromURL(*urlProducto)
		}

		var oferta *float64
		if precioNormal != nil && precioOferta != nil && *precioOferta < *precioNormal {
			oferta = precioOferta
		}
		clase, _ := tarjeta.Attr("class")
		productos = append(productos, domain.Product{
			Producto:     *nombre,
			Precio:       precioOferta,
			PrecioNormal: precioNormal,
			PrecioOferta: oferta,
			EanGtin:      ean,
			SKU:          sku,
			EnStock:      domain.BoolPtr(hasClass(clase, sitio.Setting("stock_class"))),
			URLProducto:  urlProducto,
			Imagen:       urlImagen,
		})
	})
	return productos
}

// ===== Ficha Individual =====

// CugatProduct lee la ficha individual desde JSON-LD.
func CugatProduct(sitio catalog.Site, cuerpo []byte, urlRespuesta *url.URL) *domain.Product {
	return ExtractJSONLDProduct(sitio, cuerpo, urlRespuesta)
}

// ===== Utilidades =====

// cugatPriceFrom analiza y extrae el precio numérico en pesos desde Cugat.
func cugatPriceFrom(tarjeta *goquery.Selection, sitio catalog.Site, selector string) *float64 {
	if selector == "" {
		return nil
	}
	monto := tarjeta.Find(selector).Find(sitio.Selector("price_amount")).First().Text()
	return normalize.PrecioCLP(monto)
}
