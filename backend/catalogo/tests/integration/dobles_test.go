package integration_test

import (
	"context"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/config"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
)

type mockProductoRepository struct {
	items      []domain.ProductoDTO
	adminItems []domain.ProductoAdminDTO
	total      int64
	err        error
	lastFilter domain.FiltroProductosDTO
	detalle    *domain.ProductoDetalleDTO
}

func (repositorio *mockProductoRepository) Listar(contexto context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoDTO, int64, error) {
	repositorio.lastFilter = filtro
	if repositorio.err != nil {
		return nil, 0, repositorio.err
	}
	return repositorio.items, repositorio.total, nil
}

func (repositorio *mockProductoRepository) ListarParaAdmin(contexto context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoAdminDTO, int64, error) {
	repositorio.lastFilter = filtro
	if repositorio.err != nil {
		return nil, 0, repositorio.err
	}
	return repositorio.adminItems, repositorio.total, nil
}

func (repositorio *mockProductoRepository) ObtenerPorID(contexto context.Context, identificador string) (*domain.ProductoDetalleDTO, error) {
	if repositorio.err != nil {
		return nil, repositorio.err
	}
	if repositorio.detalle != nil && repositorio.detalle.ID == identificador {
		return repositorio.detalle, nil
	}
	return nil, nil
}

type mockFuenteRepository struct {
	fuentes []domain.FuenteDTO
	err     error
}

func (repositorio *mockFuenteRepository) ListarFuentes(contexto context.Context) ([]domain.FuenteDTO, error) {
	if repositorio.err != nil {
		return nil, repositorio.err
	}
	return repositorio.fuentes, nil
}

func (repositorio *mockFuenteRepository) ActualizarEstadoFuente(contexto context.Context, identificador int, habilitada bool) (*domain.FuenteDTO, error) {
	if repositorio.err != nil {
		return nil, repositorio.err
	}
	for indice := range repositorio.fuentes {
		if repositorio.fuentes[indice].ID == identificador {
			repositorio.fuentes[indice].Habilitada = habilitada
			return &repositorio.fuentes[indice], nil
		}
	}
	return nil, nil
}

// ===== Router de prueba =====

// claveJWTDePrueba firma las cookies de las pruebas HTTP.
const claveJWTDePrueba = "catalog-test-only-key"

// bytesPorCuerpoDePrueba es el tope de cuerpo interno del servicio.
const bytesPorCuerpoDePrueba = config.BytesCuerpoPredeterminados

// limitesProductosDePrueba son los límites por defecto de internal/config.
var limitesProductosDePrueba = services.LimitesProductos{
	TamanoPagina:        config.TamanoPaginaPredeterminado,
	TamanoPaginaMaximo:  config.TamanoPaginaMaximoPredeterminado,
	LargoMaximoBusqueda: config.LargoBusquedaPredeterminado,
}

// nuevoEnrutadorDePrueba arma el router real con dobles para lo que la prueba
// no define: repositorios vacíos, base disponible, límites por defecto y logs
// descartados.
func nuevoEnrutadorDePrueba(dependencias api.Dependencias) *gin.Engine {
	gin.SetMode(gin.TestMode)
	if dependencias.Productos == nil {
		dependencias.Productos = services.NuevoServicioProductos(&mockProductoRepository{}, limitesProductosDePrueba)
	}
	if dependencias.Fuentes == nil {
		dependencias.Fuentes = services.NuevoServicioFuentes(&mockFuenteRepository{})
	}
	if dependencias.SondaBase == nil {
		dependencias.SondaBase = func(context.Context) error { return nil }
	}
	if dependencias.Registrador == nil {
		dependencias.Registrador = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	if dependencias.PlazoReadiness == 0 {
		dependencias.PlazoReadiness = config.PlazoReadinessPredeterminado
	}
	if dependencias.MaximoProductosPorLote == 0 {
		dependencias.MaximoProductosPorLote = config.ProductosPorLotePredeterminados
	}
	if dependencias.MaximoBytesCuerpo == 0 {
		dependencias.MaximoBytesCuerpo = config.BytesCuerpoPredeterminados
	}
	if dependencias.ClaveJWT == "" {
		dependencias.ClaveJWT = claveJWTDePrueba
	}
	return api.NuevoEnrutador(dependencias)
}

// cookieAdministrador es una sesión admin vigente firmada con la clave de prueba.
func cookieAdministrador(prueba *testing.T) *http.Cookie {
	prueba.Helper()
	vence := time.Now().Add(time.Hour)
	return &http.Cookie{Name: "jwt", Value: signedToken(prueba, "admin", jwt.SigningMethodHS256, claveJWTDePrueba, &vence)}
}
