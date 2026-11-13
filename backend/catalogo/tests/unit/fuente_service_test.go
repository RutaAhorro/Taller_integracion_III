package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
)

// mockFuenteRepository simula la persistencia de fuentes de scraping.
type mockFuenteRepository struct {
	fuentes []domain.FuenteDTO
	err     error
}

func (repositorio *mockFuenteRepository) ListarFuentes(ctx context.Context) ([]domain.FuenteDTO, error) {
	if repositorio.err != nil {
		return nil, repositorio.err
	}
	return repositorio.fuentes, nil
}

func (repositorio *mockFuenteRepository) ObtenerFuente(ctx context.Context, id int) (*domain.FuenteDTO, error) {
	if repositorio.err != nil {
		return nil, repositorio.err
	}
	for indice := range repositorio.fuentes {
		if repositorio.fuentes[indice].ID == id {
			return &repositorio.fuentes[indice], nil
		}
	}
	return nil, nil
}

func (repositorio *mockFuenteRepository) ActualizarEstadoFuente(ctx context.Context, id int, habilitada bool) (*domain.FuenteDTO, error) {
	if repositorio.err != nil {
		return nil, repositorio.err
	}
	for indice := range repositorio.fuentes {
		if repositorio.fuentes[indice].ID == id {
			repositorio.fuentes[indice].Habilitada = habilitada
			return &repositorio.fuentes[indice], nil
		}
	}
	return nil, nil
}

func TestFuenteService_Listar(prueba *testing.T) {
	fecha := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	mockRepo := &mockFuenteRepository{
		fuentes: []domain.FuenteDTO{
			{ID: 1, Nombre: "Jumbo", Habilitada: true, TotalSucursales: 8, TotalProductos: 120, CreadoEl: fecha},
			{ID: 2, Nombre: "Lider", Habilitada: false, TotalSucursales: 12, TotalProductos: 340, CreadoEl: fecha},
		},
	}
	service := services.NewFuenteService(mockRepo)

	fuentes, err := service.ListarFuentes(context.Background())
	if err != nil {
		prueba.Fatalf("se esperaba nil error, se obtuvo %v", err)
	}
	if len(fuentes) != 2 {
		prueba.Fatalf("se esperaban 2 fuentes, se obtuvieron %d", len(fuentes))
	}
	if fuentes[0].Nombre != "Jumbo" || !fuentes[0].Habilitada {
		prueba.Errorf("la primera fuente debía ser Jumbo habilitada: %+v", fuentes[0])
	}
}

func TestFuenteService_ActualizarFuente(prueba *testing.T) {
	fecha := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	nuevaRepo := func() *mockFuenteRepository {
		return &mockFuenteRepository{
			fuentes: []domain.FuenteDTO{
				{ID: 1, Nombre: "Jumbo", Habilitada: true, CreadoEl: fecha},
				{ID: 2, Nombre: "Lider", Habilitada: false, CreadoEl: fecha},
			},
		}
	}

	prueba.Run("Deshabilitar fuente", func(subprueba *testing.T) {
		service := services.NewFuenteService(nuevaRepo())
		habilitada := false
		fuente, err := service.ActualizarFuente(context.Background(), 1, domain.ActualizarFuenteDTO{Habilitada: &habilitada})
		if err != nil {
			subprueba.Fatalf("se esperaba nil error, se obtuvo %v", err)
		}
		if fuente == nil || fuente.Habilitada {
			subprueba.Errorf("la fuente debía quedar deshabilitada: %+v", fuente)
		}
	})

	prueba.Run("Habilitar fuente", func(subprueba *testing.T) {
		service := services.NewFuenteService(nuevaRepo())
		habilitada := true
		fuente, err := service.ActualizarFuente(context.Background(), 2, domain.ActualizarFuenteDTO{Habilitada: &habilitada})
		if err != nil {
			subprueba.Fatalf("se esperaba nil error, se obtuvo %v", err)
		}
		if fuente == nil || !fuente.Habilitada {
			subprueba.Errorf("la fuente debía quedar habilitada: %+v", fuente)
		}
	})

	prueba.Run("Fuente inexistente", func(subprueba *testing.T) {
		service := services.NewFuenteService(nuevaRepo())
		habilitada := true
		_, err := service.ActualizarFuente(context.Background(), 99, domain.ActualizarFuenteDTO{Habilitada: &habilitada})
		if !errors.Is(err, services.ErrFuenteNoEncontrada) {
			subprueba.Errorf("se esperaba ErrFuenteNoEncontrada, se obtuvo %v", err)
		}
	})

	prueba.Run("ID inválido", func(subprueba *testing.T) {
		service := services.NewFuenteService(nuevaRepo())
		habilitada := true
		_, err := service.ActualizarFuente(context.Background(), 0, domain.ActualizarFuenteDTO{Habilitada: &habilitada})
		if !errors.Is(err, services.ErrFuenteNoEncontrada) {
			subprueba.Errorf("se esperaba ErrFuenteNoEncontrada para id inválido, se obtuvo %v", err)
		}
	})

	prueba.Run("Payload sin estado", func(subprueba *testing.T) {
		service := services.NewFuenteService(nuevaRepo())
		_, err := service.ActualizarFuente(context.Background(), 1, domain.ActualizarFuenteDTO{})
		if !errors.Is(err, services.ErrEstadoFuenteRequerido) {
			subprueba.Errorf("se esperaba ErrEstadoFuenteRequerido, se obtuvo %v", err)
		}
	})

	prueba.Run("Error del repositorio", func(subprueba *testing.T) {
		repoErr := errors.New("db caída")
		service := services.NewFuenteService(&mockFuenteRepository{err: repoErr})
		if _, err := service.ListarFuentes(context.Background()); !errors.Is(err, repoErr) {
			subprueba.Errorf("se esperaba propagar el error del repositorio, se obtuvo %v", err)
		}
	})
}
