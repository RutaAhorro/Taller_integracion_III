package utils

import (
	"regexp"
	"strings"
)

var caracteresNoPermitidos = regexp.MustCompile(`[^a-zA-Z0-9\sñÑáéíóúÁÉÍÓÚ\-]`)

// SanitizarInputBusqueda limita nombres a 100 caracteres y filtra su alfabeto.
// La seguridad SQL depende de consultas parametrizadas; HTML debe escaparse al mostrarlo.
func SanitizarInputBusqueda(input string) string {
	// 1. Quitar espacios en blanco al inicio y final
	limpio := strings.TrimSpace(input)

	// 2. Limitar la longitud máxima a 100 caracteres para evitar desbordamientos o consultas pesadas
	runas := []rune(limpio)
	if len(runas) > 100 {
		limpio = string(runas[:100])
	}

	// 3. Filtrar estrictamente caracteres: Permitir solo alfanuméricos, espacios, guiones y letras con tilde
	// El alfabeto heredado no incluye signos como ;, comillas o corchetes.
	limpio = caracteresNoPermitidos.ReplaceAllString(limpio, "")

	return strings.TrimSpace(limpio)
}
