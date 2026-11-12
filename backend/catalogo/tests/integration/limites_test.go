package integration_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/ingesta"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/server"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
)

// TestIngestaRespetaElTopeDeProductosConfigurado verifica CATALOGO_MAX_BATCH_PRODUCTOS.
func TestIngestaRespetaElTopeDeProductosConfigurado(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	token := strings.Repeat("t", 32)
	producto := `{"sku":"SKU","producto":"Leche","precio_normal":1000}`
	for _, caso := range []struct {
		nombre               string
		cantidad             int
		estado, llamadasRepo int
	}{
		{"dentro del tope", 2, http.StatusOK, 1},
		{"sobre el tope", 3, http.StatusBadRequest, 0},
	} {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			almacen := &testStore{}
			enrutador := gin.New()
			ingesta.Register(enrutador, ingesta.Config{Repository: almacen, Token: token, MaxProductos: 2, MaxBodyBytes: bytesPorCuerpoDePrueba}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			productos := strings.TrimSuffix(strings.Repeat(producto+",", caso.cantidad), ",")
			peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/ingestas", strings.NewReader(`{"sucursal_id":1,"productos":[`+productos+`]}`))
			peticion.Header.Set("Authorization", "Bearer "+token)
			respuesta := httptest.NewRecorder()
			enrutador.ServeHTTP(respuesta, peticion)
			if respuesta.Code != caso.estado || almacen.calls != caso.llamadasRepo {
				subprueba.Fatalf("estado=%d llamadas=%d cuerpo=%s", respuesta.Code, almacen.calls, respuesta.Body.String())
			}
		})
	}
}

// TestRegisterRechazaTopeDeProductosNoConfigurado evita arrancar la ingesta sin tope.
func TestRegisterRechazaTopeDeProductosNoConfigurado(prueba *testing.T) {
	defer func() {
		if recover() == nil {
			prueba.Fatal("Register aceptó MaxProductos=0")
		}
	}()
	ingesta.Register(gin.New(), ingesta.Config{Repository: &testStore{}, Token: strings.Repeat("t", 32), MaxBodyBytes: bytesPorCuerpoDePrueba}, nil)
}

// TestIngestaRespetaElTamanoDeCuerpoConfigurado verifica CATALOGO_MAX_BODY_BYTES:
// el mismo lote pasa con un tope holgado y recibe 413 con uno menor que su tamaño.
func TestIngestaRespetaElTamanoDeCuerpoConfigurado(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	token := strings.Repeat("t", 32)
	cuerpo := `{"sucursal_id":1,"productos":[{"sku":"SKU","producto":"Leche","precio_normal":1000}]}`
	for _, caso := range []struct {
		nombre               string
		maximoBytes          int
		estado, llamadasRepo int
	}{
		{"cuerpo dentro del tope", len(cuerpo), http.StatusOK, 1},
		{"cuerpo sobre el tope", len(cuerpo) - 1, http.StatusRequestEntityTooLarge, 0},
	} {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			almacen := &testStore{}
			enrutador := gin.New()
			ingesta.Register(enrutador, ingesta.Config{Repository: almacen, Token: token, MaxProductos: 2, MaxBodyBytes: caso.maximoBytes}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/ingestas", strings.NewReader(cuerpo))
			peticion.Header.Set("Authorization", "Bearer "+token)
			respuesta := httptest.NewRecorder()
			enrutador.ServeHTTP(respuesta, peticion)
			if respuesta.Code != caso.estado || almacen.calls != caso.llamadasRepo {
				subprueba.Fatalf("estado=%d llamadas=%d cuerpo=%s", respuesta.Code, almacen.calls, respuesta.Body.String())
			}
		})
	}
}

// TestRegisterRechazaTamanoDeCuerpoNoConfigurado evita arrancar la ingesta sin tope de bytes.
func TestRegisterRechazaTamanoDeCuerpoNoConfigurado(prueba *testing.T) {
	defer func() {
		if recover() == nil {
			prueba.Fatal("Register aceptó MaxBodyBytes=0")
		}
	}()
	ingesta.Register(gin.New(), ingesta.Config{Repository: &testStore{}, Token: strings.Repeat("t", 32), MaxProductos: 1}, nil)
}

// TestReadinessUsaElPlazoConfigurado verifica CATALOGO_READINESS_TIMEOUT.
func TestReadinessUsaElPlazoConfigurado(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	pingBloqueado := func(contexto context.Context) error {
		<-contexto.Done()
		return contexto.Err()
	}
	enrutador := server.NewRouter(services.NewProductoService(&mockProductoRepository{}, limitesDePrueba),
		services.NewFuenteService(&mockFuenteRepository{}), "catalog-test-only-key", pingBloqueado,
		50*time.Millisecond, slog.New(slog.NewJSONHandler(io.Discard, nil)), nil)
	inicio := time.Now()
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, httptest.NewRequest(http.MethodGet, "/_catalogo/ready", nil))
	duracion := time.Since(inicio)
	if respuesta.Code != http.StatusServiceUnavailable {
		prueba.Fatalf("estado=%d cuerpo=%s", respuesta.Code, respuesta.Body.String())
	}
	if duracion < 50*time.Millisecond || duracion > time.Second {
		prueba.Fatalf("readiness no respetó el plazo de 50ms: tardó %s", duracion)
	}
}
