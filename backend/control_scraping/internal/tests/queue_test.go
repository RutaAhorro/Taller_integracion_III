package tests

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type sqlQueue struct{ saved []domain.Ejecucion }

func (q *sqlQueue) RegistrarEjecucion(_ context.Context, e domain.Ejecucion) error {
	q.saved = append(q.saved, e)
	return nil
}
func (q *sqlQueue) Pendientes(context.Context) ([]domain.Ejecucion, error) { return nil, nil }
func (q *sqlQueue) PosponerPublicacion(context.Context, uuid.UUID) error   { return nil }

func TestExecutionWithoutRedisClientStillPersistsSQL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMockScraperRepository()
	id := uuid.New()
	repo.trabajos[id] = &domain.TrabajoScraper{ID: id, CadenaID: 1, Estado: "en_progreso"}
	h := handlers.NewScraperHandler(services.NewScraperService(repo, repo))
	queue := &sqlQueue{}
	h.SetQueue(services.NewQueueService(queue, nil))
	r := gin.New()
	r.POST("/trabajos/:id/ejecutar", h.EjecutarTrabajo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/trabajos/"+id.String()+"/ejecutar", strings.NewReader(`{"spider":"jumbo_rsc"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 202 || len(queue.saved) != 1 || queue.saved[0].TrabajoID != id || w.Header().Get("X-Scraper-Queue") != "durable-stream" {
		t.Fatal(w.Code, w.Body.String(), queue.saved)
	}
}
