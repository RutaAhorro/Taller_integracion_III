package spiders

import (
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

// CompleteItem asegura que el item cumpla el esquema de Catálogo antes de
// guardarlo en el spool o publicarlo.
func CompleteItem(item domain.Item) domain.Item {
	item.Product = normalize.LimitarProducto(item.Product)
	return item
}
