package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
)

// ===== Errores y contratos del servicio =====

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
	IniciarTrabajo(contexto context.Context, entrada domain.IniciarTrabajoDTO) (*domain.TrabajoScraperDTO, error)
	FinalizarTrabajo(contexto context.Context, identificador string, entrada domain.FinalizarTrabajoDTO) (*domain.TrabajoScraperDTO, error)
	ObtenerTrabajo(contexto context.Context, identificador string) (*domain.TrabajoScraperDTO, error)
	ObtenerCadena(contexto context.Context, cadenaID int, nombre string) (*domain.CadenaSupermercado, error)
	IngestarProductos(contexto context.Context, trabajoIDStr *string, entrada domain.IngestaLoteDTO) (*domain.IngestaResultadoDTO, error)
}

// scraperService coordina la lógica de trabajos y la entrega de lotes a Catálogo.
type scraperService struct {
	trabajos domain.TrabajoRepository
	ingesta  domain.IngestaRepository
	entregas domain.EntregaRepository
}

// NewScraperService crea una nueva instancia de ScraperService con sus dependencias
func NewScraperService(trabajos domain.TrabajoRepository, ingesta domain.IngestaRepository, entregas ...domain.EntregaRepository) ScraperService {
	servicio := &scraperService{trabajos: trabajos, ingesta: ingesta}
	if len(entregas) > 0 {
		servicio.entregas = entregas[0]
	}
	return servicio
}

// ===== Ciclo de vida y consulta de trabajos =====

// IniciarTrabajo registra el arranque de una araña de scraping y retorna su DTO
func (servicio *scraperService) IniciarTrabajo(contexto context.Context, entrada domain.IniciarTrabajoDTO) (*domain.TrabajoScraperDTO, error) {
	cadena, errorObtenido := servicio.ingesta.ObtenerCadena(contexto, entrada.CadenaID, entrada.Supermercado)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if cadena == nil {
		return nil, ErrCadenaNoEncontrada
	}
	if !cadena.EstaActiva {
		return nil, ErrFuenteDeshabilitada
	}

	var usuarioUUID *uuid.UUID
	if entrada.DisparadoPorUsuarioID != nil && strings.TrimSpace(*entrada.DisparadoPorUsuarioID) != "" {
		analizado, errorObtenido := uuid.Parse(strings.TrimSpace(*entrada.DisparadoPorUsuarioID))
		if errorObtenido == nil {
			usuarioUUID = &analizado
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

	if errorObtenido := servicio.trabajos.CrearTrabajo(contexto, trabajo); errorObtenido != nil {
		return nil, errorObtenido
	}

	return servicio.mapearTrabajoDTO(trabajo, cadena.Nombre), nil
}

// FinalizarTrabajo actualiza el estado de la ejecución a 'completado' o 'fallido'
func (servicio *scraperService) FinalizarTrabajo(contexto context.Context, identificador string, entrada domain.FinalizarTrabajoDTO) (*domain.TrabajoScraperDTO, error) {
	trabajoUUID, errorObtenido := uuid.Parse(strings.TrimSpace(identificador))
	if errorObtenido != nil {
		return nil, ErrUUIDInvalido
	}

	estado := strings.ToLower(strings.TrimSpace(entrada.Estado))
	if estado != "completado" && estado != "fallido" {
		return nil, ErrEstadoTrabajoInvalido
	}

	existente, errorObtenido := servicio.trabajos.ObtenerTrabajoPorID(contexto, trabajoUUID)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if existente == nil {
		return nil, ErrTrabajoNoEncontrado
	}
	if existente.Estado == "completado" || existente.Estado == "fallido" {
		if servicio.entregas != nil && existente.Estado == estado {
			return servicio.mapearTrabajoDTO(existente, servicio.nombreTrabajo(contexto, existente)), nil
		}
		return nil, ErrTrabajoYaFinalizado
	}

	actualizado, errorObtenido := servicio.trabajos.FinalizarTrabajo(contexto, trabajoUUID, estado, entrada.ElementosExtraidos, entrada.RespuestasBloqueadas, entrada.RegistroErrores)
	if errorObtenido != nil {
		return nil, errorObtenido
	}

	cadenaNombre := ""
	cadenaNombre = servicio.nombreTrabajo(contexto, actualizado)

	return servicio.mapearTrabajoDTO(actualizado, cadenaNombre), nil
}

// ObtenerTrabajo recupera la información y métricas de un trabajo de scraping
func (servicio *scraperService) ObtenerTrabajo(contexto context.Context, identificador string) (*domain.TrabajoScraperDTO, error) {
	trabajoUUID, errorObtenido := uuid.Parse(strings.TrimSpace(identificador))
	if errorObtenido != nil {
		return nil, ErrUUIDInvalido
	}

	trabajo, errorObtenido := servicio.trabajos.ObtenerTrabajoPorID(contexto, trabajoUUID)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if trabajo == nil {
		return nil, ErrTrabajoNoEncontrado
	}

	cadenaNombre := ""
	cadenaNombre = servicio.nombreTrabajo(contexto, trabajo)

	return servicio.mapearTrabajoDTO(trabajo, cadenaNombre), nil
}

// ObtenerCadena consulta una cadena de supermercado en Catálogo por ID o nombre.
// El contrato HTTP usa 404 para ausente; se traduce al error de dominio para que
// los handlers no confundan una fuente inexistente con una respuesta vacía.
func (servicio *scraperService) ObtenerCadena(contexto context.Context, cadenaID int, nombre string) (*domain.CadenaSupermercado, error) {
	cadena, errorObtenido := servicio.ingesta.ObtenerCadena(contexto, cadenaID, nombre)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if cadena == nil {
		return nil, ErrCadenaNoEncontrada
	}
	return cadena, nil
}

// ===== Validación e ingesta de productos =====

// IngestarProductos valida la sucursal, el lote y persiste los productos y capturas de precios
func (servicio *scraperService) IngestarProductos(contexto context.Context, trabajoIDStr *string, entrada domain.IngestaLoteDTO) (*domain.IngestaResultadoDTO, error) {
	if servicio.entregas != nil {
		return servicio.ingestarDurable(contexto, trabajoIDStr, entrada)
	}
	if len(entrada.Productos) == 0 {
		return nil, ErrLoteVacio
	}
	if len(entrada.Productos) > 1000 {
		return nil, ErrLoteExcedeMaximo
	}

	var trabajoUUIDAnalizado *uuid.UUID
	var cadenaID int

	if trabajoIDStr != nil && strings.TrimSpace(*trabajoIDStr) != "" {
		analizado, errorObtenido := uuid.Parse(strings.TrimSpace(*trabajoIDStr))
		if errorObtenido != nil {
			return nil, ErrUUIDInvalido
		}

		trabajo, errorObtenido := servicio.trabajos.ObtenerTrabajoPorID(contexto, analizado)
		if errorObtenido != nil {
			return nil, errorObtenido
		}
		if trabajo == nil {
			return nil, ErrTrabajoNoEncontrado
		}
		if trabajo.Estado == "completado" || trabajo.Estado == "fallido" {
			return nil, ErrTrabajoYaFinalizado
		}

		trabajoUUIDAnalizado = &analizado
		cadenaID = trabajo.CadenaID
	}

	// Si no vino cadenaID por trabajo, resolver por nombre de supermercado si está disponible
	if cadenaID == 0 && entrada.Supermercado != nil && strings.TrimSpace(*entrada.Supermercado) != "" {
		cadena, errorObtenido := servicio.ingesta.ObtenerCadena(contexto, 0, *entrada.Supermercado)
		if errorObtenido != nil {
			return nil, errorObtenido
		}
		if cadena != nil {
			// La ingesta directa no tiene un trabajo que ya haya sido autorizado
			// al inicio; debe respetar el flag de la fuente (SUP-243).
			if trabajoUUIDAnalizado == nil && !cadena.EstaActiva {
				return nil, ErrFuenteDeshabilitada
			}
			cadenaID = cadena.ID
		}
	}

	// Obtener la sucursal de destino
	sucursal, errorObtenido := servicio.ingesta.ObtenerSucursal(contexto, cadenaID, entrada.SucursalID, entrada.CodigoSucursal)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if sucursal == nil {
		return nil, ErrSucursalNoEncontrada
	}
	if trabajoUUIDAnalizado == nil && cadenaID == 0 {
		// La ingesta directa también puede identificar la fuente sólo por
		// sucursal; en ese caso se valida la cadena antes de escribir.
		cadena, errorObtenido := servicio.ingesta.ObtenerCadena(contexto, sucursal.CadenaID, "")
		if errorObtenido != nil {
			return nil, errorObtenido
		}
		if cadena != nil && !cadena.EstaActiva {
			return nil, ErrFuenteDeshabilitada
		}
	}

	// Ejecutar la persistencia en lote
	resultado, errorObtenido := servicio.ingesta.IngestarLote(contexto, trabajoUUIDAnalizado, sucursal.ID, entrada.Productos)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if trabajoUUIDAnalizado != nil && resultado.Insertados+resultado.Actualizados > 0 {
		if errorObtenido := servicio.trabajos.IncrementarElementos(contexto, *trabajoUUIDAnalizado, resultado.Insertados+resultado.Actualizados); errorObtenido != nil {
			return nil, ErrContadorPendiente
		}
	}
	return resultado, nil
}

// ===== Adaptación de respuestas y nombres de catálogo =====

// mapearTrabajoDTO convierte el modelo persistido al contrato HTTP del trabajo.
func (servicio *scraperService) mapearTrabajoDTO(trabajo *domain.TrabajoScraper, cadenaNombre string) *domain.TrabajoScraperDTO {
	var duracion *float64
	if trabajo.FinalizadoEl != nil {
		segundos := trabajo.FinalizadoEl.Sub(trabajo.IniciadoEl).Seconds()
		duracion = &segundos
	}

	return &domain.TrabajoScraperDTO{
		ID:                   trabajo.ID.String(),
		CadenaID:             trabajo.CadenaID,
		CadenaNombre:         cadenaNombre,
		Estado:               trabajo.Estado,
		IniciadoEl:           trabajo.IniciadoEl,
		FinalizadoEl:         trabajo.FinalizadoEl,
		DuracionSegundos:     duracion,
		ElementosExtraidos:   trabajo.ElementosExtraidos,
		RespuestasBloqueadas: trabajo.RespuestasBloqueadas,
		RegistroErrores:      trabajo.RegistroErrores,
	}
}

// El nombre es información auxiliar: consultar/finalizar un trabajo no debe
// quedar bloqueado por una caída de Catálogo. El resto del DTO se conserva.
func (servicio *scraperService) nombreTrabajo(contexto context.Context, trabajo *domain.TrabajoScraper) string {
	if trabajo.CadenaNombre != "" {
		return trabajo.CadenaNombre
	}
	// Compatibilidad con trabajos históricos cuyo origen ya no estaba presente
	// al migrar. Ninguna lectura SQL cruza hacia tablas de Catálogo.
	return servicio.nombreCadena(contexto, trabajo.CadenaID)
}

// nombreCadena consulta a Catálogo con un límite para resolver trabajos históricos.
func (servicio *scraperService) nombreCadena(contexto context.Context, cadenaID int) string {
	contexto, cancelar := context.WithTimeout(contexto, time.Second)
	defer cancelar()
	if cadena, errorObtenido := servicio.ingesta.ObtenerCadena(contexto, cadenaID, ""); errorObtenido == nil && cadena != nil {
		return cadena.Nombre
	}
	return ""
}
