package handlers

import (
	"context"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

type googleErrorService struct {
	services.AuthService
	err error
}

func (s googleErrorService) GoogleLogin(context.Context, string) (*domain.Usuario, error) {
	return nil, s.err
}
func TestGoogleErrorClassificationDoesNotExposeStorage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{{"token rejected", services.ErrCredencialesInvalidas, 401}, {"inactive", services.ErrCuentaInactiva, 401}, {"configuration", services.ErrGoogleNoConfigurado, 503}, {"database", errors.New("private SQL database table and constraint"), 500}} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.POST("/google", NewAuthHandler(googleErrorService{err: tc.err}).GoogleLoginUsuario)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/google", strings.NewReader(`{"id_token":"fixture"}`))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			if w.Code != tc.status || strings.Contains(w.Body.String(), "detalle") || strings.Contains(w.Body.String(), "private") {
				t.Fatal(w.Code, w.Body)
			}
		})
	}
}
