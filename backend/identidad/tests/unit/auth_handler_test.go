package unit_test

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

func TestAuthHandler_RegistrarUsuario_AutoLogin(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newMockUsuarioRepository()
	authService := nuevoServicioAuth(repo)
	handler := handlers.NewAuthHandler(authService, false, claveJWTDePrueba)

	router := gin.New()
	router.POST("/api/v1/auth/register", handler.RegistrarUsuario)

	payload := handlers.RegistroRequest{
		Correo:         "autologin@uct.cl",
		Password:       "AutoLogin2026!",
		NombreCompleto: "Usuario AutoLogin",
	}
	body, _ := json.Marshal(payload)

	escritor := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(escritor, req)

	if escritor.Code != http.StatusCreated {
		prueba.Fatalf("Esperaba HTTP 201, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
	}

	// Verificar inyección de cookie HttpOnly en el registro
	cookies := escritor.Result().Cookies()
	var jwtCookie *http.Cookie
	for _, contexto := range cookies {
		if contexto.Name == "jwt" {
			jwtCookie = contexto
			break
		}
	}

	if jwtCookie == nil {
		prueba.Fatalf("Esperaba que se inyectara la cookie 'jwt' en el registro para auto-login")
	}

	if !jwtCookie.HttpOnly {
		prueba.Errorf("La cookie 'jwt' debe ser HttpOnly")
	}

	if jwtCookie.Value == "" {
		prueba.Errorf("El valor de la cookie 'jwt' no debe estar vacío")
	}
}

func TestAuthHandler_LoginEmiteRolCanonico(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newMockUsuarioRepository()
	hash, err := security.HashPassword("AliasLogin2026!")
	if err != nil {
		prueba.Fatal(err)
	}
	userID := uuid.New()
	repo.usuarios["alias@uct.cl"] = &domain.Usuario{
		ID: userID, Correo: "alias@uct.cl", NombreCompleto: "Alias",
		PasswordHash: &hash, Rol: " Admin ", EstaActivo: true,
	}

	handler := handlers.NewAuthHandler(nuevoServicioAuth(repo), false, claveJWTDePrueba)
	router := gin.New()
	router.POST("/api/v1/auth/login", handler.LoginUsuario)

	body, _ := json.Marshal(handlers.LoginRequest{Correo: "alias@uct.cl", Password: "AliasLogin2026!"})
	escritor := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(escritor, req)

	if escritor.Code != http.StatusOK {
		prueba.Fatalf("Esperaba HTTP 200, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	var res handlers.LoginResponse
	if err := json.Unmarshal(escritor.Body.Bytes(), &res); err != nil {
		prueba.Fatalf("Respuesta inválida: %v", err)
	}
	if res.Rol != domain.RolAdmin {
		prueba.Fatalf("La respuesta debía normalizar el rol a admin, obtuve %q", res.Rol)
	}

	var jwtCookie *http.Cookie
	for _, cookie := range escritor.Result().Cookies() {
		if cookie.Name == "jwt" {
			jwtCookie = cookie
			break
		}
	}
	if jwtCookie == nil {
		prueba.Fatal("El login debía emitir la cookie jwt")
	}
	token, err := security.ValidarToken(jwtCookie.Value, claveJWTDePrueba)
	if err != nil {
		prueba.Fatalf("Token inválido: %v", err)
	}
	claims, ok := token.Claims.(*security.JWTClaims)
	if !ok || claims.Rol != domain.RolAdmin {
		prueba.Fatalf("El JWT debía llevar el rol admin, obtuve %+v", token.Claims)
	}
}

func TestAuthHandler_ActualizarPerfil(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newMockUsuarioRepository()
	authService := nuevoServicioAuth(repo)
	handler := handlers.NewAuthHandler(authService, false, claveJWTDePrueba)

	userID := uuid.New()
	creado, err := authService.Registrar(prueba.Context(), services.RegistroDTO{
		Correo:         "perfil@uct.cl",
		Password:       "ClavePerfil2026!",
		NombreCompleto: "Perfil Original",
	})
	if err != nil {
		prueba.Fatalf("Error preparando usuario: %v", err)
	}
	userID = creado.ID

	router := gin.New()
	// Middleware simulado de autenticación que inyecta user_id
	router.PUT("/api/v1/auth/me", func(contexto *gin.Context) {
		authHeader := contexto.GetHeader("X-Test-User-ID")
		if authHeader != "" {
			contexto.Set("user_id", authHeader)
			contexto.Set("rol", "registrado")
		}
		contexto.Next()
	}, handler.ActualizarPerfil)

	prueba.Run("Actualizacion exitosa de nombre y telefono", func(prueba *testing.T) {
		nuevoNombre := "Nuevo Nombre Matus"
		nuevoTel := "+56999998888"
		body, _ := json.Marshal(domain.ActualizarPerfilDTO{
			NombreCompleto: &nuevoNombre,
			Telefono:       &nuevoTel,
		})

		escritor := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/auth/me", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", userID.String())
		router.ServeHTTP(escritor, req)

		if escritor.Code != http.StatusOK {
			prueba.Fatalf("Esperaba HTTP 200, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
		}

		var res domain.PerfilUsuarioDTO
		if err := json.Unmarshal(escritor.Body.Bytes(), &res); err != nil {
			prueba.Fatalf("Error parseando respuesta JSON: %v", err)
		}

		if res.NombreCompleto != nuevoNombre {
			prueba.Errorf("Esperaba nombre %s, obtuve %s", nuevoNombre, res.NombreCompleto)
		}
		if res.Telefono == nil || *res.Telefono != nuevoTel {
			prueba.Errorf("Esperaba telefono %s, obtuve %v", nuevoTel, res.Telefono)
		}
	})

	prueba.Run("401 Unauthorized cuando falta user_id en contexto (sin sesion)", func(prueba *testing.T) {
		nuevoNombre := "No Permitido"
		body, _ := json.Marshal(domain.ActualizarPerfilDTO{
			NombreCompleto: &nuevoNombre,
		})

		escritor := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/auth/me", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(escritor, req)

		if escritor.Code != http.StatusUnauthorized {
			prueba.Fatalf("Esperaba HTTP 401, obtuve %d", escritor.Code)
		}
	})
}

func TestAuthHandler_CerrarSesion_ExpiraCookie(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := handlers.NewAuthHandler(nuevoServicioAuth(newMockUsuarioRepository()), true, claveJWTDePrueba)
	router := gin.New()
	router.POST("/api/v1/auth/logout", handler.CerrarSesion)

	escritor := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "jwt", Value: "token-anterior"})
	router.ServeHTTP(escritor, req)

	if escritor.Code != http.StatusNoContent {
		prueba.Fatalf("Esperaba HTTP 204, obtuve %d", escritor.Code)
	}
	var jwtCookie *http.Cookie
	for _, contexto := range escritor.Result().Cookies() {
		if contexto.Name == "jwt" {
			jwtCookie = contexto
		}
	}
	if jwtCookie == nil {
		prueba.Fatal("Esperaba una cookie 'jwt' de expiración en la respuesta")
	}
	if jwtCookie.Value != "" || jwtCookie.MaxAge >= 0 {
		prueba.Errorf("La cookie debe quedar vacía y expirada; obtuve valor=%q MaxAge=%d", jwtCookie.Value, jwtCookie.MaxAge)
	}
	if !jwtCookie.HttpOnly || !jwtCookie.Secure || jwtCookie.Path != "/" {
		prueba.Errorf("La cookie de expiración debe conservar HttpOnly, Secure y Path=/ para reemplazar a la original")
	}
}
