// Handlers de autenticación: cookie de sesión, rol canónico, perfil y logout.

package unit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/api/idtoken"
)

func TestManejadorAuthRegistroIniciaSesion(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	repositorio := newMockUsuarioRepository()
	servicioAuth := nuevoServicioAuth(repositorio)
	manejador := api.NuevoManejadorAuth(servicioAuth, false, claveJWTDePrueba)

	enrutador := gin.New()
	enrutador.POST("/api/v1/auth/register", manejador.RegistrarUsuario)

	datosGoogle := api.PeticionRegistro{
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

func TestManejadorAuthLoginEmiteRolCanonico(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	repositorio := newMockUsuarioRepository()
	hash, errorOperacion := services.HashearContrasena("AliasLogin2026!")
	if errorOperacion != nil {
		prueba.Fatal(errorOperacion)
	}
	usuarioID := uuid.New()
	repositorio.usuarios["alias@uct.cl"] = &domain.Usuario{
		ID: usuarioID, Correo: "alias@uct.cl", NombreCompleto: "Alias",
		PasswordHash: &hash, Rol: " Admin ", EstaActivo: true,
	}

	manejador := api.NuevoManejadorAuth(nuevoServicioAuth(repositorio), false, claveJWTDePrueba)
	enrutador := gin.New()
	enrutador.POST("/api/v1/auth/login", manejador.LoginUsuario)

	cuerpo, _ := json.Marshal(api.PeticionLogin{Correo: "alias@uct.cl", Password: "AliasLogin2026!"})
	escritor := httptest.NewRecorder()
	peticion, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(cuerpo))
	peticion.Header.Set("Content-Type", "application/json")
	enrutador.ServeHTTP(escritor, peticion)

	if escritor.Code != http.StatusOK {
		prueba.Fatalf("Esperaba HTTP 200, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	var resultado api.RespuestaLogin
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
	token, errorOperacion := api.ValidarToken(cookieJWT.Value, claveJWTDePrueba)
	if errorOperacion != nil {
		prueba.Fatalf("Token inválido: %v", errorOperacion)
	}
	datosToken, valido := token.Claims.(*api.DatosSesion)
	if !valido || datosToken.Rol != domain.RolAdmin {
		prueba.Fatalf("El JWT debía llevar el rol admin, obtuve %+v", token.Claims)
	}
}

func TestManejadorAuthActualizarPerfil(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	repositorio := newMockUsuarioRepository()
	servicioAuth := nuevoServicioAuth(repositorio)
	manejador := api.NuevoManejadorAuth(servicioAuth, false, claveJWTDePrueba)

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

func TestManejadorAuthCerrarSesionExpiraCookie(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	manejador := api.NuevoManejadorAuth(nuevoServicioAuth(newMockUsuarioRepository()), true, claveJWTDePrueba)
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

// TestManejadorAuthRegistroRechazaCorreoAdminCon409 impide pre-registrar
// localmente un correo listado en ADMIN_EMAILS sin revelar que es admin.
func TestManejadorAuthRegistroRechazaCorreoAdminCon409(prueba *testing.T) {
	gin.SetMode(gin.TestMode)

	repositorio := newMockUsuarioRepository()
	servicioAuth := services.NuevoServicioAuth(
		repositorio,
		idtoken.Validate,
		"",
		plazoGoogleDePrueba,
		map[string]struct{}{"admin@ejemplo.cl": {}},
	)
	manejador := api.NuevoManejadorAuth(servicioAuth, false, claveJWTDePrueba)

	enrutador := gin.New()
	enrutador.POST("/api/v1/auth/register", manejador.RegistrarUsuario)

	cuerpo, _ := json.Marshal(api.PeticionRegistro{
		Correo:         "Admin@Ejemplo.CL",
		Password:       "ClaveSegura2026!",
		NombreCompleto: "Intento PreRegistro",
	})
	escritor := httptest.NewRecorder()
	peticion, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(cuerpo))
	peticion.Header.Set("Content-Type", "application/json")
	enrutador.ServeHTTP(escritor, peticion)

	if escritor.Code != http.StatusConflict {
		prueba.Fatalf("esperaba HTTP 409, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	cuerpoTexto := strings.ToLower(escritor.Body.String())
	if strings.Contains(cuerpoTexto, "admin") || strings.Contains(cuerpoTexto, "ejemplo.cl") {
		prueba.Fatalf("la respuesta 409 no debe revelar detalles del correo admin: %s", escritor.Body.String())
	}
	if len(repositorio.usuarios) != 0 {
		prueba.Fatalf("el registro rechazado no debe persistir usuario")
	}
	for _, cookie := range escritor.Result().Cookies() {
		if cookie.Name == "jwt" {
			prueba.Fatal("el registro rechazado no debe emitir cookie jwt")
		}
	}
}

// ===== Bandera de acceso local =====

// repositorioConContador registra las búsquedas por correo para comprobar que
// el rechazo por bandera no consulta la base de datos.
type repositorioConContador struct {
	*mockUsuarioRepository
	busquedasPorCorreo int
}

func (repositorio *repositorioConContador) BuscarPorCorreo(contexto context.Context, direccionCorreo string) (*domain.Usuario, error) {
	repositorio.busquedasPorCorreo++
	return repositorio.mockUsuarioRepository.BuscarPorCorreo(contexto, direccionCorreo)
}

// TestAccesoLocalDeshabilitadoPorDefecto verifica que con AuthLocalHabilitado en false
// los endpoints /register y /login respondan 403 con el formato de error del
// contrato y sin tocar el repositorio.
func TestAccesoLocalDeshabilitadoPorDefecto(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	repositorio := &repositorioConContador{mockUsuarioRepository: newMockUsuarioRepository()}
	servicioAuth := nuevoServicioAuth(repositorio)
	manejador := api.NuevoManejadorAuth(servicioAuth, false, claveJWTDePrueba)

	configuracion := config.Configuracion{
		ClaveJWT:            claveJWTDePrueba,
		Ajustes:             config.AjustesPredeterminados(),
		AuthLocalHabilitado: false,
	}

	enrutador := gin.New()
	api.RegistrarRutasAuth(enrutador.Group("/api/v1"), manejador, nil, nil, repositorio, configuracion)

	for _, ruta := range []string{"/api/v1/auth/register", "/api/v1/auth/login"} {
		cuerpo := []byte(`{"correo":"usuario@ejemplo.cl","password":"ClaveSegura2026!","nombre_completo":"Usuario"}`)
		escritor := httptest.NewRecorder()
		peticion, _ := http.NewRequest(http.MethodPost, ruta, bytes.NewBuffer(cuerpo))
		peticion.Header.Set("Content-Type", "application/json")
		enrutador.ServeHTTP(escritor, peticion)

		if escritor.Code != http.StatusForbidden {
			prueba.Fatalf("%s: esperaba HTTP 403, obtuve %d. Body: %s", ruta, escritor.Code, escritor.Body.String())
		}
		var respuesta api.RespuestaError
		if errorObtenido := json.Unmarshal(escritor.Body.Bytes(), &respuesta); errorObtenido != nil {
			prueba.Fatalf("%s: cuerpo de error inválido: %v", ruta, errorObtenido)
		}
		if respuesta.Estado != http.StatusForbidden || respuesta.Mensaje == "" || respuesta.Detalle != "" {
			prueba.Fatalf("%s: el 403 no usa el formato del contrato: %+v", ruta, respuesta)
		}
	}
	if repositorio.busquedasPorCorreo != 0 || len(repositorio.usuarios) != 0 {
		prueba.Fatalf("el acceso local deshabilitado no debe consultar ni persistir usuarios: %d búsquedas, %d usuarios", repositorio.busquedasPorCorreo, len(repositorio.usuarios))
	}
}

// TestLoginDeshabilitadoNoConsultaRepositorio comprueba que, incluso con una
// cuenta válida en el repositorio, /login queda bloqueado antes de buscarla.
func TestLoginDeshabilitadoNoConsultaRepositorio(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	hash, errorOperacion := services.HashearContrasena("ClaveSegura2026!")
	if errorOperacion != nil {
		prueba.Fatal(errorOperacion)
	}
	repositorioBase := newMockUsuarioRepository()
	repositorioBase.usuarios["usuario@ejemplo.cl"] = &domain.Usuario{
		Correo: "usuario@ejemplo.cl", PasswordHash: &hash, EstaActivo: true,
	}
	repositorio := &repositorioConContador{mockUsuarioRepository: repositorioBase}
	manejador := api.NuevoManejadorAuth(nuevoServicioAuth(repositorio), false, claveJWTDePrueba)
	configuracion := config.Configuracion{
		ClaveJWT:            claveJWTDePrueba,
		Ajustes:             config.AjustesPredeterminados(),
		AuthLocalHabilitado: false,
	}

	enrutador := gin.New()
	api.RegistrarRutasAuth(enrutador.Group("/api/v1"), manejador, nil, nil, repositorio, configuracion)

	escritor := httptest.NewRecorder()
	peticion, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"correo":"usuario@ejemplo.cl","password":"ClaveSegura2026!"}`))
	peticion.Header.Set("Content-Type", "application/json")
	enrutador.ServeHTTP(escritor, peticion)

	if escritor.Code != http.StatusForbidden {
		prueba.Fatalf("esperaba HTTP 403, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	if repositorio.busquedasPorCorreo != 0 {
		prueba.Fatalf("/login con la bandera apagada consultó el repositorio %d veces", repositorio.busquedasPorCorreo)
	}
}

// TestAccesoLocalHabilitadoPermiteRegistroYLogin verifica que con AuthLocalHabilitado
// en true las rutas /register y /login no son bloqueadas por la bandera (no responden 403)
// y que los handlers de registro y login local operan con normalidad.
func TestAccesoLocalHabilitadoPermiteRegistroYLogin(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	repositorio := newMockUsuarioRepository()
	servicioAuth := nuevoServicioAuth(repositorio)
	manejador := api.NuevoManejadorAuth(servicioAuth, false, claveJWTDePrueba)

	configuracion := config.Configuracion{
		ClaveJWT:            claveJWTDePrueba,
		Ajustes:             config.AjustesPredeterminados(),
		AuthLocalHabilitado: true,
	}

	enrutadorRutas := gin.New()
	api.RegistrarRutasAuth(enrutadorRutas.Group("/api/v1"), manejador, nil, nil, repositorio, configuracion)

	for _, ruta := range []string{"/api/v1/auth/register", "/api/v1/auth/login"} {
		escritor := httptest.NewRecorder()
		peticion, _ := http.NewRequest(http.MethodPost, ruta, bytes.NewBufferString(`{}`))
		peticion.Header.Set("Content-Type", "application/json")
		enrutadorRutas.ServeHTTP(escritor, peticion)
		if escritor.Code == http.StatusForbidden {
			prueba.Fatalf("%s: no debió responder 403 con AuthLocalHabilitado=true", ruta)
		}
	}

	enrutador := gin.New()
	enrutador.POST("/api/v1/auth/register", manejador.RegistrarUsuario)
	enrutador.POST("/api/v1/auth/login", manejador.LoginUsuario)

	cuerpoRegistro := []byte(`{"correo":"local@ejemplo.cl","password":"ClaveSegura2026!","nombre_completo":"Usuario Local"}`)
	escritorRegistro := httptest.NewRecorder()
	peticionRegistro, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(cuerpoRegistro))
	peticionRegistro.Header.Set("Content-Type", "application/json")
	enrutador.ServeHTTP(escritorRegistro, peticionRegistro)
	if escritorRegistro.Code != http.StatusCreated {
		prueba.Fatalf("esperaba HTTP 201 al registrar, obtuve %d. Body: %s", escritorRegistro.Code, escritorRegistro.Body.String())
	}

	cuerpoLogin := []byte(`{"correo":"local@ejemplo.cl","password":"ClaveSegura2026!"}`)
	escritorLogin := httptest.NewRecorder()
	peticionLogin, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(cuerpoLogin))
	peticionLogin.Header.Set("Content-Type", "application/json")
	enrutador.ServeHTTP(escritorLogin, peticionLogin)
	if escritorLogin.Code != http.StatusOK {
		prueba.Fatalf("esperaba HTTP 200 al iniciar sesión, obtuve %d. Body: %s", escritorLogin.Code, escritorLogin.Body.String())
	}
}
