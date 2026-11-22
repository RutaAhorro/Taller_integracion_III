// Pruebas de los límites por defecto de internal/config aplicados por el router:
// tope de productos por lote y plazo de readiness.
package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/config"
)

// ===== Ingesta =====

// TestIngestaRespetaElTopeDeProductos acepta config.ProductosPorLotePredeterminados y
// rechaza uno más sin llegar al repositorio.
func TestIngestaRespetaElTopeDeProductos(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	producto := `{"sku":"SKU","producto":"Leche","precio_normal":1000}`
	for _, caso := range []struct {
		nombre               string
		cantidad             int
		estado, llamadasRepo int
	}{
		{"dentro del tope", config.ProductosPorLotePredeterminados, http.StatusOK, 1},
		{"sobre el tope", config.ProductosPorLotePredeterminados + 1, http.StatusBadRequest, 0},
	} {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			almacen := &testStore{}
			enrutador := nuevoEnrutadorDePrueba(api.Dependencias{Ingesta: almacen, TokenIngesta: token})
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

// ===== Readiness =====

// TestReadinessUsaElPlazoConfigurado corta un ping bloqueado en config.PlazoReadinessPredeterminado.
func TestReadinessUsaElPlazoConfigurado(prueba *testing.T) {
	pingBloqueado := func(contexto context.Context) error {
		<-contexto.Done()
		return contexto.Err()
	}
	enrutador := nuevoEnrutadorDePrueba(api.Dependencias{SondaBase: pingBloqueado})
	inicio := time.Now()
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, httptest.NewRequest(http.MethodGet, "/_catalogo/ready", nil))
	duracion := time.Since(inicio)
	if respuesta.Code != http.StatusServiceUnavailable {
		prueba.Fatalf("estado=%d cuerpo=%s", respuesta.Code, respuesta.Body.String())
	}
	if duracion < config.PlazoReadinessPredeterminado || duracion > config.PlazoReadinessPredeterminado+time.Second {
		prueba.Fatalf("readiness no respetó el plazo de %s: tardó %s", config.PlazoReadinessPredeterminado, duracion)
	}
}
