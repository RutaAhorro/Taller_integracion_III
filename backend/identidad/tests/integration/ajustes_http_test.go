// Verifica los límites configurados desde el router público, sin infraestructura externa.
package integration_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/gin-gonic/gin"
)

// TestRouterAplicaLimitesConfigurados comprueba tamaño de cuerpo y cancelación.
func TestRouterAplicaLimitesConfigurados(prueba *testing.T) {
	ajustes := config.AjustesPredeterminados()
	ajustes.MaxBodyBytes = 8
	ajustes.RequestTimeout = 25 * time.Millisecond
	enrutador := enrutadorDePrueba(prueba, config.Config{Ajustes: ajustes}, nil)
	enrutador.POST("/limites", func(contexto *gin.Context) {
		cuerpo, errorLectura := io.ReadAll(contexto.Request.Body)
		var exceso *http.MaxBytesError
		if !errors.As(errorLectura, &exceso) || len(cuerpo) != ajustes.MaxBodyBytes {
			prueba.Error("no se aplicó el límite de cuerpo configurado")
		}
		<-contexto.Request.Context().Done()
		if !errors.Is(contexto.Request.Context().Err(), context.DeadlineExceeded) {
			prueba.Error("la petición no terminó por el plazo configurado")
		}
		contexto.Status(http.StatusNoContent)
	})
	respuesta := httptest.NewRecorder()
	inicio := time.Now()
	enrutador.ServeHTTP(respuesta, httptest.NewRequest(http.MethodPost, "/limites", strings.NewReader("123456789")))
	if respuesta.Code != http.StatusNoContent || time.Since(inicio) > time.Second {
		prueba.Fatal("el router no respetó el plazo de la petición")
	}
}

// TestReadinessUsaSuPlazo comprueba que la sonda tiene su presupuesto propio.
func TestReadinessUsaSuPlazo(prueba *testing.T) {
	ajustes := config.AjustesPredeterminados()
	ajustes.ReadinessTimeout = 20 * time.Millisecond
	enrutador := enrutadorDePrueba(prueba, config.Config{Ajustes: ajustes}, func(contexto context.Context) error {
		<-contexto.Done()
		return contexto.Err()
	})
	respuesta := httptest.NewRecorder()
	inicio := time.Now()
	enrutador.ServeHTTP(respuesta, httptest.NewRequest(http.MethodGet, "/_identidad/ready", nil))
	if respuesta.Code != http.StatusServiceUnavailable || time.Since(inicio) > time.Second {
		prueba.Fatal("la sonda no respetó su presupuesto configurado")
	}
}
