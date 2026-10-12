package tests

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
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.HandleMethodNotAllowed = true

	r.NoRoute(middleware.NotFoundHandler())
	r.NoMethod(middleware.MethodNotAllowedHandler())

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	r.GET("/panic", func(c *gin.Context) {
		panic("error crítico de prueba simulado")
	})

	r.GET("/bad-request", func(c *gin.Context) {
		middleware.ResponderError(c, http.StatusBadRequest, "Datos de entrada inválidos", errors.New("campo 'nombre' es requerido"))
	})

	return r
}

func TestNotFoundHandler(t *testing.T) {
	router := setupTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ruta-inexistente", nil)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("Esperaba código 404, pero obtuve %d", w.Code)
	}

	var resp middleware.RespuestaError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("Error al decodificar JSON de respuesta: %v", err)
	}

	if resp.Estado != 404 {
		t.Errorf("Esperaba estado=404 en el cuerpo JSON, obtuve %d", resp.Estado)
	}
	if resp.Mensaje == "" {
		t.Errorf("El mensaje de respuesta no debería estar vacío")
	}
}

func TestMethodNotAllowedHandler(t *testing.T) {
	router := setupTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ping", nil)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("Esperaba código 405, pero obtuve %d", w.Code)
	}

	var resp middleware.RespuestaError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("Error al decodificar JSON de respuesta: %v", err)
	}

	if resp.Estado != 405 {
		t.Errorf("Esperaba estado=405 en el cuerpo JSON, obtuve %d", resp.Estado)
	}
}

func TestPanicRecoveryHandler(t *testing.T) {
	router := setupTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panic", nil)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Esperaba código 500 tras panic, pero obtuve %d", w.Code)
	}

	var resp middleware.RespuestaError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("Error al decodificar JSON de respuesta: %v", err)
	}

	if resp.Estado != 500 {
		t.Errorf("Esperaba estado=500 en el cuerpo JSON, obtuve %d", resp.Estado)
	}
	if resp.Detalle != "" {
		t.Errorf("El panic filtró un detalle interno: '%s'", resp.Detalle)
	}
}

func TestResponderError(t *testing.T) {
	router := setupTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/bad-request", nil)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Esperaba código 400, pero obtuve %d", w.Code)
	}

	var resp middleware.RespuestaError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("Error al decodificar JSON de respuesta: %v", err)
	}

	if resp.Estado != 400 {
		t.Errorf("Esperaba estado=400 en el cuerpo JSON, obtuve %d", resp.Estado)
	}
	if resp.Mensaje != "Datos de entrada inválidos" {
		t.Errorf("Mensaje inesperado: %s", resp.Mensaje)
	}
	if resp.Detalle != "campo 'nombre' es requerido" {
		t.Errorf("Detalle inesperado: %s", resp.Detalle)
	}
}
