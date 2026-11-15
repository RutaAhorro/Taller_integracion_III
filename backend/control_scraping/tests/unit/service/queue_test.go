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

type sqlQueue struct{ ejecuciones []domain.Ejecucion }

func (cola *sqlQueue) RegistrarEjecucion(_ context.Context, ejecucion domain.Ejecucion) error {
	cola.ejecuciones = append(cola.ejecuciones, ejecucion)
	return nil
}
func (cola *sqlQueue) Pendientes(context.Context) ([]domain.Ejecucion, error) { return nil, nil }
func (cola *sqlQueue) PosponerPublicacion(context.Context, uuid.UUID) error   { return nil }

func TestExecutionWithoutRedisClientStillPersistsSQL(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	repositorio := newMockScraperRepository()
	repositorio.cadenas["jumbo"] = &domain.CadenaSupermercado{ID: 1, Nombre: "Jumbo", EstaActiva: true}
	identificador := uuid.New()
	repositorio.trabajos[identificador] = &domain.TrabajoScraper{ID: identificador, CadenaID: 1, Estado: "en_progreso"}
	manejador := handlers.NewScraperHandler(services.NewScraperService(repositorio, repositorio))
	cola := &sqlQueue{}
	manejador.SetQueue(services.NewQueueService(cola, nil))
	enrutador := gin.New()
	enrutador.POST("/trabajos/:id/ejecutar", manejador.EjecutarTrabajo)
	respuestaHTTP := httptest.NewRecorder()
	peticionHTTP := httptest.NewRequest("POST", "/trabajos/"+identificador.String()+"/ejecutar", strings.NewReader(`{"spider":"jumbo_rsc"}`))
	peticionHTTP.Header.Set("Content-Type", "application/json")
	enrutador.ServeHTTP(respuestaHTTP, peticionHTTP)
	if respuestaHTTP.Code != 202 || len(cola.ejecuciones) != 1 || cola.ejecuciones[0].TrabajoID != identificador || respuestaHTTP.Header().Get("X-Scraper-Queue") != "durable-stream" {
		prueba.Fatal(respuestaHTTP.Code, respuestaHTTP.Body.String(), cola.ejecuciones)
	}
}

func TestExecutionForwardsAddURLAndEnrichEAN(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	repositorio := newMockScraperRepository()
	repositorio.cadenas["jumbo"] = &domain.CadenaSupermercado{ID: 1, Nombre: "Jumbo", EstaActiva: true}
	identificador := uuid.New()
	repositorio.trabajos[identificador] = &domain.TrabajoScraper{ID: identificador, CadenaID: 1, Estado: "en_progreso"}
	manejador := handlers.NewScraperHandler(services.NewScraperService(repositorio, repositorio))
	cola := &sqlQueue{}
	manejador.SetQueue(services.NewQueueService(cola, nil))
	enrutador := gin.New()
	enrutador.POST("/trabajos/:id/ejecutar", manejador.EjecutarTrabajo)
	respuestaHTTP := httptest.NewRecorder()
	cuerpo := `{"spider":"jumbo_rsc","add_url":"https://www.jumbo.cl/despensa","enrich_ean":true}`
	peticionHTTP := httptest.NewRequest("POST", "/trabajos/"+identificador.String()+"/ejecutar", strings.NewReader(cuerpo))
	peticionHTTP.Header.Set("Content-Type", "application/json")
	enrutador.ServeHTTP(respuestaHTTP, peticionHTTP)
	if respuestaHTTP.Code != 202 {
		prueba.Fatalf("status = %d, body = %s", respuestaHTTP.Code, respuestaHTTP.Body.String())
	}
	if len(cola.ejecuciones) != 1 {
		prueba.Fatalf("ejecuciones guardadas = %d", len(cola.ejecuciones))
	}
	var carga map[string]any
	if errorObtenido := json.Unmarshal([]byte(cola.ejecuciones[0].Payload), &carga); errorObtenido != nil {
		prueba.Fatalf("payload inválido: %v", errorObtenido)
	}
	if carga["add_url"] != "https://www.jumbo.cl/despensa" {
		prueba.Errorf("add_url = %v", carga["add_url"])
	}
	if carga["enrich_ean"] != true {
		prueba.Errorf("enrich_ean = %v", carga["enrich_ean"])
	}
}

func TestExecutionRejectsEmptyAddURL(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	repositorio := newMockScraperRepository()
	repositorio.cadenas["jumbo"] = &domain.CadenaSupermercado{ID: 1, Nombre: "Jumbo", EstaActiva: true}
	identificador := uuid.New()
	repositorio.trabajos[identificador] = &domain.TrabajoScraper{ID: identificador, CadenaID: 1, Estado: "en_progreso"}
	manejador := handlers.NewScraperHandler(services.NewScraperService(repositorio, repositorio))
	cola := &sqlQueue{}
	manejador.SetQueue(services.NewQueueService(cola, nil))
	enrutador := gin.New()
	enrutador.POST("/trabajos/:id/ejecutar", manejador.EjecutarTrabajo)
	respuestaHTTP := httptest.NewRecorder()
	peticionHTTP := httptest.NewRequest("POST", "/trabajos/"+identificador.String()+"/ejecutar", strings.NewReader(`{"spider":"jumbo_rsc","add_url":"   "}`))
	peticionHTTP.Header.Set("Content-Type", "application/json")
	enrutador.ServeHTTP(respuestaHTTP, peticionHTTP)
	if respuestaHTTP.Code != 400 {
		prueba.Fatalf("status = %d, body = %s", respuestaHTTP.Code, respuestaHTTP.Body.String())
	}
	if len(cola.ejecuciones) != 0 {
		prueba.Fatalf("no se debe encolar: %+v", cola.ejecuciones)
	}
}
