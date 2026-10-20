package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	httperrors "github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/httperrors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/middleware"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestJWT_GenerarYValidarToken(t *testing.T) {
	usuarioID := "c14bd236-294a-41b8-8cbb-5473bb610415"
	rol := "admin"
	provider := "local"

	tokenStr, err := security.GenerarToken(usuarioID, rol, provider)
	if err != nil {
		t.Fatalf("Error inesperado al generar token: %v", err)
	}

	if tokenStr == "" {
		t.Fatalf("El token generado no debería estar vacío")
	}

	token, err := security.ValidarToken(tokenStr)
	if err != nil {
		t.Fatalf("Fallo en ValidarToken: %v", err)
	}

	if !token.Valid {
		t.Errorf("El token validado debería ser válido")
	}

	claims, ok := token.Claims.(*security.JWTClaims)
	if !ok {
		t.Fatalf("No se pudo convertir claims a *security.JWTClaims")
	}

	if claims.UserID != usuarioID {
		t.Errorf("UserID esperado %s, obtenido %s", usuarioID, claims.UserID)
	}
	if claims.Rol != rol {
		t.Errorf("Rol esperado %s, obtenido %s", rol, claims.Rol)
	}
	if claims.Provider != provider {
		t.Errorf("Provider esperado %s, obtenido %s", provider, claims.Provider)
	}
}

func TestJWT_TokenInvalido(t *testing.T) {
	_, err := security.ValidarToken("token.falso.invalido")
	if err == nil {
		t.Errorf("Se esperaba error al validar token falso, pero se aceptó")
	}
}

func TestMiddleware_RequireAuth(t *testing.T) {
	r := gin.New()
	r.Use(httperrors.ErrorHandler())

	r.GET("/protegido", middleware.RequireAuth(), func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		rol, _ := c.Get("rol")
		provider, _ := c.Get("provider")

		c.JSON(http.StatusOK, gin.H{
			"user_id":  userID,
			"rol":      rol,
			"provider": provider,
		})
	})

	t.Run("Sin cookie jwt", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protegido", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Esperaba 401, obtuve %d", w.Code)
		}
	})

	t.Run("Cookie con token adulterado", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protegido", nil)
		req.AddCookie(&http.Cookie{
			Name:  "jwt",
			Value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.invalidsignature",
		})
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Esperaba 401, obtuve %d", w.Code)
		}
	})

	t.Run("Cookie jwt valida exitosa", func(t *testing.T) {
		tokenStr, err := security.GenerarToken("usr-123", "colaborador", "google")
		if err != nil {
			t.Fatalf("Error generando token: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protegido", nil)
		req.AddCookie(&http.Cookie{
			Name:  "jwt",
			Value: tokenStr,
		})
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Esperaba 200, obtuve %d", w.Code)
		}

		var res map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &res)

		if res["user_id"] != "usr-123" || res["rol"] != "colaborador" || res["provider"] != "google" {
			t.Errorf("Claims devueltos no coinciden con los inyectados: %v", res)
		}
	})
}

func TestMiddleware_RequireAuthRevalidaEstadoYRolEnBaseDeDatos(t *testing.T) {
	repo := newMockUsuarioRepository()
	usuarioID := uuid.New()
	repo.usuarios["admin@uct.cl"] = &domain.Usuario{
		ID: usuarioID, Correo: "admin@uct.cl", NombreCompleto: "Admin",
		Rol: domain.RolAdminLegacy, EstaActivo: true,
	}

	r := gin.New()
	r.Use(httperrors.ErrorHandler())
	r.GET("/protegido", middleware.RequireAuth(repo), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id": c.GetString("user_id"),
			"rol":     c.GetString("rol"),
		})
	})

	// El JWT dice "registrado", pero la base de datos manda: rol admin.
	tokenStr, err := security.GenerarToken(usuarioID.String(), domain.RolRegistrado, "local")
	if err != nil {
		t.Fatalf("Error generando token: %v", err)
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protegido", nil)
	req.AddCookie(&http.Cookie{Name: "jwt", Value: tokenStr})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Esperaba 200, obtuve %d. Body: %s", w.Code, w.Body.String())
	}
	var res map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("Respuesta inválida: %v", err)
	}
	if res["rol"] != domain.RolAdmin {
		t.Fatalf("El rol debía normalizarse desde la base de datos a admin, obtuve %v", res["rol"])
	}

	// Desactivar la cuenta invalida el token en la siguiente petición.
	repo.usuarios["admin@uct.cl"].EstaActivo = false
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/protegido", nil)
	req.AddCookie(&http.Cookie{Name: "jwt", Value: tokenStr})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Una cuenta desactivada debía recibir 401, obtuve %d. Body: %s", w.Code, w.Body.String())
	}

	// Un token válido para un usuario inexistente tampoco sirve.
	tokenInexistente, err := security.GenerarToken(uuid.NewString(), domain.RolAdmin, "local")
	if err != nil {
		t.Fatalf("Error generando token: %v", err)
	}
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/protegido", nil)
	req.AddCookie(&http.Cookie{Name: "jwt", Value: tokenInexistente})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Un usuario inexistente debía recibir 401, obtuve %d. Body: %s", w.Code, w.Body.String())
	}
}

func init() {
	if os.Getenv("JWT_SECRET") == "" {
		os.Setenv("JWT_SECRET", "test-secret-key-32-characters-long-jwt!")
	}
}
