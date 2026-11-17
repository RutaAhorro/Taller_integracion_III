package middleware_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/middleware"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter() *gin.Engine {
	enrutador := gin.New()
	enrutador.Use(middleware.ErrorHandler())
	enrutador.HandleMethodNotAllowed = true

	enrutador.NoRoute(middleware.NotFoundHandler())
	enrutador.NoMethod(middleware.MethodNotAllowedHandler())

	enrutador.GET("/ping", func(contexto *gin.Context) {
		contexto.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	enrutador.GET("/panic", func(contexto *gin.Context) {
		panic("error crítico de prueba simulado")
	})

	enrutador.GET("/bad-request", func(contexto *gin.Context) {
		middleware.ResponderError(contexto, http.StatusBadRequest, "Datos de entrada inválidos", errors.New("campo 'nombre' es requerido"))
	})

	return enrutador
}

func TestNotFoundHandler(prueba *testing.T) {
	enrutador := setupTestRouter()
	respuestaHTTP := httptest.NewRecorder()
	peticion, _ := http.NewRequest("GET", "/ruta-inexistente", nil)

	enrutador.ServeHTTP(respuestaHTTP, peticion)

	if respuestaHTTP.Code != http.StatusNotFound {
		prueba.Fatalf("Esperaba código 404, pero obtuve %d", respuestaHTTP.Code)
	}

	var respuesta middleware.RespuestaError
	errorObtenido := json.Unmarshal(respuestaHTTP.Body.Bytes(), &respuesta)
	if errorObtenido != nil {
		prueba.Fatalf("Error al decodificar JSON de respuesta: %v", errorObtenido)
	}

	if respuesta.Estado != 404 {
		prueba.Errorf("Esperaba estado=404 en el cuerpo JSON, obtuve %d", respuesta.Estado)
	}
	if respuesta.Mensaje == "" {
		prueba.Errorf("El mensaje de respuesta no debería estar vacío")
	}
}

func TestMethodNotAllowedHandler(prueba *testing.T) {
	enrutador := setupTestRouter()
	respuestaHTTP := httptest.NewRecorder()
	peticion, _ := http.NewRequest("POST", "/ping", nil)

	enrutador.ServeHTTP(respuestaHTTP, peticion)

	if respuestaHTTP.Code != http.StatusMethodNotAllowed {
		prueba.Fatalf("Esperaba código 405, pero obtuve %d", respuestaHTTP.Code)
	}

	var respuesta middleware.RespuestaError
	errorObtenido := json.Unmarshal(respuestaHTTP.Body.Bytes(), &respuesta)
	if errorObtenido != nil {
		prueba.Fatalf("Error al decodificar JSON de respuesta: %v", errorObtenido)
	}

	if respuesta.Estado != 405 {
		prueba.Errorf("Esperaba estado=405 en el cuerpo JSON, obtuve %d", respuesta.Estado)
	}
}

func TestPanicRecoveryHandler(prueba *testing.T) {
	enrutador := setupTestRouter()
	respuestaHTTP := httptest.NewRecorder()
	peticion, _ := http.NewRequest("GET", "/panic", nil)

	enrutador.ServeHTTP(respuestaHTTP, peticion)

	if respuestaHTTP.Code != http.StatusInternalServerError {
		prueba.Fatalf("Esperaba código 500 tras panic, pero obtuve %d", respuestaHTTP.Code)
	}

	var respuesta middleware.RespuestaError
	errorObtenido := json.Unmarshal(respuestaHTTP.Body.Bytes(), &respuesta)
	if errorObtenido != nil {
		prueba.Fatalf("Error al decodificar JSON de respuesta: %v", errorObtenido)
	}

	if respuesta.Estado != 500 {
		prueba.Errorf("Esperaba estado=500 en el cuerpo JSON, obtuve %d", respuesta.Estado)
	}
	if respuesta.Detalle != "" {
		prueba.Errorf("El panic filtró un detalle interno: '%s'", respuesta.Detalle)
	}
}

func TestResponderError(prueba *testing.T) {
	enrutador := setupTestRouter()
	respuestaHTTP := httptest.NewRecorder()
	peticion, _ := http.NewRequest("GET", "/bad-request", nil)

	enrutador.ServeHTTP(respuestaHTTP, peticion)

	if respuestaHTTP.Code != http.StatusBadRequest {
		prueba.Fatalf("Esperaba código 400, pero obtuve %d", respuestaHTTP.Code)
	}

	var respuesta middleware.RespuestaError
	errorObtenido := json.Unmarshal(respuestaHTTP.Body.Bytes(), &respuesta)
	if errorObtenido != nil {
		prueba.Fatalf("Error al decodificar JSON de respuesta: %v", errorObtenido)
	}

	if respuesta.Estado != 400 {
		prueba.Errorf("Esperaba estado=400 en el cuerpo JSON, obtuve %d", respuesta.Estado)
	}
	if respuesta.Mensaje != "Datos de entrada inválidos" {
		prueba.Errorf("Mensaje inesperado: %s", respuesta.Mensaje)
	}
	if respuesta.Detalle != "campo 'nombre' es requerido" {
		prueba.Errorf("Detalle inesperado: %s", respuesta.Detalle)
	}
}
