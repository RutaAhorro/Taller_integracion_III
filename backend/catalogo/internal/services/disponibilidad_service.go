package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/google/uuid"
)

// Errores del servicio de disponibilidad. El adaptador HTTP los traduce a
// respuestas 400 sin exponer detalles internos.
var (
	ErrLoteDisponibilidadVacio    = errors.New("la consulta de disponibilidad requiere al menos un producto")
	ErrLoteDisponibilidadExcesivo = errors.New("la consulta de disponibilidad supera el máximo de productos")
	ErrIDProductoDisponibilidad   = errors.New("el identificador de producto no es un UUID válido")
)

// LimitesDisponibilidad agrupa los valores configurables de la consulta:
// cantidad máxima de IDs y antigüedad máxima de una captura para considerarla
// vigente. Se leen una sola vez en internal/config.
type LimitesDisponibilidad struct {
	MaxProductos     int
	AntiguedadMaxima time.Duration
}

// DisponibilidadService define la lógica de negocio de la consulta interna.
type DisponibilidadService interface {
	Consultar(ctx context.Context, productoIDs []string) (*domain.RespuestaDisponibilidadDTO, error)
}

// disponibilidadService implementa la validación del lote y la regla de
// vigencia de las capturas.
type disponibilidadService struct {
	repositorio domain.DisponibilidadRepository
	limites     LimitesDisponibilidad
	ahora       func() time.Time
}

// NewDisponibilidadService inyecta el repositorio, los límites configurados y
// el reloj. El reloj se recibe como función para que las pruebas puedan fijar
// la antigüedad de las capturas sin depender de la hora real.
func NewDisponibilidadService(repositorio domain.DisponibilidadRepository, limites LimitesDisponibilidad, ahora func() time.Time) DisponibilidadService {
	if repositorio == nil {
		panic("DisponibilidadService requiere un repositorio")
	}
	if limites.MaxProductos < 1 {
		panic("LimitesDisponibilidad.MaxProductos debe ser positivo")
	}
	if limites.AntiguedadMaxima <= 0 {
		panic("LimitesDisponibilidad.AntiguedadMaxima debe ser positiva")
	}
	if ahora == nil {
		ahora = time.Now
	}
	return &disponibilidadService{repositorio: repositorio, limites: limites, ahora: ahora}
}

// Consultar valida el lote, consulta la persistencia y construye un resultado
// por cada ID solicitado. Los IDs válidos que no existen se informan como
// "no_encontrado"; una captura vencida se informa como "desconocido" y sin
// precios para no presentar datos antiguos como vigentes.
func (servicio *disponibilidadService) Consultar(ctx context.Context, productoIDs []string) (*domain.RespuestaDisponibilidadDTO, error) {
	idsNormalizados, errorValidacion := servicio.normalizarYValidar(productoIDs)
	if errorValidacion != nil {
		return nil, errorValidacion
	}

	registros, errorConsulta := servicio.repositorio.ConsultarPorIDs(ctx, idsNormalizados)
	if errorConsulta != nil {
		return nil, errorConsulta
	}

	registrosPorID := make(map[string]domain.ProductoDisponibilidadRegistro, len(registros))
	for _, registro := range registros {
		registrosPorID[registro.ProductoID] = registro
	}

	items := make([]domain.DisponibilidadItemDTO, 0, len(idsNormalizados))
	for _, productoID := range idsNormalizados {
		registro, encontrado := registrosPorID[productoID]
		items = append(items, servicio.construirItem(productoID, registro, encontrado))
	}
	return &domain.RespuestaDisponibilidadDTO{Productos: items}, nil
}

// normalizarYValidar comprueba el tope del lote, descarta espacios, valida los
// UUID y elimina repetidos conservando el orden de la primera aparición.
func (servicio *disponibilidadService) normalizarYValidar(productoIDs []string) ([]string, error) {
	if len(productoIDs) == 0 {
		return nil, ErrLoteDisponibilidadVacio
	}
	if len(productoIDs) > servicio.limites.MaxProductos {
		return nil, ErrLoteDisponibilidadExcesivo
	}

	idsNormalizados := make([]string, 0, len(productoIDs))
	vistos := make(map[string]bool, len(productoIDs))
	for _, productoID := range productoIDs {
		identificador := strings.TrimSpace(productoID)
		identificadorParseado, errorParseo := uuid.Parse(identificador)
		if errorParseo != nil {
			return nil, ErrIDProductoDisponibilidad
		}
		identificadorCanonico := identificadorParseado.String()
		if vistos[identificadorCanonico] {
			continue
		}
		vistos[identificadorCanonico] = true
		idsNormalizados = append(idsNormalizados, identificadorCanonico)
	}
	return idsNormalizados, nil
}

// construirItem aplica la regla de vigencia y arma la respuesta de un producto.
// La captura se compara contra el umbral configurado; si falta o está vencida,
// no se publica stock ni precio.
func (servicio *disponibilidadService) construirItem(productoID string, registro domain.ProductoDisponibilidadRegistro, encontrado bool) domain.DisponibilidadItemDTO {
	if !encontrado {
		return domain.DisponibilidadItemDTO{
			ProductoID:     productoID,
			EstadoProducto: domain.EstadoProductoNoEncontrado,
			EstadoPrecio:   domain.EstadoPrecioSinCaptura,
		}
	}

	item := domain.DisponibilidadItemDTO{
		ProductoID:  productoID,
		CapturadoEl: registro.CapturadoEl,
		Producto: &domain.ProductoDisponibilidadDTO{
			Nombre:       registro.Nombre,
			Marca:        registro.Marca,
			Categoria:    registro.Categoria,
			Supermercado: registro.Supermercado,
			SucursalID:   registro.SucursalID,
			Sucursal:     registro.Sucursal,
			Unidad:       registro.Unidad,
			URLImagen:    registro.URLImagen,
		},
	}

	if registro.CapturadoEl == nil {
		item.EstadoProducto = domain.EstadoProductoDesconocido
		item.EstadoPrecio = domain.EstadoPrecioSinCaptura
		return item
	}

	antiguedad := servicio.ahora().UTC().Sub(registro.CapturadoEl.UTC())
	if antiguedad > servicio.limites.AntiguedadMaxima {
		item.EstadoProducto = domain.EstadoProductoDesconocido
		item.EstadoPrecio = domain.EstadoPrecioVencido
		return item
	}

	enStock := registro.EnStock
	item.EnStock = &enStock
	item.EstadoPrecio = domain.EstadoPrecioVigente
	if enStock {
		item.EstadoProducto = domain.EstadoProductoDisponible
	} else {
		item.EstadoProducto = domain.EstadoProductoSinStock
	}
	item.Producto.PrecioNormal = registro.PrecioNormal
	item.Producto.PrecioOferta = registro.PrecioOferta
	item.Producto.Precio = precioEfectivo(registro.PrecioNormal, registro.PrecioOferta)
	return item
}

// precioEfectivo replica la regla del catálogo público: se muestra la oferta
// cuando existe y, si no, el precio normal.
func precioEfectivo(precioNormal, precioOferta *float64) *float64 {
	if precioOferta != nil {
		return precioOferta
	}
	return precioNormal
}
