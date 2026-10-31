// Package normalize replica la normalización del scraper de Python para que
// los productos de Go sean comparables con los ya ingestados.
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

// PrecioCLP convierte un importe en texto o número a un float con dos
// decimales, replicando normalizar_precio_clp de Python.
func PrecioCLP(value any) *float64 {
	text, ok := priceText(value)
	if !ok {
		return nil
	}
	amount, ok := parseAmount(text)
	if !ok {
		return nil
	}
	if amount.Sign() < 0 || amount.Cmp(maxPrice) > 0 {
		return nil
	}

	// ROUND_HALF_UP sobre valores positivos: (x*100 + 0.5) truncado.
	scaled := new(big.Rat).Mul(amount, oneHundred)
	scaled.Add(scaled, oneHalf)
	cents := new(big.Int).Quo(scaled.Num(), scaled.Denom())
	rounded := new(big.Rat).SetFrac(cents, big.NewInt(100))
	result, _ := rounded.Float64()
	return &result
}

func priceText(value any) (string, bool) {
	switch v := value.(type) {
	case nil:
		return "", false
	case bool:
		return "", false
	case string:
		text := strings.TrimSpace(moneyCleanup.ReplaceAllString(v, ""))
		if text == "" || !priceChars.MatchString(text) {
			return "", false
		}
		return text, true
	case json.Number:
		return v.String(), true
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "", false
		}
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), true
	case int:
		return strconv.Itoa(v), true
	case int8:
		return strconv.FormatInt(int64(v), 10), true
	case int16:
		return strconv.FormatInt(int64(v), 10), true
	case int32:
		return strconv.FormatInt(int64(v), 10), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case uint:
		return strconv.FormatUint(uint64(v), 10), true
	case uint8:
		return strconv.FormatUint(uint64(v), 10), true
	case uint16:
		return strconv.FormatUint(uint64(v), 10), true
	case uint32:
		return strconv.FormatUint(uint64(v), 10), true
	case uint64:
		return strconv.FormatUint(v, 10), true
	default:
		return "", false
	}
}

func parseAmount(text string) (*big.Rat, bool) {
	lastDot := strings.LastIndex(text, ".")
	lastComma := strings.LastIndex(text, ",")

	switch {
	case lastDot >= 0 && lastComma >= 0:
		decimalSeparator := "."
		groupingSeparator := ","
		if lastComma > lastDot {
			decimalSeparator = ","
			groupingSeparator = "."
		}
		integer, fraction, found := strings.Cut(text, decimalSeparator)
		if !found {
			return nil, false
		}
		text = strings.ReplaceAll(integer, groupingSeparator, "") + "." + fraction
	case lastDot >= 0 || lastComma >= 0:
		separator := "."
		if lastComma >= 0 {
			separator = ","
		}
		parts := strings.Split(text, separator)
		switch {
		case len(parts) > 2:
			if usesThousandsGroups(parts) {
				text = strings.Join(parts, "")
			} else {
				text = strings.Join(parts[:len(parts)-1], "") + "." + parts[len(parts)-1]
			}
		case len(parts) == 2 && len(parts[1]) == 3 &&
			len(strings.TrimLeft(parts[0], "-")) >= 1 &&
			len(strings.TrimLeft(parts[0], "-")) <= 3:
			text = strings.Join(parts, "")
		case separator == ",":
			text = strings.Replace(text, ",", ".", 1)
		}
	}

	amount, ok := new(big.Rat).SetString(text)
	return amount, ok
}

func usesThousandsGroups(parts []string) bool {
	first := strings.TrimLeft(parts[0], "-")
	if len(first) < 1 || len(first) > 3 {
		return false
	}
	for _, part := range parts[1:] {
		if len(part) != 3 {
			return false
		}
	}
	return true
}

// EAN conserva ceros iniciales y descarta cualquier carácter que no sea
// dígito, espacio o guion, igual que normalizar_ean_gtin.
func EAN(value any) *string {
	var text string
	switch v := value.(type) {
	case nil:
		return nil
	case bool:
		return nil
	case string:
		text = strings.TrimSpace(v)
	case json.Number:
		text = v.String()
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v {
			return nil
		}
		text = strconv.FormatInt(int64(v), 10)
	case float32:
		f := float64(v)
		if math.Trunc(f) != f {
			return nil
		}
		text = strconv.FormatInt(int64(f), 10)
	case int:
		text = strconv.Itoa(v)
	case int8:
		text = strconv.FormatInt(int64(v), 10)
	case int16:
		text = strconv.FormatInt(int64(v), 10)
	case int32:
		text = strconv.FormatInt(int64(v), 10)
	case int64:
		text = strconv.FormatInt(v, 10)
	case uint:
		text = strconv.FormatUint(uint64(v), 10)
	case uint8:
		text = strconv.FormatUint(uint64(v), 10)
	case uint16:
		text = strconv.FormatUint(uint64(v), 10)
	case uint32:
		text = strconv.FormatUint(uint64(v), 10)
	case uint64:
		text = strconv.FormatUint(v, 10)
	default:
		return nil
	}

	if text == "" {
		return nil
	}
	var digits strings.Builder
	for _, r := range text {
		switch {
		case unicode.IsDigit(r):
			digits.WriteRune(r)
		case unicode.IsSpace(r) || r == '-':
			continue
		default:
			return nil
		}
	}
	if digits.Len() == 0 {
		return nil
	}
	result := digits.String()
	return &result
}

// Truncar limita una cadena al número de caracteres (runas) que acepta el
// esquema PostgreSQL. No corta bytes de una secuencia UTF-8.
func Truncar(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

// TruncarPtr aplica Truncar conservando nil.
func TruncarPtr(value *string, max int) *string {
	if value == nil {
		return nil
	}
	truncated := Truncar(*value, max)
	return &truncated
}

// LimitarProducto recorta los campos que Catálogo persiste en columnas
// VARCHAR de scraper.productos_crudos y deriva el SKU si falta. Se aplica antes
// de guardar en el spool, no sólo antes del POST: si se activa el rollback a
// Python, outbox.replay reenvía el documento almacenado sin volver a truncar y
// Catálogo rechazaría el lote completo.
func LimitarProducto(product domain.Product) domain.Product {
	if product.SKU == nil || strings.TrimSpace(*product.SKU) == "" {
		if product.URLProducto != nil {
			product.SKU = SKUFromURL(*product.URLProducto)
		}
	}
	product.Producto = Truncar(product.Producto, 255)
	product.SKU = TruncarPtr(product.SKU, 100)
	product.EanGtin = TruncarPtr(product.EanGtin, 50)
	product.Marca = TruncarPtr(product.Marca, 100)
	product.Categoria = Truncar(product.Categoria, 100)
	product.FormatoCrudo = TruncarPtr(product.FormatoCrudo, 100)
	product.MecanicaPromocion = TruncarPtr(product.MecanicaPromocion, 255)
	return product
}

// SKUFromURL obtiene un identificador estable de la URL de una ficha cuando
// el HTML no publica SKU. Prefiere el ID numérico final del slug
// ("...-1993238/p"); si no existe, usa el slug completo, que también es único
// por producto en las cadenas WooCommerce como Cugat.
func SKUFromURL(rawURL string) *string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil
	}
	segments := make([]string, 0, 8)
	for _, segment := range strings.Split(parsed.Path, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	for len(segments) > 0 {
		last := segments[len(segments)-1]
		if strings.EqualFold(last, "p") || strings.EqualFold(last, "product") || strings.EqualFold(last, "producto") {
			segments = segments[:len(segments)-1]
			continue
		}
		break
	}
	if len(segments) == 0 {
		return nil
	}

	candidate := segments[len(segments)-1]
	if isDigits(candidate) {
		return &candidate
	}
	if match := skuSuffixPattern.FindStringSubmatch(candidate); len(match) == 2 {
		return &match[1]
	}
	if len([]rune(candidate)) < 3 {
		return nil
	}
	return &candidate
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

// Texto colapsa espacios y devuelve nil para cadenas vacías.
func Texto(value any) *string {
	if value == nil {
		return nil
	}
	text := strings.Join(strings.Fields(pyString(value)), " ")
	if text == "" {
		return nil
	}
	return &text
}

func pyString(value any) string {
	switch v := value.(type) {
	case bool:
		if v {
			return "True"
		}
		return "False"
	case float64:
		text := strconv.FormatFloat(v, 'f', -1, 64)
		if !strings.ContainsAny(text, ".eE") {
			text += ".0"
		}
		return text
	case float32:
		text := strconv.FormatFloat(float64(v), 'f', -1, 32)
		if !strings.ContainsAny(text, ".eE") {
			text += ".0"
		}
		return text
	default:
		return fmt.Sprint(value)
	}
}
