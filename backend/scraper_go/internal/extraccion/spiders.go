// Archivo spiders.go define los contratos de Spider, Emitter y Fetcher, el
// registro de spiders y el ciclo común de recorrido por categorías o fichas.
package extraccion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
)

// ===== Nombres de Spiders =====

const (
	JumboName       = "jumbo_rsc"
	SantaIsabelName = "santa_isabel_rsc"
	CugatName       = "cugat_rsc"
	AcuentaName     = "acuenta_rsc"
	LiderName       = "lider_rsc"
)

// ===== Contratos =====

// Emitter recibe cada producto ya normalizado. El worker lo conecta con el
// spool y la entrega por lotes.
type Emitter func(dominio.Item) error

// Fetcher permite probar los spiders sin red y deja la puerta abierta para
// extraer el motor de fetch a otro servicio más adelante.
type Fetcher interface {
	Fetch(contexto context.Context, urlCruda string) (*Response, error)
}

// Spider es el contrato mínimo de una cadena: nombre y ejecución de un Job.
type Spider interface {
	Name() string
	Run(contexto context.Context, trabajo dominio.Job, emitir Emitter) error
}

// ===== Construcción y Registro =====

// Build construye el spider correspondiente al parser declarado en el catálogo.
func Build(sitio Site, obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) (Spider, error) {
	switch sitio.Parser {
	case "jumbo":
		return nuevoJumbo(sitio, obtenedor, directorioCategorias, maxPaginas, registrador), nil
	case "santa_isabel":
		return nuevoSantaIsabel(sitio, obtenedor, directorioCategorias, maxPaginas, registrador), nil
	case "cugat":
		return nuevoCugat(sitio, obtenedor, directorioCategorias, maxPaginas, registrador), nil
	case "acuenta":
		return nuevoAcuenta(sitio, obtenedor, directorioCategorias, maxPaginas, registrador), nil
	case "lider":
		return nuevoLider(sitio, obtenedor, directorioCategorias, maxPaginas, registrador), nil
	default:
		return nil, fmt.Errorf("parser desconocido %q", sitio.Parser)
	}
}

// Registry contiene los spiders disponibles indexados por nombre.
type Registry struct {
	spiders map[string]Spider
}

// NewRegistryFromConfig inyecta el catálogo y las categorías ya leídos al arrancar.
func NewRegistryFromConfig(configuracion config.Config, obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) (*Registry, error) {
	sitios, errorObtenido := Load(configuracion.SpidersFile)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	for indice := range sitios.Sites {
		sitio := &sitios.Sites[indice]
		sitio.ExtraCategories = configuracion.CategoryURLs[sitio.CategoryEnv]
	}
	return NewRegistryFromCatalog(sitios, obtenedor, directorioCategorias, maxPaginas, registrador)
}

// NewRegistryFromCatalog construye todos los spiders declarados en el catálogo.
func NewRegistryFromCatalog(sitios Catalog, obtenedor Fetcher, directorioCategorias string, maxPaginas int, registrador *slog.Logger) (*Registry, error) {
	registro := &Registry{spiders: make(map[string]Spider, len(sitios.Sites))}
	for _, sitio := range sitios.Sites {
		spider, errorObtenido := Build(sitio, obtenedor, directorioCategorias, maxPaginas, registrador)
		if errorObtenido != nil {
			return nil, errorObtenido
		}
		registro.Register(spider)
	}
	return registro, nil
}

// Register añade o reemplaza un spider en el registro.
func (registro *Registry) Register(spider Spider) {
	if registro.spiders == nil {
		registro.spiders = make(map[string]Spider)
	}
	registro.spiders[spider.Name()] = spider
}

// Get devuelve el spider con ese nombre y si existe.
func (registro *Registry) Get(nombre string) (Spider, bool) {
	spider, valido := registro.spiders[nombre]
	return spider, valido
}

// ===== Completado de Items =====

// CompleteItem asegura que el item cumpla el esquema de Catálogo antes de
// guardarlo en el spool o publicarlo.
func CompleteItem(elemento dominio.Item) dominio.Item {
	elemento.Product = dominio.LimitarProducto(elemento.Product)
	return elemento
}

// ===== Ciclo Común de Spiders =====

// configuracionSpider declara sólo las funciones de parsing. El resto de la
// configuración sale de Site.
type configuracionSpider struct {
	extraerProductos func([]byte, *url.URL) []dominio.Product
	extraerProducto  func([]byte, *url.URL, string) *dominio.Product
}

// spiderBase concentra el comportamiento común de los spiders de supermercados.
type spiderBase struct {
	obtenedor            Fetcher
	directorioCategorias string
	maxPaginas           int
	registrador          *slog.Logger
	sitio                Site
	configuracion        configuracionSpider

	// materializar sólo lo implementa Jumbo para enriquecer EAN desde la ficha.
	materializar func(context.Context, dominio.Product, string, *url.URL, bool) (dominio.Item, error)
}

// nuevoSpiderBase crea una instancia base compartida con configuración del catálogo.
func nuevoSpiderBase(
	obtenedor Fetcher,
	directorioCategorias string,
	maxPaginas int,
	registrador *slog.Logger,
	sitio Site,
	configuracion configuracionSpider,
) spiderBase {
	if maxPaginas <= 0 {
		maxPaginas = config.DefaultMaxPages
	}
	if registrador == nil {
		registrador = slog.Default()
	}
	return spiderBase{
		obtenedor:            obtenedor,
		directorioCategorias: directorioCategorias,
		maxPaginas:           maxPaginas,
		registrador:          registrador,
		sitio:                sitio,
		configuracion:        configuracion,
	}
}

// Name devuelve el identificador oficial del spider.
func (base *spiderBase) Name() string {
	return base.sitio.Name
}

// Run ejecuta la rutina de extracción del spider para el trabajo indicado.
func (base *spiderBase) Run(contexto context.Context, trabajo dominio.Job, emitir Emitter) error {
	if trabajo.ProductURL != "" {
		return base.ejecutarProducto(contexto, trabajo, emitir)
	}
	if trabajo.AddURL != "" {
		if errorObtenido := base.agregarURLCategoria(trabajo.AddURL); errorObtenido != nil {
			return errorObtenido
		}
	}
	urls, errorObtenido := base.urlsIniciales(trabajo.CategoryURLs)
	if errorObtenido != nil {
		return errorObtenido
	}
	for _, urlCategoria := range urls {
		if errorObtenido := base.ejecutarCategoria(contexto, urlCategoria, trabajo.EnrichEAN, emitir); errorObtenido != nil {
			return errorObtenido
		}
	}
	return nil
}

// ejecutarProducto atiende un scraping puntual. Nunca cae al crawl de categorías.
func (base *spiderBase) ejecutarProducto(contexto context.Context, trabajo dominio.Job, emitir Emitter) error {
	if errorObtenido := base.sitio.ValidateProductURL(trabajo.ProductURL); errorObtenido != nil {
		return errorObtenido
	}
	respuesta, errorObtenido := base.obtenedor.Fetch(contexto, trabajo.ProductURL)
	if errorObtenido != nil {
		return errorObtenido
	}
	if respuesta.Status == http.StatusNotFound {
		base.registrador.Warn(base.sitio.Name+"_product_not_found", "url", trabajo.ProductURL)
		return nil
	}
	producto := base.configuracion.extraerProducto(respuesta.Body, respuesta.URL, trabajo.ProductURL)
	if producto == nil {
		base.registrador.Warn(base.sitio.Name+"_product_without_data", "url", trabajo.ProductURL)
		return nil
	}
	return emitir(base.convertirAItem(*producto, "producto", trabajo.ProductURL))
}

// ejecutarCategoria procesa la secuencia de páginas pertenecientes a una categoría.
func (base *spiderBase) ejecutarCategoria(contexto context.Context, urlCategoria string, enriquecerEAN bool, emitir Emitter) error {
	categoria := CategoryFromURL(base.sitio, urlCategoria)
	for pagina := StartPage(base.sitio, urlCategoria); pagina <= base.maxPaginas; pagina++ {
		urlPagina := PageURL(base.sitio, urlCategoria, pagina)
		respuesta, errorObtenido := base.obtenedor.Fetch(contexto, urlPagina)
		if errorObtenido != nil {
			if errors.Is(errorObtenido, ErrRobotsDisallowed) {
				base.registrador.Warn(base.sitio.Name+"_category_robots_disallowed", "url", urlPagina)
				return nil
			}
			var errorEstado *HTTPStatusError
			if errors.As(errorObtenido, &errorEstado) && errorEstado.Status == http.StatusNotFound {
				base.registrador.Info(base.sitio.Name+"_pagination_end", "url", urlPagina, "page", pagina)
				return nil
			}
			return errorObtenido
		}
		if respuesta.Status == http.StatusNotFound {
			base.registrador.Info(base.sitio.Name+"_pagination_end", "url", urlPagina, "page", pagina)
			return nil
		}

		productos := base.configuracion.extraerProductos(respuesta.Body, respuesta.URL)
		if len(productos) == 0 {
			base.registrador.Warn(base.sitio.Name+"_category_without_products", "url", urlPagina, "page", pagina)
			return nil
		}
		for _, producto := range productos {
			elemento, errorObtenido := base.construirItem(contexto, producto, categoria, respuesta.URL, enriquecerEAN)
			if errorObtenido != nil {
				return errorObtenido
			}
			if errorObtenido := emitir(elemento); errorObtenido != nil {
				return errorObtenido
			}
		}
		if pagina == base.maxPaginas {
			base.registrador.Warn(base.sitio.Name+"_pagination_limit", "url", urlCategoria, "max_pages", base.maxPaginas)
		}
	}
	return nil
}

// construirItem empaqueta un producto asociándole su supermercado de origen.
func (base *spiderBase) construirItem(
	contexto context.Context,
	producto dominio.Product,
	categoria string,
	urlRespuesta *url.URL,
	enriquecerEAN bool,
) (dominio.Item, error) {
	if base.materializar != nil {
		return base.materializar(contexto, producto, categoria, urlRespuesta, enriquecerEAN)
	}
	return base.convertirAItem(producto, categoria, textoURL(urlRespuesta)), nil
}

// urlsIniciales determina las URLs iniciales de extracción a partir del trabajo.
func (base *spiderBase) urlsIniciales(sobrescrituras []string) ([]string, error) {
	urls := append([]string(nil), sobrescrituras...)
	if len(urls) == 0 {
		urls = leerURLsCategoria(filepath.Join(base.directorioCategorias, base.sitio.CategoryFile))
		urls = append(urls, base.sitio.ExtraCategories...)
	}
	urls = eliminarDuplicadosTexto(urls)
	if len(urls) == 0 {
		return append([]string(nil), base.sitio.DefaultCategories...), nil
	}
	for _, urlCruda := range urls {
		if errorObtenido := base.sitio.ValidateCategoryURL(urlCruda); errorObtenido != nil {
			return nil, errorObtenido
		}
	}
	return urls, nil
}

// agregarURLCategoria agrega una categoría validando formato y unicidad.
func (base *spiderBase) agregarURLCategoria(urlCruda string) error {
	return guardarURLCategoria(
		filepath.Join(base.directorioCategorias, base.sitio.CategoryFile),
		urlCruda,
		base.sitio.ValidateCategoryURL,
	)
}

// convertirAItem convierte un producto en elemento listo para ser emitido.
func (base *spiderBase) convertirAItem(producto dominio.Product, categoria, urlRespuesta string) dominio.Item {
	if producto.URLProducto == nil || *producto.URLProducto == "" {
		producto.URLProducto = dominio.StringPtr(urlRespuesta)
	}
	producto.Categoria = categoria
	return dominio.Item{Product: producto, Supermercado: base.sitio.Supermarket}
}

// ===== Carga y Persistencia de Categorías =====

// CollectURLs combina las URLs recibidas por argumento con las de un archivo,
// una por línea, ignorando líneas vacías y comentarios.
func CollectURLs(valores []string, archivo string) ([]string, error) {
	resultado := append([]string(nil), valores...)
	if archivo == "" {
		return resultado, nil
	}
	cuerpo, errorObtenido := os.ReadFile(archivo)
	if errorObtenido != nil {
		return nil, fmt.Errorf("abrir %s: %w", archivo, errorObtenido)
	}
	for _, linea := range strings.Split(string(cuerpo), "\n") {
		linea = strings.TrimSpace(linea)
		if linea == "" || strings.HasPrefix(linea, "#") {
			continue
		}
		resultado = append(resultado, linea)
	}
	return resultado, nil
}

// leerURLsCategoria lee las URLs de categorías desde un archivo en disco.
func leerURLsCategoria(archivo string) []string {
	urls, errorObtenido := CollectURLs(nil, archivo)
	if errorObtenido != nil {
		return nil
	}
	return urls
}

// guardarURLCategoria agrega una categoría en disco validando formato y unicidad.
func guardarURLCategoria(archivo, urlCruda string, validar func(string) error) error {
	if errorObtenido := validar(urlCruda); errorObtenido != nil {
		return errorObtenido
	}
	for _, existente := range leerURLsCategoria(archivo) {
		if existente == urlCruda {
			return nil
		}
	}
	if errorObtenido := os.MkdirAll(filepath.Dir(archivo), 0o755); errorObtenido != nil {
		return errorObtenido
	}
	manejador, errorObtenido := os.OpenFile(archivo, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if errorObtenido != nil {
		return errorObtenido
	}
	defer manejador.Close()
	_, errorObtenido = manejador.WriteString(urlCruda + "\n")
	return errorObtenido
}

// eliminarDuplicadosTexto remueve valores duplicados preservando el orden de aparición.
func eliminarDuplicadosTexto(valores []string) []string {
	vistos := make(map[string]struct{}, len(valores))
	resultado := make([]string, 0, len(valores))
	for _, valor := range valores {
		if _, valido := vistos[valor]; valido {
			continue
		}
		vistos[valor] = struct{}{}
		resultado = append(resultado, valor)
	}
	return resultado
}
