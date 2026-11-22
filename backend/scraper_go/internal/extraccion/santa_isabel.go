// santa_isabel.go implementa el spider de Santa Isabel y sus parsers de
// window.__renderData y JSON-LD.
package extraccion

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
)

// ===== Estructura y Constructor =====

// SantaIsabel ejecuta el spider de Santa Isabel.
type SantaIsabel struct {
	spiderBase
}

// nuevoSantaIsabel instancia el spider específico de Santa Isabel usando el catálogo central.
func nuevoSantaIsabel(sitio Site, obtenedor Fetcher, rutaCategorias string, maxPaginas int, registrador *slog.Logger) *SantaIsabel {
	return &SantaIsabel{spiderBase: nuevoSpiderBase(obtenedor, rutaCategorias, maxPaginas, registrador, sitio, configuracionSpider{
		extraerProductos: func(cuerpo []byte, urlRespuesta *url.URL) []dominio.Product {
			return SantaIsabelProducts(sitio, cuerpo, urlRespuesta)
		},
		extraerProducto: func(cuerpo []byte, urlRespuesta *url.URL, solicitado string) *dominio.Product {
			return SantaIsabelProduct(sitio, cuerpo, solicitado, urlRespuesta)
		},
	})}
}

// ===== Extracción Principal =====

var patronDatosRender = regexp.MustCompile(`window\.__renderData\s*=\s*("(?:\\.|[^"\\])*")`)

// SantaIsabelProducts usa window.__renderData y, si no existe, JSON-LD.
func SantaIsabelProducts(sitio Site, cuerpo []byte, urlBase *url.URL) []dominio.Product {
	if productos := SantaIsabelRenderData(sitio, cuerpo, urlBase); len(productos) > 0 {
		return productos
	}
	return SantaIsabelJSONLD(sitio, cuerpo, urlBase)
}

// SantaIsabelProduct elige la ficha solicitada dentro de los productos.
func SantaIsabelProduct(sitio Site, cuerpo []byte, solicitado string, urlRespuesta *url.URL) *dominio.Product {
	return SelectProductByURL(SantaIsabelProducts(sitio, cuerpo, urlRespuesta), solicitado, urlRespuesta)
}

// SantaIsabelRenderData conserva el parser anterior a 2026 como respaldo de
// rollback.
func SantaIsabelRenderData(sitio Site, cuerpo []byte, urlBase *url.URL) []dominio.Product {
	coincidencia := patronDatosRender.FindSubmatch(cuerpo)
	if len(coincidencia) < 2 {
		return nil
	}
	var datosRender string
	if errorObtenido := json.Unmarshal(coincidencia[1], &datosRender); errorObtenido != nil {
		return nil
	}

	var carga map[string]any
	decodificador := json.NewDecoder(strings.NewReader(datosRender))
	decodificador.UseNumber()
	if errorObtenido := decodificador.Decode(&carga); errorObtenido != nil {
		return nil
	}
	plp, _ := carga["plp"].(map[string]any)
	productosPLP, _ := plp["plp_products"].(map[string]any)
	productosCrudos, _ := productosPLP["products"].([]any)

	var productos []dominio.Product
	for _, productoCrudo := range productosCrudos {
		producto, valido := productoCrudo.(map[string]any)
		if !valido {
			continue
		}
		nombre := dominio.Texto(producto["productName"])
		elemento := primerMapa(producto["items"])
		if nombre == nil || elemento == nil {
			continue
		}
		vendedor := vendedorSantaIsabel(elemento["sellers"], sitio.Setting("seller"))
		if vendedor == nil {
			continue
		}

		oferta, _ := vendedor["commertialOffer"].(map[string]any)
		if oferta == nil {
			oferta = map[string]any{}
		}
		precio := dominio.PrecioCLP(oferta["Price"])
		if precio == nil {
			continue
		}
		precioNormal := primerPrecioNoCero(oferta, "ListPrice", "PriceWithoutDiscount")
		if precioNormal == nil {
			precioNormal = precio
		}
		var precioOferta *float64
		if *precio < *precioNormal {
			precioOferta = precio
		}

		var conStock *bool
		if cantidad := valorNumerico(oferta["AvailableQuantity"]); cantidad != nil {
			valor := *cantidad > 0
			conStock = &valor
		}

		var imagen *string
		imagenes, _ := elemento["images"].([]any)
		if len(imagenes) > 0 {
			if primero, valido := imagenes[0].(map[string]any); valido {
				if urlImagen, valido := primero["imageUrl"].(string); valido {
					imagen = &urlImagen
				}
			}
		}

		slug, _ := producto["linkText"].(string)
		referencia := ""
		if slug != "" {
			referencia = "/" + slug + sitio.Setting("product_suffix")
		}
		urlProducto := ResolveURL(urlBase, referencia)

		var nombresPromociones []string
		avances, _ := oferta["teasers"].([]any)
		for _, avanceCrudo := range avances {
			avance, valido := avanceCrudo.(map[string]any)
			if !valido {
				continue
			}
			if promocion, valido := avance["name"].(string); valido && promocion != "" {
				nombresPromociones = append(nombresPromociones, promocion)
			}
		}

		formato := unirValoresComoTexto(elemento["unitMultiplier"], elemento["measurementUnit"])

		var mecanicas *string
		if len(nombresPromociones) > 0 {
			valor := strings.Join(nombresPromociones, sitio.Setting("promotion_separator"))
			mecanicas = &valor
		}

		productos = append(productos, dominio.Product{
			Producto:          *nombre,
			Precio:            precio,
			PrecioNormal:      precioNormal,
			PrecioOferta:      precioOferta,
			EanGtin:           dominio.EAN(elemento["ean"]),
			SKU:               primeraCadenaVerdadera(producto["productReference"], elemento["itemId"], producto["productId"]),
			Marca:             dominio.Texto(producto["brand"]),
			FormatoCrudo:      formato,
			MecanicaPromocion: mecanicas,
			EnStock:           conStock,
			URLProducto:       &urlProducto,
			Imagen:            imagen,
		})
	}
	return productos
}

// ===== Extracción JSON-LD =====

// SantaIsabelJSONLD extrae las fichas schema.org/Product actuales de Santa Isabel.
func SantaIsabelJSONLD(sitio Site, cuerpo []byte, urlBase *url.URL) []dominio.Product {
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

			var imagen any = entrada["image"]
			if listaImagenes, valido := imagen.([]any); valido && len(listaImagenes) > 0 {
				imagen = listaImagenes[0]
			}
			var punteroImagen *string
			if valor, valido := imagen.(string); valido && valor != "" {
				punteroImagen = &valor
			}

			var urlProducto *string
			if urlCruda, valido := entrada["url"].(string); valido && urlCruda != "" {
				resuelto := ResolveURL(urlBase, urlCruda)
				urlProducto = &resuelto
			}

			valorSKU := primerValor(entrada, "sku", "productID", "productId", "mpn")
			var sku *string
			if valorSKU != nil {
				valor := valorComoTexto(valorSKU)
				sku = &valor
			}
			if sku == nil && urlProducto != nil {
				sku = dominio.SKUFromURL(*urlProducto)
			}

			var ofertaReal *float64
			if precioNormal != nil && precioOferta != nil && *precioOferta < *precioNormal {
				ofertaReal = precioOferta
			}

			productos = append(productos, dominio.Product{
				Producto:          *nombre,
				Precio:            precioOferta,
				PrecioNormal:      precioNormal,
				PrecioOferta:      ofertaReal,
				EanGtin:           dominio.EAN(primerValor(entrada, "gtin", "gtin8", "gtin12", "gtin13", "ean")),
				SKU:               sku,
				Marca:             dominio.Texto(marca),
				FormatoCrudo:      dominio.Texto(primerValor(entrada, "format", "size", "description")),
				MecanicaPromocion: dominio.Texto(primerValor(oferta, "promotion", "promotionMechanic", "offerDescription")),
				EnStock:           valorDisponibilidad(oferta["availability"]),
				URLProducto:       urlProducto,
				Imagen:            punteroImagen,
			})
		}
	})
	return productos
}

// ===== Vendedores y Precios =====

// vendedorSantaIsabel recupera los datos del vendedor predeterminado en Santa Isabel.
func vendedorSantaIsabel(valor any, nombreVendedor string) map[string]any {
	vendedores, _ := valor.([]any)
	for _, vendedorCrudo := range vendedores {
		vendedor, valido := vendedorCrudo.(map[string]any)
		if !valido {
			continue
		}
		nombre, _ := vendedor["sellerName"].(string)
		if strings.EqualFold(nombre, nombreVendedor) {
			return vendedor
		}
	}
	return nil
}

// ===== Utilidades y Tipos Dinámicos =====

// primerMapa retorna el primer submapa válido encontrado en una lista heterogénea.
func primerMapa(valor any) map[string]any {
	valores, _ := valor.([]any)
	for _, crudo := range valores {
		if mapeado, valido := crudo.(map[string]any); valido {
			return mapeado
		}
	}
	return nil
}

// primerPrecioNoCero obtiene el primer valor de precio numérico superior a cero.
func primerPrecioNoCero(datos map[string]any, claves ...string) *float64 {
	for _, clave := range claves {
		if precio := dominio.PrecioCLP(datos[clave]); precio != nil && *precio != 0 {
			return precio
		}
	}
	return nil
}

// primeraCadenaVerdadera obtiene la primera cadena de texto no vacía.
func primeraCadenaVerdadera(valores ...any) *string {
	for _, valor := range valores {
		if !esValorVerdadero(valor) {
			continue
		}
		texto := valorComoTexto(valor)
		if texto == "" {
			continue
		}
		return &texto
	}
	return nil
}

// esValorVerdadero evalúa si un valor booleano o de texto representa una afirmación positiva.
func esValorVerdadero(valor any) bool {
	switch tipado := valor.(type) {
	case nil:
		return false
	case string:
		return tipado != ""
	case bool:
		return tipado
	case json.Number:
		numero, errorObtenido := tipado.Float64()
		return errorObtenido == nil && numero != 0
	case float64:
		return tipado != 0
	case float32:
		return tipado != 0
	case int:
		return tipado != 0
	case int64:
		return tipado != 0
	default:
		return true
	}
}

// valorNumerico convierte un valor numérico genérico o string a float64.
func valorNumerico(valor any) *float64 {
	var numero float64
	switch tipado := valor.(type) {
	case nil:
		return nil
	case json.Number:
		analizada, errorObtenido := tipado.Float64()
		if errorObtenido != nil {
			return nil
		}
		numero = analizada
	case float64:
		numero = tipado
	case float32:
		numero = float64(tipado)
	case int:
		numero = float64(tipado)
	case int64:
		numero = float64(tipado)
	case bool:
		if tipado {
			numero = 1
		}
	case string:
		analizada, errorObtenido := strconv.ParseFloat(strings.TrimSpace(tipado), 64)
		if errorObtenido != nil {
			return nil
		}
		numero = analizada
	default:
		return nil
	}
	return &numero
}

// unirValoresComoTexto une una colección de elementos de texto usando un separador.
func unirValoresComoTexto(valores ...any) *string {
	var partes []string
	for _, valor := range valores {
		if valor != nil {
			partes = append(partes, valorComoTexto(valor))
		}
	}
	if len(partes) == 0 {
		return nil
	}
	texto := strings.Join(partes, " ")
	if texto == "" {
		return nil
	}
	return &texto
}
