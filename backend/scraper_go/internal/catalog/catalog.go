// Package catalog carga la configuración de los sitios de scraping. Los
// valores que dependen del entorno o de cada tienda viven en spiders.json;
// este paquete sólo los valida y los expone tipados.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

//go:embed spiders.json
var embedded []byte

// FileEnv permite reemplazar el catálogo embebido sin recompilar el servicio.
const FileEnv = "SCRAPER_SPIDERS_FILE"

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
	Patterns          map[string]string   `json:"patterns,omitempty"`
	Limits            map[string]int      `json:"limits,omitempty"`
	Settings          map[string]string   `json:"settings,omitempty"`
	Lists             map[string][]string `json:"lists,omitempty"`

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
	KeepExisting bool   `json:"keep_existing"`
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
	catalogo, errorObtenido := parse(embedded)
	if errorObtenido != nil {
		panic(fmt.Sprintf("catálogo de spiders inválido: %v", errorObtenido))
	}
	return catalogo
}

// FromEnv carga el catálogo embebido o el archivo indicado por
// SCRAPER_SPIDERS_FILE. Devuelve error para que el arranque falle rápido.
func FromEnv() (Catalog, error) {
	ruta := strings.TrimSpace(os.Getenv(FileEnv))
	if ruta == "" {
		return parse(embedded)
	}
	cuerpo, errorObtenido := os.ReadFile(ruta)
	if errorObtenido != nil {
		return Catalog{}, fmt.Errorf("leer %s: %w", FileEnv, errorObtenido)
	}
	catalogo, errorObtenido := parse(cuerpo)
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

// ===== Decodificación =====

// parse deserializa la especificación JSON de spiders y valida su estructura.
func parse(cuerpo []byte) (Catalog, error) {
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
		if errorObtenido := sitio.prepare(); errorObtenido != nil {
			return Catalog{}, fmt.Errorf("sitio %q: %w", sitio.Name, errorObtenido)
		}
		if _, duplicado := vistos[sitio.Name]; duplicado {
			return Catalog{}, fmt.Errorf("nombre de sitio duplicado %q", sitio.Name)
		}
		vistos[sitio.Name] = struct{}{}
	}
	return catalogo, nil
}

// ===== Inicialización de Sitio =====

// prepare compila las expresiones regulares y valida los límites requeridos por sitio.
func (sitio *Site) prepare() error {
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
	sitio.compiled = make(map[string]*regexp.Regexp, len(sitio.Patterns))
	for nombre, patron := range sitio.Patterns {
		compilado, errorObtenido := regexp.Compile(patron)
		if errorObtenido != nil {
			return fmt.Errorf("patrón %q inválido: %w", nombre, errorObtenido)
		}
		sitio.compiled[nombre] = compilado
	}
	if sitio.CategoryFrom.IDPattern != "" {
		compilado, errorObtenido := regexp.Compile(sitio.CategoryFrom.IDPattern)
		if errorObtenido != nil {
			return fmt.Errorf("category_from.id_pattern inválido: %w", errorObtenido)
		}
		sitio.compiled["category_id"] = compilado
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

// Regexp devuelve un patrón compilado del sitio.
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
