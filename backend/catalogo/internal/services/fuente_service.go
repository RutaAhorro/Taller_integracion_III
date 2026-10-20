package services

import (
	"context"
	"errors"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
)

var (
	ErrFuenteNoEncontrada    = errors.New("fuente no encontrada")
	ErrEstadoFuenteRequerido = errors.New("debe especificar el estado de la fuente (habilitada)")
)

// FuenteService define el contrato de negocio de las fuentes de scraping.
type FuenteService interface {
	ListarFuentes(ctx context.Context) ([]domain.FuenteDTO, error)
	ActualizarFuente(ctx context.Context, id int, input domain.ActualizarFuenteDTO) (*domain.FuenteDTO, error)
}

type fuenteService struct {
	repo domain.FuenteRepository
}

// NewFuenteService instancia el servicio de fuentes inyectando su repositorio.
func NewFuenteService(repo domain.FuenteRepository) FuenteService {
	return &fuenteService{repo: repo}
}

// ListarFuentes entrega todas las cadenas para el panel de administración.
func (s *fuenteService) ListarFuentes(ctx context.Context) ([]domain.FuenteDTO, error) {
	return s.repo.ListarFuentes(ctx)
}

// ActualizarFuente habilita o deshabilita una fuente.
func (s *fuenteService) ActualizarFuente(ctx context.Context, id int, input domain.ActualizarFuenteDTO) (*domain.FuenteDTO, error) {
	if id <= 0 {
		return nil, ErrFuenteNoEncontrada
	}
	if input.Habilitada == nil {
		return nil, ErrEstadoFuenteRequerido
	}
	fuente, err := s.repo.ActualizarEstadoFuente(ctx, id, *input.Habilitada)
	if err != nil {
		return nil, err
	}
	if fuente == nil {
		return nil, ErrFuenteNoEncontrada
	}
	return fuente, nil
}
