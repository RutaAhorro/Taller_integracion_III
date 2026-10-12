package domain

import "time"

// IniciarTrabajoDTO contiene los datos requeridos para registrar el inicio de una sesión de scraping.
type IniciarTrabajoDTO struct {
	CadenaID              int     `json:"cadena_id" binding:"required"`
	Supermercado          string  `json:"supermercado,omitempty"`
	DisparadoPorUsuarioID *string `json:"disparado_por_usuario_id,omitempty"`
}

// TrabajoScraperDTO representa la información detallada de una ejecución de scraping,
// optimizada para auditoría, endpoints de control y consumo por el bot de Discord.
type TrabajoScraperDTO struct {
	ID                 string     `json:"id"`
	CadenaID           int        `json:"cadena_id"`
	CadenaNombre       string     `json:"cadena_nombre,omitempty"`
	Estado             string     `json:"estado"`
	IniciadoEl         time.Time  `json:"iniciado_el"`
	FinalizadoEl       *time.Time `json:"finalizado_el,omitempty"`
	DuracionSegundos   *float64   `json:"duracion_segundos,omitempty"`
	ElementosExtraidos int        `json:"elementos_extraidos"`
	RegistroErrores    *string    `json:"registro_errores,omitempty"`
}

// FinalizarTrabajoDTO define el payload para dar término a una ejecución de scraping.
type FinalizarTrabajoDTO struct {
	Estado             string  `json:"estado" binding:"required"` // 'completado' o 'fallido'
	ElementosExtraidos *int    `json:"elementos_extraidos,omitempty"`
	RegistroErrores    *string `json:"registro_errores,omitempty"`
}

// ProductoScrapeadoDTO representa cada item extraído por los spiders (Jumbo, Santa Isabel, etc.)
// compatible con el formato normalizado del microservicio de scraping.
type ProductoScrapeadoDTO struct {
	SKU               string   `json:"sku" binding:"required"`
	EANGTIN           *string  `json:"ean_gtin,omitempty"`
	Producto          string   `json:"producto" binding:"required"`
	Marca             *string  `json:"marca,omitempty"`
	Categoria         *string  `json:"categoria,omitempty"`
	FormatoCrudo      *string  `json:"formato_crudo,omitempty"`
	Precio            *float64 `json:"precio,omitempty"`
	PrecioNormal      float64  `json:"precio_normal" binding:"required"`
	PrecioOferta      *float64 `json:"precio_oferta,omitempty"`
	PrecioTarjeta     *float64 `json:"precio_tarjeta,omitempty"`
	PrecioPorUnidad   *float64 `json:"precio_por_unidad,omitempty"`
	MetricaUnidad     *string  `json:"metrica_unidad,omitempty"`
	MecanicaPromocion *string  `json:"mecanica_promocion,omitempty"`
	EnStock           *bool    `json:"en_stock,omitempty"`
	URLProducto       *string  `json:"url_producto,omitempty"`
	Imagen            *string  `json:"imagen,omitempty"`
	URLImagen         *string  `json:"url_imagen,omitempty"`
}

// IngestaLoteDTO es el contenedor de un lote de productos scrapeados enviados en un único request HTTP.
type IngestaLoteDTO struct {
	LoteID         string                 `json:"-"` // Idempotency-Key; no cambia el payload público.
	SucursalID     *int                   `json:"sucursal_id,omitempty"`
	CodigoSucursal *string                `json:"codigo_sucursal,omitempty"`
	Supermercado   *string                `json:"supermercado,omitempty"`
	Productos      []ProductoScrapeadoDTO `json:"productos" binding:"required,min=1"`
}

// IngestaResultadoDTO resume el resultado de procesar un lote de productos,
// diseñado con métricas claras para respuestas de API y alertas del bot de Discord.
type IngestaResultadoDTO struct {
	TrabajoID          *string  `json:"trabajo_id,omitempty"`
	SucursalID         int      `json:"sucursal_id"`
	TotalRecibidos     int      `json:"total_recibidos"`
	Insertados         int      `json:"insertados"`
	Actualizados       int      `json:"actualizados"`
	PreciosRegistrados int      `json:"precios_registrados"`
	Errores            []string `json:"errores,omitempty"`
	Mensaje            string   `json:"mensaje"`
}
