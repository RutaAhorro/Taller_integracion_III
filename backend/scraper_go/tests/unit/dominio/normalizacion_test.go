// normalizacion_test.go verifica la normalización de precios CLP, códigos EAN,
// textos, truncado UTF-8 y derivación de SKU en el paquete dominio.
package dominio_test

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
)

// ===== Precios y Códigos EAN =====

// TestPrecioCLP verifica el parseo de montos en pesos chilenos con separadores de miles y decimales.
func TestPrecioCLP(prueba *testing.T) {
	casos := []struct {
		valor    any
		esperado float64
	}{
		{"$1.290", 1290},
		{"1290", 1290},
		{"1290.00", 1290},
		{"$1.290,50", 1290.5},
		{"1,290.50", 1290.5},
		{1290, 1290},
		{1290.0, 1290},
	}
	for _, caso := range casos {
		obtenido := dominio.PrecioCLP(caso.valor)
		if obtenido == nil || *obtenido != caso.esperado {
			prueba.Errorf("PrecioCLP(%v) = %v, esperado %v", caso.valor, obtenido, caso.esperado)
		}
	}

	for _, valorInvalido := range []any{nil, true, "", "$abc", -1, math.Inf(1)} {
		if obtenido := dominio.PrecioCLP(valorInvalido); obtenido != nil {
			prueba.Errorf("PrecioCLP(%v) = %v, esperado nil", valorInvalido, *obtenido)
		}
	}
}

// TestEAN verifica la limpieza de guiones y el rechazo de cadenas no numéricas.
func TestEAN(prueba *testing.T) {
	casos := []struct {
		valor    any
		esperado string
	}{
		{" 00-7800000000001 ", "007800000000001"},
		{7800000000001, "7800000000001"},
	}
	for _, caso := range casos {
		obtenido := dominio.EAN(caso.valor)
		if obtenido == nil || *obtenido != caso.esperado {
			prueba.Errorf("EAN(%v) = %v, esperado %q", caso.valor, obtenido, caso.esperado)
		}
	}
	if obtenido := dominio.EAN("EAN-unknown"); obtenido != nil {
		prueba.Errorf("EAN inválido = %v, esperado nil", *obtenido)
	}
}

// ===== Texto y Truncado =====

// TestTexto verifica el colapso de espacios en blanco y saltos de línea.
func TestTexto(prueba *testing.T) {
	obtenido := dominio.Texto("  Leche\n entera   1 L ")
	if obtenido == nil || *obtenido != "Leche entera 1 L" {
		prueba.Errorf("Texto = %v", obtenido)
	}
	if obtenido := dominio.Texto(" \n "); obtenido != nil {
		prueba.Errorf("Texto vacío = %v, esperado nil", *obtenido)
	}
}

// TestTruncar verifica el recorte por runas preservando secuencias UTF-8 válidas.
func TestTruncar(prueba *testing.T) {
	if obtenido := dominio.Truncar("Leche entera", 100); obtenido != "Leche entera" {
		prueba.Errorf("Truncar corto = %q", obtenido)
	}
	if obtenido := dominio.Truncar("Leche entera", 5); obtenido != "Leche" {
		prueba.Errorf("Truncar = %q, esperado %q", obtenido, "Leche")
	}
	obtenido := dominio.Truncar("Bebida azúcar 1 L", 8)
	if obtenido != "Bebida a" || !utf8.ValidString(obtenido) {
		prueba.Errorf("Truncar UTF-8 = %q, válido=%v", obtenido, utf8.ValidString(obtenido))
	}
	if obtenido := dominio.Truncar("Leche", 0); obtenido != "" {
		prueba.Errorf("Truncar con límite 0 = %q", obtenido)
	}
	valor := strings.Repeat("a", 120)
	if truncado := dominio.TruncarPtr(&valor, 100); truncado == nil || len(*truncado) != 100 {
		prueba.Errorf("TruncarPtr = %v", truncado)
	}
	if truncado := dominio.TruncarPtr(nil, 100); truncado != nil {
		prueba.Errorf("TruncarPtr(nil) = %v", truncado)
	}
}

// ===== Derivación de SKU y Límites =====

// TestSKUFromURL verifica la extracción del código SKU desde distintos formatos de URL.
func TestSKUFromURL(prueba *testing.T) {
	casos := []struct {
		url      string
		esperado string
	}{
		{"https://www.santaisabel.cl/mas-limonada-frambuesa-con-gas-1600cc-1993238/p", "1993238"},
		{"https://www.santaisabel.cl/mas-limonada-frambuesa-con-gas-1600cc-1993238/p/", "1993238"},
		{"https://cugat.cl/producto/leche-entera-pitrufquen-1lt/", "leche-entera-pitrufquen-1lt"},
		{"https://www.jumbo.cl/palta-hass-jumbo-malla-1-kg/p?foo=bar", "palta-hass-jumbo-malla-1-kg"},
		{"https://example.test/producto/12345", "12345"},
		{"", ""},
	}
	for _, caso := range casos {
		obtenido := dominio.SKUFromURL(caso.url)
		if caso.esperado == "" {
			if obtenido != nil {
				prueba.Errorf("SKUFromURL(%q) = %q, esperado nil", caso.url, *obtenido)
			}
			continue
		}
		if obtenido == nil || *obtenido != caso.esperado {
			prueba.Errorf("SKUFromURL(%q) = %v, esperado %q", caso.url, obtenido, caso.esperado)
		}
	}
}

// TestLimitarProductoRecortaColumnasYDerivaSKU comprueba que LimitarProducto ajuste cada campo a los límites del esquema.
func TestLimitarProductoRecortaColumnasYDerivaSKU(prueba *testing.T) {
	textoLargo := strings.Repeat("á", 400)
	urlProducto := "https://www.santaisabel.cl/producto-1993238/p"
	producto := dominio.LimitarProducto(dominio.Product{
		Producto:          textoLargo,
		SKU:               dominio.StringPtr(strings.Repeat("s", 120)),
		EanGtin:           dominio.StringPtr(strings.Repeat("9", 60)),
		Marca:             dominio.StringPtr(textoLargo),
		Categoria:         textoLargo,
		FormatoCrudo:      dominio.StringPtr(textoLargo),
		MecanicaPromocion: dominio.StringPtr(textoLargo),
		URLProducto:       &urlProducto,
	})

	verificaciones := []struct {
		nombre string
		valor  string
		limite int
	}{
		{"producto", producto.Producto, dominio.MaxProductoLength},
		{"sku", *producto.SKU, dominio.MaxCampoCortoLength},
		{"ean_gtin", *producto.EanGtin, dominio.MaxEANLength},
		{"marca", *producto.Marca, dominio.MaxCampoCortoLength},
		{"categoria", producto.Categoria, dominio.MaxCampoCortoLength},
		{"formato_crudo", *producto.FormatoCrudo, dominio.MaxCampoCortoLength},
		{"mecanica_promocion", *producto.MecanicaPromocion, dominio.MaxProductoLength},
	}
	for _, verificacion := range verificaciones {
		if !utf8.ValidString(verificacion.valor) {
			prueba.Errorf("%s no es UTF-8 válido", verificacion.nombre)
		}
		if obtenido := utf8.RuneCountInString(verificacion.valor); obtenido != verificacion.limite {
			prueba.Errorf("%s tiene %d caracteres, esperado %d", verificacion.nombre, obtenido, verificacion.limite)
		}
	}

	derivado := dominio.LimitarProducto(dominio.Product{
		Producto:    "Producto sin SKU",
		URLProducto: &urlProducto,
	})
	if derivado.SKU == nil || *derivado.SKU != "1993238" {
		prueba.Errorf("SKU derivado = %v, esperado 1993238", derivado.SKU)
	}
}
