// cugat.go implementa el spider de Cugat y la extracción de tarjetas WooCommerce
// o fichas JSON-LD.
package extraccion

import (
	"bytes"
	"log/slog"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
)

// ===== Estructura y Constructor =====

// Cugat ejecuta el spider de Cugat.
type Cugat struct {
	spiderBase
}

// nuevoCugat instancia el spider específico de Cugat usando el catálogo central.
func nuevoCugat(sitio Site, obtenedor Fetcher, rutaCategorias string, maxPaginas int, registrador *slog.Logger) *Cugat {
	return &Cugat{spiderBase: nuevoSpiderBase(obtenedor, rutaCategorias, maxPaginas, registrador, sitio, configuracionSpider{
		extraerProductos: func(cuerpo []byte, urlRespuesta *url.URL) []dominio.Product {
			return CugatProducts(sitio, cuerpo, urlRespuesta)
		},
		extraerProducto: func(cuerpo []byte, urlRespuesta *url.URL, _ string) *dominio.Product {
			return CugatProduct(sitio, cuerpo, urlRespuesta)
		},
	})}
}

// ===== Extracción Principal =====

// CugatProducts extrae las tarjetas del listado de WooCommerce.
func CugatProducts(sitio Site, cuerpo []byte, urlRespuesta *url.URL) []dominio.Product {
	selectorTarjeta := sitio.Selector("product_card")
	if selectorTarjeta == "" {
		return nil
	}
	documento, errorObtenido := goquery.NewDocumentFromReader(bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return nil
	}
	var productos []dominio.Product
	documento.Find(selectorTarjeta).Each(func(_ int, tarjeta *goquery.Selection) {
		nombre := dominio.Texto(primerTexto(tarjeta, sitio.Selector("title")))
		if nombre == nil {
			return
		}
		precioOferta := precioCugatDesde(tarjeta, sitio, sitio.Selector("price_offer"))
		precioNormal := precioCugatDesde(tarjeta, sitio, sitio.Selector("price_normal"))
		if precioOferta == nil {
			precioOferta = precioCugatDesde(tarjeta, sitio, sitio.Selector("price_fallback"))
		}
		if precioNormal == nil {
			precioNormal = precioOferta
		}

		enlace := primerAtributo(tarjeta, sitio.Selector("title"), "href")
		imagen := primerAtributo(tarjeta, sitio.Selector("image"), "data-src")
		if imagen == "" {
			imagen = primerAtributo(tarjeta, sitio.Selector("image"), "data-lazy-src")
		}
		if imagen == "" {
			imagen = primerAtributo(tarjeta, sitio.Selector("noscript")+" img", "src")
		}
		if imagen == "" {
			imagen = imagenNoscript(tarjeta, sitio.Selector("noscript"))
		}
		if imagen == "" {
			imagen = primerAtributo(tarjeta, sitio.Selector("image"), "src")
		}

		carrito := tarjeta.Find(sitio.Selector("cart")).First()
		skuCarrito, _ := carrito.Attr("data-product_sku")
		productoID, _ := carrito.Attr("data-product_id")
		if productoID == "" {
			tarjetaID, _ := tarjeta.Attr("id")
			productoID = strings.TrimPrefix(tarjetaID, sitio.Setting("card_id_prefix"))
		}
		ean := dominio.EAN(skuCarrito)
		sku := primerPunteroCadenaNoNulo(punteroCadenaNoVacia(productoID), ean)

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
			sku = dominio.SKUFromURL(*urlProducto)
		}

		var oferta *float64
		if precioNormal != nil && precioOferta != nil && *precioOferta < *precioNormal {
			oferta = precioOferta
		}
		clase, _ := tarjeta.Attr("class")
		productos = append(productos, dominio.Product{
			Producto:     *nombre,
			Precio:       precioOferta,
			PrecioNormal: precioNormal,
			PrecioOferta: oferta,
			EanGtin:      ean,
			SKU:          sku,
			EnStock:      dominio.BoolPtr(contieneClase(clase, sitio.Setting("stock_class"))),
			URLProducto:  urlProducto,
			Imagen:       urlImagen,
		})
	})
	return productos
}

// ===== Ficha Individual =====

// CugatProduct lee la ficha individual desde JSON-LD.
func CugatProduct(sitio Site, cuerpo []byte, urlRespuesta *url.URL) *dominio.Product {
	return ExtractJSONLDProduct(sitio, cuerpo, urlRespuesta)
}

// ===== Utilidades =====

// precioCugatDesde analiza y extrae el precio numérico en pesos desde Cugat.
func precioCugatDesde(tarjeta *goquery.Selection, sitio Site, selector string) *float64 {
	if selector == "" {
		return nil
	}
	monto := tarjeta.Find(selector).Find(sitio.Selector("price_amount")).First().Text()
	return dominio.PrecioCLP(monto)
}
