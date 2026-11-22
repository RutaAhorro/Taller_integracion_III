// Archivo normalizacion.go implementa la limpieza de precios CLP, códigos
// EAN/GTIN, recorte de cadenas UTF-8 y derivación de SKU desde la URL.
package dominio

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
)

// ===== Constantes de Límites del Esquema =====

const (
	// MaxProductoLength es el límite VARCHAR de nombre de producto y promoción en Catálogo.
	MaxProductoLength = 255
	// MaxCampoCortoLength es el límite VARCHAR de SKU, marca, categoría y formato en Catálogo.
	MaxCampoCortoLength = 100
	// MaxEANLength es el límite VARCHAR del campo EAN/GTIN en Catálogo.
	MaxEANLength = 50
	// MinSKUSlugRunes es la longitud mínima para aceptar un slug no numérico como SKU.
	MinSKUSlugRunes = 3
)

var (
	patronPrecio    = regexp.MustCompile(`-?\d[\d.,]*`)
	patronSufijoSKU = regexp.MustCompile(`-(\d{4,})$`)
	cien            = big.NewInt(100)
	mitad           = big.NewRat(1, 2)
)

// ===== Normalización de Precios =====

// PrecioCLP convierte un precio crudo a float64 con redondeo HALF_UP a dos
// decimales. Acepta separadores chilenos ("$1.290", "$1.290,50") y anglosajones
// ("1,290.50"). Devuelve nil cuando el valor no es un monto positivo.
func PrecioCLP(valor any) *float64 {
	texto, valido := formatearEntradaPrecio(valor)
	if !valido {
		return nil
	}
	coincidencia := patronPrecio.FindString(texto)
	if coincidencia == "" {
		return nil
	}
	monto, valido := analizarMonto(coincidencia)
	if !valido || monto.Sign() <= 0 {
		return nil
	}

	escalado := new(big.Rat).Mul(monto, new(big.Rat).SetInt(cien))
	escalado.Add(escalado, mitad)
	centavos := new(big.Int).Quo(escalado.Num(), escalado.Denom())
	resultado, _ := new(big.Rat).Quo(new(big.Rat).SetInt(centavos), new(big.Rat).SetInt(cien)).Float64()
	if resultado <= 0 || math.IsNaN(resultado) || math.IsInf(resultado, 0) {
		return nil
	}
	return &resultado
}

// formatearEntradaPrecio prepara la cadena de precio eliminando símbolos monetarios.
func formatearEntradaPrecio(valor any) (string, bool) {
	switch tipado := valor.(type) {
	case nil:
		return "", false
	case bool:
		return "", false
	case string:
		return strings.TrimSpace(tipado), true
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

// analizarMonto analiza y convierte un monto en pesos chilenos a número racional.
func analizarMonto(texto string) (*big.Rat, bool) {
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
			if usaGruposDeMiles(partes) {
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

// usaGruposDeMiles comprueba si el número emplea formato de puntos de miles.
func usaGruposDeMiles(partes []string) bool {
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
func LimitarProducto(producto Product) Product {
	if producto.SKU == nil || strings.TrimSpace(*producto.SKU) == "" {
		if producto.URLProducto != nil {
			producto.SKU = SKUFromURL(*producto.URLProducto)
		}
	}
	producto.Producto = Truncar(producto.Producto, MaxProductoLength)
	producto.SKU = TruncarPtr(producto.SKU, MaxCampoCortoLength)
	producto.EanGtin = TruncarPtr(producto.EanGtin, MaxEANLength)
	producto.Marca = TruncarPtr(producto.Marca, MaxCampoCortoLength)
	producto.Categoria = Truncar(producto.Categoria, MaxCampoCortoLength)
	producto.FormatoCrudo = TruncarPtr(producto.FormatoCrudo, MaxCampoCortoLength)
	producto.MecanicaPromocion = TruncarPtr(producto.MecanicaPromocion, MaxProductoLength)
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
	segmentos := SegmentosNoVacios(analizada.Path)
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
	if EsSoloDigitos(candidato) {
		return &candidato
	}
	if coincidencia := patronSufijoSKU.FindStringSubmatch(candidato); len(coincidencia) == 2 {
		return &coincidencia[1]
	}
	if len([]rune(candidato)) < MinSKUSlugRunes {
		return nil
	}
	return &candidato
}

// SegmentosNoVacios divide una ruta URL por barras descartando segmentos vacíos.
func SegmentosNoVacios(ruta string) []string {
	var segmentos []string
	for _, segmento := range strings.Split(ruta, "/") {
		if segmento != "" {
			segmentos = append(segmentos, segmento)
		}
	}
	return segmentos
}

// EsSoloDigitos comprueba si todos los caracteres de la cadena son dígitos ASCII.
func EsSoloDigitos(valor string) bool {
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
	texto := strings.Join(strings.Fields(formatearCadena(valor)), " ")
	if texto == "" {
		return nil
	}
	return &texto
}

// formatearCadena limpia espacios redundantes y formatea valores primitivos en cadenas.
func formatearCadena(valor any) string {
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
