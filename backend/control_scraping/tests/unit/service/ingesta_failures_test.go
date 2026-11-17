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
	incrementos   int
	errorContador error
}

func (contador *failingCounter) IncrementarElementos(contexto context.Context, identificador uuid.UUID, cantidad int) error {
	contador.incrementos++
	if contador.errorContador != nil {
		return contador.errorContador
	}
	return contador.mockScraperRepository.IncrementarElementos(contexto, identificador, cantidad)
}

func TestCatalogFailureDoesNotIncrementJob(prueba *testing.T) {
	for _, errorCatalogo := range []error{services.ErrCatalogoNoDisponible, services.ErrCatalogoTimeout} {
		repositorio := newMockScraperRepository()
		identificador := uuid.New()
		repositorio.trabajos[identificador] = &domain.TrabajoScraper{ID: identificador, CadenaID: 1, Estado: "en_progreso"}
		clienteRemoto := newMockScraperRepository()
		clienteRemoto.errorSimulado = errorCatalogo
		trabajos := &failingCounter{mockScraperRepository: repositorio}
		servicio := services.NewScraperService(trabajos, clienteRemoto)
		cadena := identificador.String()
		_, errorObtenido := servicio.IngestarProductos(context.Background(), &cadena, domain.IngestaLoteDTO{Productos: []domain.ProductoScrapeadoDTO{{SKU: "SKU"}}})
		if !errors.Is(errorObtenido, errorCatalogo) || trabajos.incrementos != 0 || repositorio.trabajos[identificador].ElementosExtraidos != 0 {
			prueba.Fatalf("error=%v incrementos=%d", errorObtenido, trabajos.incrementos)
		}
	}
}

func TestCommittedCatalogBatchAndFailedCounterIsExplicit(prueba *testing.T) {
	repositorio := newMockScraperRepository()
	identificador := uuid.New()
	repositorio.trabajos[identificador] = &domain.TrabajoScraper{ID: identificador, CadenaID: 1, Estado: "en_progreso"}
	trabajos := &failingCounter{mockScraperRepository: repositorio, errorContador: errors.New("SQL error")}
	servicio := services.NewScraperService(trabajos, repositorio)
	cadena := identificador.String()
	_, errorObtenido := servicio.IngestarProductos(context.Background(), &cadena, domain.IngestaLoteDTO{Productos: []domain.ProductoScrapeadoDTO{{SKU: "SKU"}}})
	if !errors.Is(errorObtenido, services.ErrContadorPendiente) || trabajos.incrementos != 1 {
		prueba.Fatal(errorObtenido, trabajos.incrementos)
	}
}
