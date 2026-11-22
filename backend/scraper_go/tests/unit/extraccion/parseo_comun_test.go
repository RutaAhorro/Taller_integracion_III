// parseo_comun_test.go verifica la construcción de URLs de paginación, la
// derivación de categorías desde la URL y las utilidades comunes de extracción.
package extraccion_test

import (
	"regexp"
	"testing"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
)

// ===== Paginación y Categorías =====

// TestPageURL verifica la generación de URLs de página en modos query y path.
func TestPageURL(prueba *testing.T) {
	casos := []struct {
		nombre   string
		urlCruda string
		pagina   int
		esperado string
	}{
		{
			nombre:   "jumbo_rsc",
			urlCruda: "https://www.jumbo.cl/frutas-y-verduras/verduras",
			pagina:   2,
			esperado: "https://www.jumbo.cl/frutas-y-verduras/verduras?page=2",
		},
		{
			nombre:   "santa_isabel_rsc",
			urlCruda: "https://www.santaisabel.cl/despensa",
			pagina:   2,
			esperado: "https://www.santaisabel.cl/despensa?page=2",
		},
		{
			nombre:   "cugat_rsc",
			urlCruda: "https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/",
			pagina:   2,
			esperado: "https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/page/2/",
		},
		{
			nombre:   "cugat_rsc",
			urlCruda: "https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/page/2/",
			pagina:   3,
			esperado: "https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/page/3/",
		},
		{
			nombre:   "acuenta_rsc",
			urlCruda: "https://www.acuenta.cl/ca/congelados/04?currentPage=2&_rsc=2x5uv",
			pagina:   3,
			esperado: "https://www.acuenta.cl/ca/congelados/04?currentPage=3",
		},
		{
			nombre:   "acuenta_rsc",
			urlCruda: "https://www.acuenta.cl/ca/congelados/04",
			pagina:   1,
			esperado: "https://www.acuenta.cl/ca/congelados/04?currentPage=1",
		},
		{
			nombre:   "lider_rsc",
			urlCruda: "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836?sort=price",
			pagina:   2,
			esperado: "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836?sort=price&page=2",
		},
		{
			nombre:   "lider_rsc",
			urlCruda: "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836?page=2",
			pagina:   3,
			esperado: "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836?page=3",
		},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre+" "+caso.esperado, func(prueba *testing.T) {
			if obtenido := extraccion.PageURL(sitioDelCatalogo(prueba, caso.nombre), caso.urlCruda, caso.pagina); obtenido != caso.esperado {
				prueba.Errorf("PageURL() = %q, esperado %q", obtenido, caso.esperado)
			}
		})
	}
}

// TestStartPage verifica la lectura de página inicial desde la query string cuando el sitio lo permite.
func TestStartPage(prueba *testing.T) {
	acuenta := sitioDelCatalogo(prueba, "acuenta_rsc")
	if obtenido := extraccion.StartPage(acuenta, "https://www.acuenta.cl/ca/congelados/04?currentPage=3"); obtenido != 3 {
		prueba.Errorf("StartPage(acuenta) = %d, esperado 3", obtenido)
	}
	if obtenido := extraccion.StartPage(acuenta, "https://www.acuenta.cl/ca/congelados/04"); obtenido != 1 {
		prueba.Errorf("StartPage(acuenta sin página) = %d, esperado 1", obtenido)
	}

	lider := sitioDelCatalogo(prueba, "lider_rsc")
	if obtenido := extraccion.StartPage(lider, "https://super.lider.cl/browse/frutas/22884697?page=2"); obtenido != 2 {
		prueba.Errorf("StartPage(lider) = %d, esperado 2", obtenido)
	}

	jumbo := sitioDelCatalogo(prueba, "jumbo_rsc")
	if obtenido := extraccion.StartPage(jumbo, "https://www.jumbo.cl/frutas?page=4"); obtenido != 1 {
		prueba.Errorf("StartPage(jumbo) = %d, esperado 1", obtenido)
	}
}

// TestCategoryFromURL verifica la extracción del slug de categoría para las cinco cadenas.
func TestCategoryFromURL(prueba *testing.T) {
	casos := []struct {
		nombre   string
		urlCruda string
		esperado string
	}{
		{"jumbo_rsc", "https://www.jumbo.cl/frutas-y-verduras/verduras/", "verduras"},
		{"santa_isabel_rsc", "https://www.santaisabel.cl/despensa", "despensa"},
		{"cugat_rsc", "https://cugat.cl/categoria-producto/bebidas-jugos-y-aguas/page/2/", "bebidas-jugos-y-aguas"},
		{"acuenta_rsc", "https://www.acuenta.cl/ca/congelados/04", "congelados"},
		{"lider_rsc", "https://super.lider.cl/browse/frutas-y-verduras/frutas/22884697_93034836", "frutas"},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			if obtenido := extraccion.CategoryFromURL(sitioDelCatalogo(prueba, caso.nombre), caso.urlCruda); obtenido != caso.esperado {
				prueba.Errorf("CategoryFromURL() = %q, esperado %q", obtenido, caso.esperado)
			}
		})
	}

	cugat := sitioDelCatalogo(prueba, "cugat_rsc")
	if obtenido := extraccion.CategoryFromURL(cugat, "https://cugat.cl/"); obtenido != "sin_categoria" {
		prueba.Errorf("CategoryFromURL() fallback = %q, esperado sin_categoria", obtenido)
	}
}

// ===== Resolución y Selección de Productos =====

// TestResolveURL verifica la resolución de rutas relativas frente a la URL base de respuesta.
func TestResolveURL(prueba *testing.T) {
	urlBase := urlDePrueba(prueba, "https://cugat.cl/categoria-producto/bebidas/page/2/")
	if obtenido := extraccion.ResolveURL(urlBase, "/producto/uno/"); obtenido != "https://cugat.cl/producto/uno/" {
		prueba.Errorf("ResolveURL() = %q", obtenido)
	}
	if obtenido := extraccion.ResolveURL(nil, "/producto/uno/"); obtenido != "/producto/uno/" {
		prueba.Errorf("ResolveURL(nil) = %q", obtenido)
	}
}

// TestSelectProductByURL verifica el filtrado de candidatos por coincidencia de host+path.
func TestSelectProductByURL(prueba *testing.T) {
	urlTomate := "https://www.jumbo.cl/p/tomate"
	urlLechuga := "https://www.jumbo.cl/p/lechuga"
	productos := []dominio.Product{
		{Producto: "Tomate", URLProducto: &urlTomate},
		{Producto: "Lechuga", URLProducto: &urlLechuga},
	}

	seleccionado := extraccion.SelectProductByURL(productos, "https://www.jumbo.cl/p/lechuga?utm=1", urlDePrueba(prueba, "https://www.jumbo.cl/cat"))
	if seleccionado == nil || seleccionado.Producto != "Lechuga" {
		prueba.Errorf("SelectProductByURL() = %+v", seleccionado)
	}

	if obtenido := extraccion.SelectProductByURL(productos, "https://www.jumbo.cl/p/otro", urlDePrueba(prueba, "https://www.jumbo.cl/cat")); obtenido != nil {
		prueba.Errorf("SelectProductByURL() con varios candidatos = %+v, esperado nil", obtenido)
	}

	unico := []dominio.Product{{Producto: "Único", URLProducto: &urlTomate}}
	if obtenido := extraccion.SelectProductByURL(unico, "https://www.jumbo.cl/p/otro", urlDePrueba(prueba, "https://www.jumbo.cl/cat")); obtenido == nil || obtenido.Producto != "Único" {
		prueba.Errorf("SelectProductByURL() con un candidato = %+v", obtenido)
	}
}

// TestFormatFromTextYTitleCase verifica la extracción de unidad de medida y capitalización de títulos.
func TestFormatFromTextYTitleCase(prueba *testing.T) {
	patron := regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*(?:kg|g|l)\b`)
	if obtenido := extraccion.FormatFromText("Bebida 1.5 L", patron); obtenido == nil || *obtenido != "1.5 L" {
		prueba.Errorf("FormatFromText() = %v", obtenido)
	}
	if obtenido := extraccion.FormatFromText("Pan", patron); obtenido != nil {
		prueba.Errorf("FormatFromText(sin formato) = %v", obtenido)
	}
	if obtenido := extraccion.TitleCase("coca cola 1.5l"); obtenido != "Coca Cola 1.5L" {
		prueba.Errorf("TitleCase() = %q", obtenido)
	}
}
