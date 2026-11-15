// Package input carga listas de URLs desde argumentos o archivos.
package input

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ===== Carga de URLs =====

// Collect combina las URLs recibidas por argumento con las de un archivo, una
// por línea. Ignora líneas vacías y comentarios.
func Collect(valores []string, archivo string) ([]string, error) {
	resultado := append([]string(nil), valores...)
	if archivo == "" {
		return resultado, nil
	}
	manejador, errorObtenido := os.Open(archivo)
	if errorObtenido != nil {
		return nil, fmt.Errorf("abrir %s: %w", archivo, errorObtenido)
	}
	defer manejador.Close()

	escaner := bufio.NewScanner(manejador)
	for escaner.Scan() {
		linea := strings.TrimSpace(escaner.Text())
		if linea == "" || strings.HasPrefix(linea, "#") {
			continue
		}
		resultado = append(resultado, linea)
	}
	if errorObtenido := escaner.Err(); errorObtenido != nil {
		return nil, fmt.Errorf("leer %s: %w", archivo, errorObtenido)
	}
	return resultado, nil
}
