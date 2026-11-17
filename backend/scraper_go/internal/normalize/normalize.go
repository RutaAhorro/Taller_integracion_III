// Package normalize provee funciones de normalización de precios, EAN y texto
// para asegurar consistencia en los productos ingestados.
package normalize

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

var (
	moneyCleanup     = regexp.MustCompile(`(?i)\s|\$|CLP`)
	priceChars       = regexp.MustCompile(`^-?[0-9.,]+$`)
	skuSuffixPattern = regexp.MustCompile(`-([0-9]+)$`)
	maxPrice         = big.NewRat(999999999999, 100) // 9.999.999.999,99
	oneHundred       = big.NewRat(100, 1)
	oneHalf          = big.NewRat(1, 2)
)

// ===== Conversión de Precios =====

// PrecioCLP convierte un importe en texto o número a un float con dos decimales.
func PrecioCLP(valor any) *float64 {
	texto, valido := priceText(valor)
	if !valido {
		return nil
	}
	monto, valido := parseAmount(texto)
	if !valido {
		return nil
	}
	if monto.Sign() < 0 || monto.Cmp(maxPrice) > 0 {
		return nil
	}

	// ROUND_HALF_UP sobre valores positivos: (x*100 + 0.5) truncado.
	escalado := new(big.Rat).Mul(monto, oneHundred)
	escalado.Add(escalado, oneHalf)
	centavos := new(big.Int).Quo(escalado.Num(), escalado.Denom())
	redondeado := new(big.Rat).SetFrac(centavos, big.NewInt(100))
	resultado, _ := redondeado.Float64()
	return &resultado
}

// priceText aísla los caracteres correspondientes al precio en una cadena de texto.
func priceText(valor any) (string, bool) {
	switch tipado := valor.(type) {
	case nil:
		return "", false
	case bool:
		return "", false
	case string:
		texto := strings.TrimSpace(moneyCleanup.ReplaceAllString(tipado, ""))
		if texto == "" || !priceChars.MatchString(texto) {
			return "", false
		}
		return texto, true
	case json.Number:
		return tipado.String(), true
	case float64:
		if math.IsNaN(tipado) || math.IsInf(tipado, 0) {
			return "", false
		}
		return strconv.FormatFloat(tipado, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(tipado), 'f', -1, 32), true
	case int:
		return strconv.Itoa(tipado), true
	case int8:
		return strconv.FormatInt(int64(tipado), 10), true
	case int16:
		return strconv.FormatInt(int64(tipado), 10), true
	case int32:
		return strconv.FormatInt(int64(tipado), 10), true
	case int64:
		return strconv.FormatInt(tipado, 10), true
	case uint:
		return strconv.FormatUint(uint64(tipado), 10), true
	case uint8:
		return strconv.FormatUint(uint64(tipado), 10), true
	case uint16:
		return strconv.FormatUint(uint64(tipado), 10), true
	case uint32:
		return strconv.FormatUint(uint64(tipado), 10), true
	case uint64:
		return strconv.FormatUint(tipado, 10), true
	default:
		return "", false
	}
}

// parseAmount analiza y convierte un monto en pesos chilenos a float64.
func parseAmount(texto string) (*big.Rat, bool) {
	ultimoPunto := strings.LastIndex(texto, ".")
	ultimaComa := strings.LastIndex(texto, ",")

	switch {
	case ultimoPunto >= 0 && ultimaComa >= 0:
		separadorDecimal := "."
		separadorMiles := ","
		if ultimaComa > ultimoPunto {
			separadorDecimal = ","
			separadorMiles = "."
		}
		entero, fraccion, encontrado := strings.Cut(texto, separadorDecimal)
		if !encontrado {
			return nil, false
		}
		texto = strings.ReplaceAll(entero, separadorMiles, "") + "." + fraccion
	case ultimoPunto >= 0 || ultimaComa >= 0:
		separador := "."
		if ultimaComa >= 0 {
			separador = ","
		}
		partes := strings.Split(texto, separador)
		switch {
		case len(partes) > 2:
			if usesThousandsGroups(partes) {
				texto = strings.Join(partes, "")
			} else {
				texto = strings.Join(partes[:len(partes)-1], "") + "." + partes[len(partes)-1]
			}
		case len(partes) == 2 && len(partes[1]) == 3 &&
			len(strings.TrimLeft(partes[0], "-")) >= 1 &&
			len(strings.TrimLeft(partes[0], "-")) <= 3:
			texto = strings.Join(partes, "")
		case separador == ",":
			texto = strings.Replace(texto, ",", ".", 1)
		}
	}

	monto, valido := new(big.Rat).SetString(texto)
	return monto, valido
}

// usesThousandsGroups comprueba si el número emplea formato de puntos de miles.
func usesThousandsGroups(partes []string) bool {
	primero := strings.TrimLeft(partes[0], "-")
	if len(primero) < 1 || len(primero) > 3 {
		return false
	}
	for _, parte := range partes[1:] {
		if len(parte) != 3 {
			return false
		}
	}
	return true
}

// ===== Normalización de EAN =====

// EAN conserva ceros iniciales y descarta cualquier carácter que no sea
// dígito, espacio o guion, igual que normalizar_ean_gtin.
func EAN(valor any) *string {
	var texto string
	switch tipado := valor.(type) {
	case nil:
		return nil
	case bool:
		return nil
	case string:
		texto = strings.TrimSpace(tipado)
	case json.Number:
		texto = tipado.String()
	case float64:
		if math.IsNaN(tipado) || math.IsInf(tipado, 0) || math.Trunc(tipado) != tipado {
			return nil
		}
		texto = strconv.FormatInt(int64(tipado), 10)
	case float32:
		numero := float64(tipado)
		if math.Trunc(numero) != numero {
			return nil
		}
		texto = strconv.FormatInt(int64(numero), 10)
	case int:
		texto = strconv.Itoa(tipado)
	case int8:
		texto = strconv.FormatInt(int64(tipado), 10)
	case int16:
		texto = strconv.FormatInt(int64(tipado), 10)
	case int32:
		texto = strconv.FormatInt(int64(tipado), 10)
	case int64:
		texto = strconv.FormatInt(tipado, 10)
	case uint:
		texto = strconv.FormatUint(uint64(tipado), 10)
	case uint8:
		texto = strconv.FormatUint(uint64(tipado), 10)
	case uint16:
		texto = strconv.FormatUint(uint64(tipado), 10)
	case uint32:
		texto = strconv.FormatUint(uint64(tipado), 10)
	case uint64:
		texto = strconv.FormatUint(tipado, 10)
	default:
		return nil
	}

	if texto == "" {
		return nil
	}
	var digitos strings.Builder
	for _, caracter := range texto {
		switch {
		case unicode.IsDigit(caracter):
			digitos.WriteRune(caracter)
		case unicode.IsSpace(caracter) || caracter == '-':
			continue
		default:
			return nil
		}
	}
	if digitos.Len() == 0 {
		return nil
	}
	resultado := digitos.String()
	return &resultado
}

// ===== Recorte de Texto =====

// Truncar limita una cadena al número de caracteres (runas) que acepta el
// esquema PostgreSQL. No corta bytes de una secuencia UTF-8.
func Truncar(valor string, maximo int) string {
	if maximo <= 0 {
		return ""
	}
	runas := []rune(valor)
	if len(runas) <= maximo {
		return valor
	}
	return string(runas[:maximo])
}

// TruncarPtr aplica Truncar conservando nil.
func TruncarPtr(valor *string, maximo int) *string {
	if valor == nil {
		return nil
	}
	truncado := Truncar(*valor, maximo)
	return &truncado
}

// ===== Productos y SKU =====

// LimitarProducto recorta los campos que Catálogo persiste en columnas
// VARCHAR de scraper.productos_crudos y deriva el SKU si falta. Se aplica antes
// de guardar en el spool para asegurar que los lotes sean siempre válidos.
func LimitarProducto(producto domain.Product) domain.Product {
	if producto.SKU == nil || strings.TrimSpace(*producto.SKU) == "" {
		if producto.URLProducto != nil {
			producto.SKU = SKUFromURL(*producto.URLProducto)
		}
	}
	producto.Producto = Truncar(producto.Producto, 255)
	producto.SKU = TruncarPtr(producto.SKU, 100)
	producto.EanGtin = TruncarPtr(producto.EanGtin, 50)
	producto.Marca = TruncarPtr(producto.Marca, 100)
	producto.Categoria = Truncar(producto.Categoria, 100)
	producto.FormatoCrudo = TruncarPtr(producto.FormatoCrudo, 100)
	producto.MecanicaPromocion = TruncarPtr(producto.MecanicaPromocion, 255)
	return producto
}

// SKUFromURL obtiene un identificador estable de la URL de una ficha cuando
// el HTML no publica SKU. Prefiere el ID numérico final del slug
// ("...-1993238/p"); si no existe, usa el slug completo, que también es único
// por producto en las cadenas WooCommerce como Cugat.
func SKUFromURL(urlCruda string) *string {
	analizada, errorObtenido := url.Parse(strings.TrimSpace(urlCruda))
	if errorObtenido != nil {
		return nil
	}
	segmentos := make([]string, 0, 8)
	for _, segmento := range strings.Split(analizada.Path, "/") {
		if segmento != "" {
			segmentos = append(segmentos, segmento)
		}
	}
	for len(segmentos) > 0 {
		ultimo := segmentos[len(segmentos)-1]
		if strings.EqualFold(ultimo, "p") || strings.EqualFold(ultimo, "product") || strings.EqualFold(ultimo, "producto") {
			segmentos = segmentos[:len(segmentos)-1]
			continue
		}
		break
	}
	if len(segmentos) == 0 {
		return nil
	}

	candidato := segmentos[len(segmentos)-1]
	if isDigits(candidato) {
		return &candidato
	}
	if coincidencia := skuSuffixPattern.FindStringSubmatch(candidato); len(coincidencia) == 2 {
		return &coincidencia[1]
	}
	if len([]rune(candidato)) < 3 {
		return nil
	}
	return &candidato
}

// isDigits comprueba si todos los caracteres de la cadena son dígitos numéricos.
func isDigits(valor string) bool {
	if valor == "" {
		return false
	}
	for _, caracter := range valor {
		if caracter < '0' || caracter > '9' {
			return false
		}
	}
	return true
}

// ===== Normalización de Texto =====

// Texto colapsa espacios y devuelve nil para cadenas vacías.
func Texto(valor any) *string {
	if valor == nil {
		return nil
	}
	texto := strings.Join(strings.Fields(formatString(valor)), " ")
	if texto == "" {
		return nil
	}
	return &texto
}

// formatString limpia espacios redundantes y caracteres de control en cadenas.
func formatString(valor any) string {
	switch tipado := valor.(type) {
	case bool:
		if tipado {
			return "True"
		}
		return "False"
	case float64:
		texto := strconv.FormatFloat(tipado, 'f', -1, 64)
		if !strings.ContainsAny(texto, ".eE") {
			texto += ".0"
		}
		return texto
	case float32:
		texto := strconv.FormatFloat(float64(tipado), 'f', -1, 32)
		if !strings.ContainsAny(texto, ".eE") {
			texto += ".0"
		}
		return texto
	default:
		return fmt.Sprint(tipado)
	}
}
