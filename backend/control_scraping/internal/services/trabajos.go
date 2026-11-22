// Package services contiene las reglas de coordinación de scraping: ciclo de
// vida de trabajos, entrega durable de lotes a Catálogo y publicación de
// ejecuciones. Depende sólo de los contratos de domain.
package services

import (
	"context"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/google/uuid"
)

// plazoNombreCadena acota la consulta auxiliar del nombre de una cadena: un
// trabajo se consulta o finaliza aunque Catálogo no responda.
const plazoNombreCadena = time.Second

// ===== Servicio de trabajos =====

// ServicioTrabajos registra, consulta y cierra trabajos de scraping.
type ServicioTrabajos struct {
	trabajos domain.TrabajoRepository
	catalogo domain.CatalogoRepository
}

// NuevoServicioTrabajos compone el servicio con su persistencia y Catálogo.
func NuevoServicioTrabajos(trabajos domain.TrabajoRepository, catalogo domain.CatalogoRepository) *ServicioTrabajos {
	return &ServicioTrabajos{trabajos: trabajos, catalogo: catalogo}
}

// ===== Ciclo de vida =====

// IniciarTrabajo valida en Catálogo que la cadena exista y esté activa, y
// registra el trabajo en progreso con la instantánea del nombre de la cadena.
func (servicio *ServicioTrabajos) IniciarTrabajo(contexto context.Context, entrada domain.IniciarTrabajoDTO) (*domain.TrabajoScraperDTO, error) {
	cadena, errorObtenido := servicio.ObtenerCadena(contexto, entrada.CadenaID, entrada.Supermercado)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if !cadena.EstaActiva {
		return nil, domain.ErrFuenteDeshabilitada
	}
	// El campo es auditoría opcional: el bot envía su propio ID de usuario, que
	// no es un UUID, y se registra como NULL (ver SUP-394).
	var usuarioID *uuid.UUID
	if entrada.DisparadoPorUsuarioID != nil {
		if analizado, errorAnalisis := uuid.Parse(strings.TrimSpace(*entrada.DisparadoPorUsuarioID)); errorAnalisis == nil {
			usuarioID = &analizado
		}
	}
	trabajo := &domain.TrabajoScraper{
		ID:                    uuid.New(),
		CadenaID:              cadena.ID,
		CadenaNombre:          cadena.Nombre,
		DisparadoPorUsuarioID: usuarioID,
		Estado:                domain.EstadoEnProgreso,
		IniciadoEl:            time.Now(),
	}
	if errorObtenido := servicio.trabajos.CrearTrabajo(contexto, trabajo); errorObtenido != nil {
		return nil, errorObtenido
	}
	return mapearTrabajo(trabajo, cadena.Nombre), nil
}

// FinalizarTrabajo cierra el trabajo como completado o fallido. Repetir el
// mismo cierre es idempotente y devuelve el trabajo sin cambiarlo.
func (servicio *ServicioTrabajos) FinalizarTrabajo(contexto context.Context, identificador string, entrada domain.FinalizarTrabajoDTO) (*domain.TrabajoScraperDTO, error) {
	trabajoID, errorObtenido := uuid.Parse(strings.TrimSpace(identificador))
	if errorObtenido != nil {
		return nil, domain.ErrUUIDInvalido
	}
	estado := strings.ToLower(strings.TrimSpace(entrada.Estado))
	if !domain.TrabajoTerminado(estado) {
		return nil, domain.ErrEstadoTrabajoInvalido
	}
	existente, errorObtenido := servicio.trabajos.ObtenerTrabajoPorID(contexto, trabajoID)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if existente == nil {
		return nil, domain.ErrTrabajoNoEncontrado
	}
	if domain.TrabajoTerminado(existente.Estado) {
		if existente.Estado == estado {
			return mapearTrabajo(existente, servicio.nombreTrabajo(contexto, existente)), nil
		}
		return nil, domain.ErrTrabajoYaFinalizado
	}
	actualizado, errorObtenido := servicio.trabajos.FinalizarTrabajo(contexto, trabajoID, estado, entrada.ElementosExtraidos, entrada.RespuestasBloqueadas, entrada.RegistroErrores)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if actualizado == nil {
		return nil, domain.ErrTrabajoNoEncontrado
	}
	return mapearTrabajo(actualizado, servicio.nombreTrabajo(contexto, actualizado)), nil
}

// ===== Consultas =====

// ObtenerTrabajo devuelve el estado y las métricas de un trabajo.
func (servicio *ServicioTrabajos) ObtenerTrabajo(contexto context.Context, identificador string) (*domain.TrabajoScraperDTO, error) {
	trabajoID, errorObtenido := uuid.Parse(strings.TrimSpace(identificador))
	if errorObtenido != nil {
		return nil, domain.ErrUUIDInvalido
	}
	trabajo, errorObtenido := servicio.trabajos.ObtenerTrabajoPorID(contexto, trabajoID)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if trabajo == nil {
		return nil, domain.ErrTrabajoNoEncontrado
	}
	return mapearTrabajo(trabajo, servicio.nombreTrabajo(contexto, trabajo)), nil
}

// ObtenerCadena consulta una cadena en Catálogo por ID o nombre. Catálogo
// responde 404 si no existe; se traduce a ErrCadenaNoEncontrada para no
// confundir una fuente inexistente con una respuesta vacía.
func (servicio *ServicioTrabajos) ObtenerCadena(contexto context.Context, cadenaID int, nombre string) (*domain.CadenaSupermercado, error) {
	cadena, errorObtenido := servicio.catalogo.ObtenerCadena(contexto, cadenaID, nombre)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if cadena == nil {
		return nil, domain.ErrCadenaNoEncontrada
	}
	return cadena, nil
}

// ===== Adaptación de respuestas =====

// mapearTrabajo convierte el modelo persistido al contrato HTTP del trabajo.
func mapearTrabajo(trabajo *domain.TrabajoScraper, cadenaNombre string) *domain.TrabajoScraperDTO {
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

// nombreTrabajo usa la instantánea guardada. Los trabajos anteriores a
// 06_propiedad_permisos.sql cuya cadena ya no existía quedaron con
// cadena_nombre vacío; para ellos se consulta Catálogo con un plazo corto y,
// si falla, se responde sin nombre.
func (servicio *ServicioTrabajos) nombreTrabajo(contexto context.Context, trabajo *domain.TrabajoScraper) string {
	if trabajo.CadenaNombre != "" {
		return trabajo.CadenaNombre
	}
	contexto, cancelar := context.WithTimeout(contexto, plazoNombreCadena)
	defer cancelar()
	if cadena, errorObtenido := servicio.catalogo.ObtenerCadena(contexto, trabajo.CadenaID, ""); errorObtenido == nil && cadena != nil {
		return cadena.Nombre
	}
	return ""
}
