// Package extraccion carga la configuración de los sitios de scraping, ejecuta
// el acceso HTTP con políticas de robots.txt y parsea las cinco cadenas de supermercados.
package extraccion

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
)

//go:embed spiders.json
var catalogoEmbebidoJSON []byte

// MaxSHA1HexLength es la longitud máxima en hexadecimal de un hash SHA-1 para truncado de SKU.
const MaxSHA1HexLength = 40

// ===== Modelos de Catálogo =====

// Catalog agrupa todos los sitios configurados.
type Catalog struct {
	Sites []Site `json:"sites"`
}

// Site declara lo que cambia entre tiendas. Los parsers y el ciclo común leen
// de aquí en vez de repetir literales en el código.
type Site struct {
	Name              string              `json:"name"`
	Parser            string              `json:"parser"`
	Supermarket       string              `json:"supermarket"`
	CategoryFile      string              `json:"category_file"`
	CategoryEnv       string              `json:"category_env"`
	DefaultCategories []string            `json:"default_categories"`
	AllowedHosts      []string            `json:"allowed_hosts"`
	CategoryURL       URLRule             `json:"category_url"`
	ProductURL        URLRule             `json:"product_url"`
	Pagination        Pagination          `json:"pagination"`
	CategoryFrom      CategoryFrom        `json:"category_from"`
	Selectors         map[string]string   `json:"selectors,omitempty"`
	Limits            map[string]int      `json:"limits,omitempty"`
	Settings          map[string]string   `json:"settings,omitempty"`
	Lists             map[string][]string `json:"lists,omitempty"`
	ExtraCategories   []string            `json:"-"`

	hosts    map[string]struct{}
	compiled map[string]*regexp.Regexp
}

// URLRule describe cómo validar las URLs que acepta un sitio.
type URLRule struct {
	Prefix      string `json:"prefix,omitempty"`
	Suffix      string `json:"suffix,omitempty"`
	MinSegments int    `json:"min_segments,omitempty"`
	Message     string `json:"message"`
}

// Pagination describe cómo se construye la URL de cada página. Mode puede ser
// "query" (parámetro de query) o "path" (segmento de ruta).
type Pagination struct {
	Mode         string `json:"mode"`
	Param        string `json:"param,omitempty"`
	PathSegment  string `json:"path_segment,omitempty"`
	IncludeFirst bool   `json:"include_first"`
	ReplaceQuery bool   `json:"replace_query"`
	StartFromURL bool   `json:"start_from_url"`
}

// CategoryFrom describe cómo derivar el nombre de categoría desde la URL.
type CategoryFrom struct {
	Mode        string `json:"mode"` // "last", "cugat", "lider" o "acuenta"
	StripPrefix string `json:"strip_prefix,omitempty"`
	PageSegment string `json:"page_segment,omitempty"`
	IDPattern   string `json:"id_pattern,omitempty"`
	Fallback    string `json:"fallback,omitempty"`
}

// ===== Carga de Spiders Embebidos =====

// Default devuelve el catálogo embebido. Un fallo aquí es un error de build.
func Default() Catalog {
	catalogo, errorObtenido := decodificarCatalogo(catalogoEmbebidoJSON)
	if errorObtenido != nil {
		panic(fmt.Sprintf("catálogo de spiders inválido: %v", errorObtenido))
	}
	return catalogo
}

// Load carga el catálogo embebido o el archivo externo indicado al arrancar.
func Load(ruta string) (Catalog, error) {
	ruta = strings.TrimSpace(ruta)
	if ruta == "" {
		return decodificarCatalogo(catalogoEmbebidoJSON)
	}
	cuerpo, errorObtenido := os.ReadFile(ruta)
	if errorObtenido != nil {
		return Catalog{}, fmt.Errorf("leer catálogo de spiders: %w", errorObtenido)
	}
	catalogo, errorObtenido := decodificarCatalogo(cuerpo)
	if errorObtenido != nil {
		return Catalog{}, fmt.Errorf("%s: %w", ruta, errorObtenido)
	}
	return catalogo, nil
}

// ===== Búsqueda y Accesores =====

// Get busca un sitio por nombre.
func (catalogo Catalog) Get(nombre string) (Site, bool) {
	for _, sitio := range catalogo.Sites {
		if sitio.Name == nombre {
			return sitio, true
		}
	}
	return Site{}, false
}

// Names devuelve los nombres en el orden del catálogo.
func (catalogo Catalog) Names() []string {
	nombres := make([]string, 0, len(catalogo.Sites))
	for _, sitio := range catalogo.Sites {
		nombres = append(nombres, sitio.Name)
	}
	return nombres
}

// Regexp devuelve un patrón compilado del modo de categoría.
func (sitio Site) Regexp(nombre string) *regexp.Regexp {
	return sitio.compiled[nombre]
}

// Setting devuelve un valor de texto del sitio.
func (sitio Site) Setting(nombre string) string {
	return sitio.Settings[nombre]
}

// Limit devuelve un límite numérico del sitio.
func (sitio Site) Limit(nombre string) int {
	return sitio.Limits[nombre]
}

// List devuelve una lista de valores del sitio.
func (sitio Site) List(nombre string) []string {
	return sitio.Lists[nombre]
}

// Selector devuelve un selector CSS del sitio.
func (sitio Site) Selector(nombre string) string {
	return sitio.Selectors[nombre]
}

// ===== Validación de URLs =====

// ValidateCategoryURL comprueba que la URL sea HTTPS del sitio y cumpla las
// restricciones de path declaradas para categorías.
func (sitio Site) ValidateCategoryURL(urlCruda string) error {
	return sitio.validarReglaURL(urlCruda, sitio.CategoryURL)
}

// ValidateProductURL comprueba que la URL sea HTTPS del sitio y cumpla las
// restricciones de path declaradas para fichas de producto.
func (sitio Site) ValidateProductURL(urlCruda string) error {
	return sitio.validarReglaURL(urlCruda, sitio.ProductURL)
}

// validarReglaURL comprueba un esquema HTTPS, host permitido y segmentos de ruta.
func (sitio Site) validarReglaURL(urlCruda string, regla URLRule) error {
	mensaje := regla.Message
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return errors.New(mensaje)
	}
	if analizada.Scheme != "https" {
		return fmt.Errorf("%s", mensaje)
	}
	if _, valido := sitio.hosts[strings.ToLower(analizada.Hostname())]; !valido {
		return fmt.Errorf("%s", mensaje)
	}
	if regla.Prefix != "" && !strings.HasPrefix(analizada.Path, regla.Prefix) {
		return errors.New(mensaje)
	}
	if regla.Suffix != "" && !strings.HasSuffix(strings.TrimRight(analizada.Path, "/"), regla.Suffix) {
		return errors.New(mensaje)
	}
	if regla.MinSegments > 0 && len(dominio.SegmentosNoVacios(analizada.Path)) < regla.MinSegments {
		return errors.New(mensaje)
	}
	return nil
}

// ===== Decodificación e Inicialización =====

// decodificarCatalogo deserializa la especificación JSON de spiders y valida su estructura.
func decodificarCatalogo(cuerpo []byte) (Catalog, error) {
	var catalogo Catalog
	if errorObtenido := json.Unmarshal(cuerpo, &catalogo); errorObtenido != nil {
		return Catalog{}, errorObtenido
	}
	if len(catalogo.Sites) == 0 {
		return Catalog{}, fmt.Errorf("el catálogo no declara sitios")
	}
	vistos := make(map[string]struct{}, len(catalogo.Sites))
	for indice := range catalogo.Sites {
		sitio := &catalogo.Sites[indice]
		if errorObtenido := sitio.prepararSitio(); errorObtenido != nil {
			return Catalog{}, fmt.Errorf("sitio %q: %w", sitio.Name, errorObtenido)
		}
		if _, duplicado := vistos[sitio.Name]; duplicado {
			return Catalog{}, fmt.Errorf("nombre de sitio duplicado %q", sitio.Name)
		}
		vistos[sitio.Name] = struct{}{}
	}
	return catalogo, nil
}

// prepararSitio valida los datos comunes y específicos que necesita cada parser.
func (sitio *Site) prepararSitio() error {
	if sitio.Name == "" {
		return fmt.Errorf("falta name")
	}
	if sitio.Parser == "" {
		return fmt.Errorf("falta parser")
	}
	if sitio.Supermarket == "" {
		return fmt.Errorf("falta supermarket")
	}
	if sitio.CategoryFile == "" {
		return fmt.Errorf("falta category_file")
	}
	if len(sitio.AllowedHosts) == 0 {
		return fmt.Errorf("falta allowed_hosts")
	}
	if sitio.CategoryURL.Message == "" || sitio.ProductURL.Message == "" {
		return fmt.Errorf("faltan mensajes de validación de URL")
	}
	sitio.hosts = make(map[string]struct{}, len(sitio.AllowedHosts))
	for _, servidor := range sitio.AllowedHosts {
		sitio.hosts[strings.ToLower(strings.TrimSpace(servidor))] = struct{}{}
	}
	sitio.compiled = make(map[string]*regexp.Regexp, 1)
	if sitio.CategoryFrom.IDPattern != "" {
		compilado, errorObtenido := regexp.Compile(sitio.CategoryFrom.IDPattern)
		if errorObtenido != nil {
			return fmt.Errorf("category_from.id_pattern inválido: %w", errorObtenido)
		}
		sitio.compiled["category_id"] = compilado
	}
	if errorObtenido := sitio.validarConfiguracionParser(); errorObtenido != nil {
		return errorObtenido
	}
	switch sitio.Pagination.Mode {
	case "", "none":
	case "query":
		if sitio.Pagination.Param == "" {
			return fmt.Errorf("pagination.param es obligatorio en modo query")
		}
	case "path":
		if sitio.Pagination.PathSegment == "" {
			return fmt.Errorf("pagination.path_segment es obligatorio en modo path")
		}
	default:
		return fmt.Errorf("pagination.mode desconocido %q", sitio.Pagination.Mode)
	}
	return nil
}

// validarConfiguracionParser rechaza catálogos que dejarían al extractor sin los datos que consume.
func (sitio *Site) validarConfiguracionParser() error {
	var selectoresRequeridos []string
	var ajustesRequeridos []string
	var limitesRequeridos []string
	var listasRequeridas []string

	switch sitio.Parser {
	case "jumbo":
		selectoresRequeridos = []string{"json_ld"}
		ajustesRequeridos = []string{"sku_prefix", "sku_trailing_segment"}
		limitesRequeridos = []string{"sku_max_length", "slug_max_length", "hash_length"}
	case "santa_isabel":
		selectoresRequeridos = []string{"json_ld"}
		ajustesRequeridos = []string{"seller", "product_suffix", "promotion_separator"}
	case "cugat":
		selectoresRequeridos = []string{
			"json_ld", "product_card", "title", "price_offer", "price_normal",
			"price_fallback", "price_amount", "cart", "image", "noscript",
		}
		ajustesRequeridos = []string{"card_id_prefix", "stock_class"}
	case "acuenta":
		selectoresRequeridos = []string{"card_link", "image"}
		ajustesRequeridos = []string{"sku_prefix", "image_base", "image_suffix", "promotion"}
		limitesRequeridos = []string{"rsc_window", "sku_max_length", "name_min_intersection", "token_min_length"}
		listasRequeridas = []string{"brands"}
	case "lider":
		selectoresRequeridos = []string{
			"json_ld", "next_data", "product_card", "product_card_fallback",
			"card_title", "product_link", "price_block", "price_current",
			"price_normal", "image", "brand",
		}
		ajustesRequeridos = []string{"promotion"}
		listasRequeridas = []string{"stock_in", "stock_out"}
	default:
		return fmt.Errorf("parser desconocido %q", sitio.Parser)
	}

	for _, clave := range selectoresRequeridos {
		if strings.TrimSpace(sitio.Selectors[clave]) == "" {
			return fmt.Errorf("falta selectors.%s para el parser %q", clave, sitio.Parser)
		}
	}
	for _, clave := range ajustesRequeridos {
		if strings.TrimSpace(sitio.Settings[clave]) == "" {
			return fmt.Errorf("falta settings.%s para el parser %q", clave, sitio.Parser)
		}
	}
	for _, clave := range limitesRequeridos {
		valor, existe := sitio.Limits[clave]
		if !existe || valor <= 0 {
			return fmt.Errorf("limits.%s debe ser mayor que cero para el parser %q", clave, sitio.Parser)
		}
	}
	if sitio.Parser == "jumbo" && sitio.Limits["hash_length"] > MaxSHA1HexLength {
		return fmt.Errorf("limits.hash_length no puede superar %d para el parser %q", MaxSHA1HexLength, sitio.Parser)
	}
	for _, clave := range listasRequeridas {
		valores := sitio.Lists[clave]
		if len(valores) == 0 {
			return fmt.Errorf("falta lists.%s para el parser %q", clave, sitio.Parser)
		}
		for _, valor := range valores {
			if strings.TrimSpace(valor) == "" {
				return fmt.Errorf("lists.%s contiene un valor vacío para el parser %q", clave, sitio.Parser)
			}
		}
	}
	return nil
}
