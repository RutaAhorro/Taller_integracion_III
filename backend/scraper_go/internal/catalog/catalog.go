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

// Default devuelve el catálogo embebido. Un fallo aquí es un error de build.
func Default() Catalog {
	catalog, err := parse(embedded)
	if err != nil {
		panic(fmt.Sprintf("catálogo de spiders inválido: %v", err))
	}
	return catalog
}

// FromEnv carga el catálogo embebido o el archivo indicado por
// SCRAPER_SPIDERS_FILE. Devuelve error para que el arranque falle rápido.
func FromEnv() (Catalog, error) {
	path := strings.TrimSpace(os.Getenv(FileEnv))
	if path == "" {
		return parse(embedded)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("leer %s: %w", FileEnv, err)
	}
	catalog, err := parse(body)
	if err != nil {
		return Catalog{}, fmt.Errorf("%s: %w", path, err)
	}
	return catalog, nil
}

// Get busca un sitio por nombre.
func (c Catalog) Get(name string) (Site, bool) {
	for _, site := range c.Sites {
		if site.Name == name {
			return site, true
		}
	}
	return Site{}, false
}

// Names devuelve los nombres en el orden del catálogo.
func (c Catalog) Names() []string {
	names := make([]string, 0, len(c.Sites))
	for _, site := range c.Sites {
		names = append(names, site.Name)
	}
	return names
}

func parse(body []byte) (Catalog, error) {
	var catalog Catalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return Catalog{}, err
	}
	if len(catalog.Sites) == 0 {
		return Catalog{}, fmt.Errorf("el catálogo no declara sitios")
	}
	seen := make(map[string]struct{}, len(catalog.Sites))
	for index := range catalog.Sites {
		site := &catalog.Sites[index]
		if err := site.prepare(); err != nil {
			return Catalog{}, fmt.Errorf("sitio %q: %w", site.Name, err)
		}
		if _, duplicate := seen[site.Name]; duplicate {
			return Catalog{}, fmt.Errorf("nombre de sitio duplicado %q", site.Name)
		}
		seen[site.Name] = struct{}{}
	}
	return catalog, nil
}

func (s *Site) prepare() error {
	if s.Name == "" {
		return fmt.Errorf("falta name")
	}
	if s.Parser == "" {
		return fmt.Errorf("falta parser")
	}
	if s.Supermarket == "" {
		return fmt.Errorf("falta supermarket")
	}
	if s.CategoryFile == "" {
		return fmt.Errorf("falta category_file")
	}
	if len(s.AllowedHosts) == 0 {
		return fmt.Errorf("falta allowed_hosts")
	}
	if s.CategoryURL.Message == "" || s.ProductURL.Message == "" {
		return fmt.Errorf("faltan mensajes de validación de URL")
	}
	s.hosts = make(map[string]struct{}, len(s.AllowedHosts))
	for _, host := range s.AllowedHosts {
		s.hosts[strings.ToLower(strings.TrimSpace(host))] = struct{}{}
	}
	s.compiled = make(map[string]*regexp.Regexp, len(s.Patterns))
	for name, pattern := range s.Patterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("patrón %q inválido: %w", name, err)
		}
		s.compiled[name] = compiled
	}
	if s.CategoryFrom.IDPattern != "" {
		compiled, err := regexp.Compile(s.CategoryFrom.IDPattern)
		if err != nil {
			return fmt.Errorf("category_from.id_pattern inválido: %w", err)
		}
		s.compiled["category_id"] = compiled
	}
	switch s.Pagination.Mode {
	case "", "none":
	case "query":
		if s.Pagination.Param == "" {
			return fmt.Errorf("pagination.param es obligatorio en modo query")
		}
	case "path":
		if s.Pagination.PathSegment == "" {
			return fmt.Errorf("pagination.path_segment es obligatorio en modo path")
		}
	default:
		return fmt.Errorf("pagination.mode desconocido %q", s.Pagination.Mode)
	}
	return nil
}

// Regexp devuelve un patrón compilado del sitio.
func (s Site) Regexp(name string) *regexp.Regexp {
	return s.compiled[name]
}

// Setting devuelve un valor de texto del sitio.
func (s Site) Setting(name string) string {
	return s.Settings[name]
}

// Limit devuelve un límite numérico del sitio.
func (s Site) Limit(name string) int {
	return s.Limits[name]
}

// List devuelve una lista de valores del sitio.
func (s Site) List(name string) []string {
	return s.Lists[name]
}

// Selector devuelve un selector CSS del sitio.
func (s Site) Selector(name string) string {
	return s.Selectors[name]
}
