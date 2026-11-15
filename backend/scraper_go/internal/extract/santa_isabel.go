package extract

import (
	"bytes"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/catalog"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

var renderDataPattern = regexp.MustCompile(`window\.__renderData\s*=\s*("(?:\\.|[^"\\])*")`)

// ===== Extracción Principal =====

// SantaIsabelProducts usa window.__renderData y, si no existe, JSON-LD.
func SantaIsabelProducts(sitio catalog.Site, cuerpo []byte, urlBase *url.URL) []domain.Product {
	if productos := SantaIsabelRenderData(sitio, cuerpo, urlBase); len(productos) > 0 {
		return productos
	}
	return SantaIsabelJSONLD(sitio, cuerpo, urlBase)
}

// SantaIsabelProduct elige la ficha solicitada dentro de los productos.
func SantaIsabelProduct(sitio catalog.Site, cuerpo []byte, solicitado string, urlRespuesta *url.URL) *domain.Product {
	return SelectProductByURL(SantaIsabelProducts(sitio, cuerpo, urlRespuesta), solicitado, urlRespuesta)
}

// SantaIsabelRenderData conserva el parser anterior a 2026 como respaldo de
// rollback.
func SantaIsabelRenderData(sitio catalog.Site, cuerpo []byte, urlBase *url.URL) []domain.Product {
	coincidencia := renderDataPattern.FindSubmatch(cuerpo)
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

	var productos []domain.Product
	for _, productoCrudo := range productosCrudos {
		producto, valido := productoCrudo.(map[string]any)
		if !valido {
			continue
		}
		nombre := normalize.Texto(producto["productName"])
		elemento := firstMap(producto["items"])
		if nombre == nil || elemento == nil {
			continue
		}
		vendedor := santaIsabelSeller(elemento["sellers"], sitio.Setting("seller"))
		if vendedor == nil {
			continue
		}

		oferta, _ := vendedor["commertialOffer"].(map[string]any)
		if oferta == nil {
			oferta = map[string]any{}
		}
		precio := normalize.PrecioCLP(oferta["Price"])
		if precio == nil {
			continue
		}
		precioNormal := firstNonZeroPrice(oferta, "ListPrice", "PriceWithoutDiscount")
		if precioNormal == nil {
			precioNormal = precio
		}
		var precioOferta *float64
		if *precio < *precioNormal {
			precioOferta = precio
		}

		var conStock *bool
		if cantidad := numberValue(oferta["AvailableQuantity"]); cantidad != nil {
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

		formato := joinValueStrings(elemento["unitMultiplier"], elemento["measurementUnit"])

		var mecanicas *string
		if len(nombresPromociones) > 0 {
			valor := strings.Join(nombresPromociones, sitio.Setting("promotion_separator"))
			mecanicas = &valor
		}

		productos = append(productos, domain.Product{
			Producto:          *nombre,
			Precio:            precio,
			PrecioNormal:      precioNormal,
			PrecioOferta:      precioOferta,
			EanGtin:           normalize.EAN(elemento["ean"]),
			SKU:               firstTruthyString(elemento["itemId"], producto["productId"]),
			Marca:             normalize.Texto(producto["brand"]),
			FormatoCrudo:      formato,
			MecanicaPromocion: mecanicas,
			EnStock:           conStock,
			URLProducto:       &urlProducto,
			Imagen:            imagen,
		})
	}
	return productos
}

// SantaIsabelJSONLD lee el ItemList schema.org que publica el sitio desde 2026.
func SantaIsabelJSONLD(sitio catalog.Site, cuerpo []byte, urlBase *url.URL) []domain.Product {
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
		decodificador.UseNumber()
		if errorObtenido := decodificador.Decode(&datos); errorObtenido != nil {
			return
		}
		for _, valor := range walkJSON(datos) {
			entrada, valido := valor.(map[string]any)
			if !valido {
				continue
			}
			valorTipo, _ := entrada["@type"].(string)
			if valorTipo != "Product" {
				continue
			}
			nombre := normalize.Texto(entrada["name"])
			if nombre == nil {
				continue
			}

			oferta := schemaOffer(entrada)
			precio := firstPrice(oferta, "price")
			if precio == nil {
				precio = firstPrice(entrada, "price")
			}
			if precio == nil {
				continue
			}

			precioNormal := firstPrice(oferta, "listPrice", "priceWithoutDiscount")
			var precioOferta *float64
			if precioNormal != nil && *precioNormal > *precio {
				precioOferta = precio
			} else {
				precioNormal = precio
			}

			urlCruda, _ := entrada["url"].(string)
			var urlProducto *string
			if urlCruda != "" {
				resuelto := ResolveURL(urlBase, urlCruda)
				urlProducto = &resuelto
			}
			clave := strings.ToLower(*nombre)
			if urlProducto != nil {
				clave += "|" + *urlProducto
			}
			if _, duplicado := vistos[clave]; duplicado {
				continue
			}
			vistos[clave] = struct{}{}

			var imagen *string
			switch valorImagen := entrada["image"].(type) {
			case string:
				imagen = &valorImagen
			case []any:
				if len(valorImagen) > 0 {
					if primero, valido := valorImagen[0].(string); valido {
						imagen = &primero
					}
				}
			}
			var marca any = entrada["brand"]
			if mapaMarcas, valido := marca.(map[string]any); valido {
				marca = mapaMarcas["name"]
			}
			sku := firstTruthyString(entrada["sku"], entrada["productID"], entrada["productId"], entrada["mpn"])
			if sku == nil {
				sku = normalize.SKUFromURL(urlCruda)
			}

			productos = append(productos, domain.Product{
				Producto:          *nombre,
				Precio:            precio,
				PrecioNormal:      precioNormal,
				PrecioOferta:      precioOferta,
				EanGtin:           normalize.EAN(firstValue(entrada, "gtin", "gtin8", "gtin12", "gtin13", "ean")),
				SKU:               sku,
				Marca:             normalize.Texto(marca),
				EnStock:           stockValue(oferta["availability"]),
				URLProducto:       urlProducto,
				Imagen:            imagen,
				MecanicaPromocion: normalize.Texto(firstValue(oferta, "promotion", "promotionMechanic", "description")),
			})
		}
	})
	return productos
}

// ===== Vendedores y Precios =====

// santaIsabelSeller recupera los datos del vendedor predeterminado en Santa Isabel.
func santaIsabelSeller(valor any, nombreVendedor string) map[string]any {
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

// firstMap retorna el primer submapa válido encontrado en una lista heterogénea.
func firstMap(valor any) map[string]any {
	valores, _ := valor.([]any)
	for _, crudo := range valores {
		if mapeado, valido := crudo.(map[string]any); valido {
			return mapeado
		}
	}
	return nil
}

// firstNonZeroPrice obtiene el primer valor de precio numérico superior a cero.
func firstNonZeroPrice(datos map[string]any, claves ...string) *float64 {
	for _, clave := range claves {
		if precio := normalize.PrecioCLP(datos[clave]); precio != nil && *precio != 0 {
			return precio
		}
	}
	return nil
}

// firstTruthyString obtiene la primera cadena de texto no vacía.
func firstTruthyString(valores ...any) *string {
	for _, valor := range valores {
		if !isTruthy(valor) {
			continue
		}
		texto := valueString(valor)
		if texto == "" {
			continue
		}
		return &texto
	}
	return nil
}

// isTruthy evalúa si un valor booleano o de texto representa una afirmación positiva.
func isTruthy(valor any) bool {
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

// numberValue convierte un valor numérico genérico o string a float64.
func numberValue(valor any) *float64 {
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

// joinValueStrings une una colección de elementos de texto usando un separador.
func joinValueStrings(valores ...any) *string {
	var partes []string
	for _, valor := range valores {
		if valor != nil {
			partes = append(partes, valueString(valor))
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
