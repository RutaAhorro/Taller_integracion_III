package spiders

import (
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

// ===== Completado de Items =====

// CompleteItem asegura que el item cumpla el esquema de Catálogo antes de
// guardarlo en el spool o publicarlo.
func CompleteItem(elemento domain.Item) domain.Item {
	elemento.Product = normalize.LimitarProducto(elemento.Product)
	return elemento
}
