// Autorización por rol sobre la sesión revalidada.

package unit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/middleware"
	"github.com/gin-gonic/gin"
)

func TestRequireRole(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	casos := []struct {
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

	for _, caso := range casos {
		prueba.Run(caso.name, func(prueba *testing.T) {
			enrutador := gin.New()
			enrutador.Use(func(contexto *gin.Context) {
				if caso.setContext {
					contexto.Set("rol", caso.rolesContexto)
				}
				contexto.Next()
			})
			enrutador.GET("/test-protected", middleware.RequireRole(caso.rolesRequerido...), func(contexto *gin.Context) {
				contexto.JSON(http.StatusOK, gin.H{"mensaje": "acceso concedido"})
			})

			peticionHTTP, _ := http.NewRequest(http.MethodGet, "/test-protected", nil)
			escritor := httptest.NewRecorder()
			enrutador.ServeHTTP(escritor, peticionHTTP)

			if escritor.Code != caso.expectedStatus {
				prueba.Errorf("se esperaba status %d, se obtuvo %d. Body: %s", caso.expectedStatus, escritor.Code, escritor.Body.String())
			}
		})
	}
}
