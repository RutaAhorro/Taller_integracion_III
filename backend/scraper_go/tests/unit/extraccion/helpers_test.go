// helpers_test.go reúne los dobles de prueba y aserciones compartidas por los
// tests unitarios del paquete extraccion.
package extraccion_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
)

// ===== Dobles de Prueba HTTP =====

type obtenedorFalso struct {
	respuestas map[string]*extraccion.Response
	errores    map[string]error
	llamadas   []string
}

// Fetch devuelve la respuesta o error preconfigurado para la URL solicitada.
func (falso *obtenedorFalso) Fetch(_ context.Context, urlCruda string) (*extraccion.Response, error) {
	falso.llamadas = append(falso.llamadas, urlCruda)
	if errorConfigurado, valido := falso.errores[urlCruda]; valido {
		return nil, errorConfigurado
	}
	if respuestaConfigurada, valido := falso.respuestas[urlCruda]; valido {
		return respuestaConfigurada, nil
	}
	return &extraccion.Response{URL: urlObligatoria(urlCruda), Status: http.StatusNotFound}, nil
}

// respuestaExitosa construye una respuesta HTTP 200 de prueba.
func respuestaExitosa(urlCruda, cuerpo string) *extraccion.Response {
	return &extraccion.Response{URL: urlObligatoria(urlCruda), Status: http.StatusOK, Body: []byte(cuerpo)}
}

// urlObligatoria parsea una URL de prueba o aborta con panic si es inválida.
func urlObligatoria(urlCruda string) *url.URL {
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		panic(errorObtenido)
	}
	return analizada
}

// urlDePrueba parsea una URL validando con testing.T.
func urlDePrueba(prueba *testing.T, urlCruda string) *url.URL {
	prueba.Helper()
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		prueba.Fatalf("url.Parse(%q) error = %v", urlCruda, errorObtenido)
	}
	return analizada
}

// registradorSilencioso devuelve un logger que descarta la salida durante los tests.
func registradorSilencioso() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ===== Auxiliares de Catálogo y Spiders =====

// sitioDelCatalogo recupera un sitio del catálogo embebido por su nombre.
func sitioDelCatalogo(prueba *testing.T, nombre string) extraccion.Site {
	prueba.Helper()
	sitio, valido := extraccion.Default().Get(nombre)
	if !valido {
		prueba.Fatalf("falta el sitio %q en el catálogo", nombre)
	}
	return sitio
}

// construirSpiderDePrueba instancia un spider desde el catálogo embebido con categorías adicionales opcionales.
func construirSpiderDePrueba(
	prueba *testing.T,
	nombre string,
	obtenedor extraccion.Fetcher,
	directorioCategorias string,
	maxPaginas int,
	categoriasExtra ...string,
) extraccion.Spider {
	prueba.Helper()
	sitio := sitioDelCatalogo(prueba, nombre)
	if len(categoriasExtra) > 0 {
		sitio.ExtraCategories = append([]string(nil), categoriasExtra...)
	}
	spider, errorObtenido := extraccion.Build(sitio, obtenedor, directorioCategorias, maxPaginas, registradorSilencioso())
	if errorObtenido != nil {
		prueba.Fatalf("Build(%q) error = %v", nombre, errorObtenido)
	}
	return spider
}

// recolectarItems ejecuta un spider y devuelve todos los ítems emitidos.
func recolectarItems(prueba *testing.T, spider extraccion.Spider, trabajo dominio.Job) []dominio.Item {
	prueba.Helper()
	var elementos []dominio.Item
	if errorObtenido := spider.Run(context.Background(), trabajo, func(elemento dominio.Item) error {
		elementos = append(elementos, elemento)
		return nil
	}); errorObtenido != nil {
		prueba.Fatalf("%s.Run() error = %v", spider.Name(), errorObtenido)
	}
	return elementos
}

// ===== Aserciones de Producto =====

// valorCadena desreferencia un puntero a string o devuelve cadena vacía.
func valorCadena(valor *string) string {
	if valor == nil {
		return ""
	}
	return *valor
}

// valorFlotante desreferencia un puntero a float64 informando si existía.
func valorFlotante(valor *float64) (float64, bool) {
	if valor == nil {
		return 0, false
	}
	return *valor, true
}

// valorBooleano desreferencia un puntero a bool informando si existía.
func valorBooleano(valor *bool) (bool, bool) {
	if valor == nil {
		return false, false
	}
	return *valor, true
}

// verificarCamposProducto compara los campos textuales de un producto contra un mapa esperado.
func verificarCamposProducto(prueba *testing.T, producto dominio.Product, esperados map[string]string) {
	prueba.Helper()
	for campo, esperado := range esperados {
		var obtenido string
		switch campo {
		case "producto":
			obtenido = producto.Producto
		case "sku":
			obtenido = valorCadena(producto.SKU)
		case "ean":
			obtenido = valorCadena(producto.EanGtin)
		case "marca":
			obtenido = valorCadena(producto.Marca)
		case "formato":
			obtenido = valorCadena(producto.FormatoCrudo)
		case "promocion":
			obtenido = valorCadena(producto.MecanicaPromocion)
		case "imagen":
			obtenido = valorCadena(producto.Imagen)
		case "url":
			obtenido = valorCadena(producto.URLProducto)
		case "categoria":
			obtenido = producto.Categoria
		}
		if obtenido != esperado {
			prueba.Errorf("%s = %q, esperado %q", campo, obtenido, esperado)
		}
	}
}
