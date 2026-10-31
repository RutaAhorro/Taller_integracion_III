package spool

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

// Este archivo replica el formato JSON de scraper_core/outbox.py:
//
//	data = json.dumps(product, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
//	fingerprint = sha256(json.dumps([market, data]).encode()).hexdigest()
//
// La reproducción debe ser byte a byte: el fingerprint es la clave UNIQUE de
// SQLite y de ella depende que un rollback a Python (o un reprocesamiento
// sobre el mismo PVC) no duplique las capturas append-only en PostgreSQL.
//
// Diferencias relevantes frente a encoding/json de Go:
//   - las claves se ordenan alfabéticamente;
//   - un float integral se serializa como 1290.0, no como 1290;
//   - el JSON exterior usa los separadores por defecto (", " y ": ") y
//     ensure_ascii=True, por lo que todo carácter no ASCII se escapa.

// productFieldOrder es el orden alfabético que produce sort_keys=True para el
// contrato de domain.Product (sin "supermercado", que viaja en el lote).
var productFieldOrder = [...]string{
	"categoria",
	"ean_gtin",
	"en_stock",
	"formato_crudo",
	"imagen",
	"marca",
	"mecanica_promocion",
	"precio",
	"precio_normal",
	"precio_oferta",
	"producto",
	"sku",
	"url_producto",
}

// canonicalProductJSON serializa el producto igual que outbox.py. Se usa tanto
// para la columna product como para calcular el fingerprint, de modo que ambos
// documentos quedan idénticos a los que escribiría Python.
func canonicalProductJSON(product domain.Product) ([]byte, error) {
	values := make([]any, 0, len(productFieldOrder))
	for range productFieldOrder {
		values = append(values, nil)
	}
	values[0] = product.Categoria
	values[1] = nullableString(product.EanGtin)
	values[2] = nullableBool(product.EnStock)
	values[3] = nullableString(product.FormatoCrudo)
	values[4] = nullableString(product.Imagen)
	values[5] = nullableString(product.Marca)
	values[6] = nullableString(product.MecanicaPromocion)
	values[7] = nullableFloat(product.Precio)
	values[8] = nullableFloat(product.PrecioNormal)
	values[9] = nullableFloat(product.PrecioOferta)
	values[10] = product.Producto
	values[11] = nullableString(product.SKU)
	values[12] = nullableString(product.URLProducto)

	var builder strings.Builder
	builder.WriteByte('{')
	for index, name := range productFieldOrder {
		if index > 0 {
			builder.WriteByte(',')
		}
		writePythonString(&builder, name, false)
		builder.WriteByte(':')
		if err := writePythonValue(&builder, values[index], false); err != nil {
			return nil, err
		}
	}
	builder.WriteByte('}')
	return []byte(builder.String()), nil
}

// pythonFingerprint construye json.dumps([market, data]) con los valores por
// defecto de Python (espaciado y ensure_ascii=True) y devuelve su SHA-256.
func pythonFingerprint(market *string, data []byte) (string, error) {
	var builder strings.Builder
	builder.WriteByte('[')
	if err := writePythonValue(&builder, nullableString(market), true); err != nil {
		return "", err
	}
	builder.WriteString(", ")
	writePythonString(&builder, string(data), true)
	builder.WriteByte(']')

	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:]), nil
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableBool(value *bool) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func writePythonValue(builder *strings.Builder, value any, ascii bool) error {
	switch typed := value.(type) {
	case nil:
		builder.WriteString("null")
	case string:
		writePythonString(builder, typed, ascii)
	case bool:
		if typed {
			builder.WriteString("true")
		} else {
			builder.WriteString("false")
		}
	case float64:
		literal, err := pythonFloatRepr(typed)
		if err != nil {
			return err
		}
		builder.WriteString(literal)
	default:
		return fmt.Errorf("tipo no soportado en el JSON canónico: %T", value)
	}
	return nil
}

// writePythonString replica json.dumps para cadenas. Con ascii=true escapa todo
// carácter fuera de 0x20..0x7e (incluidos los pares subrogados); con ascii=false
// conserva el UTF-8 y sólo escapa comillas, barra invertida y controles.
func writePythonString(builder *strings.Builder, value string, ascii bool) {
	builder.WriteByte('"')
	for _, character := range value {
		switch character {
		case '"':
			builder.WriteString(`\"`)
		case '\\':
			builder.WriteString(`\\`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		case '\b':
			builder.WriteString(`\b`)
		case '\f':
			builder.WriteString(`\f`)
		default:
			switch {
			case character < 0x20:
				fmt.Fprintf(builder, `\u%04x`, character)
			case ascii && character > 0x7e && character <= 0xffff:
				fmt.Fprintf(builder, `\u%04x`, character)
			case ascii && character > 0xffff:
				offset := character - 0x10000
				high := 0xd800 + (offset >> 10)
				low := 0xdc00 + (offset & 0x3ff)
				fmt.Fprintf(builder, `\u%04x\u%04x`, high, low)
			default:
				builder.WriteRune(character)
			}
		}
	}
	builder.WriteByte('"')
}

// pythonFloatRepr replica float.__repr__, que es lo que usa json.dumps. Go y
// Python comparten el algoritmo de dígitos más cortos, pero difieren en el
// formato: Python siempre incluye ".0" en los enteros y usa notación científica
// sólo cuando el punto decimal queda fuera de (-4, 16].
func pythonFloatRepr(value float64) (string, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "", fmt.Errorf("precio no finito no serializable: %v", value)
	}
	if value == 0 {
		if math.Signbit(value) {
			return "-0.0", nil
		}
		return "0.0", nil
	}

	negative := math.Signbit(value)
	absolute := math.Abs(value)
	scientific := strconv.FormatFloat(absolute, 'e', -1, 64)
	mantissa, exponentText, found := strings.Cut(scientific, "e")
	if !found {
		return "", fmt.Errorf("formato inesperado para %v", value)
	}
	exponent, err := strconv.Atoi(exponentText)
	if err != nil {
		return "", err
	}
	digits := strings.Replace(mantissa, ".", "", 1)
	decimalPoint := exponent + 1

	var output string
	switch {
	case decimalPoint <= -4 || decimalPoint > 16:
		output = digits[:1]
		if len(digits) > 1 {
			output += "." + digits[1:]
		}
		sign := "+"
		if exponent < 0 {
			sign = "-"
		}
		absoluteExponent := exponent
		if absoluteExponent < 0 {
			absoluteExponent = -absoluteExponent
		}
		exponentDigits := strconv.Itoa(absoluteExponent)
		if len(exponentDigits) < 2 {
			exponentDigits = "0" + exponentDigits
		}
		output += "e" + sign + exponentDigits
	case decimalPoint <= 0:
		output = "0." + strings.Repeat("0", -decimalPoint) + digits
	case decimalPoint >= len(digits):
		output = digits + strings.Repeat("0", decimalPoint-len(digits)) + ".0"
	default:
		output = digits[:decimalPoint] + "." + digits[decimalPoint:]
	}

	if negative {
		output = "-" + output
	}
	return output, nil
}
