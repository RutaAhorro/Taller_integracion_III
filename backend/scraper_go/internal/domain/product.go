// Package domain define el contrato de producto y trabajo que comparten el
// worker, los spiders y los adaptadores de salida.
package domain

// Product es el documento que se guarda en el spool y se envía a la API.
// Los campos opcionales usan punteros para conservar los null que publica el
// scraper de Python; no se debe usar omitempty en este contrato.
type Product struct {
	Producto          string   `json:"producto"`
	Precio            *float64 `json:"precio"`
	PrecioNormal      *float64 `json:"precio_normal"`
	PrecioOferta      *float64 `json:"precio_oferta"`
	EanGtin           *string  `json:"ean_gtin"`
	SKU               *string  `json:"sku"`
	Marca             *string  `json:"marca"`
	FormatoCrudo      *string  `json:"formato_crudo"`
	MecanicaPromocion *string  `json:"mecanica_promocion"`
	EnStock           *bool    `json:"en_stock"`
	URLProducto       *string  `json:"url_producto"`
	Imagen            *string  `json:"imagen"`
	Categoria         string   `json:"categoria"`
}

// Item agrega el supermercado, que viaja en el nivel superior del lote y no
// dentro del producto.
type Item struct {
	Product
	Supermercado string `json:"supermercado"`
}

// Job representa un trabajo recibido desde la cola.
type Job struct {
	WorkID       string
	Spider       string
	ProductURL   string
	AddURL       string
	CategoryURLs []string
	EnrichEAN    bool
}

func StringPtr(value string) *string {
	return &value
}

func BoolPtr(value bool) *bool {
	return &value
}
