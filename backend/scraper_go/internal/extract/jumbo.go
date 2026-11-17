package extract

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

// ===== Extracción Principal =====

// JumboProducts extrae las fichas schema.org/Product publicadas por Jumbo.
func JumboProducts(sitio catalog.Site, cuerpo []byte) []domain.Product {
	selector := sitio.Selector("json_ld")
	if selector == "" {
		return nil
	}
	documento, errorObtenido := goquery.NewDocumentFromReader(bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return nil
	}

	var productos []domain.Product
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
		for _, valor := range walkJSON(datos) {
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
			nombre := normalize.Texto(valorNombre)
			if nombre == nil {
				continue
			}
			if _, duplicado := vistos[*nombre]; duplicado {
				continue
			}
			vistos[*nombre] = struct{}{}

			oferta := schemaOffer(entrada)
			precioNormal := extractNormalPrice(entrada)
			precioOferta := firstPrice(oferta, "price")
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
			valorSKU := firstValue(entrada, "sku", "productID", "productId")
			var sku *string
			if valorSKU != nil {
				valor := valueString(valorSKU)
				sku = &valor
			} else if urlProducto != nil {
				sku = JumboSKUFromURL(sitio, *urlProducto)
			}

			var punteroImagen *string
			if valor, valido := imagen.(string); valido {
				punteroImagen = &valor
			}

			productos = append(productos, domain.Product{
				Producto:          *nombre,
				Precio:            precioOferta,
				PrecioNormal:      precioNormal,
				PrecioOferta:      precioOferta,
				EanGtin:           normalize.EAN(firstValue(entrada, "gtin", "gtin8", "gtin12", "gtin13", "ean")),
				SKU:               sku,
				Marca:             normalize.Texto(marca),
				FormatoCrudo:      normalize.Texto(firstValue(entrada, "format", "size", "description")),
				MecanicaPromocion: normalize.Texto(firstValue(oferta, "promotion", "promotionMechanic", "offerDescription")),
				EnStock:           stockValue(disponibilidad),
				URLProducto:       urlProducto,
				Imagen:            punteroImagen,
			})
		}
	})
	return productos
}

// ===== Ficha Individual y SKU =====

// JumboProduct elige la ficha solicitada dentro de los productos extraídos.
func JumboProduct(sitio catalog.Site, cuerpo []byte, solicitado string, urlRespuesta *url.URL) *domain.Product {
	return SelectProductByURL(JumboProducts(sitio, cuerpo), solicitado, urlRespuesta)
}

// JumboSKUFromURL deriva un SKU estable desde la URL cuando la ficha no publica
// uno. Si el resultado excede el límite, lo trunca y agrega un hash corto.
func JumboSKUFromURL(sitio catalog.Site, urlCruda string) *string {
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return nil
	}
	segmentos := nonEmptySegments(analizada.Path)
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
