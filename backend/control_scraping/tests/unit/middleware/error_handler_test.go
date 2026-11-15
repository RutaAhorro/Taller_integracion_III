package middleware_test

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/middleware"
	"github.com/gin-gonic/gin"
)

func TestInternalDiagnosticsAreNotReturned(t *testing.T) {
	for _, path := range []string{"/panic", "/context", "/internal"} {
		t.Run(path, func(t *testing.T) {
			r := gin.New()
			r.Use(middleware.ErrorHandler())
			r.GET("/panic", func(contexto *gin.Context) { panic("private database relation and secret") })
			r.GET("/context", func(contexto *gin.Context) { _ = contexto.Error(errors.New("private database relation and secret")) })
			r.GET("/internal", func(contexto *gin.Context) {
				middleware.ResponderError(contexto, 500, "Error interno.", errors.New("private database relation and secret"))
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			if w.Code != 500 || strings.Contains(w.Body.String(), "detalle") || strings.Contains(w.Body.String(), "private") {
				t.Fatal(w.Code, w.Body)
			}
		})
	}
}
