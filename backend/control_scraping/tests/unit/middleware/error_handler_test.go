package middleware_test

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/middleware"
	"github.com/gin-gonic/gin"
)

func TestInternalDiagnosticsAreNotReturned(prueba *testing.T) {
	for _, ruta := range []string{"/panic", "/context", "/internal"} {
		prueba.Run(ruta, func(prueba *testing.T) {
			enrutador := gin.New()
			enrutador.Use(middleware.ErrorHandler())
			enrutador.GET("/panic", func(contexto *gin.Context) { panic("private database relation and secret") })
			enrutador.GET("/context", func(contexto *gin.Context) { _ = contexto.Error(errors.New("private database relation and secret")) })
			enrutador.GET("/internal", func(contexto *gin.Context) {
				middleware.ResponderError(contexto, 500, "Error interno.", errors.New("private database relation and secret"))
			})
			respuestaHTTP := httptest.NewRecorder()
			enrutador.ServeHTTP(respuestaHTTP, httptest.NewRequest("GET", ruta, nil))
			if respuestaHTTP.Code != 500 || strings.Contains(respuestaHTTP.Body.String(), "detalle") || strings.Contains(respuestaHTTP.Body.String(), "private") {
				prueba.Fatal(respuestaHTTP.Code, respuestaHTTP.Body)
			}
		})
	}
}
