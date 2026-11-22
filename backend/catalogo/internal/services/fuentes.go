// Fuentes de scraping: listado para el panel y cambio del flag de activación.

package services

import (
	"context"
	"errors"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
)

// ===== Errores =====

// Errores del servicio de fuentes que el handler traduce a 404 y 400.
var (
	ErrFuenteNoEncontrada    = errors.New("fuente no encontrada")
	ErrEstadoFuenteRequerido = errors.New("debe especificar el estado de la fuente (habilitada)")
)

// ===== Servicio =====

// ServicioFuentes administra las cadenas como fuentes de scraping.
type ServicioFuentes struct {
	repositorio domain.RepositorioFuentes
}

// NuevoServicioFuentes inyecta el repositorio de fuentes.
func NuevoServicioFuentes(repositorio domain.RepositorioFuentes) *ServicioFuentes {
	return &ServicioFuentes{repositorio: repositorio}
}

// ListarFuentes entrega todas las cadenas para el panel de administración.
func (servicio *ServicioFuentes) ListarFuentes(contexto context.Context) ([]domain.FuenteDTO, error) {
	return servicio.repositorio.ListarFuentes(contexto)
}

// ActualizarFuente habilita o deshabilita una fuente existente.
func (servicio *ServicioFuentes) ActualizarFuente(contexto context.Context, fuenteID int, cambio domain.ActualizarFuenteDTO) (*domain.FuenteDTO, error) {
	if cambio.Habilitada == nil {
		return nil, ErrEstadoFuenteRequerido
	}
	fuente, errorActualizacion := servicio.repositorio.ActualizarEstadoFuente(contexto, fuenteID, *cambio.Habilitada)
	if errorActualizacion != nil {
		return nil, errorActualizacion
	}
	if fuente == nil {
		return nil, ErrFuenteNoEncontrada
	}
	return fuente, nil
}
