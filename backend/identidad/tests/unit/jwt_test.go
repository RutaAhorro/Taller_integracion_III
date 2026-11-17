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
	provider := "local"

	tokenStr, err := security.GenerarToken(usuarioID, rol, provider, claveJWTDePrueba)
	if err != nil {
		prueba.Fatalf("Error inesperado al generar token: %v", err)
	}

	if tokenStr == "" {
		prueba.Fatalf("El token generado no debería estar vacío")
	}

	token, err := security.ValidarToken(tokenStr, claveJWTDePrueba)
	if err != nil {
		prueba.Fatalf("Fallo en ValidarToken: %v", err)
	}

	if !token.Valid {
		prueba.Errorf("El token validado debería ser válido")
	}

	claims, ok := token.Claims.(*security.JWTClaims)
	if !ok {
		prueba.Fatalf("No se pudo convertir claims a *security.JWTClaims")
	}

	if claims.UserID != usuarioID {
		prueba.Errorf("UserID esperado %s, obtenido %s", usuarioID, claims.UserID)
	}
	if claims.Rol != rol {
		prueba.Errorf("Rol esperado %s, obtenido %s", rol, claims.Rol)
	}
	if claims.Provider != provider {
		prueba.Errorf("Provider esperado %s, obtenido %s", provider, claims.Provider)
	}
}

func TestJWT_TokenInvalido(prueba *testing.T) {
	_, err := security.ValidarToken("token.falso.invalido", claveJWTDePrueba)
	if err == nil {
		prueba.Errorf("Se esperaba error al validar token falso, pero se aceptó")
	}
}

func TestMiddleware_RequireAuth(prueba *testing.T) {
	repo := newMockUsuarioRepository()
	usuarioID := uuid.New()
	repo.usuarios["middleware@uct.cl"] = &domain.Usuario{
		ID: usuarioID, Correo: "middleware@uct.cl", NombreCompleto: "Middleware",
		Rol: domain.RolColaborador, EstaActivo: true,
	}

	peticion := gin.New()
	peticion.Use(httperrors.ErrorHandler())

	peticion.GET("/protegido", middleware.RequireAuth(repo, claveJWTDePrueba), func(contexto *gin.Context) {
		userID, _ := contexto.Get("user_id")
		rol, _ := contexto.Get("rol")
		provider, _ := contexto.Get("provider")

		contexto.JSON(http.StatusOK, gin.H{
			"user_id":  userID,
			"rol":      rol,
			"provider": provider,
		})
	})

	prueba.Run("Sin cookie jwt", func(prueba *testing.T) {
		escritor := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protegido", nil)
		peticion.ServeHTTP(escritor, req)

		if escritor.Code != http.StatusUnauthorized {
			prueba.Fatalf("Esperaba 401, obtuve %d", escritor.Code)
		}
	})

	prueba.Run("Cookie con token adulterado", func(prueba *testing.T) {
		escritor := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protegido", nil)
		req.AddCookie(&http.Cookie{
			Name:  "jwt",
			Value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.invalidsignature",
		})
		peticion.ServeHTTP(escritor, req)

		if escritor.Code != http.StatusUnauthorized {
			prueba.Fatalf("Esperaba 401, obtuve %d", escritor.Code)
		}
	})

	prueba.Run("Cookie jwt valida exitosa", func(prueba *testing.T) {
		tokenStr, err := security.GenerarToken(usuarioID.String(), domain.RolColaborador, "google", claveJWTDePrueba)
		if err != nil {
			prueba.Fatalf("Error generando token: %v", err)
		}

		escritor := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protegido", nil)
		req.AddCookie(&http.Cookie{
			Name:  "jwt",
			Value: tokenStr,
		})
		peticion.ServeHTTP(escritor, req)

		if escritor.Code != http.StatusOK {
			prueba.Fatalf("Esperaba 200, obtuve %d", escritor.Code)
		}

		var res map[string]interface{}
		json.Unmarshal(escritor.Body.Bytes(), &res)

		if res["user_id"] != usuarioID.String() || res["rol"] != domain.RolColaborador || res["provider"] != "google" {
			prueba.Errorf("Claims devueltos no coinciden con los inyectados: %v", res)
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
	repo := newMockUsuarioRepository()
	usuarioID := uuid.New()
	repo.usuarios["admin@uct.cl"] = &domain.Usuario{
		ID: usuarioID, Correo: "admin@uct.cl", NombreCompleto: "Admin",
		Rol: domain.RolAdmin, EstaActivo: true,
	}

	peticion := gin.New()
	peticion.Use(httperrors.ErrorHandler())
	peticion.GET("/protegido", middleware.RequireAuth(repo, claveJWTDePrueba), func(contexto *gin.Context) {
		contexto.JSON(http.StatusOK, gin.H{
			"user_id": contexto.GetString("user_id"),
			"rol":     contexto.GetString("rol"),
		})
	})

	// El JWT dice "registrado", pero la base de datos manda: rol admin.
	tokenStr, err := security.GenerarToken(usuarioID.String(), domain.RolRegistrado, "local", claveJWTDePrueba)
	if err != nil {
		prueba.Fatalf("Error generando token: %v", err)
	}
	escritor := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protegido", nil)
	req.AddCookie(&http.Cookie{Name: "jwt", Value: tokenStr})
	peticion.ServeHTTP(escritor, req)
	if escritor.Code != http.StatusOK {
		prueba.Fatalf("Esperaba 200, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	var res map[string]interface{}
	if err := json.Unmarshal(escritor.Body.Bytes(), &res); err != nil {
		prueba.Fatalf("Respuesta inválida: %v", err)
	}
	if res["rol"] != domain.RolAdmin {
		prueba.Fatalf("El rol debía normalizarse desde la base de datos a admin, obtuve %v", res["rol"])
	}

	// Desactivar la cuenta invalida el token en la siguiente petición.
	repo.usuarios["admin@uct.cl"].EstaActivo = false
	escritor = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/protegido", nil)
	req.AddCookie(&http.Cookie{Name: "jwt", Value: tokenStr})
	peticion.ServeHTTP(escritor, req)
	if escritor.Code != http.StatusUnauthorized {
		prueba.Fatalf("Una cuenta desactivada debía recibir 401, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
	}

	// Un token válido para un usuario inexistente tampoco sirve.
	tokenInexistente, err := security.GenerarToken(uuid.NewString(), domain.RolAdmin, "local", claveJWTDePrueba)
	if err != nil {
		prueba.Fatalf("Error generando token: %v", err)
	}
	escritor = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/protegido", nil)
	req.AddCookie(&http.Cookie{Name: "jwt", Value: tokenInexistente})
	peticion.ServeHTTP(escritor, req)
	if escritor.Code != http.StatusUnauthorized {
		prueba.Fatalf("Un usuario inexistente debía recibir 401, obtuve %d. Body: %s", escritor.Code, escritor.Body.String())
	}
}

func TestJWTRejectsUnsafeTokens(prueba *testing.T) {
	key := []byte("sup272-test-only-key-at-least-thirty-two-bytes")
	for _, tc := range []struct {
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
		prueba.Run(tc.name, func(prueba *testing.T) {
			token, err := jwt.NewWithClaims(tc.method, tc.claims).SignedString(key)
			if err != nil {
				prueba.Fatal(err)
			}
			if _, err = security.ValidarToken(token, claveJWTDePrueba); err == nil {
				prueba.Fatal("token inseguro aceptado")
			}
		})
	}
	prueba.Run("no fallback", func(prueba *testing.T) {
		if _, err := security.GenerarToken("user", "registrado", "local", ""); err == nil {
			prueba.Fatal("emisión sin clave")
		}
		if _, err := security.ValidarToken("token", ""); err == nil {
			prueba.Fatal("validación sin clave")
		}
	})
}

// Tokens arbitrarios no pueden causar panic ni entregar una sesión incompleta.
func FuzzJWTParser(archivo *testing.F) {
	for _, value := range []string{"", "not-a-token", "eyJhbGciOiJub25lIn0.eyJyb2wiOiJhZG1pbiJ9.", strings.Repeat("x", 4097)} {
		archivo.Add(value)
	}
	archivo.Fuzz(func(prueba *testing.T, value string) {
		token, err := security.ValidarToken(value, claveJWTDePrueba)
		if err != nil {
			return
		}
		if token == nil || !token.Valid {
			prueba.Fatal("token aceptado sin validez")
		}
		claims, ok := token.Claims.(*security.JWTClaims)
		if !ok || claims.ExpiresAt == nil || strings.TrimSpace(claims.UserID) == "" || strings.TrimSpace(claims.Rol) == "" || strings.TrimSpace(claims.Provider) == "" {
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
