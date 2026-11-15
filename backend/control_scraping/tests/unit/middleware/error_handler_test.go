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
	for _, path := range []string{"/panic", "/context", "/internal"} {
		prueba.Run(path, func(prueba *testing.T) {
			router := gin.New()
			router.Use(middleware.ErrorHandler())
			router.GET("/panic", func(contexto *gin.Context) { panic("private database relation and secret") })
			router.GET("/context", func(contexto *gin.Context) { _ = contexto.Error(errors.New("private database relation and secret")) })
			router.GET("/internal", func(contexto *gin.Context) {
				middleware.ResponderError(contexto, 500, "Error interno.", errors.New("private database relation and secret"))
			})
			respuestaHTTP := httptest.NewRecorder()
			router.ServeHTTP(respuestaHTTP, httptest.NewRequest("GET", path, nil))
			if respuestaHTTP.Code != 500 || strings.Contains(respuestaHTTP.Body.String(), "detalle") || strings.Contains(respuestaHTTP.Body.String(), "private") {
				prueba.Fatal(respuestaHTTP.Code, respuestaHTTP.Body)
			}
		})
	}
}
