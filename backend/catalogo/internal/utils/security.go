package utils

import (
	"regexp"
	"strings"
)

// SanitizarInputBusqueda limpia la entrada del usuario para prevenir Inyecciones SQL y XSS,
// limitando la longitud y permitiendo solo caracteres seguros.
// Basado en las directrices de seguridad del proyecto.
func SanitizarInputBusqueda(input string) string {
	// 1. Quitar espacios en blanco al inicio y final
	limpio := strings.TrimSpace(input)

	// 2. Limitar la longitud máxima a 100 caracteres para evitar desbordamientos o consultas pesadas
	if len(limpio) > 100 {
		limpio = limpio[:100]
	}

	// 3. Filtrar estrictamente caracteres: Permitir solo alfanuméricos, espacios, guiones y letras con tilde
	// Todo carácter especial extraño (como ;, ', ", --, etc.) será eliminado automáticamente
	reg := regexp.MustCompile(`[^a-zA-Z0-9\sñÑáéíóúÁÉÍÓÚ\-]`)
	limpio = reg.ReplaceAllString(limpio, "")

	return limpio
}
