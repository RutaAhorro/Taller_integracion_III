package spiders

import (
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

// CompleteItem asegura que el item cumpla el esquema de Catálogo antes de
// guardarlo en el spool o publicarlo. El truncado se hace aquí, no sólo antes
// del POST, para que outbox.replay de Python sobre el mismo PVC también envíe
// valores válidos si se activa el rollback.
func CompleteItem(item domain.Item) domain.Item {
	item.Product = normalize.LimitarProducto(item.Product)
	return item
}
