// Sesión: emisión y validación del JWT, tokens inseguros, revalidación en base
// de datos y autorización por rol.

package unit_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ===== JWT y revalidación =====

func TestJWTGenerarYValidarToken(prueba *testing.T) {
	usuarioID := "c14bd236-294a-41b8-8cbb-5473bb610415"
	rol := "admin"
	proveedor := "local"

	tokenTexto, errorObtenido := api.GenerarToken(usuarioID, rol, proveedor, claveJWTDePrueba)
	if errorObtenido != nil {
		prueba.Fatalf("Error inesperado al generar token: %v", errorObtenido)
	}

	if tokenTexto == "" {
		prueba.Fatalf("El token generado no debería estar vacío")
	}

	token, errorObtenido := api.ValidarToken(tokenTexto, claveJWTDePrueba)
	if errorObtenido != nil {
		prueba.Fatalf("Fallo en ValidarToken: %v", errorObtenido)
	}

	if !token.Valid {
		prueba.Errorf("El token validado debería ser válido")
	}

	datosToken, valido := token.Claims.(*api.DatosSesion)
	if !valido {
		prueba.Fatalf("No se pudo convertir claims a *api.DatosSesion")
	}

	if datosToken.UserID != usuarioID {
		prueba.Errorf("UserID esperado %s, obtenido %s", usuarioID, datosToken.UserID)
	}
	if datosToken.Rol != rol {
		prueba.Errorf("Rol esperado %s, obtenido %s", rol, datosToken.Rol)
	}
	if datosToken.Provider != proveedor {
		prueba.Errorf("Provider esperado %s, obtenido %s", proveedor, datosToken.Provider)
	}
}

func TestJWTTokenMalformado(prueba *testing.T) {
	_, errorObtenido := api.ValidarToken("token.falso.invalido", claveJWTDePrueba)
	if errorObtenido == nil {
		prueba.Errorf("Se esperaba error al validar token falso, pero se aceptó")
	}
}

func TestExigirSesion(prueba *testing.T) {
	repositorio := newMockUsuarioRepository()
	usuarioID := uuid.New()
	repositorio.usuarios["middleware@uct.cl"] = &domain.Usuario{
		ID: usuarioID, Correo: "middleware@uct.cl", NombreCompleto: "Middleware",
		Rol: domain.RolColaborador, EstaActivo: true,
	}

	enrutador := gin.New()
	enrutador.Use(api.ManejarErrores())

	enrutador.GET("/protegido", api.ExigirSesion(repositorio, claveJWTDePrueba), func(contexto *gin.Context) {
		idUsuario, _ := contexto.Get("user_id")
		rol, _ := contexto.Get("rol")
		proveedor, _ := contexto.Get("provider")

		contexto.JSON(http.StatusOK, gin.H{
			"user_id":  idUsuario,
			"rol":      rol,
			"provider": proveedor,
		})
	})

	prueba.Run("Sin cookie jwt", func(prueba *testing.T) {
		escritor := httptest.NewRecorder()
		peticionHTTP, _ := http.NewRequest("GET", "/protegido", nil)
		enrutador.ServeHTTP(escritor, peticionHTTP)

		if escritor.Code != http.StatusUnauthorized {
			prueba.Fatalf("Esperaba 401, obtuve %d", escritor.Code)
		}
	})

	prueba.Run("Cookie con token adulterado", func(prueba *testing.T) {
		escritor := httptest.NewRecorder()
		peticionHTTP, _ := http.NewRequest("GET", "/protegido", nil)
		peticionHTTP.AddCookie(&http.Cookie{
			Name:  "jwt",
			Value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.invalidsignature",
		})
		enrutador.ServeHTTP(escritor, peticionHTTP)

		if escritor.Code != http.StatusUnauthorized {
			prueba.Fatalf("Esperaba 401, obtuve %d", escritor.Code)
		}
	})

	prueba.Run("Cookie jwt valida exitosa", func(prueba *testing.T) {
		tokenTexto, errorObtenido := api.GenerarToken(usuarioID.String(), domain.RolColaborador, "google", claveJWTDePrueba)
		if errorObtenido != nil {
			prueba.Fatalf("Error generando token: %v", errorObtenido)
		}

		escritor := httptest.NewRecorder()
		peticionHTTP, _ := http.NewRequest("GET", "/protegido", nil)
		peticionHTTP.AddCookie(&http.Cookie{
			Name:  "jwt",
			Value: tokenTexto,
		})
		enrutador.ServeHTTP(escritor, peticionHTTP)

		if escritor.Code != http.StatusOK {
			prueba.Fatalf("Esperaba 200, obtuve %d", escritor.Code)
		}

		var resultado map[string]interface{}
		json.Unmarshal(escritor.Body.Bytes(), &resultado)

		if resultado["user_id"] != usuarioID.String() || resultado["rol"] != domain.RolColaborador || resultado["provider"] != "google" {
			prueba.Errorf("Claims devueltos no coinciden con los inyectados: %v", resultado)
		}
	})
}

func TestExigirSesionExigeRepositorio(prueba *testing.T) {
	defer func() {
		if recover() == nil {
			prueba.Fatal("ExigirSesion(nil) debía entrar en pánico para no omitir la revalidación de sesión")
		}
	}()
	api.ExigirSesion(nil, claveJWTDePrueba)
}

func TestExigirSesionRevalidaEstadoYRolEnBaseDeDatos(prueba *testing.T) {
	repositorio := newMockUsuarioRepository()
	usuarioID := uuid.New()
	repositorio.usuarios["admin@uct.cl"] = &domain.Usuario{
		ID: usuarioID, Correo: "admin@uct.cl", NombreCompleto: "Admin",
		Rol: domain.RolAdmin, EstaActivo: true,
	}

	enrutador := gin.New()
	enrutador.Use(api.ManejarErrores())
	enrutador.GET("/protegido", api.ExigirSesion(repositorio, claveJWTDePrueba), func(contexto *gin.Context) {
		contexto.JSON(http.StatusOK, gin.H{
			"user_id": contexto.GetString("user_id"),
			"rol":     contexto.GetString("rol"),
		})
	})

	// El JWT dice "registrado", pero la base de datos manda: rol admin.
	tokenTexto, errorObtenido := api.GenerarToken(usuarioID.String(), domain.RolRegistrado, "local", claveJWTDePrueba)
	if errorObtenido != nil {
		prueba.Fatalf("Error generando token: %v", errorObtenido)
	}
	escritor := httptest.NewRecorder()
	peticionHTTP, _ := http.NewRequest(http.MethodGet, "/protegido", nil)
	peticionHTTP.AddCookie(&http.Cookie{Name: "jwt", Value: tokenTexto})
	enrutador.ServeHTTP(escritor, peticionHTTP)
	if escritor.Code != http.StatusOK {
		prueba.Fatalf("Esperaba 200, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	var resultado map[string]interface{}
	if errorOperacion := json.Unmarshal(escritor.Body.Bytes(), &resultado); errorOperacion != nil {
		prueba.Fatalf("Respuesta inválida: %v", errorOperacion)
	}
	if resultado["rol"] != domain.RolAdmin {
		prueba.Fatalf("El rol debía normalizarse desde la base de datos a admin, obtuve %v", resultado["rol"])
	}

	// Desactivar la cuenta invalida el token en la siguiente petición.
	repositorio.usuarios["admin@uct.cl"].EstaActivo = false
	escritor = httptest.NewRecorder()
	peticionHTTP, _ = http.NewRequest(http.MethodGet, "/protegido", nil)
	peticionHTTP.AddCookie(&http.Cookie{Name: "jwt", Value: tokenTexto})
	enrutador.ServeHTTP(escritor, peticionHTTP)
	if escritor.Code != http.StatusUnauthorized {
		prueba.Fatalf("Una cuenta desactivada debía recibir 401, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
	}

	// Un token válido para un usuario inexistente tampoco sirve.
	tokenInexistente, errorObtenido := api.GenerarToken(uuid.NewString(), domain.RolAdmin, "local", claveJWTDePrueba)
	if errorObtenido != nil {
		prueba.Fatalf("Error generando token: %v", errorObtenido)
	}
	escritor = httptest.NewRecorder()
	peticionHTTP, _ = http.NewRequest(http.MethodGet, "/protegido", nil)
	peticionHTTP.AddCookie(&http.Cookie{Name: "jwt", Value: tokenInexistente})
	enrutador.ServeHTTP(escritor, peticionHTTP)
	if escritor.Code != http.StatusUnauthorized {
		prueba.Fatalf("Un usuario inexistente debía recibir 401, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
	}
}

func TestJWTRejectsUnsafeTokens(prueba *testing.T) {
	clave := []byte("sup272-test-only-key-at-least-thirty-two-bytes")
	for _, caso := range []struct {
		name   string
		method jwt.SigningMethod
		claims jwt.MapClaims
	}{
		{"HS384", jwt.SigningMethodHS384, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix()}},
		{"HS512", jwt.SigningMethodHS512, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix()}},
		{"missing expiration", jwt.SigningMethodHS256, jwt.MapClaims{"user_id": "fixture"}},
		{"missing identity", jwt.SigningMethodHS256, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix(), "rol": "admin", "provider": "local"}},
		{"missing role", jwt.SigningMethodHS256, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix(), "user_id": "fixture", "provider": "local"}},
		{"missing provider", jwt.SigningMethodHS256, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix(), "user_id": "fixture", "rol": "admin"}},
		{"expired", jwt.SigningMethodHS256, jwt.MapClaims{"exp": time.Now().Add(-time.Second).Unix()}},
		{"not yet valid", jwt.SigningMethodHS256, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix(), "nbf": time.Now().Add(time.Hour).Unix()}},
	} {
		prueba.Run(caso.name, func(prueba *testing.T) {
			token, errorObtenido := jwt.NewWithClaims(caso.method, caso.claims).SignedString(clave)
			if errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
			if _, errorObtenido = api.ValidarToken(token, claveJWTDePrueba); errorObtenido == nil {
				prueba.Fatal("token inseguro aceptado")
			}
		})
	}
	prueba.Run("no fallback", func(prueba *testing.T) {
		if _, errorObtenido := api.GenerarToken("user", "registrado", "local", ""); errorObtenido == nil {
			prueba.Fatal("emisión sin clave")
		}
		if _, errorObtenido := api.ValidarToken("token", ""); errorObtenido == nil {
			prueba.Fatal("validación sin clave")
		}
	})
}

// Tokens arbitrarios no pueden causar panic ni entregar una sesión incompleta.
func FuzzJWTParser(generador *testing.F) {
	for _, valor := range []string{"", "not-a-token", "eyJhbGciOiJub25lIn0.eyJyb2wiOiJhZG1pbiJ9.", strings.Repeat("x", 4097)} {
		generador.Add(valor)
	}
	generador.Fuzz(func(prueba *testing.T, valor string) {
		token, errorObtenido := api.ValidarToken(valor, claveJWTDePrueba)
		if errorObtenido != nil {
			return
		}
		if token == nil || !token.Valid {
			prueba.Fatal("token aceptado sin validez")
		}
		datosToken, valido := token.Claims.(*api.DatosSesion)
		if !valido || datosToken.ExpiresAt == nil || strings.TrimSpace(datosToken.UserID) == "" || strings.TrimSpace(datosToken.Rol) == "" || strings.TrimSpace(datosToken.Provider) == "" {
			prueba.Fatal("sesión incompleta")
		}
	})
}

// ===== Rol =====

func TestExigirRol(prueba *testing.T) {
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
			enrutador.GET("/test-protected", api.ExigirRol(caso.rolesRequerido...), func(contexto *gin.Context) {
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
