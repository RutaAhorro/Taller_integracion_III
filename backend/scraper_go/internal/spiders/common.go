package spiders

import (
	"os"
	"path/filepath"
	"strings"
)

// ===== Carga de Categorías =====

// readCategoryURLs lee las URLs de categorías desde un archivo en disco.
func readCategoryURLs(archivo string) []string {
	cuerpo, errorObtenido := os.ReadFile(archivo)
	if errorObtenido != nil {
		return nil
	}
	var urls []string
	for _, linea := range strings.Split(string(cuerpo), "\n") {
		linea = strings.TrimSpace(linea)
		if linea == "" || strings.HasPrefix(linea, "#") {
			continue
		}
		urls = append(urls, linea)
	}
	return urls
}

// appendCategoryURL agrega una categoría validando formato y unicidad.
func appendCategoryURL(archivo, urlCruda string, validar func(string) error) error {
	if errorObtenido := validar(urlCruda); errorObtenido != nil {
		return errorObtenido
	}
	for _, existente := range readCategoryURLs(archivo) {
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

// envCategoryURLs recupera URLs de categorías declaradas en variables de entorno.
func envCategoryURLs(nombre string) []string {
	var urls []string
	for _, crudo := range strings.Split(os.Getenv(nombre), ",") {
		if valor := strings.TrimSpace(crudo); valor != "" {
			urls = append(urls, valor)
		}
	}
	return urls
}

// dedupeStrings remueve valores duplicados preservando el orden de aparición.
func dedupeStrings(valores []string) []string {
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
