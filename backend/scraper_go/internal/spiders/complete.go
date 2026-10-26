package spiders

import (
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

// CompleteItem asegura los campos que Catálogo exige antes de guardar el item
// en el spool o publicarlo. Algunas fichas comercian sin SKU en el HTML; en
// ese caso se deriva un identificador estable de la URL del producto.
func CompleteItem(item domain.Item) domain.Item {
	if item.SKU == nil || strings.TrimSpace(*item.SKU) == "" {
		if item.URLProducto != nil {
			item.SKU = normalize.SKUFromURL(*item.URLProducto)
		}
	}
	if item.SKU != nil {
		sku := normalize.Truncar(*item.SKU, 100)
		item.SKU = &sku
	}
	return item
}
