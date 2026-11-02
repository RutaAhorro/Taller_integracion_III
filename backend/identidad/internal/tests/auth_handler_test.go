package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestAuthHandler_RegistrarUsuario_AutoLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newMockUsuarioRepository()
	authService := services.NewAuthService(repo)
	handler := handlers.NewAuthHandler(authService)

	router := gin.New()
	router.POST("/api/v1/auth/register", handler.RegistrarUsuario)

	payload := handlers.RegistroRequest{
		Correo:         "autologin@uct.cl",
		Password:       "AutoLogin2026!",
		NombreCompleto: "Usuario AutoLogin",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Esperaba HTTP 201, obtuve %d. Body: %s", w.Code, w.Body.String())
	}

	// Verificar inyección de cookie HttpOnly en el registro
	cookies := w.Result().Cookies()
	var jwtCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "jwt" {
			jwtCookie = c
			break
		}
	}

	if jwtCookie == nil {
		t.Fatalf("Esperaba que se inyectara la cookie 'jwt' en el registro para auto-login")
	}

	if !jwtCookie.HttpOnly {
		t.Errorf("La cookie 'jwt' debe ser HttpOnly")
	}

	if jwtCookie.Value == "" {
		t.Errorf("El valor de la cookie 'jwt' no debe estar vacío")
	}
}

func TestAuthHandler_LoginNormalizaRolLegacy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newMockUsuarioRepository()
	hash, err := security.HashPassword("AliasLogin2026!")
	if err != nil {
		t.Fatal(err)
	}
	userID := uuid.New()
	repo.usuarios["alias@uct.cl"] = &domain.Usuario{
		ID: userID, Correo: "alias@uct.cl", NombreCompleto: "Alias",
		PasswordHash: &hash, Rol: domain.RolAdminLegacy, EstaActivo: true,
	}

	handler := handlers.NewAuthHandler(services.NewAuthService(repo))
	router := gin.New()
	router.POST("/api/v1/auth/login", handler.LoginUsuario)

	body, _ := json.Marshal(handlers.LoginRequest{Correo: "alias@uct.cl", Password: "AliasLogin2026!"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Esperaba HTTP 200, obtuve %d. Body: %s", w.Code, w.Body.String())
	}
	var res handlers.LoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("Respuesta inválida: %v", err)
	}
	if res.Rol != domain.RolAdmin {
		t.Fatalf("La respuesta debía normalizar el rol a admin, obtuve %q", res.Rol)
	}

	var jwtCookie *http.Cookie
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "jwt" {
			jwtCookie = cookie
			break
		}
	}
	if jwtCookie == nil {
		t.Fatal("El login debía emitir la cookie jwt")
	}
	token, err := security.ValidarToken(jwtCookie.Value)
	if err != nil {
		t.Fatalf("Token inválido: %v", err)
	}
	claims, ok := token.Claims.(*security.JWTClaims)
	if !ok || claims.Rol != domain.RolAdmin {
		t.Fatalf("El JWT debía llevar el rol admin, obtuve %+v", token.Claims)
	}
}

func TestAuthHandler_ActualizarPerfil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newMockUsuarioRepository()
	authService := services.NewAuthService(repo)
	handler := handlers.NewAuthHandler(authService)

	userID := uuid.New()
	creado, err := authService.Registrar(t.Context(), services.RegistroDTO{
		Correo:         "perfil@uct.cl",
		Password:       "ClavePerfil2026!",
		NombreCompleto: "Perfil Original",
	})
	if err != nil {
		t.Fatalf("Error preparando usuario: %v", err)
	}
	userID = creado.ID

	router := gin.New()
	// Middleware simulado de autenticación que inyecta user_id
	router.PUT("/api/v1/auth/me", func(c *gin.Context) {
		authHeader := c.GetHeader("X-Test-User-ID")
		if authHeader != "" {
			c.Set("user_id", authHeader)
			c.Set("rol", "registrado")
		}
		c.Next()
	}, handler.ActualizarPerfil)

	t.Run("Actualizacion exitosa de nombre y telefono", func(t *testing.T) {
		nuevoNombre := "Nuevo Nombre Matus"
		nuevoTel := "+56999998888"
		body, _ := json.Marshal(domain.ActualizarPerfilDTO{
			NombreCompleto: &nuevoNombre,
			Telefono:       &nuevoTel,
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/auth/me", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", userID.String())
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Esperaba HTTP 200, obtuve %d. Body: %s", w.Code, w.Body.String())
		}

		var res domain.PerfilUsuarioDTO
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("Error parseando respuesta JSON: %v", err)
		}

		if res.NombreCompleto != nuevoNombre {
			t.Errorf("Esperaba nombre %s, obtuve %s", nuevoNombre, res.NombreCompleto)
		}
		if res.Telefono == nil || *res.Telefono != nuevoTel {
			t.Errorf("Esperaba telefono %s, obtuve %v", nuevoTel, res.Telefono)
		}
	})

	t.Run("401 Unauthorized cuando falta user_id en contexto (sin sesion)", func(t *testing.T) {
		nuevoNombre := "No Permitido"
		body, _ := json.Marshal(domain.ActualizarPerfilDTO{
			NombreCompleto: &nuevoNombre,
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/auth/me", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Esperaba HTTP 401, obtuve %d", w.Code)
		}
	})
}

func TestAuthHandler_CerrarSesion_ExpiraCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := handlers.NewAuthHandler(services.NewAuthService(newMockUsuarioRepository()), true)
	router := gin.New()
	router.POST("/api/v1/auth/logout", handler.CerrarSesion)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "jwt", Value: "token-anterior"})
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("Esperaba HTTP 204, obtuve %d", w.Code)
	}
	var jwtCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "jwt" {
			jwtCookie = c
		}
	}
	if jwtCookie == nil {
		t.Fatal("Esperaba una cookie 'jwt' de expiración en la respuesta")
	}
	if jwtCookie.Value != "" || jwtCookie.MaxAge >= 0 {
		t.Errorf("La cookie debe quedar vacía y expirada; obtuve valor=%q MaxAge=%d", jwtCookie.Value, jwtCookie.MaxAge)
	}
	if !jwtCookie.HttpOnly || !jwtCookie.Secure || jwtCookie.Path != "/" {
		t.Errorf("La cookie de expiración debe conservar HttpOnly, Secure y Path=/ para reemplazar a la original")
	}
}
