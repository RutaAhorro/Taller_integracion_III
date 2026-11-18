// Handlers de autenticación: cookie de sesión, rol canónico, perfil y logout.

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

	repositorio := newMockUsuarioRepository()
	servicioAuth := nuevoServicioAuth(repositorio)
	manejador := handlers.NewAuthHandler(servicioAuth, false, claveJWTDePrueba)

	enrutador := gin.New()
	enrutador.POST("/api/v1/auth/register", manejador.RegistrarUsuario)

	datosGoogle := handlers.RegistroRequest{
		Correo:         "autologin@uct.cl",
		Password:       "AutoLogin2026!",
		NombreCompleto: "Usuario AutoLogin",
	}
	cuerpo, _ := json.Marshal(datosGoogle)

	escritor := httptest.NewRecorder()
	peticion, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(cuerpo))
	peticion.Header.Set("Content-Type", "application/json")
	enrutador.ServeHTTP(escritor, peticion)

	if escritor.Code != http.StatusCreated {
		prueba.Fatalf("Esperaba HTTP 201, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
	}

	// Verificar inyección de cookie HttpOnly en el registro
	cookiesRespuesta := escritor.Result().Cookies()
	var cookieJWT *http.Cookie
	for _, cookie := range cookiesRespuesta {
		if cookie.Name == "jwt" {
			cookieJWT = cookie
			break
		}
	}

	if cookieJWT == nil {
		prueba.Fatalf("Esperaba que se inyectara la cookie 'jwt' en el registro para auto-login")
	}

	if !cookieJWT.HttpOnly {
		prueba.Errorf("La cookie 'jwt' debe ser HttpOnly")
	}

	if cookieJWT.Value == "" {
		prueba.Errorf("El valor de la cookie 'jwt' no debe estar vacío")
	}
}

func TestAuthHandler_LoginEmiteRolCanonico(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	repositorio := newMockUsuarioRepository()
	hash, errorOperacion := security.HashPassword("AliasLogin2026!")
	if errorOperacion != nil {
		prueba.Fatal(errorOperacion)
	}
	usuarioID := uuid.New()
	repositorio.usuarios["alias@uct.cl"] = &domain.Usuario{
		ID: usuarioID, Correo: "alias@uct.cl", NombreCompleto: "Alias",
		PasswordHash: &hash, Rol: " Admin ", EstaActivo: true,
	}

	manejador := handlers.NewAuthHandler(nuevoServicioAuth(repositorio), false, claveJWTDePrueba)
	enrutador := gin.New()
	enrutador.POST("/api/v1/auth/login", manejador.LoginUsuario)

	cuerpo, _ := json.Marshal(handlers.LoginRequest{Correo: "alias@uct.cl", Password: "AliasLogin2026!"})
	escritor := httptest.NewRecorder()
	peticion, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(cuerpo))
	peticion.Header.Set("Content-Type", "application/json")
	enrutador.ServeHTTP(escritor, peticion)

	if escritor.Code != http.StatusOK {
		prueba.Fatalf("Esperaba HTTP 200, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	var resultado handlers.LoginResponse
	if errorObtenido := json.Unmarshal(escritor.Body.Bytes(), &resultado); errorObtenido != nil {
		prueba.Fatalf("Respuesta inválida: %v", errorObtenido)
	}
	if resultado.Rol != domain.RolAdmin {
		prueba.Fatalf("La respuesta debía normalizar el rol a admin, obtuve %q", resultado.Rol)
	}

	var cookieJWT *http.Cookie
	for _, cookie := range escritor.Result().Cookies() {
		if cookie.Name == "jwt" {
			cookieJWT = cookie
			break
		}
	}
	if cookieJWT == nil {
		prueba.Fatal("El login debía emitir la cookie jwt")
	}
	token, errorOperacion := security.ValidarToken(cookieJWT.Value, claveJWTDePrueba)
	if errorOperacion != nil {
		prueba.Fatalf("Token inválido: %v", errorOperacion)
	}
	datosToken, valido := token.Claims.(*security.JWTClaims)
	if !valido || datosToken.Rol != domain.RolAdmin {
		prueba.Fatalf("El JWT debía llevar el rol admin, obtuve %+v", token.Claims)
	}
}

func TestAuthHandler_ActualizarPerfil(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	repositorio := newMockUsuarioRepository()
	servicioAuth := nuevoServicioAuth(repositorio)
	manejador := handlers.NewAuthHandler(servicioAuth, false, claveJWTDePrueba)

	usuarioID := uuid.New()
	creado, errorObtenido := servicioAuth.Registrar(prueba.Context(), services.RegistroDTO{
		Correo:         "perfil@uct.cl",
		Password:       "ClavePerfil2026!",
		NombreCompleto: "Perfil Original",
	})
	if errorObtenido != nil {
		prueba.Fatalf("Error preparando usuario: %v", errorObtenido)
	}
	usuarioID = creado.ID

	enrutador := gin.New()
	// Middleware simulado de autenticación que inyecta user_id
	enrutador.PUT("/api/v1/auth/me", func(contexto *gin.Context) {
		cabeceraAuth := contexto.GetHeader("X-Test-User-ID")
		if cabeceraAuth != "" {
			contexto.Set("user_id", cabeceraAuth)
			contexto.Set("rol", "registrado")
		}
		contexto.Next()
	}, manejador.ActualizarPerfil)

	prueba.Run("Actualizacion exitosa de nombre y telefono", func(prueba *testing.T) {
		nuevoNombre := "Nuevo Nombre Matus"
		nuevoTel := "+56999998888"
		cuerpo, _ := json.Marshal(domain.ActualizarPerfilDTO{
			NombreCompleto: &nuevoNombre,
			Telefono:       &nuevoTel,
		})

		escritor := httptest.NewRecorder()
		peticion, _ := http.NewRequest(http.MethodPut, "/api/v1/auth/me", bytes.NewBuffer(cuerpo))
		peticion.Header.Set("Content-Type", "application/json")
		peticion.Header.Set("X-Test-User-ID", usuarioID.String())
		enrutador.ServeHTTP(escritor, peticion)

		if escritor.Code != http.StatusOK {
			prueba.Fatalf("Esperaba HTTP 200, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
		}

		var resultado domain.PerfilUsuarioDTO
		if errorOperacion := json.Unmarshal(escritor.Body.Bytes(), &resultado); errorOperacion != nil {
			prueba.Fatalf("Error parseando respuesta JSON: %v", errorOperacion)
		}

		if resultado.NombreCompleto != nuevoNombre {
			prueba.Errorf("Esperaba nombre %s, obtuve %s", nuevoNombre, resultado.NombreCompleto)
		}
		if resultado.Telefono == nil || *resultado.Telefono != nuevoTel {
			prueba.Errorf("Esperaba telefono %s, obtuve %v", nuevoTel, resultado.Telefono)
		}
	})

	prueba.Run("401 Unauthorized cuando falta user_id en contexto (sin sesion)", func(prueba *testing.T) {
		nuevoNombre := "No Permitido"
		cuerpo, _ := json.Marshal(domain.ActualizarPerfilDTO{
			NombreCompleto: &nuevoNombre,
		})

		escritor := httptest.NewRecorder()
		peticion, _ := http.NewRequest(http.MethodPut, "/api/v1/auth/me", bytes.NewBuffer(cuerpo))
		peticion.Header.Set("Content-Type", "application/json")
		enrutador.ServeHTTP(escritor, peticion)

		if escritor.Code != http.StatusUnauthorized {
			prueba.Fatalf("Esperaba HTTP 401, obtuve %d", escritor.Code)
		}
	})
}

func TestAuthHandler_CerrarSesion_ExpiraCookie(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	manejador := handlers.NewAuthHandler(nuevoServicioAuth(newMockUsuarioRepository()), true, claveJWTDePrueba)
	enrutador := gin.New()
	enrutador.POST("/api/v1/auth/logout", manejador.CerrarSesion)

	escritor := httptest.NewRecorder()
	peticion, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	peticion.AddCookie(&http.Cookie{Name: "jwt", Value: "token-anterior"})
	enrutador.ServeHTTP(escritor, peticion)

	if escritor.Code != http.StatusNoContent {
		prueba.Fatalf("Esperaba HTTP 204, obtuve %d", escritor.Code)
	}
	var cookieJWT *http.Cookie
	for _, cookie := range escritor.Result().Cookies() {
		if cookie.Name == "jwt" {
			cookieJWT = cookie
		}
	}
	if cookieJWT == nil {
		prueba.Fatal("Esperaba una cookie 'jwt' de expiración en la respuesta")
	}
	if cookieJWT.Value != "" || cookieJWT.MaxAge >= 0 {
		prueba.Errorf("La cookie debe quedar vacía y expirada; obtuve valor=%q MaxAge=%d", cookieJWT.Value, cookieJWT.MaxAge)
	}
	if !cookieJWT.HttpOnly || !cookieJWT.Secure || cookieJWT.Path != "/" {
		prueba.Errorf("La cookie de expiración debe conservar HttpOnly, Secure y Path=/ para reemplazar a la original")
	}
}
