package service_test

import (
	"context"
	"encoding/json"
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

func (cola *sqlQueue) RegistrarEjecucion(_ context.Context, ejecucion domain.Ejecucion) error {
	cola.saved = append(cola.saved, ejecucion)
	return nil
}
func (cola *sqlQueue) Pendientes(context.Context) ([]domain.Ejecucion, error) { return nil, nil }
func (cola *sqlQueue) PosponerPublicacion(context.Context, uuid.UUID) error   { return nil }

func TestExecutionWithoutRedisClientStillPersistsSQL(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMockScraperRepository()
	repo.cadenas["jumbo"] = &domain.CadenaSupermercado{ID: 1, Nombre: "Jumbo", EstaActiva: true}
	id := uuid.New()
	repo.trabajos[id] = &domain.TrabajoScraper{ID: id, CadenaID: 1, Estado: "en_progreso"}
	manejador := handlers.NewScraperHandler(services.NewScraperService(repo, repo))
	queue := &sqlQueue{}
	manejador.SetQueue(services.NewQueueService(queue, nil))
	peticion := gin.New()
	peticion.POST("/trabajos/:id/ejecutar", manejador.EjecutarTrabajo)
	respuestaHTTP := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/trabajos/"+id.String()+"/ejecutar", strings.NewReader(`{"spider":"jumbo_rsc"}`))
	req.Header.Set("Content-Type", "application/json")
	peticion.ServeHTTP(respuestaHTTP, req)
	if respuestaHTTP.Code != 202 || len(queue.saved) != 1 || queue.saved[0].TrabajoID != id || respuestaHTTP.Header().Get("X-Scraper-Queue") != "durable-stream" {
		prueba.Fatal(respuestaHTTP.Code, respuestaHTTP.Body.String(), queue.saved)
	}
}

func TestExecutionForwardsAddURLAndEnrichEAN(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMockScraperRepository()
	repo.cadenas["jumbo"] = &domain.CadenaSupermercado{ID: 1, Nombre: "Jumbo", EstaActiva: true}
	id := uuid.New()
	repo.trabajos[id] = &domain.TrabajoScraper{ID: id, CadenaID: 1, Estado: "en_progreso"}
	manejador := handlers.NewScraperHandler(services.NewScraperService(repo, repo))
	queue := &sqlQueue{}
	manejador.SetQueue(services.NewQueueService(queue, nil))
	peticion := gin.New()
	peticion.POST("/trabajos/:id/ejecutar", manejador.EjecutarTrabajo)
	respuestaHTTP := httptest.NewRecorder()
	body := `{"spider":"jumbo_rsc","add_url":"https://www.jumbo.cl/despensa","enrich_ean":true}`
	req := httptest.NewRequest("POST", "/trabajos/"+id.String()+"/ejecutar", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	peticion.ServeHTTP(respuestaHTTP, req)
	if respuestaHTTP.Code != 202 {
		prueba.Fatalf("status = %d, body = %s", respuestaHTTP.Code, respuestaHTTP.Body.String())
	}
	if len(queue.saved) != 1 {
		prueba.Fatalf("ejecuciones guardadas = %d", len(queue.saved))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(queue.saved[0].Payload), &payload); err != nil {
		prueba.Fatalf("payload inválido: %v", err)
	}
	if payload["add_url"] != "https://www.jumbo.cl/despensa" {
		prueba.Errorf("add_url = %v", payload["add_url"])
	}
	if payload["enrich_ean"] != true {
		prueba.Errorf("enrich_ean = %v", payload["enrich_ean"])
	}
}

func TestExecutionRejectsEmptyAddURL(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMockScraperRepository()
	repo.cadenas["jumbo"] = &domain.CadenaSupermercado{ID: 1, Nombre: "Jumbo", EstaActiva: true}
	id := uuid.New()
	repo.trabajos[id] = &domain.TrabajoScraper{ID: id, CadenaID: 1, Estado: "en_progreso"}
	manejador := handlers.NewScraperHandler(services.NewScraperService(repo, repo))
	queue := &sqlQueue{}
	manejador.SetQueue(services.NewQueueService(queue, nil))
	peticion := gin.New()
	peticion.POST("/trabajos/:id/ejecutar", manejador.EjecutarTrabajo)
	respuestaHTTP := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/trabajos/"+id.String()+"/ejecutar", strings.NewReader(`{"spider":"jumbo_rsc","add_url":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	peticion.ServeHTTP(respuestaHTTP, req)
	if respuestaHTTP.Code != 400 {
		prueba.Fatalf("status = %d, body = %s", respuestaHTTP.Code, respuestaHTTP.Body.String())
	}
	if len(queue.saved) != 0 {
		prueba.Fatalf("no se debe encolar: %+v", queue.saved)
	}
}
