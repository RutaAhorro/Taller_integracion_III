// jumbo.go implementa el spider de Jumbo, su enriquecimiento de EAN desde la
// ficha individual y la extracción de productos desde bloques JSON-LD.
package extraccion

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
)

// ===== Estructura y Constructor =====

// Jumbo ejecuta el spider de Jumbo.
type Jumbo struct {
	spiderBase
}

// nuevoJumbo instancia el spider específico de Jumbo usando el catálogo central.
func nuevoJumbo(sitio Site, obtenedor Fetcher, rutaCategorias string, maxPaginas int, registrador *slog.Logger) *Jumbo {
	spider := &Jumbo{spiderBase: nuevoSpiderBase(obtenedor, rutaCategorias, maxPaginas, registrador, sitio, configuracionSpider{
		extraerProductos: func(cuerpo []byte, _ *url.URL) []dominio.Product {
			return JumboProducts(sitio, cuerpo)
		},
		extraerProducto: func(cuerpo []byte, urlRespuesta *url.URL, solicitado string) *dominio.Product {
			return JumboProduct(sitio, cuerpo, solicitado, urlRespuesta)
		},
	})}
	spider.spiderBase.materializar = spider.enriquecer
	return spider
}

// ===== Enriquecimiento =====

// enriquecer consulta la ficha del producto cuando el listado no publica EAN/GTIN.
func (jumbo *Jumbo) enriquecer(
	contexto context.Context,
	producto dominio.Product,
	categoria string,
	urlRespuesta *url.URL,
	enriquecerEAN bool,
) (dominio.Item, error) {
	urlPagina := textoURL(urlRespuesta)
	if !enriquecerEAN || producto.EanGtin != nil || producto.URLProducto == nil || *producto.URLProducto == "" {
		return jumbo.convertirAItem(producto, categoria, urlPagina), nil
	}

	urlDetalle := ResolveURL(urlRespuesta, *producto.URLProducto)
	if errorObtenido := jumbo.sitio.ValidateProductURL(urlDetalle); errorObtenido != nil {
		jumbo.registrador.Warn("jumbo_ean_url_invalid", "url", urlDetalle, "error", errorObtenido)
		return jumbo.convertirAItem(producto, categoria, urlPagina), nil
	}
	respuesta, errorObtenido := jumbo.obtenedor.Fetch(contexto, urlDetalle)
	if errorObtenido != nil {
		jumbo.registrador.Warn("jumbo_ean_fetch_failed", "url", urlDetalle, "error", errorObtenido)
		return jumbo.convertirAItem(producto, categoria, urlPagina), nil
	}
	if respuesta.Status == http.StatusNotFound {
		return jumbo.convertirAItem(producto, categoria, urlPagina), nil
	}

	enriquecido := producto
	if detalle := JumboProduct(jumbo.sitio, respuesta.Body, urlDetalle, respuesta.URL); detalle != nil {
		if detalle.EanGtin != nil && *detalle.EanGtin != "" {
			enriquecido.EanGtin = detalle.EanGtin
		}
		if detalle.SKU != nil && *detalle.SKU != "" {
			enriquecido.SKU = detalle.SKU
		}
	}
	if enriquecido.EanGtin == nil {
		jumbo.registrador.Info("jumbo_ean_missing", "url", urlDetalle)
	}
	return jumbo.convertirAItem(enriquecido, categoria, urlPagina), nil
}

// ===== Extracción Principal =====

// JumboProducts extrae las fichas schema.org/Product publicadas por Jumbo.
func JumboProducts(sitio Site, cuerpo []byte) []dominio.Product {
	selector := sitio.Selector("json_ld")
	if selector == "" {
		return nil
	}
	documento, errorObtenido := goquery.NewDocumentFromReader(bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return nil
	}

	var productos []dominio.Product
	vistos := make(map[string]struct{})
	documento.Find(selector).Each(func(_ int, seleccion *goquery.Selection) {
		var datos any
		decodificador := json.NewDecoder(strings.NewReader(seleccion.Text()))
		// UseNumber conserva la diferencia entre 1290 y 1290.0, igual que
		// evitar desbordamientos o parsing incorrecto. Importa para no inventar EAN/SKU.
		decodificador.UseNumber()
		if errorObtenido := decodificador.Decode(&datos); errorObtenido != nil {
			return
		}
		for _, valor := range recorrerJSON(datos) {
			entrada, valido := valor.(map[string]any)
			if !valido {
				continue
			}
			if valorTipo, _ := entrada["@type"].(string); valorTipo != "Product" {
				continue
			}
			valorNombre, valido := entrada["name"].(string)
			if !valido {
				continue
			}
			nombre := dominio.Texto(valorNombre)
			if nombre == nil {
				continue
			}
			if _, duplicado := vistos[*nombre]; duplicado {
				continue
			}
			vistos[*nombre] = struct{}{}

			oferta := ofertaSchema(entrada)
			precioNormal := extraerPrecioNormal(entrada)
			precioOferta := primerPrecio(oferta, "price")
			if precioOferta == nil {
				precioOferta = precioNormal
			}
			if precioNormal == nil {
				precioNormal = precioOferta
			}

			var marca any = entrada["brand"]
			if mapaMarcas, valido := marca.(map[string]any); valido {
				marca = mapaMarcas["name"]
			}
			disponibilidad := oferta["availability"]

			var imagen any = entrada["image"]
			if listaImagenes, valido := imagen.([]any); valido && len(listaImagenes) > 0 {
				imagen = listaImagenes[0]
			}

			var urlProducto *string
			if urlCruda, valido := entrada["url"].(string); valido {
				urlProducto = &urlCruda
			}
			valorSKU := primerValor(entrada, "sku", "productID", "productId")
			var sku *string
			if valorSKU != nil {
				valor := valorComoTexto(valorSKU)
				sku = &valor
			} else if urlProducto != nil {
				sku = JumboSKUFromURL(sitio, *urlProducto)
			}

			var punteroImagen *string
			if valor, valido := imagen.(string); valido {
				punteroImagen = &valor
			}

			productos = append(productos, dominio.Product{
				Producto:          *nombre,
				Precio:            precioOferta,
				PrecioNormal:      precioNormal,
				PrecioOferta:      precioOferta,
				EanGtin:           dominio.EAN(primerValor(entrada, "gtin", "gtin8", "gtin12", "gtin13", "ean")),
				SKU:               sku,
				Marca:             dominio.Texto(marca),
				FormatoCrudo:      dominio.Texto(primerValor(entrada, "format", "size", "description")),
				MecanicaPromocion: dominio.Texto(primerValor(oferta, "promotion", "promotionMechanic", "offerDescription")),
				EnStock:           valorDisponibilidad(disponibilidad),
				URLProducto:       urlProducto,
				Imagen:            punteroImagen,
			})
		}
	})
	return productos
}

// ===== Ficha Individual y SKU =====

// JumboProduct elige la ficha solicitada dentro de los productos extraídos.
func JumboProduct(sitio Site, cuerpo []byte, solicitado string, urlRespuesta *url.URL) *dominio.Product {
	return SelectProductByURL(JumboProducts(sitio, cuerpo), solicitado, urlRespuesta)
}

// JumboSKUFromURL deriva un SKU estable desde la URL cuando la ficha no publica
// uno. Si el resultado excede el límite, lo trunca y agrega un hash corto.
func JumboSKUFromURL(sitio Site, urlCruda string) *string {
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return nil
	}
	segmentos := dominio.SegmentosNoVacios(analizada.Path)
	resto := sitio.Setting("sku_trailing_segment")
	if len(segmentos) > 0 && resto != "" && strings.EqualFold(segmentos[len(segmentos)-1], resto) {
		segmentos = segmentos[:len(segmentos)-1]
	}
	if len(segmentos) == 0 {
		return nil
	}

	prefijo := sitio.Setting("sku_prefix")
	slug := segmentos[len(segmentos)-1]
	sku := prefijo + slug
	if len([]rune(sku)) <= sitio.Limit("sku_max_length") {
		return &sku
	}

	suma := sha1.Sum([]byte(urlCruda))
	sufijo := hex.EncodeToString(suma[:])[:sitio.Limit("hash_length")]
	runas := []rune(slug)
	if len(runas) > sitio.Limit("slug_max_length") {
		runas = runas[:sitio.Limit("slug_max_length")]
	}
	truncado := prefijo + string(runas) + "-" + sufijo
	return &truncado
}
