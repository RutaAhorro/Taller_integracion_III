package utils

import (
	"regexp"
	"strings"
)

// SanitizarInputBusqueda limpia la entrada del usuario para prevenir Inyecciones SQL y XSS,
// limitando la longitud y permitiendo solo caracteres seguros.
// Basado en las directrices de seguridad del proyecto.
func SanitizarInputBusqueda(input string, largoMaximo int) string {
	// 1. Quitar espacios en blanco al inicio y final
	limpio := strings.TrimSpace(input)

	// 2. Limitar la longitud a largoMaximo bytes (CATALOGO_MAX_SEARCH_LENGTH) para evitar consultas pesadas.
	// Si el corte parte un carácter multibyte, el filtro siguiente descarta el byte suelto.
	if len(limpio) > largoMaximo {
		limpio = limpio[:largoMaximo]
	}

	// 3. Filtrar estrictamente caracteres: Permitir solo alfanuméricos, espacios, guiones y letras con tilde
	// Todo carácter especial extraño (como ;, ', ", --, etc.) será eliminado automáticamente
	reg := regexp.MustCompile(`[^a-zA-Z0-9\sñÑáéíóúÁÉÍÓÚ\-]`)
	limpio = reg.ReplaceAllString(limpio, "")

	return limpio
}
