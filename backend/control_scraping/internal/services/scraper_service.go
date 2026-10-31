package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
)

// Errores de negocio para el microservicio de scraping
var (
	ErrTrabajoNoEncontrado   = errors.New("trabajo de scraping no encontrado")
	ErrTrabajoYaFinalizado   = errors.New("el trabajo de scraping ya ha sido finalizado")
	ErrCadenaNoEncontrada    = errors.New("cadena de supermercado no encontrada")
	ErrSucursalNoEncontrada  = errors.New("sucursal de supermercado no encontrada")
	ErrLoteVacio             = errors.New("el lote de productos no puede estar vacío")
	ErrLoteExcedeMaximo      = errors.New("el lote excede el tamaño máximo permitido de 1000 productos")
	ErrEstadoTrabajoInvalido = errors.New("el estado debe ser 'completado' o 'fallido'")
	ErrUUIDInvalido          = errors.New("el identificador proporcionado no es un UUID válido")
	ErrFuenteDeshabilitada   = errors.New("la fuente de scraping se encuentra deshabilitada")
	ErrCatalogoNoDisponible  = errors.New("Catálogo no está disponible")
	ErrCatalogoTimeout       = errors.New("Catálogo no respondió a tiempo")
	ErrContadorPendiente     = errors.New("Catálogo confirmó el lote y el contador está pendiente; conservar la clave del lote")
	ErrIdempotencia          = errors.New("la clave ya fue utilizada con otro contenido")
	ErrEntregaPendiente      = errors.New("el lote está registrado y pendiente de confirmación")
	ErrEntregaFallida        = errors.New("hay una entrega fallida o rechazada; requiere revisión")
	ErrCatalogoRechazo       = errors.New("Catálogo rechazó el lote")
)

// ScraperService define la lógica de negocio para auditar trabajos e ingestar catálogos
type ScraperService interface {
	IniciarTrabajo(ctx context.Context, input domain.IniciarTrabajoDTO) (*domain.TrabajoScraperDTO, error)
	FinalizarTrabajo(ctx context.Context, id string, input domain.FinalizarTrabajoDTO) (*domain.TrabajoScraperDTO, error)
	ObtenerTrabajo(ctx context.Context, id string) (*domain.TrabajoScraperDTO, error)
	ObtenerCadena(ctx context.Context, cadenaID int, nombre string) (*domain.CadenaSupermercado, error)
	IngestarProductos(ctx context.Context, trabajoIDStr *string, input domain.IngestaLoteDTO) (*domain.IngestaResultadoDTO, error)
}

type scraperService struct {
	trabajos domain.TrabajoRepository
	ingesta  domain.IngestaRepository
	entregas domain.EntregaRepository
}

// NewScraperService crea una nueva instancia de ScraperService con sus dependencias
func NewScraperService(trabajos domain.TrabajoRepository, ingesta domain.IngestaRepository, entregas ...domain.EntregaRepository) ScraperService {
	s := &scraperService{trabajos: trabajos, ingesta: ingesta}
	if len(entregas) > 0 {
		s.entregas = entregas[0]
	}
	return s
}

// IniciarTrabajo registra el arranque de una araña de scraping y retorna su DTO
func (s *scraperService) IniciarTrabajo(ctx context.Context, input domain.IniciarTrabajoDTO) (*domain.TrabajoScraperDTO, error) {
	cadena, err := s.ingesta.ObtenerCadena(ctx, input.CadenaID, input.Supermercado)
	if err != nil {
		return nil, err
	}
	if cadena == nil {
		return nil, ErrCadenaNoEncontrada
	}
	if !cadena.EstaActiva {
		return nil, ErrFuenteDeshabilitada
	}

	var usuarioUUID *uuid.UUID
	if input.DisparadoPorUsuarioID != nil && strings.TrimSpace(*input.DisparadoPorUsuarioID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*input.DisparadoPorUsuarioID))
		if err == nil {
			usuarioUUID = &parsed
		}
	}

	trabajo := &domain.TrabajoScraper{
		ID:                    uuid.New(),
		CadenaID:              cadena.ID,
		CadenaNombre:          cadena.Nombre,
		DisparadoPorUsuarioID: usuarioUUID,
		Estado:                "en_progreso",
		IniciadoEl:            time.Now(),
		ElementosExtraidos:    0,
	}

	if err := s.trabajos.CrearTrabajo(ctx, trabajo); err != nil {
		return nil, err
	}

	return s.mapearTrabajoDTO(trabajo, cadena.Nombre), nil
}

// FinalizarTrabajo actualiza el estado de la ejecución a 'completado' o 'fallido'
func (s *scraperService) FinalizarTrabajo(ctx context.Context, id string, input domain.FinalizarTrabajoDTO) (*domain.TrabajoScraperDTO, error) {
	trabajoUUID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, ErrUUIDInvalido
	}

	estado := strings.ToLower(strings.TrimSpace(input.Estado))
	if estado != "completado" && estado != "fallido" {
		return nil, ErrEstadoTrabajoInvalido
	}

	existente, err := s.trabajos.ObtenerTrabajoPorID(ctx, trabajoUUID)
	if err != nil {
		return nil, err
	}
	if existente == nil {
		return nil, ErrTrabajoNoEncontrado
	}
	if existente.Estado == "completado" || existente.Estado == "fallido" {
		if s.entregas != nil && existente.Estado == estado {
			return s.mapearTrabajoDTO(existente, s.nombreTrabajo(ctx, existente)), nil
		}
		return nil, ErrTrabajoYaFinalizado
	}

	actualizado, err := s.trabajos.FinalizarTrabajo(ctx, trabajoUUID, estado, input.ElementosExtraidos, input.RespuestasBloqueadas, input.RegistroErrores)
	if err != nil {
		return nil, err
	}

	cadenaNombre := ""
	cadenaNombre = s.nombreTrabajo(ctx, actualizado)

	return s.mapearTrabajoDTO(actualizado, cadenaNombre), nil
}

// ObtenerTrabajo recupera la información y métricas de un trabajo de scraping
func (s *scraperService) ObtenerTrabajo(ctx context.Context, id string) (*domain.TrabajoScraperDTO, error) {
	trabajoUUID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, ErrUUIDInvalido
	}

	trabajo, err := s.trabajos.ObtenerTrabajoPorID(ctx, trabajoUUID)
	if err != nil {
		return nil, err
	}
	if trabajo == nil {
		return nil, ErrTrabajoNoEncontrado
	}

	cadenaNombre := ""
	cadenaNombre = s.nombreTrabajo(ctx, trabajo)

	return s.mapearTrabajoDTO(trabajo, cadenaNombre), nil
}

// ObtenerCadena consulta una cadena de supermercado en Catálogo por ID o nombre.
// El contrato HTTP usa 404 para ausente; se traduce al error de dominio para que
// los handlers no confundan una fuente inexistente con una respuesta vacía.
func (s *scraperService) ObtenerCadena(ctx context.Context, cadenaID int, nombre string) (*domain.CadenaSupermercado, error) {
	cadena, err := s.ingesta.ObtenerCadena(ctx, cadenaID, nombre)
	if err != nil {
		return nil, err
	}
	if cadena == nil {
		return nil, ErrCadenaNoEncontrada
	}
	return cadena, nil
}

// IngestarProductos valida la sucursal, el lote y persiste los productos y capturas de precios
func (s *scraperService) IngestarProductos(ctx context.Context, trabajoIDStr *string, input domain.IngestaLoteDTO) (*domain.IngestaResultadoDTO, error) {
	if s.entregas != nil {
		return s.ingestarDurable(ctx, trabajoIDStr, input)
	}
	if len(input.Productos) == 0 {
		return nil, ErrLoteVacio
	}
	if len(input.Productos) > 1000 {
		return nil, ErrLoteExcedeMaximo
	}

	var parsedTrabajoUUID *uuid.UUID
	var cadenaID int

	if trabajoIDStr != nil && strings.TrimSpace(*trabajoIDStr) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*trabajoIDStr))
		if err != nil {
			return nil, ErrUUIDInvalido
		}

		trabajo, err := s.trabajos.ObtenerTrabajoPorID(ctx, parsed)
		if err != nil {
			return nil, err
		}
		if trabajo == nil {
			return nil, ErrTrabajoNoEncontrado
		}
		if trabajo.Estado == "completado" || trabajo.Estado == "fallido" {
			return nil, ErrTrabajoYaFinalizado
		}

		parsedTrabajoUUID = &parsed
		cadenaID = trabajo.CadenaID
	}

	// Si no vino cadenaID por trabajo, resolver por nombre de supermercado si está disponible
	if cadenaID == 0 && input.Supermercado != nil && strings.TrimSpace(*input.Supermercado) != "" {
		cadena, err := s.ingesta.ObtenerCadena(ctx, 0, *input.Supermercado)
		if err != nil {
			return nil, err
		}
		if cadena != nil {
			// La ingesta directa no tiene un trabajo que ya haya sido autorizado
			// al inicio; debe respetar el flag de la fuente (SUP-243).
			if parsedTrabajoUUID == nil && !cadena.EstaActiva {
				return nil, ErrFuenteDeshabilitada
			}
			cadenaID = cadena.ID
		}
	}

	// Obtener la sucursal de destino
	sucursal, err := s.ingesta.ObtenerSucursal(ctx, cadenaID, input.SucursalID, input.CodigoSucursal)
	if err != nil {
		return nil, err
	}
	if sucursal == nil {
		return nil, ErrSucursalNoEncontrada
	}
	if parsedTrabajoUUID == nil && cadenaID == 0 {
		// La ingesta directa también puede identificar la fuente sólo por
		// sucursal; en ese caso se valida la cadena antes de escribir.
		cadena, err := s.ingesta.ObtenerCadena(ctx, sucursal.CadenaID, "")
		if err != nil {
			return nil, err
		}
		if cadena != nil && !cadena.EstaActiva {
			return nil, ErrFuenteDeshabilitada
		}
	}

	// Ejecutar la persistencia en lote
	resultado, err := s.ingesta.IngestarLote(ctx, parsedTrabajoUUID, sucursal.ID, input.Productos)
	if err != nil {
		return nil, err
	}
	if parsedTrabajoUUID != nil && resultado.Insertados+resultado.Actualizados > 0 {
		if err := s.trabajos.IncrementarElementos(ctx, *parsedTrabajoUUID, resultado.Insertados+resultado.Actualizados); err != nil {
			return nil, ErrContadorPendiente
		}
	}
	return resultado, nil
}

func (s *scraperService) mapearTrabajoDTO(t *domain.TrabajoScraper, cadenaNombre string) *domain.TrabajoScraperDTO {
	var duracion *float64
	if t.FinalizadoEl != nil {
		seg := t.FinalizadoEl.Sub(t.IniciadoEl).Seconds()
		duracion = &seg
	}

	return &domain.TrabajoScraperDTO{
		ID:                   t.ID.String(),
		CadenaID:             t.CadenaID,
		CadenaNombre:         cadenaNombre,
		Estado:               t.Estado,
		IniciadoEl:           t.IniciadoEl,
		FinalizadoEl:         t.FinalizadoEl,
		DuracionSegundos:     duracion,
		ElementosExtraidos:   t.ElementosExtraidos,
		RespuestasBloqueadas: t.RespuestasBloqueadas,
		RegistroErrores:      t.RegistroErrores,
	}
}

// El nombre es información auxiliar: consultar/finalizar un trabajo no debe
// quedar bloqueado por una caída de Catálogo. El resto del DTO se conserva.
func (s *scraperService) nombreTrabajo(ctx context.Context, trabajo *domain.TrabajoScraper) string {
	if trabajo.CadenaNombre != "" {
		return trabajo.CadenaNombre
	}
	// Compatibilidad con trabajos históricos cuyo origen ya no estaba presente
	// al migrar. Ninguna lectura SQL cruza hacia tablas de Catálogo.
	return s.nombreCadena(ctx, trabajo.CadenaID)
}

func (s *scraperService) nombreCadena(ctx context.Context, cadenaID int) string {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if cadena, err := s.ingesta.ObtenerCadena(ctx, cadenaID, ""); err == nil && cadena != nil {
		return cadena.Nombre
	}
	return ""
}
