package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/google/uuid"
)

type failingCounter struct {
	*mockScraperRepository
	increments int
	errCounter error
}

func (contador *failingCounter) IncrementarElementos(ctx context.Context, id uuid.UUID, cantidad int) error {
	contador.increments++
	if contador.errCounter != nil {
		return contador.errCounter
	}
	return contador.mockScraperRepository.IncrementarElementos(ctx, id, cantidad)
}

func TestCatalogFailureDoesNotIncrementJob(prueba *testing.T) {
	for _, catalogErr := range []error{services.ErrCatalogoNoDisponible, services.ErrCatalogoTimeout} {
		repo := newMockScraperRepository()
		id := uuid.New()
		repo.trabajos[id] = &domain.TrabajoScraper{ID: id, CadenaID: 1, Estado: "en_progreso"}
		remote := newMockScraperRepository()
		remote.err = catalogErr
		jobs := &failingCounter{mockScraperRepository: repo}
		service := services.NewScraperService(jobs, remote)
		str := id.String()
		_, err := service.IngestarProductos(context.Background(), &str, domain.IngestaLoteDTO{Productos: []domain.ProductoScrapeadoDTO{{SKU: "SKU"}}})
		if !errors.Is(err, catalogErr) || jobs.increments != 0 || repo.trabajos[id].ElementosExtraidos != 0 {
			prueba.Fatalf("error=%v increments=%d", err, jobs.increments)
		}
	}
}

func TestCommittedCatalogBatchAndFailedCounterIsExplicit(prueba *testing.T) {
	repo := newMockScraperRepository()
	id := uuid.New()
	repo.trabajos[id] = &domain.TrabajoScraper{ID: id, CadenaID: 1, Estado: "en_progreso"}
	jobs := &failingCounter{mockScraperRepository: repo, errCounter: errors.New("SQL error")}
	service := services.NewScraperService(jobs, repo)
	str := id.String()
	_, err := service.IngestarProductos(context.Background(), &str, domain.IngestaLoteDTO{Productos: []domain.ProductoScrapeadoDTO{{SKU: "SKU"}}})
	if !errors.Is(err, services.ErrContadorPendiente) || jobs.increments != 1 {
		prueba.Fatal(err, jobs.increments)
	}
}
