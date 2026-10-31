package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
)

func TestProductoHandler_ObtenerProductos(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := &mockProductoRepository{
		items: []domain.ProductoDTO{
			{
				ID:           "test-uuid-1",
				Nombre:       "Arroz Grado 1",
				Marca:        "Tucapel",
				Categoria:    "Despensa",
				Supermercado: "Lider",
				Precio:       1290,
				PrecioNormal: 1490,
				EnOferta:     true,
			},
		},
		total: 1,
	}

	service := services.NewProductoService(mockRepo)
	handler := handlers.NewProductoHandler(service)

	router := gin.New()
	router.GET("/api/v1/productos", handler.ObtenerProductos)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/productos?page=2&limit=10&categoria=Despensa&supermercado=Lider&precio_min=1000&precio_max=2000&en_oferta=true&sort_by=nombre&order=desc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba código 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}

	var response domain.PaginaProductosDTO
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("error parseando JSON de respuesta: %v", err)
	}

	if len(response.Data) != 1 {
		t.Fatalf("se esperaba 1 producto, se obtuvieron %d", len(response.Data))
	}

	if response.Data[0].Nombre != "Arroz Grado 1" {
		t.Errorf("se esperaba 'Arroz Grado 1', se obtuvo '%s'", response.Data[0].Nombre)
	}
}

func TestProductoHandler_BuscarProductos(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := &mockProductoRepository{
		items: []domain.ProductoDTO{
			{
				ID:           "prod-leche-1",
				Nombre:       "Leche Entera Colun 1L",
				Marca:        "Colun",
				Categoria:    "Lácteos",
				Supermercado: "Lider",
				Precio:       990,
				PrecioNormal: 1290,
				EnOferta:     true,
			},
		},
		total: 1,
	}

	service := services.NewProductoService(mockRepo)
	handler := handlers.NewProductoHandler(service)

	router := gin.New()
	router.GET("/api/v1/productos/buscar", handler.BuscarProductos)

	t.Run("400 Bad Request cuando falta parámetro q", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/productos/buscar", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("se esperaba HTTP 400, se obtuvo %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("400 Bad Request cuando q tiene menos de 3 caracteres", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/productos/buscar?q=le", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("se esperaba HTTP 400 por búsqueda corta, se obtuvo %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("200 OK con búsqueda válida de al menos 3 caracteres", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/productos/buscar?q=leche&page=1&limit=5", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("se esperaba HTTP 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
		}

		var res domain.PaginaProductosDTO
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("error parseando JSON: %v", err)
		}

		if len(res.Data) != 1 {
			t.Fatalf("se esperaba 1 resultado, se obtuvieron %d", len(res.Data))
		}
		if res.Data[0].Nombre != "Leche Entera Colun 1L" {
			t.Errorf("se esperaba 'Leche Entera Colun 1L', se obtuvo '%s'", res.Data[0].Nombre)
		}
		if mockRepo.lastFilter.Query != "leche" {
			t.Errorf("se esperaba query 'leche', se obtuvo '%s'", mockRepo.lastFilter.Query)
		}
	})
}

func TestProductoHandler_ObtenerDetalleProducto(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fecha := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	oferta := 890.0
	mockRepo := &mockProductoRepository{
		detalle: &domain.ProductoDetalleDTO{
			ProductoDTO: domain.ProductoDTO{
				ID:           "prod-uuid-xyz",
				Nombre:       "Leche Chocolate 1L",
				Marca:        "Colun",
				Categoria:    "Lácteos",
				Supermercado: "Jumbo",
				Precio:       890,
				PrecioNormal: 1090,
				PrecioOferta: &oferta,
				EnOferta:     true,
			},
			Historial: []domain.HistorialPrecioDTO{
				{
					PrecioNormal: 1090,
					PrecioOferta: &oferta,
					CapturadoEl:  fecha,
				},
			},
		},
	}

	service := services.NewProductoService(mockRepo)
	handler := handlers.NewProductoHandler(service)

	router := gin.New()
	router.GET("/api/v1/productos/:id", handler.ObtenerDetalleProducto)

	t.Run("200 OK cuando existe", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/productos/prod-uuid-xyz", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("se esperaba HTTP 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
		}

		var res domain.ProductoDetalleDTO
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("error parseando respuesta: %v", err)
		}

		if res.ID != "prod-uuid-xyz" {
			t.Errorf("se esperaba prod-uuid-xyz, se obtuvo %s", res.ID)
		}
		if len(res.Historial) != 1 {
			t.Errorf("se esperaba 1 registro histórico, se obtuvieron %d", len(res.Historial))
		}
	})

	t.Run("404 Not Found cuando no existe", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/productos/no-existe", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("se esperaba HTTP 404, se obtuvo %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

func TestProductoHandler_ObtenerProductosAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oferta := 1490
	fecha := time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC)
	mockRepo := &mockProductoRepository{
		adminItems: []domain.ProductoAdminDTO{
			{
				ID:                 "admin-prod-99",
				SKU:                "SKU-JUMBO-99",
				Nombre:             "Aceite Vegetal 1L",
				Marca:              "Chef",
				Categoria:          "Despensa",
				Supermercado:       "Jumbo",
				PrecioNormal:       1990,
				PrecioOferta:       &oferta,
				EnOferta:           true,
				URLImagen:          "https://img.jumbo.cl/aceite.jpg",
				Unidad:             "1 L",
				EnStock:            true,
				UltimaExtraccionEl: fecha,
				SucursalID:         12,
			},
		},
		total: 1,
	}

	service := services.NewProductoService(mockRepo)
	handler := handlers.NewProductoHandler(service)

	router := gin.New()
	router.GET("/api/v1/admin/productos", handler.ObtenerProductosAdmin)

	t.Run("200 OK con filtros completos de administración", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/productos?page=1&limit=10&sku=SKU-JUMBO-99&en_stock=true&sort_by=sku&order=asc", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("se esperaba HTTP 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
		}

		var res domain.PaginaProductosAdminDTO
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("error parseando JSON: %v", err)
		}

		if len(res.Data) != 1 {
			t.Fatalf("se esperaba 1 producto admin, se obtuvieron %d", len(res.Data))
		}
		if res.Data[0].SKU != "SKU-JUMBO-99" {
			t.Errorf("se esperaba SKU 'SKU-JUMBO-99', se obtuvo '%s'", res.Data[0].SKU)
		}
		if res.Data[0].PrecioNormal != 1990 {
			t.Errorf("se esperaba PrecioNormal=1990, se obtuvo %d", res.Data[0].PrecioNormal)
		}
		if res.Data[0].PrecioOferta == nil || *res.Data[0].PrecioOferta != 1490 {
			t.Errorf("se esperaba PrecioOferta=1490")
		}
		if !res.Data[0].EnStock {
			t.Errorf("se esperaba EnStock=true")
		}
	})

	t.Run("400 Bad Request cuando búsqueda por texto libre q es corta", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/productos?q=ab", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("se esperaba HTTP 400 por búsqueda corta, se obtuvo %d. Body: %s", w.Code, w.Body.String())
		}
	})
}
