package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
)

// mockScraperRepository implementa la interfaz ScraperRepository para pruebas unitarias
type mockScraperRepository struct {
	trabajos   map[uuid.UUID]*domain.TrabajoScraper
	cadenas    map[string]*domain.CadenaSupermercado
	sucursales map[int]*domain.SucursalSupermercado
	err        error
}

func newMockScraperRepository() *mockScraperRepository {
	return &mockScraperRepository{
		trabajos:   make(map[uuid.UUID]*domain.TrabajoScraper),
		cadenas:    make(map[string]*domain.CadenaSupermercado),
		sucursales: make(map[int]*domain.SucursalSupermercado),
	}
}

func (m *mockScraperRepository) CrearTrabajo(ctx context.Context, trabajo *domain.TrabajoScraper) error {
	if m.err != nil {
		return m.err
	}
	m.trabajos[trabajo.ID] = trabajo
	return nil
}

func (m *mockScraperRepository) ObtenerTrabajoPorID(ctx context.Context, id uuid.UUID) (*domain.TrabajoScraper, error) {
	if m.err != nil {
		return nil, m.err
	}
	t, ok := m.trabajos[id]
	if !ok {
		return nil, nil
	}
	return t, nil
}

func (m *mockScraperRepository) FinalizarTrabajo(ctx context.Context, id uuid.UUID, estado string, elementosExtraidos *int, registroErrores *string) (*domain.TrabajoScraper, error) {
	if m.err != nil {
		return nil, m.err
	}
	t, ok := m.trabajos[id]
	if !ok {
		return nil, nil
	}
	ahora := time.Now()
	t.Estado = estado
	t.FinalizadoEl = &ahora
	if elementosExtraidos != nil {
		t.ElementosExtraidos = *elementosExtraidos
	}
	t.RegistroErrores = registroErrores
	return t, nil
}

func (m *mockScraperRepository) ObtenerCadena(ctx context.Context, cadenaID int, nombreCadena string) (*domain.CadenaSupermercado, error) {
	if m.err != nil {
		return nil, m.err
	}
	if cadenaID > 0 {
		for _, c := range m.cadenas {
			if c.ID == cadenaID {
				return c, nil
			}
		}
	}
	if nombreCadena != "" {
		if c, ok := m.cadenas[strings.ToLower(nombreCadena)]; ok {
			return c, nil
		}
		// Simula la creación automática en mock
		nueva := &domain.CadenaSupermercado{
			ID:         len(m.cadenas) + 1,
			Nombre:     nombreCadena,
			EstaActiva: true,
			CreadoEl:   time.Now(),
		}
		m.cadenas[strings.ToLower(nombreCadena)] = nueva
		return nueva, nil
	}
	return nil, nil
}

func (m *mockScraperRepository) ObtenerSucursal(ctx context.Context, cadenaID int, sucursalID *int, codigoSucursal *string) (*domain.SucursalSupermercado, error) {
	if m.err != nil {
		return nil, m.err
	}
	if sucursalID != nil && *sucursalID > 0 {
		if s, ok := m.sucursales[*sucursalID]; ok {
			return s, nil
		}
	}
	// Retornar primera sucursal mock
	cod := "CENTRAL-001"
	s := &domain.SucursalSupermercado{
		ID:             1,
		CadenaID:       cadenaID,
		CodigoSucursal: &cod,
		Nombre:         "Sucursal Mock",
		EstaActiva:     true,
	}
	m.sucursales[1] = s
	return s, nil
}

func (m *mockScraperRepository) IngestarLote(ctx context.Context, trabajoID *uuid.UUID, sucursalID int, productos []domain.ProductoScrapeadoDTO) (*domain.IngestaResultadoDTO, error) {
	if m.err != nil {
		return nil, m.err
	}

	var trabajoIDStr *string
	if trabajoID != nil {
		str := trabajoID.String()
		trabajoIDStr = &str
	}

	return &domain.IngestaResultadoDTO{
		TrabajoID:          trabajoIDStr,
		SucursalID:         sucursalID,
		TotalRecibidos:     len(productos),
		Insertados:         len(productos),
		Actualizados:       0,
		PreciosRegistrados: len(productos),
		Mensaje:            "Lote procesado exitosamente",
	}, nil
}

func (m *mockScraperRepository) IncrementarElementos(ctx context.Context, id uuid.UUID, cantidad int) error {
	if m.err != nil {
		return m.err
	}
	if t, ok := m.trabajos[id]; ok {
		t.ElementosExtraidos += cantidad
	}
	return nil
}

// -------------------------------------------------------------
// Pruebas unitarias de ScraperService
// -------------------------------------------------------------

func TestScraperService_IniciarTrabajo_Exitoso(t *testing.T) {
	mockRepo := newMockScraperRepository()
	mockRepo.cadenas["jumbo"] = &domain.CadenaSupermercado{
		ID:         1,
		Nombre:     "Jumbo",
		EstaActiva: true,
	}

	service := services.NewScraperService(mockRepo, mockRepo)

	input := domain.IniciarTrabajoDTO{
		CadenaID:     1,
		Supermercado: "Jumbo",
	}

	trabajo, err := service.IniciarTrabajo(context.Background(), input)
	if err != nil {
		t.Fatalf("se esperaba nil error al iniciar trabajo, se obtuvo: %v", err)
	}

	if trabajo.Estado != "en_progreso" {
		t.Errorf("se esperaba estado 'en_progreso', se obtuvo: %s", trabajo.Estado)
	}
	if trabajo.CadenaID != 1 {
		t.Errorf("se esperaba cadenaID=1, se obtuvo: %d", trabajo.CadenaID)
	}
}

func TestScraperService_FinalizarTrabajo_Validaciones(t *testing.T) {
	mockRepo := newMockScraperRepository()
	service := services.NewScraperService(mockRepo, mockRepo)

	trabajoID := uuid.New()
	mockRepo.trabajos[trabajoID] = &domain.TrabajoScraper{
		ID:         trabajoID,
		CadenaID:   1,
		Estado:     "en_progreso",
		IniciadoEl: time.Now().Add(-10 * time.Minute),
	}

	// 1. Estado inválido
	_, err := service.FinalizarTrabajo(context.Background(), trabajoID.String(), domain.FinalizarTrabajoDTO{
		Estado: "pausado",
	})
	if !errors.Is(err, services.ErrEstadoTrabajoInvalido) {
		t.Errorf("se esperaba ErrEstadoTrabajoInvalido, se obtuvo: %v", err)
	}

	// 2. Finalización exitosa
	items := 150
	trabajoFin, err := service.FinalizarTrabajo(context.Background(), trabajoID.String(), domain.FinalizarTrabajoDTO{
		Estado:             "completado",
		ElementosExtraidos: &items,
	})
	if err != nil {
		t.Fatalf("se esperaba nil error, se obtuvo: %v", err)
	}
	if trabajoFin.Estado != "completado" {
		t.Errorf("se esperaba 'completado', se obtuvo: %s", trabajoFin.Estado)
	}
	if trabajoFin.ElementosExtraidos != 150 {
		t.Errorf("se esperaba 150 items, se obtuvo: %d", trabajoFin.ElementosExtraidos)
	}
	if trabajoFin.DuracionSegundos == nil || *trabajoFin.DuracionSegundos <= 0 {
		t.Errorf("se esperaba duracion calculada positiva, se obtuvo: %v", trabajoFin.DuracionSegundos)
	}

	// 3. Intento de re-finalizar un trabajo ya completado
	_, err = service.FinalizarTrabajo(context.Background(), trabajoID.String(), domain.FinalizarTrabajoDTO{
		Estado: "completado",
	})
	if !errors.Is(err, services.ErrTrabajoYaFinalizado) {
		t.Errorf("se esperaba ErrTrabajoYaFinalizado, se obtuvo: %v", err)
	}
}

func TestScraperService_IngestarProductos_LimitesYValidaciones(t *testing.T) {
	mockRepo := newMockScraperRepository()
	service := services.NewScraperService(mockRepo, mockRepo)

	// 1. Lote vacío
	_, err := service.IngestarProductos(context.Background(), nil, domain.IngestaLoteDTO{
		Productos: []domain.ProductoScrapeadoDTO{},
	})
	if !errors.Is(err, services.ErrLoteVacio) {
		t.Errorf("se esperaba ErrLoteVacio, se obtuvo: %v", err)
	}

	// 2. Lote exitoso
	ean := "7802900001234"
	resultado, err := service.IngestarProductos(context.Background(), nil, domain.IngestaLoteDTO{
		Supermercado: stringPtr("Jumbo"),
		Productos: []domain.ProductoScrapeadoDTO{
			{
				SKU:          "SKU-1",
				EANGTIN:      &ean,
				Producto:     "Leche Colun Entera 1L",
				PrecioNormal: 1290,
			},
		},
	})
	if err != nil {
		t.Fatalf("se esperaba ingesta exitosa, se obtuvo error: %v", err)
	}
	if resultado.TotalRecibidos != 1 || resultado.Insertados != 1 {
		t.Errorf("conteo incorrecto en resultado: %+v", resultado)
	}
}

// -------------------------------------------------------------
// Pruebas de integración HTTP de ScraperHandler
// -------------------------------------------------------------

func TestScraperHandler_EndpointsHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := newMockScraperRepository()
	mockRepo.cadenas["jumbo"] = &domain.CadenaSupermercado{
		ID:         1,
		Nombre:     "Jumbo",
		EstaActiva: true,
	}

	service := services.NewScraperService(mockRepo, mockRepo)
	handler := handlers.NewScraperHandler(service)

	router := gin.New()
	v1 := router.Group("/api/v1/scraper")
	{
		v1.POST("/trabajos", handler.IniciarTrabajo)
		v1.GET("/trabajos/:id", handler.ObtenerTrabajo)
		v1.PUT("/trabajos/:id/finalizar", handler.FinalizarTrabajo)
		v1.POST("/trabajos/:id/productos", handler.IngestarProductosConTrabajo)
		v1.POST("/productos", handler.IngestarProductosDirecto)
	}

	// 1. Iniciar trabajo
	bodyIniciar := domain.IniciarTrabajoDTO{
		CadenaID:     1,
		Supermercado: "Jumbo",
	}
	jsonBody, _ := json.Marshal(bodyIniciar)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/scraper/trabajos", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("se esperaba código 201, se obtuvo: %d. Body: %s", w.Code, w.Body.String())
	}

	var trabajoCreado domain.TrabajoScraperDTO
	if err := json.Unmarshal(w.Body.Bytes(), &trabajoCreado); err != nil {
		t.Fatalf("error decodificando json: %v", err)
	}
	if trabajoCreado.ID == "" {
		t.Fatalf("se esperaba ID generado para el trabajo")
	}

	// 2. Ingestar productos vinculados al trabajo
	ean := "7801234567890"
	lote := domain.IngestaLoteDTO{
		Productos: []domain.ProductoScrapeadoDTO{
			{
				SKU:          "TEST-001",
				EANGTIN:      &ean,
				Producto:     "Arroz 1kg",
				PrecioNormal: 1400,
			},
		},
	}
	jsonLote, _ := json.Marshal(lote)
	reqLote, _ := http.NewRequest(http.MethodPost, "/api/v1/scraper/trabajos/"+trabajoCreado.ID+"/productos", bytes.NewBuffer(jsonLote))
	reqLote.Header.Set("Content-Type", "application/json")
	wLote := httptest.NewRecorder()
	router.ServeHTTP(wLote, reqLote)

	if wLote.Code != http.StatusOK {
		t.Fatalf("se esperaba código 200 en ingesta, se obtuvo: %d. Body: %s", wLote.Code, wLote.Body.String())
	}

	// 3. Consultar trabajo por ID
	reqConsultar, _ := http.NewRequest(http.MethodGet, "/api/v1/scraper/trabajos/"+trabajoCreado.ID, nil)
	wConsultar := httptest.NewRecorder()
	router.ServeHTTP(wConsultar, reqConsultar)

	if wConsultar.Code != http.StatusOK {
		t.Fatalf("se esperaba código 200 al consultar trabajo, se obtuvo: %d", wConsultar.Code)
	}

	// 4. Finalizar trabajo
	bodyFin := domain.FinalizarTrabajoDTO{
		Estado: "completado",
	}
	jsonFin, _ := json.Marshal(bodyFin)
	reqFin, _ := http.NewRequest(http.MethodPut, "/api/v1/scraper/trabajos/"+trabajoCreado.ID+"/finalizar", bytes.NewBuffer(jsonFin))
	reqFin.Header.Set("Content-Type", "application/json")
	wFin := httptest.NewRecorder()
	router.ServeHTTP(wFin, reqFin)

	if wFin.Code != http.StatusOK {
		t.Fatalf("se esperaba código 200 al finalizar trabajo, se obtuvo: %d", wFin.Code)
	}
}

func stringPtr(s string) *string {
	return &s
}

// -------------------------------------------------------------
// SUP-243: Control respeta el flag de fuente habilitada
// -------------------------------------------------------------

func TestScraperService_IniciarTrabajo_FuenteDeshabilitada(t *testing.T) {
	mockRepo := newMockScraperRepository()
	mockRepo.cadenas["jumbo"] = &domain.CadenaSupermercado{
		ID:         1,
		Nombre:     "Jumbo",
		EstaActiva: false,
	}

	service := services.NewScraperService(mockRepo, mockRepo)

	_, err := service.IniciarTrabajo(context.Background(), domain.IniciarTrabajoDTO{
		CadenaID:     1,
		Supermercado: "Jumbo",
	})
	if !errors.Is(err, services.ErrFuenteDeshabilitada) {
		t.Fatalf("se esperaba ErrFuenteDeshabilitada, se obtuvo: %v", err)
	}
}

func TestScraperService_ObtenerCadena(t *testing.T) {
	mockRepo := newMockScraperRepository()
	mockRepo.cadenas["jumbo"] = &domain.CadenaSupermercado{
		ID:         1,
		Nombre:     "Jumbo",
		EstaActiva: true,
	}

	service := services.NewScraperService(mockRepo, mockRepo)

	cadena, err := service.ObtenerCadena(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("se esperaba nil error, se obtuvo: %v", err)
	}
	if cadena == nil || !cadena.EstaActiva {
		t.Fatalf("se esperaba la cadena habilitada, se obtuvo: %+v", cadena)
	}
}

func TestScraperService_ObtenerCadena_NoEncontrada(t *testing.T) {
	mockRepo := newMockScraperRepository()
	service := services.NewScraperService(mockRepo, mockRepo)

	if _, err := service.ObtenerCadena(context.Background(), 99, ""); !errors.Is(err, services.ErrCadenaNoEncontrada) {
		t.Fatalf("se esperaba ErrCadenaNoEncontrada, se obtuvo: %v", err)
	}
}

func TestScraperHandler_EjecutarTrabajo_FuenteDeshabilitada(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := newMockScraperRepository()
	mockRepo.cadenas["jumbo"] = &domain.CadenaSupermercado{ID: 1, Nombre: "Jumbo", EstaActiva: false}
	trabajoID := uuid.New()
	mockRepo.trabajos[trabajoID] = &domain.TrabajoScraper{ID: trabajoID, CadenaID: 1, CadenaNombre: "Jumbo", Estado: "en_progreso"}

	handler := handlers.NewScraperHandler(services.NewScraperService(mockRepo, mockRepo))
	handler.SetQueue(services.NewQueueService(&sqlQueue{}, nil))
	router := gin.New()
	router.POST("/trabajos/:id/ejecutar", handler.EjecutarTrabajo)

	req, _ := http.NewRequest(http.MethodPost, "/trabajos/"+trabajoID.String()+"/ejecutar", bytes.NewBufferString(`{"spider":"jumbo_rsc"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "fuente_deshabilitada") {
		t.Fatalf("se esperaba 409 fuente_deshabilitada, se obtuvo: %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestScraperHandler_EjecutarTrabajo_CadenaNoEncontrada(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := newMockScraperRepository()
	trabajoID := uuid.New()
	mockRepo.trabajos[trabajoID] = &domain.TrabajoScraper{ID: trabajoID, CadenaID: 99, Estado: "en_progreso"}

	handler := handlers.NewScraperHandler(services.NewScraperService(mockRepo, mockRepo))
	handler.SetQueue(services.NewQueueService(&sqlQueue{}, nil))
	router := gin.New()
	router.POST("/trabajos/:id/ejecutar", handler.EjecutarTrabajo)

	req, _ := http.NewRequest(http.MethodPost, "/trabajos/"+trabajoID.String()+"/ejecutar", bytes.NewBufferString(`{"spider":"jumbo_rsc"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("se esperaba 404 para cadena inexistente, se obtuvo: %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestScraperHandler_IniciarTrabajo_FuenteDeshabilitada(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := newMockScraperRepository()
	mockRepo.cadenas["jumbo"] = &domain.CadenaSupermercado{
		ID:         1,
		Nombre:     "Jumbo",
		EstaActiva: false,
	}

	handler := handlers.NewScraperHandler(services.NewScraperService(mockRepo, mockRepo))
	router := gin.New()
	router.POST("/trabajos", handler.IniciarTrabajo)

	body, _ := json.Marshal(domain.IniciarTrabajoDTO{CadenaID: 1, Supermercado: "Jumbo"})
	req, _ := http.NewRequest(http.MethodPost, "/trabajos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("se esperaba 409 para fuente deshabilitada, se obtuvo: %d. Body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "fuente_deshabilitada") {
		t.Errorf("se esperaba el código 'fuente_deshabilitada' en la respuesta: %s", w.Body.String())
	}
}
