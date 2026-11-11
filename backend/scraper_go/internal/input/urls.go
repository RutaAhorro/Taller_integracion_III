// Package input carga listas de URLs desde argumentos o archivos.
package input

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Collect combina las URLs recibidas por argumento con las de un archivo, una
// por línea. Ignora líneas vacías y comentarios.
func Collect(values []string, file string) ([]string, error) {
	result := append([]string(nil), values...)
	if file == "" {
		return result, nil
	}
	handle, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("abrir %s: %w", file, err)
	}
	defer handle.Close()

	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		result = append(result, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("leer %s: %w", file, err)
	}
	return result, nil
}
