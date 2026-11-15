package service_test

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

// ===== Repositorio simulado para las pruebas =====

// mockScraperRepository implementa la interfaz ScraperRepository para pruebas unitarias
type mockScraperRepository struct {
	trabajos      map[uuid.UUID]*domain.TrabajoScraper
	cadenas       map[string]*domain.CadenaSupermercado
	sucursales    map[int]*domain.SucursalSupermercado
	errorSimulado error
}

// newMockScraperRepository crea los mapas vacíos requeridos por el servicio simulado.
func newMockScraperRepository() *mockScraperRepository {
	return &mockScraperRepository{
		trabajos:   make(map[uuid.UUID]*domain.TrabajoScraper),
		cadenas:    make(map[string]*domain.CadenaSupermercado),
		sucursales: make(map[int]*domain.SucursalSupermercado),
	}
}

// CrearTrabajo conserva el trabajo recibido para que las pruebas consulten su estado.
func (repositorioSimulado *mockScraperRepository) CrearTrabajo(contexto context.Context, trabajo *domain.TrabajoScraper) error {
	if repositorioSimulado.errorSimulado != nil {
		return repositorioSimulado.errorSimulado
	}
	repositorioSimulado.trabajos[trabajo.ID] = trabajo
	return nil
}

// ObtenerTrabajoPorID recupera un trabajo guardado o informa que no existe.
func (repositorioSimulado *mockScraperRepository) ObtenerTrabajoPorID(contexto context.Context, identificador uuid.UUID) (*domain.TrabajoScraper, error) {
	if repositorioSimulado.errorSimulado != nil {
		return nil, repositorioSimulado.errorSimulado
	}
	trabajo, esValido := repositorioSimulado.trabajos[identificador]
	if !esValido {
		return nil, nil
	}
	return trabajo, nil
}

// FinalizarTrabajo actualiza los campos de cierre del trabajo simulado.
func (repositorioSimulado *mockScraperRepository) FinalizarTrabajo(contexto context.Context, identificador uuid.UUID, estado string, elementosExtraidos *int, respuestasBloqueadas *int, registroErrores *string) (*domain.TrabajoScraper, error) {
	if repositorioSimulado.errorSimulado != nil {
		return nil, repositorioSimulado.errorSimulado
	}
	trabajo, esValido := repositorioSimulado.trabajos[identificador]
	if !esValido {
		return nil, nil
	}
	ahora := time.Now()
	trabajo.Estado = estado
	trabajo.FinalizadoEl = &ahora
	if elementosExtraidos != nil {
		trabajo.ElementosExtraidos = *elementosExtraidos
	}
	if respuestasBloqueadas != nil {
		trabajo.RespuestasBloqueadas = *respuestasBloqueadas
	}
	trabajo.RegistroErrores = registroErrores
	return trabajo, nil
}

// ObtenerCadena busca por ID o nombre y crea una cadena simulada si hace falta.
func (repositorioSimulado *mockScraperRepository) ObtenerCadena(contexto context.Context, cadenaID int, nombreCadena string) (*domain.CadenaSupermercado, error) {
	if repositorioSimulado.errorSimulado != nil {
		return nil, repositorioSimulado.errorSimulado
	}
	if cadenaID > 0 {
		for _, cadena := range repositorioSimulado.cadenas {
			if cadena.ID == cadenaID {
				return cadena, nil
			}
		}
	}
	if nombreCadena != "" {
		if cadena, esValido := repositorioSimulado.cadenas[strings.ToLower(nombreCadena)]; esValido {
			return cadena, nil
		}
		// Simula la creación automática en mock
		nueva := &domain.CadenaSupermercado{
			ID:         len(repositorioSimulado.cadenas) + 1,
			Nombre:     nombreCadena,
			EstaActiva: true,
			CreadoEl:   time.Now(),
		}
		repositorioSimulado.cadenas[strings.ToLower(nombreCadena)] = nueva
		return nueva, nil
	}
	return nil, nil
}

// ObtenerSucursal recupera o crea la sucursal usada por las pruebas de ingesta.
func (repositorioSimulado *mockScraperRepository) ObtenerSucursal(contexto context.Context, cadenaID int, sucursalID *int, codigoSucursal *string) (*domain.SucursalSupermercado, error) {
	if repositorioSimulado.errorSimulado != nil {
		return nil, repositorioSimulado.errorSimulado
	}
	if sucursalID != nil && *sucursalID > 0 {
		if sucursal, esValido := repositorioSimulado.sucursales[*sucursalID]; esValido {
			return sucursal, nil
		}
	}
	// Retornar primera sucursal mock
	cod := "CENTRAL-001"
	sucursal := &domain.SucursalSupermercado{
		ID:             1,
		CadenaID:       cadenaID,
		CodigoSucursal: &cod,
		Nombre:         "Sucursal Mock",
		EstaActiva:     true,
	}
	repositorioSimulado.sucursales[1] = sucursal
	return sucursal, nil
}

// IngestarLote simula una ingesta exitosa con contadores coherentes.
func (repositorioSimulado *mockScraperRepository) IngestarLote(contexto context.Context, trabajoID *uuid.UUID, sucursalID int, productos []domain.ProductoScrapeadoDTO) (*domain.IngestaResultadoDTO, error) {
	if repositorioSimulado.errorSimulado != nil {
		return nil, repositorioSimulado.errorSimulado
	}

	var trabajoIDStr *string
	if trabajoID != nil {
		cadena := trabajoID.String()
		trabajoIDStr = &cadena
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

// IncrementarElementos actualiza el contador del trabajo simulado.
func (repositorioSimulado *mockScraperRepository) IncrementarElementos(contexto context.Context, identificador uuid.UUID, cantidad int) error {
	if repositorioSimulado.errorSimulado != nil {
		return repositorioSimulado.errorSimulado
	}
	if prueba, esValido := repositorioSimulado.trabajos[identificador]; esValido {
		prueba.ElementosExtraidos += cantidad
	}
	return nil
}

// ===== Pruebas unitarias del servicio =====

// TestScraperService_IniciarTrabajo_Exitoso comprueba la creación de un trabajo válido.
func TestScraperService_IniciarTrabajo_Exitoso(prueba *testing.T) {
	repositorioSimulado := newMockScraperRepository()
	repositorioSimulado.cadenas["jumbo"] = &domain.CadenaSupermercado{
		ID:         1,
		Nombre:     "Jumbo",
		EstaActiva: true,
	}

	servicio := services.NewScraperService(repositorioSimulado, repositorioSimulado)

	entrada := domain.IniciarTrabajoDTO{
		CadenaID:     1,
		Supermercado: "Jumbo",
	}

	trabajo, errorObtenido := servicio.IniciarTrabajo(context.Background(), entrada)
	if errorObtenido != nil {
		prueba.Fatalf("se esperaba nil error al iniciar trabajo, se obtuvo: %v", errorObtenido)
	}

	if trabajo.Estado != "en_progreso" {
		prueba.Errorf("se esperaba estado 'en_progreso', se obtuvo: %s", trabajo.Estado)
	}
	if trabajo.CadenaID != 1 {
		prueba.Errorf("se esperaba cadenaID=1, se obtuvo: %d", trabajo.CadenaID)
	}
}

// TestScraperService_FinalizarTrabajo_Validaciones comprueba estados válidos e inválidos.
func TestScraperService_FinalizarTrabajo_Validaciones(prueba *testing.T) {
	repositorioSimulado := newMockScraperRepository()
	servicio := services.NewScraperService(repositorioSimulado, repositorioSimulado)

	trabajoID := uuid.New()
	repositorioSimulado.trabajos[trabajoID] = &domain.TrabajoScraper{
		ID:         trabajoID,
		CadenaID:   1,
		Estado:     "en_progreso",
		IniciadoEl: time.Now().Add(-10 * time.Minute),
	}

	// 1. Estado inválido
	_, errorObtenido := servicio.FinalizarTrabajo(context.Background(), trabajoID.String(), domain.FinalizarTrabajoDTO{
		Estado: "pausado",
	})
	if !errors.Is(errorObtenido, services.ErrEstadoTrabajoInvalido) {
		prueba.Errorf("se esperaba ErrEstadoTrabajoInvalido, se obtuvo: %v", errorObtenido)
	}

	// 2. Finalización exitosa
	elementos := 150
	bloqueados := 2
	trabajoFin, errorObtenido := servicio.FinalizarTrabajo(context.Background(), trabajoID.String(), domain.FinalizarTrabajoDTO{
		Estado:               "completado",
		ElementosExtraidos:   &elementos,
		RespuestasBloqueadas: &bloqueados,
	})
	if errorObtenido != nil {
		prueba.Fatalf("se esperaba nil error, se obtuvo: %v", errorObtenido)
	}
	if trabajoFin.Estado != "completado" {
		prueba.Errorf("se esperaba 'completado', se obtuvo: %s", trabajoFin.Estado)
	}
	if trabajoFin.ElementosExtraidos != 150 {
		prueba.Errorf("se esperaba 150 items, se obtuvo: %d", trabajoFin.ElementosExtraidos)
	}
	if trabajoFin.RespuestasBloqueadas != 2 {
		prueba.Errorf("se esperaban 2 respuestas bloqueadas, se obtuvo: %d", trabajoFin.RespuestasBloqueadas)
	}
	if trabajoFin.DuracionSegundos == nil || *trabajoFin.DuracionSegundos <= 0 {
		prueba.Errorf("se esperaba duracion calculada positiva, se obtuvo: %v", trabajoFin.DuracionSegundos)
	}

	// 3. Intento de re-finalizar un trabajo ya completado
	_, errorObtenido = servicio.FinalizarTrabajo(context.Background(), trabajoID.String(), domain.FinalizarTrabajoDTO{
		Estado: "completado",
	})
	if !errors.Is(errorObtenido, services.ErrTrabajoYaFinalizado) {
		prueba.Errorf("se esperaba ErrTrabajoYaFinalizado, se obtuvo: %v", errorObtenido)
	}
}

// TestScraperService_IngestarProductos_LimitesYValidaciones comprueba lotes vacíos y válidos.
func TestScraperService_IngestarProductos_LimitesYValidaciones(prueba *testing.T) {
	repositorioSimulado := newMockScraperRepository()
	servicio := services.NewScraperService(repositorioSimulado, repositorioSimulado)

	// 1. Lote vacío
	_, errorObtenido := servicio.IngestarProductos(context.Background(), nil, domain.IngestaLoteDTO{
		Productos: []domain.ProductoScrapeadoDTO{},
	})
	if !errors.Is(errorObtenido, services.ErrLoteVacio) {
		prueba.Errorf("se esperaba ErrLoteVacio, se obtuvo: %v", errorObtenido)
	}

	// 2. Lote exitoso
	ean := "7802900001234"
	resultado, errorObtenido := servicio.IngestarProductos(context.Background(), nil, domain.IngestaLoteDTO{
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
	if errorObtenido != nil {
		prueba.Fatalf("se esperaba ingesta exitosa, se obtuvo error: %v", errorObtenido)
	}
	if resultado.TotalRecibidos != 1 || resultado.Insertados != 1 {
		prueba.Errorf("conteo incorrecto en resultado: %+v", resultado)
	}
}

// TestScraperService_IngestaDirectaPorSucursalFuenteDeshabilitada rechaza fuentes apagadas.
func TestScraperService_IngestaDirectaPorSucursalFuenteDeshabilitada(prueba *testing.T) {
	repositorioSimulado := newMockScraperRepository()
	repositorioSimulado.cadenas["jumbo"] = &domain.CadenaSupermercado{
		ID:         1,
		Nombre:     "Jumbo",
		EstaActiva: false,
	}
	repositorioSimulado.sucursales[1] = &domain.SucursalSupermercado{
		ID:         1,
		CadenaID:   1,
		EstaActiva: true,
	}
	servicio := services.NewScraperService(repositorioSimulado, repositorioSimulado)
	sucursalID := 1

	_, errorObtenido := servicio.IngestarProductos(context.Background(), nil, domain.IngestaLoteDTO{
		SucursalID: &sucursalID,
		Productos: []domain.ProductoScrapeadoDTO{
			{SKU: "SKU-1", Producto: "Leche", PrecioNormal: 1290},
		},
	})
	if !errors.Is(errorObtenido, services.ErrFuenteDeshabilitada) {
		prueba.Fatalf("se esperaba ErrFuenteDeshabilitada, se obtuvo: %v", errorObtenido)
	}
}

// TestScraperService_IngestaDirectaFuenteDeshabilitada rechaza ingesta de una fuente apagada.
func TestScraperService_IngestaDirectaFuenteDeshabilitada(prueba *testing.T) {
	repositorioSimulado := newMockScraperRepository()
	repositorioSimulado.cadenas["jumbo"] = &domain.CadenaSupermercado{
		ID:         1,
		Nombre:     "Jumbo",
		EstaActiva: false,
	}
	servicio := services.NewScraperService(repositorioSimulado, repositorioSimulado)

	_, errorObtenido := servicio.IngestarProductos(context.Background(), nil, domain.IngestaLoteDTO{
		Supermercado: stringPtr("Jumbo"),
		Productos: []domain.ProductoScrapeadoDTO{
			{SKU: "SKU-1", Producto: "Leche", PrecioNormal: 1290},
		},
	})
	if !errors.Is(errorObtenido, services.ErrFuenteDeshabilitada) {
		prueba.Fatalf("se esperaba ErrFuenteDeshabilitada, se obtuvo: %v", errorObtenido)
	}
}

// ===== Pruebas HTTP del handler =====

// TestScraperHandler_EndpointsHTTP comprueba el ciclo HTTP de un trabajo.
func TestScraperHandler_EndpointsHTTP(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	repositorioSimulado := newMockScraperRepository()
	repositorioSimulado.cadenas["jumbo"] = &domain.CadenaSupermercado{
		ID:         1,
		Nombre:     "Jumbo",
		EstaActiva: true,
	}

	servicio := services.NewScraperService(repositorioSimulado, repositorioSimulado)
	manejador := handlers.NewScraperHandler(servicio)

	enrutador := gin.New()
	v1 := enrutador.Group("/api/v1/scraper")
	{
		v1.POST("/trabajos", manejador.IniciarTrabajo)
		v1.GET("/trabajos/:id", manejador.ObtenerTrabajo)
		v1.PUT("/trabajos/:id/finalizar", manejador.FinalizarTrabajo)
		v1.POST("/trabajos/:id/productos", manejador.IngestarProductosConTrabajo)
		v1.POST("/productos", manejador.IngestarProductosDirecto)
	}

	// 1. Iniciar trabajo
	cuerpoInicio := domain.IniciarTrabajoDTO{
		CadenaID:     1,
		Supermercado: "Jumbo",
	}
	cuerpoJSON, _ := json.Marshal(cuerpoInicio)
	peticion, _ := http.NewRequest(http.MethodPost, "/api/v1/scraper/trabajos", bytes.NewBuffer(cuerpoJSON))
	peticion.Header.Set("Content-Type", "application/json")
	respuestaHTTP := httptest.NewRecorder()
	enrutador.ServeHTTP(respuestaHTTP, peticion)

	if respuestaHTTP.Code != http.StatusCreated {
		prueba.Fatalf("se esperaba código 201, se obtuvo: %d. Body: %s", respuestaHTTP.Code, respuestaHTTP.Body.String())
	}

	var trabajoCreado domain.TrabajoScraperDTO
	if errorObtenido := json.Unmarshal(respuestaHTTP.Body.Bytes(), &trabajoCreado); errorObtenido != nil {
		prueba.Fatalf("error decodificando json: %v", errorObtenido)
	}
	if trabajoCreado.ID == "" {
		prueba.Fatalf("se esperaba ID generado para el trabajo")
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
	JSONLote, _ := json.Marshal(lote)
	peticionLote, _ := http.NewRequest(http.MethodPost, "/api/v1/scraper/trabajos/"+trabajoCreado.ID+"/productos", bytes.NewBuffer(JSONLote))
	peticionLote.Header.Set("Content-Type", "application/json")
	escritorLote := httptest.NewRecorder()
	enrutador.ServeHTTP(escritorLote, peticionLote)

	if escritorLote.Code != http.StatusOK {
		prueba.Fatalf("se esperaba código 200 en ingesta, se obtuvo: %d. Body: %s", escritorLote.Code, escritorLote.Body.String())
	}

	// 3. Consultar trabajo por ID
	peticionConsulta, _ := http.NewRequest(http.MethodGet, "/api/v1/scraper/trabajos/"+trabajoCreado.ID, nil)
	escritorConsulta := httptest.NewRecorder()
	enrutador.ServeHTTP(escritorConsulta, peticionConsulta)

	if escritorConsulta.Code != http.StatusOK {
		prueba.Fatalf("se esperaba código 200 al consultar trabajo, se obtuvo: %d", escritorConsulta.Code)
	}

	// 4. Finalizar trabajo
	cuerpoFinal := domain.FinalizarTrabajoDTO{
		Estado: "completado",
	}
	JSONFinal, _ := json.Marshal(cuerpoFinal)
	peticionFinalizacion, _ := http.NewRequest(http.MethodPut, "/api/v1/scraper/trabajos/"+trabajoCreado.ID+"/finalizar", bytes.NewBuffer(JSONFinal))
	peticionFinalizacion.Header.Set("Content-Type", "application/json")
	escritorFinalizacion := httptest.NewRecorder()
	enrutador.ServeHTTP(escritorFinalizacion, peticionFinalizacion)

	if escritorFinalizacion.Code != http.StatusOK {
		prueba.Fatalf("se esperaba código 200 al finalizar trabajo, se obtuvo: %d", escritorFinalizacion.Code)
	}
}

// stringPtr devuelve un puntero al valor entregado para construir DTO de prueba.
func stringPtr(valor string) *string {
	return &valor
}

// ===== Reglas de habilitación de fuentes =====

// TestScraperService_IniciarTrabajo_FuenteDeshabilitada impide iniciar una fuente apagada.
func TestScraperService_IniciarTrabajo_FuenteDeshabilitada(prueba *testing.T) {
	repositorioSimulado := newMockScraperRepository()
	repositorioSimulado.cadenas["jumbo"] = &domain.CadenaSupermercado{
		ID:         1,
		Nombre:     "Jumbo",
		EstaActiva: false,
	}

	servicio := services.NewScraperService(repositorioSimulado, repositorioSimulado)

	_, errorObtenido := servicio.IniciarTrabajo(context.Background(), domain.IniciarTrabajoDTO{
		CadenaID:     1,
		Supermercado: "Jumbo",
	})
	if !errors.Is(errorObtenido, services.ErrFuenteDeshabilitada) {
		prueba.Fatalf("se esperaba ErrFuenteDeshabilitada, se obtuvo: %v", errorObtenido)
	}
}

// TestScraperService_ObtenerCadena devuelve la cadena activa resuelta por ID.
func TestScraperService_ObtenerCadena(prueba *testing.T) {
	repositorioSimulado := newMockScraperRepository()
	repositorioSimulado.cadenas["jumbo"] = &domain.CadenaSupermercado{
		ID:         1,
		Nombre:     "Jumbo",
		EstaActiva: true,
	}

	servicio := services.NewScraperService(repositorioSimulado, repositorioSimulado)

	cadena, errorObtenido := servicio.ObtenerCadena(context.Background(), 1, "")
	if errorObtenido != nil {
		prueba.Fatalf("se esperaba nil error, se obtuvo: %v", errorObtenido)
	}
	if cadena == nil || !cadena.EstaActiva {
		prueba.Fatalf("se esperaba la cadena habilitada, se obtuvo: %+v", cadena)
	}
}

// TestScraperService_ObtenerCadena_NoEncontrada traduce la ausencia a error de dominio.
func TestScraperService_ObtenerCadena_NoEncontrada(prueba *testing.T) {
	repositorioSimulado := newMockScraperRepository()
	servicio := services.NewScraperService(repositorioSimulado, repositorioSimulado)

	if _, errorObtenido := servicio.ObtenerCadena(context.Background(), 99, ""); !errors.Is(errorObtenido, services.ErrCadenaNoEncontrada) {
		prueba.Fatalf("se esperaba ErrCadenaNoEncontrada, se obtuvo: %v", errorObtenido)
	}
}

// TestScraperHandler_IngestaDirectaFuenteDeshabilitada verifica el rechazo HTTP de fuentes apagadas.
func TestScraperHandler_IngestaDirectaFuenteDeshabilitada(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	repositorioSimulado := newMockScraperRepository()
	repositorioSimulado.cadenas["jumbo"] = &domain.CadenaSupermercado{ID: 1, Nombre: "Jumbo", EstaActiva: false}
	manejador := handlers.NewScraperHandler(services.NewScraperService(repositorioSimulado, repositorioSimulado))
	enrutador := gin.New()
	enrutador.POST("/productos", manejador.IngestarProductosDirecto)

	cuerpo := `{"supermercado":"Jumbo","productos":[{"sku":"SKU-1","producto":"Leche","precio_normal":1290}]}`
	peticion, _ := http.NewRequest(http.MethodPost, "/productos", bytes.NewBufferString(cuerpo))
	peticion.Header.Set("Content-Type", "application/json")
	respuestaHTTP := httptest.NewRecorder()
	enrutador.ServeHTTP(respuestaHTTP, peticion)

	if respuestaHTTP.Code != http.StatusConflict || !strings.Contains(respuestaHTTP.Body.String(), "fuente_deshabilitada") {
		prueba.Fatalf("se esperaba 409 fuente_deshabilitada, se obtuvo: %d. Body: %s", respuestaHTTP.Code, respuestaHTTP.Body.String())
	}
}

// TestScraperHandler_EjecutarTrabajo_FuenteDeshabilitada impide encolar una fuente apagada.
func TestScraperHandler_EjecutarTrabajo_FuenteDeshabilitada(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	repositorioSimulado := newMockScraperRepository()
	repositorioSimulado.cadenas["jumbo"] = &domain.CadenaSupermercado{ID: 1, Nombre: "Jumbo", EstaActiva: false}
	trabajoID := uuid.New()
	repositorioSimulado.trabajos[trabajoID] = &domain.TrabajoScraper{ID: trabajoID, CadenaID: 1, CadenaNombre: "Jumbo", Estado: "en_progreso"}

	manejador := handlers.NewScraperHandler(services.NewScraperService(repositorioSimulado, repositorioSimulado))
	manejador.SetQueue(services.NewQueueService(&sqlQueue{}, nil))
	enrutador := gin.New()
	enrutador.POST("/trabajos/:id/ejecutar", manejador.EjecutarTrabajo)

	peticion, _ := http.NewRequest(http.MethodPost, "/trabajos/"+trabajoID.String()+"/ejecutar", bytes.NewBufferString(`{"spider":"jumbo_rsc"}`))
	peticion.Header.Set("Content-Type", "application/json")
	respuestaHTTP := httptest.NewRecorder()
	enrutador.ServeHTTP(respuestaHTTP, peticion)

	if respuestaHTTP.Code != http.StatusConflict || !strings.Contains(respuestaHTTP.Body.String(), "fuente_deshabilitada") {
		prueba.Fatalf("se esperaba 409 fuente_deshabilitada, se obtuvo: %d. Body: %s", respuestaHTTP.Code, respuestaHTTP.Body.String())
	}
}

// TestScraperHandler_EjecutarTrabajo_CadenaNoEncontrada verifica el 404 al no resolver la cadena.
func TestScraperHandler_EjecutarTrabajo_CadenaNoEncontrada(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	repositorioSimulado := newMockScraperRepository()
	trabajoID := uuid.New()
	repositorioSimulado.trabajos[trabajoID] = &domain.TrabajoScraper{ID: trabajoID, CadenaID: 99, Estado: "en_progreso"}

	manejador := handlers.NewScraperHandler(services.NewScraperService(repositorioSimulado, repositorioSimulado))
	manejador.SetQueue(services.NewQueueService(&sqlQueue{}, nil))
	enrutador := gin.New()
	enrutador.POST("/trabajos/:id/ejecutar", manejador.EjecutarTrabajo)

	peticion, _ := http.NewRequest(http.MethodPost, "/trabajos/"+trabajoID.String()+"/ejecutar", bytes.NewBufferString(`{"spider":"jumbo_rsc"}`))
	peticion.Header.Set("Content-Type", "application/json")
	respuestaHTTP := httptest.NewRecorder()
	enrutador.ServeHTTP(respuestaHTTP, peticion)

	if respuestaHTTP.Code != http.StatusNotFound {
		prueba.Fatalf("se esperaba 404 para cadena inexistente, se obtuvo: %d. Body: %s", respuestaHTTP.Code, respuestaHTTP.Body.String())
	}
}

// TestScraperHandler_IniciarTrabajo_FuenteDeshabilitada verifica la respuesta de inicio HTTP.
func TestScraperHandler_IniciarTrabajo_FuenteDeshabilitada(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	repositorioSimulado := newMockScraperRepository()
	repositorioSimulado.cadenas["jumbo"] = &domain.CadenaSupermercado{
		ID:         1,
		Nombre:     "Jumbo",
		EstaActiva: false,
	}

	manejador := handlers.NewScraperHandler(services.NewScraperService(repositorioSimulado, repositorioSimulado))
	enrutador := gin.New()
	enrutador.POST("/trabajos", manejador.IniciarTrabajo)

	cuerpo, _ := json.Marshal(domain.IniciarTrabajoDTO{CadenaID: 1, Supermercado: "Jumbo"})
	peticion, _ := http.NewRequest(http.MethodPost, "/trabajos", bytes.NewBuffer(cuerpo))
	peticion.Header.Set("Content-Type", "application/json")
	respuestaHTTP := httptest.NewRecorder()
	enrutador.ServeHTTP(respuestaHTTP, peticion)

	if respuestaHTTP.Code != http.StatusConflict {
		prueba.Fatalf("se esperaba 409 para fuente deshabilitada, se obtuvo: %d. Body: %s", respuestaHTTP.Code, respuestaHTTP.Body.String())
	}
	if !strings.Contains(respuestaHTTP.Body.String(), "fuente_deshabilitada") {
		prueba.Errorf("se esperaba el código 'fuente_deshabilitada' en la respuesta: %s", respuestaHTTP.Body.String())
	}
}
