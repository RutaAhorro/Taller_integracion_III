package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/middleware"
	"github.com/gin-gonic/gin"
)

func TestRequireRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		rolesContexto  string
		setContext     bool
		rolesRequerido []string
		expectedStatus int
	}{
		{
			name:           "Sin rol en contexto retorna 403",
			setContext:     false,
			rolesRequerido: []string{"admin"},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Rol vacio en contexto retorna 403",
			setContext:     true,
			rolesContexto:  "",
			rolesRequerido: []string{"admin"},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Usuario registrado intentando acceder a ruta admin retorna 403",
			setContext:     true,
			rolesContexto:  "registrado",
			rolesRequerido: []string{"admin"},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Usuario admin accediendo a ruta admin retorna 200",
			setContext:     true,
			rolesContexto:  "admin",
			rolesRequerido: []string{"admin"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Rol en mayusculas coincide case-insensitively y retorna 200",
			setContext:     true,
			rolesContexto:  "ADMIN",
			rolesRequerido: []string{"admin"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Usuario con uno de multiples roles permitidos retorna 200",
			setContext:     true,
			rolesContexto:  "superadmin",
			rolesRequerido: []string{"admin", "superadmin"},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				if tt.setContext {
					c.Set("rol", tt.rolesContexto)
				}
				c.Next()
			})
			r.GET("/test-protected", middleware.RequireRole(tt.rolesRequerido...), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"mensaje": "acceso concedido"})
			})

			req, _ := http.NewRequest(http.MethodGet, "/test-protected", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("se esperaba status %d, se obtuvo %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}
