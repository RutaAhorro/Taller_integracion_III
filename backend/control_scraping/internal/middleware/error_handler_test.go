package middleware

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestInternalDiagnosticsAreNotReturned(t *testing.T) {
	for _, path := range []string{"/panic", "/context", "/internal"} {
		t.Run(path, func(t *testing.T) {
			r := gin.New()
			r.Use(ErrorHandler())
			r.GET("/panic", func(c *gin.Context) { panic("private database relation and secret") })
			r.GET("/context", func(c *gin.Context) { _ = c.Error(errors.New("private database relation and secret")) })
			r.GET("/internal", func(c *gin.Context) {
				ResponderError(c, 500, "Error interno.", errors.New("private database relation and secret"))
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			if w.Code != 500 || strings.Contains(w.Body.String(), "detalle") || strings.Contains(w.Body.String(), "private") {
				t.Fatal(w.Code, w.Body)
			}
		})
	}
}
