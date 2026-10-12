package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
)

// mockProductoRepository simula la capa de acceso a datos para pruebas unitarias
type mockProductoRepository struct {
	items      []domain.ProductoDTO
	adminItems []domain.ProductoAdminDTO
	total      int64
	err        error
	lastFilter domain.FiltroProductosDTO
	detalle    *domain.ProductoDetalleDTO
}

func (m *mockProductoRepository) Listar(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoDTO, int64, error) {
	m.lastFilter = filtro
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.items, m.total, nil
}

func (m *mockProductoRepository) ListarParaAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoAdminDTO, int64, error) {
	m.lastFilter = filtro
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.adminItems, m.total, nil
}

func (m *mockProductoRepository) ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.detalle != nil && m.detalle.ID == id {
		return m.detalle, nil
	}
	return nil, nil
}

func TestProductoService_ValoresPorDefecto(t *testing.T) {
	mockRepo := &mockProductoRepository{
		items: []domain.ProductoDTO{
			{ID: "prod-1", Nombre: "Leche", Precio: 1000},
		},
		total: 45,
	}

	service := services.NewProductoService(mockRepo)

	res, err := service.ObtenerCatalogo(context.Background(), domain.FiltroProductosDTO{})
	if err != nil {
		t.Fatalf("se esperaba nil error, se obtuvo: %v", err)
	}

	if mockRepo.lastFilter.Page != 1 {
		t.Errorf("se esperaba Page=1, se obtuvo %d", mockRepo.lastFilter.Page)
	}
	if mockRepo.lastFilter.Limit != 20 {
		t.Errorf("se esperaba Limit=20, se obtuvo %d", mockRepo.lastFilter.Limit)
	}
	if mockRepo.lastFilter.SortBy != "precio" {
		t.Errorf("se esperaba SortBy=precio, se obtuvo %s", mockRepo.lastFilter.SortBy)
	}
	if mockRepo.lastFilter.Order != "asc" {
		t.Errorf("se esperaba Order=asc, se obtuvo %s", mockRepo.lastFilter.Order)
	}

	if res.Paginacion.TotalRegistros != 45 {
		t.Errorf("se esperaba TotalRegistros=45, se obtuvo %d", res.Paginacion.TotalRegistros)
	}
	if res.Paginacion.TotalPaginas != 3 {
		t.Errorf("se esperaba TotalPaginas=3, se obtuvo %d", res.Paginacion.TotalPaginas)
	}
	if res.Paginacion.PaginaActual != 1 {
		t.Errorf("se esperaba PaginaActual=1, se obtuvo %d", res.Paginacion.PaginaActual)
	}
}

func TestProductoService_SanitizacionYLimites(t *testing.T) {
	mockRepo := &mockProductoRepository{
		items: []domain.ProductoDTO{},
		total: 0,
	}

	service := services.NewProductoService(mockRepo)

	filtro := domain.FiltroProductosDTO{
		Page:         -5,
		Limit:        500,
		Categoria:    "Lácteos'; DROP TABLE usuarios;--",
		Supermercado: "Jumbo <script>",
		Marca:        "Colun@#$%",
		SortBy:       "invalido",
		Order:        "DESC",
	}

	_, err := service.ObtenerCatalogo(context.Background(), filtro)
	if err != nil {
		t.Fatalf("se esperaba nil error, se obtuvo: %v", err)
	}

	if mockRepo.lastFilter.Page != 1 {
		t.Errorf("se esperaba Page=1 al recibir valor negativo, se obtuvo %d", mockRepo.lastFilter.Page)
	}
	if mockRepo.lastFilter.Limit != 100 {
		t.Errorf("se esperaba Limit acotado a 100, se obtuvo %d", mockRepo.lastFilter.Limit)
	}
	if mockRepo.lastFilter.SortBy != "precio" {
		t.Errorf("se esperaba SortBy por defecto ante valor no permitido, se obtuvo %s", mockRepo.lastFilter.SortBy)
	}
	if mockRepo.lastFilter.Order != "desc" {
		t.Errorf("se esperaba Order=desc, se obtuvo %s", mockRepo.lastFilter.Order)
	}
}

func TestProductoService_Busqueda_Minimo3Caracteres(t *testing.T) {
	mockRepo := &mockProductoRepository{
		items: []domain.ProductoDTO{
			{ID: "p1", Nombre: "Leche Colun", Precio: 1000},
		},
		total: 1,
	}

	service := services.NewProductoService(mockRepo)

	casosError := []struct {
		nombre string
		query  string
	}{
		{"2 letras", "le"},
		{"1 letra", "a"},
		{"espacios con 2 letras", "  ab  "},
		{"caracteres especiales sanitizados que quedan cortos", "!@#ab$$%"},
	}

	for _, tc := range casosError {
		t.Run(tc.nombre, func(t *testing.T) {
			_, err := service.ObtenerCatalogo(context.Background(), domain.FiltroProductosDTO{Query: tc.query})
			if !errors.Is(err, services.ErrBusquedaCorta) {
				t.Errorf("para '%s' se esperaba ErrBusquedaCorta, se obtuvo %v", tc.query, err)
			}
		})
	}

	t.Run("3 letras validas", func(t *testing.T) {
		res, err := service.ObtenerCatalogo(context.Background(), domain.FiltroProductosDTO{Query: "lec"})
		if err != nil {
			t.Fatalf("se esperaba exito, se obtuvo error: %v", err)
		}
		if res.TotalRegistros != 1 {
			t.Errorf("se esperaba 1 registro, se obtuvieron %d", res.TotalRegistros)
		}
		if mockRepo.lastFilter.Query != "lec" {
			t.Errorf("se esperaba query 'lec', se obtuvo '%s'", mockRepo.lastFilter.Query)
		}
	})
}

func TestProductoService_ObtenerPorID(t *testing.T) {
	fecha := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	oferta := 990.0
	mockRepo := &mockProductoRepository{
		detalle: &domain.ProductoDetalleDTO{
			ProductoDTO: domain.ProductoDTO{
				ID:           "test-id-123",
				Nombre:       "Leche Colun",
				Precio:       990,
				PrecioNormal: 1290,
				PrecioOferta: &oferta,
				EnOferta:     true,
			},
			Historial: []domain.HistorialPrecioDTO{
				{
					PrecioNormal: 1290,
					PrecioOferta: &oferta,
					CapturadoEl:  fecha,
				},
			},
		},
	}

	service := services.NewProductoService(mockRepo)

	t.Run("Exitoso", func(t *testing.T) {
		res, err := service.ObtenerPorID(context.Background(), "test-id-123")
		if err != nil {
			t.Fatalf("se esperaba nil error, se obtuvo %v", err)
		}
		if res.ID != "test-id-123" {
			t.Errorf("se esperaba ID test-id-123, se obtuvo %s", res.ID)
		}
		if len(res.Historial) != 1 {
			t.Errorf("se esperaba 1 registro histórico, se obtuvieron %d", len(res.Historial))
		}
	})

	t.Run("No encontrado", func(t *testing.T) {
		_, err := service.ObtenerPorID(context.Background(), "inexistente")
		if !errors.Is(err, services.ErrProductoNoEncontrado) {
			t.Errorf("se esperaba ErrProductoNoEncontrado, se obtuvo %v", err)
		}
	})

	t.Run("ID vacio", func(t *testing.T) {
		_, err := service.ObtenerPorID(context.Background(), "   ")
		if !errors.Is(err, services.ErrProductoNoEncontrado) {
			t.Errorf("se esperaba ErrProductoNoEncontrado para id vacio, se obtuvo %v", err)
		}
	})
}

func TestProductoService_ObtenerCatalogoAdmin(t *testing.T) {
	stock := true
	oferta := 990
	fecha := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	mockRepo := &mockProductoRepository{
		adminItems: []domain.ProductoAdminDTO{
			{
				ID:                 "admin-prod-1",
				SKU:                "SKU12345",
				Nombre:             "Leche Entera",
				Marca:              "Colun",
				Categoria:          "Lácteos",
				Supermercado:       "Lider",
				PrecioNormal:       1290,
				PrecioOferta:       &oferta,
				EnOferta:           true,
				URLImagen:          "https://img.ejemplo.com/leche.jpg",
				Unidad:             "1 L",
				EnStock:            true,
				UltimaExtraccionEl: fecha,
				SucursalID:         10,
			},
		},
		total: 50,
	}

	service := services.NewProductoService(mockRepo)

	t.Run("Valores por defecto y cálculo de paginación", func(t *testing.T) {
		res, err := service.ObtenerCatalogoAdmin(context.Background(), domain.FiltroProductosDTO{})
		if err != nil {
			t.Fatalf("se esperaba nil error, se obtuvo: %v", err)
		}
		if mockRepo.lastFilter.Page != 1 {
			t.Errorf("se esperaba Page=1, se obtuvo %d", mockRepo.lastFilter.Page)
		}
		if mockRepo.lastFilter.Limit != 20 {
			t.Errorf("se esperaba Limit=20, se obtuvo %d", mockRepo.lastFilter.Limit)
		}
		if res.Paginacion.TotalPaginas != 3 {
			t.Errorf("se esperaba 3 páginas para 50 registros, se obtuvo %d", res.Paginacion.TotalPaginas)
		}
		if len(res.Data) != 1 {
			t.Fatalf("se esperaba 1 producto admin, se obtuvieron %d", len(res.Data))
		}
		if res.Data[0].SKU != "SKU12345" {
			t.Errorf("se esperaba SKU 'SKU12345', se obtuvo '%s'", res.Data[0].SKU)
		}
		if res.Data[0].SucursalID != 10 {
			t.Errorf("se esperaba SucursalID 10, se obtuvo %d", res.Data[0].SucursalID)
		}
	})

	t.Run("Sanitización de SKU y filtros", func(t *testing.T) {
		filtro := domain.FiltroProductosDTO{
			SKU:     "SKU-ABC-123!@#$%",
			EnStock: &stock,
			SortBy:  "sku",
			Order:   "DESC",
		}
		_, err := service.ObtenerCatalogoAdmin(context.Background(), filtro)
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if mockRepo.lastFilter.SKU != "SKU-ABC-123" {
			t.Errorf("se esperaba SKU sanitizado 'SKU-ABC-123', se obtuvo '%s'", mockRepo.lastFilter.SKU)
		}
		if mockRepo.lastFilter.EnStock == nil || !*mockRepo.lastFilter.EnStock {
			t.Errorf("se esperaba EnStock=true")
		}
		if mockRepo.lastFilter.SortBy != "sku" {
			t.Errorf("se esperaba SortBy=sku, se obtuvo %s", mockRepo.lastFilter.SortBy)
		}
	})
}
