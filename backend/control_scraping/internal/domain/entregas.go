// Modelos y contratos de la entrega durable de lotes de productos a Catálogo,
// y de las consultas HTTP internas que Control hace a Catálogo.
package domain

import (
	"context"

	"github.com/google/uuid"
)

// ===== Límites y estados de una entrega =====

// MaximoProductosPorLote es el mismo tope que aplica Catálogo a una ingesta.
const MaximoProductosPorLote = 1000

// Estados persistidos en scraper.entregas_ingesta.estado (CHECK en la base).
const (
	EntregaPendiente  = "pendiente"
	EntregaConfirmada = "confirmada"
	EntregaFallida    = "fallida"
)

// Códigos guardados en ultimo_error al reprogramar una entrega; también se
// devuelven en el campo "codigo" de las respuestas HTTP equivalentes.
const (
	CodigoCatalogoNoDisponible = "catalogo_unavailable"
	CodigoContadorPendiente    = "contador_pendiente"
	CodigoLoteRechazado        = "lote_rechazado"
)

// ===== Modelo persistido =====

// Entrega mapea scraper.entregas_ingesta. Payload y Resultado son documentos
// JSON opacos para la persistencia.
type Entrega struct {
	ID          uuid.UUID
	TrabajoID   *uuid.UUID
	CadenaID    int
	SucursalID  *int
	PayloadHash string
	Payload     string
	Estado      string
	Intentos    int
	LeaseToken  *uuid.UUID
	Resultado   *string
}

// TableName fija la tabla donde Control guarda las entregas de ingesta.
func (Entrega) TableName() string { return "scraper.entregas_ingesta" }

// EntregaRepository define la persistencia y el ciclo de vida de lotes durables.
type EntregaRepository interface {
	Registrar(contexto context.Context, entrega *Entrega) (*Entrega, error)
	Adquirir(contexto context.Context, identificador *uuid.UUID) (*Entrega, error)
	FijarSucursal(contexto context.Context, identificador uuid.UUID, sucursalID int) (int, error)
	Confirmar(contexto context.Context, entrega *Entrega, resultado IngestaResultadoDTO) error
	Reprogramar(contexto context.Context, entrega *Entrega, codigo string, permanente bool) error
}

// ===== Contratos HTTP de ingesta =====

// ProductoScrapeadoDTO es cada producto extraído por Scraper Go; se reenvía a
// Catálogo sin transformar.
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

// IngestaLoteDTO es el cuerpo de los dos endpoints de ingesta.
type IngestaLoteDTO struct {
	LoteID         string                 `json:"-"` // Idempotency-Key; no forma parte del JSON público.
	SucursalID     *int                   `json:"sucursal_id,omitempty"`
	CodigoSucursal *string                `json:"codigo_sucursal,omitempty"`
	Supermercado   *string                `json:"supermercado,omitempty"`
	Productos      []ProductoScrapeadoDTO `json:"productos" binding:"required,min=1"`
}

// IngestaResultadoDTO resume el resultado de un lote; lo devuelve Catálogo y
// Control lo reenvía a Scraper Go.
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

// ===== Datos y contrato de Catálogo =====

// CadenaSupermercado es la parte de la respuesta de Catálogo que Control usa.
type CadenaSupermercado struct {
	ID         int    `json:"id"`
	Nombre     string `json:"nombre"`
	EstaActiva bool   `json:"esta_activa"`
}

// SucursalSupermercado es la parte de la respuesta de Catálogo que Control usa.
type SucursalSupermercado struct {
	ID       int `json:"id"`
	CadenaID int `json:"cadena_id"`
}

// CatalogoRepository es el contrato consumidor de la API interna de Catálogo.
// Una cadena o sucursal ausente se devuelve como nil sin error.
type CatalogoRepository interface {
	ObtenerCadena(contexto context.Context, cadenaID int, nombreCadena string) (*CadenaSupermercado, error)
	ObtenerSucursal(contexto context.Context, cadenaID int, sucursalID *int, codigoSucursal *string) (*SucursalSupermercado, error)
	IngestarLote(contexto context.Context, trabajoID *uuid.UUID, sucursalID int, productos []ProductoScrapeadoDTO) (*IngestaResultadoDTO, error)
}

// ===== Trazabilidad en el contexto =====

// claveLoteID y claveIDPeticion evitan colisiones con otras claves del contexto.
type (
	claveLoteID     struct{}
	claveIDPeticion struct{}
)

// ConLoteID adjunta al contexto la clave idempotente del lote en curso; el
// cliente de Catálogo la envía como lote_id.
func ConLoteID(contexto context.Context, identificador string) context.Context {
	return context.WithValue(contexto, claveLoteID{}, identificador)
}

// LoteID recupera la clave idempotente adjunta al contexto.
func LoteID(contexto context.Context) string {
	identificador, _ := contexto.Value(claveLoteID{}).(string)
	return identificador
}

// ConIDPeticion adjunta el X-Request-ID recibido para propagarlo a Catálogo.
func ConIDPeticion(contexto context.Context, identificador string) context.Context {
	return context.WithValue(contexto, claveIDPeticion{}, identificador)
}

// IDPeticion recupera el X-Request-ID adjunto al contexto.
func IDPeticion(contexto context.Context) string {
	identificador, _ := contexto.Value(claveIDPeticion{}).(string)
	return identificador
}
