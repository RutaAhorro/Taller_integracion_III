// JWT de sesión: emisión, validación, tokens inseguros y revalidación en base de datos.

package unit_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	httperrors "github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/httperrors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/middleware"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestJWT_GenerarYValidarToken(prueba *testing.T) {
	usuarioID := "c14bd236-294a-41b8-8cbb-5473bb610415"
	rol := "admin"
	proveedor := "local"

	tokenTexto, errorObtenido := security.GenerarToken(usuarioID, rol, proveedor, claveJWTDePrueba)
	if errorObtenido != nil {
		prueba.Fatalf("Error inesperado al generar token: %v", errorObtenido)
	}

	if tokenTexto == "" {
		prueba.Fatalf("El token generado no debería estar vacío")
	}

	token, errorObtenido := security.ValidarToken(tokenTexto, claveJWTDePrueba)
	if errorObtenido != nil {
		prueba.Fatalf("Fallo en ValidarToken: %v", errorObtenido)
	}

	if !token.Valid {
		prueba.Errorf("El token validado debería ser válido")
	}

	datosToken, valido := token.Claims.(*security.JWTClaims)
	if !valido {
		prueba.Fatalf("No se pudo convertir claims a *security.JWTClaims")
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

func TestJWT_TokenInvalido(prueba *testing.T) {
	_, errorObtenido := security.ValidarToken("token.falso.invalido", claveJWTDePrueba)
	if errorObtenido == nil {
		prueba.Errorf("Se esperaba error al validar token falso, pero se aceptó")
	}
}

func TestMiddleware_RequireAuth(prueba *testing.T) {
	repositorio := newMockUsuarioRepository()
	usuarioID := uuid.New()
	repositorio.usuarios["middleware@uct.cl"] = &domain.Usuario{
		ID: usuarioID, Correo: "middleware@uct.cl", NombreCompleto: "Middleware",
		Rol: domain.RolColaborador, EstaActivo: true,
	}

	enrutador := gin.New()
	enrutador.Use(httperrors.ErrorHandler())

	enrutador.GET("/protegido", middleware.RequireAuth(repositorio, claveJWTDePrueba), func(contexto *gin.Context) {
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
		tokenTexto, errorObtenido := security.GenerarToken(usuarioID.String(), domain.RolColaborador, "google", claveJWTDePrueba)
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

func TestMiddleware_RequireAuthExigeRepositorio(prueba *testing.T) {
	defer func() {
		if recover() == nil {
			prueba.Fatal("RequireAuth(nil) debía entrar en pánico para no omitir la revalidación de sesión")
		}
	}()
	middleware.RequireAuth(nil, claveJWTDePrueba)
}

func TestMiddleware_RequireAuthRevalidaEstadoYRolEnBaseDeDatos(prueba *testing.T) {
	repositorio := newMockUsuarioRepository()
	usuarioID := uuid.New()
	repositorio.usuarios["admin@uct.cl"] = &domain.Usuario{
		ID: usuarioID, Correo: "admin@uct.cl", NombreCompleto: "Admin",
		Rol: domain.RolAdmin, EstaActivo: true,
	}

	enrutador := gin.New()
	enrutador.Use(httperrors.ErrorHandler())
	enrutador.GET("/protegido", middleware.RequireAuth(repositorio, claveJWTDePrueba), func(contexto *gin.Context) {
		contexto.JSON(http.StatusOK, gin.H{
			"user_id": contexto.GetString("user_id"),
			"rol":     contexto.GetString("rol"),
		})
	})

	// El JWT dice "registrado", pero la base de datos manda: rol admin.
	tokenTexto, errorObtenido := security.GenerarToken(usuarioID.String(), domain.RolRegistrado, "local", claveJWTDePrueba)
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
	tokenInexistente, errorObtenido := security.GenerarToken(uuid.NewString(), domain.RolAdmin, "local", claveJWTDePrueba)
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
			if _, errorObtenido = security.ValidarToken(token, claveJWTDePrueba); errorObtenido == nil {
				prueba.Fatal("token inseguro aceptado")
			}
		})
	}
	prueba.Run("no fallback", func(prueba *testing.T) {
		if _, errorObtenido := security.GenerarToken("user", "registrado", "local", ""); errorObtenido == nil {
			prueba.Fatal("emisión sin clave")
		}
		if _, errorObtenido := security.ValidarToken("token", ""); errorObtenido == nil {
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
		token, errorObtenido := security.ValidarToken(valor, claveJWTDePrueba)
		if errorObtenido != nil {
			return
		}
		if token == nil || !token.Valid {
			prueba.Fatal("token aceptado sin validez")
		}
		datosToken, valido := token.Claims.(*security.JWTClaims)
		if !valido || datosToken.ExpiresAt == nil || strings.TrimSpace(datosToken.UserID) == "" || strings.TrimSpace(datosToken.Rol) == "" || strings.TrimSpace(datosToken.Provider) == "" {
			prueba.Fatal("sesión incompleta")
		}
	})
}

// TestClaveJWTInyectadaIgnoraElEntorno preserva la instantánea del arranque:
// un token sólo valida con la clave recibida, aunque el entorno defina otra.
func TestClaveJWTInyectadaIgnoraElEntorno(prueba *testing.T) {
	clave, claveDelEntorno := strings.Repeat("a", 32), strings.Repeat("b", 32)
	prueba.Setenv("JWT_SECRET", claveDelEntorno)
	token, errorToken := security.GenerarToken("cuenta-prueba", "registrado", "local", clave)
	if errorToken != nil {
		prueba.Fatal(errorToken)
	}
	if _, errorValidacion := security.ValidarToken(token, clave); errorValidacion != nil {
		prueba.Fatal("la clave inyectada no se conservó")
	}
	if _, errorValidacion := security.ValidarToken(token, claveDelEntorno); errorValidacion == nil {
		prueba.Fatal("se firmó con el entorno en lugar de la clave inyectada")
	}
}
