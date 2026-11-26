// Login con Google: validación criptográfica, claims, vinculación de cuentas
// y clasificación de errores, sin acceso a la red de Google.

package unit_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
)

func TestGoogleClaimsAndInactiveAccounts(prueba *testing.T) {
	for _, caso := range []struct {
		name      string
		payload   *idtoken.Payload
		wantError bool
	}{
		{"verified", &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "google-123", Claims: map[string]any{"email": "fixture@example.test", "email_verified": true, "name": "Fixture"}}, false},
		{"optional name", &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "google-123", Claims: map[string]any{"email": "fixture@example.test", "email_verified": true}}, false},
		{"name wrong type", &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "google-123", Claims: map[string]any{"email": "fixture@example.test", "email_verified": true, "name": 17}}, false},
		{"missing email", &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "google-123", Claims: map[string]any{"email_verified": true}}, true},
		{"email wrong type", &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "google-123", Claims: map[string]any{"email": 17, "email_verified": true}}, true},
		{"unverified", &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "google-123", Claims: map[string]any{"email": "fixture@example.test", "email_verified": false}}, true},
		{"verification wrong type", &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "google-123", Claims: map[string]any{"email": "fixture@example.test", "email_verified": "true"}}, true},
		{"missing subject", &idtoken.Payload{Issuer: "https://accounts.google.com", Claims: map[string]any{"email": "fixture@example.test", "email_verified": true}}, true},
		{"wrong issuer", &idtoken.Payload{Issuer: "https://evil.example", Subject: "google-123", Claims: map[string]any{"email": "fixture@example.test", "email_verified": true}}, true},
		{"nil", nil, true},
	} {
		prueba.Run(caso.name, func(prueba *testing.T) {
			repositorio := newMockUsuarioRepository()
			servicio := services.NuevoServicioAuth(repositorio, func(contexto context.Context, token, audiencia string) (*idtoken.Payload, error) {
				if audiencia != "fixture-google-client" {
					prueba.Fatal("audience perdido")
				}
				plazo, valido := contexto.Deadline()
				if !valido || time.Until(plazo) > 5*time.Second {
					prueba.Fatal("Google sin deadline")
				}
				return caso.payload, nil
			}, clienteGoogleDePrueba, plazoGoogleDePrueba, nil)
			usuario, errorObtenido := servicio.LoginGoogle(prueba.Context(), "fixture")
			if (errorObtenido != nil) != caso.wantError {
				prueba.Fatalf("%v %v", usuario, errorObtenido)
			}
			if !caso.wantError {
				if usuario.PasswordHash != nil {
					prueba.Fatal("SSO con password local")
				}
				if _, errorObtenido = servicio.Login(prueba.Context(), usuario.Correo, "anything"); !errors.Is(errorObtenido, services.ErrCredencialesInvalidas) {
					prueba.Fatal("SSO provocó panic o aceptó password")
				}
			}
		})
	}
	for _, nombre := range []string{"inactive", "different google subject"} {
		prueba.Run(nombre, func(prueba *testing.T) {
			repositorio := newMockUsuarioRepository()
			sujeto := "another-subject"
			usuario := &domain.Usuario{ID: uuid.New(), Correo: "fixture@example.test", EstaActivo: true}
			if nombre == "inactive" {
				usuario.EstaActivo = false
			} else {
				usuario.GoogleID = &sujeto
			}
			repositorio.usuarios[usuario.Correo] = usuario
			servicio := services.NuevoServicioAuth(repositorio, func(context.Context, string, string) (*idtoken.Payload, error) {
				return &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "google-123", Claims: map[string]any{"email": usuario.Correo, "email_verified": true}}, nil
			}, clienteGoogleDePrueba, plazoGoogleDePrueba, nil)
			if _, errorObtenido := servicio.LoginGoogle(prueba.Context(), "fixture"); errorObtenido == nil {
				prueba.Fatal("cuenta no autorizada aceptada")
			}
		})
	}
}

// El conflicto concurrente de SSO no equivale a autorización: volver a leer
// y comprobar estado/subject, incluso cuando otro request creó la cuenta.
type googleRaceRepository struct {
	*mockUsuarioRepository
	competing *domain.Usuario
	created   bool
}

// BuscarPorCorreo no encuentra la cuenta en la primera consulta y sí en la
// segunda, como si otra petición la hubiera creado en medio.
func (repositorio *googleRaceRepository) BuscarPorCorreo(contexto context.Context, direccionCorreo string) (*domain.Usuario, error) {
	if !repositorio.created {
		return nil, nil
	}
	return repositorio.competing, nil
}

// Crear falla con correo duplicado para forzar la ruta de carrera.
func (repositorio *googleRaceRepository) Crear(contexto context.Context, usuario *domain.Usuario) error {
	repositorio.created = true
	return domain.ErrCorreoDuplicado
}

func TestGoogleConcurrentCreationRechecksAccount(prueba *testing.T) {
	for _, nombre := range []string{"same account", "inactive competitor", "different subject"} {
		prueba.Run(nombre, func(prueba *testing.T) {
			sujeto := "google-123"
			if nombre == "different subject" {
				sujeto = "other-google"
			}
			competidor := &domain.Usuario{ID: uuid.New(), Correo: "race@example.test", GoogleID: &sujeto, EstaActivo: nombre != "inactive competitor"}
			repositorio := &googleRaceRepository{mockUsuarioRepository: newMockUsuarioRepository(), competing: competidor}
			servicio := services.NuevoServicioAuth(repositorio, func(context.Context, string, string) (*idtoken.Payload, error) {
				return &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "google-123", Claims: map[string]any{"email": "race@example.test", "email_verified": true}}, nil
			}, clienteGoogleDePrueba, plazoGoogleDePrueba, nil)
			usuario, errorObtenido := servicio.LoginGoogle(prueba.Context(), "fixture")
			if nombre == "same account" {
				if errorObtenido != nil || usuario.ID != competidor.ID {
					prueba.Fatalf("%v %v", usuario, errorObtenido)
				}
			} else if errorObtenido == nil {
				prueba.Fatal("conflicto autorizó cuenta incorrecta")
			}
		})
	}
}

// googlePayload arma claims válidos de Google para un sujeto y correo.
func googlePayload(sujeto, direccionCorreo string) *idtoken.Payload {
	return &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: sujeto,
		Claims: map[string]any{"email": direccionCorreo, "email_verified": true, "name": "Fixture"}}
}

// W1 (#99), medida provisoria: la primera vinculación con Google invalida la
// contraseña local, porque el registro local nunca verificó el correo.
func TestGoogleVinculaCuentaLocalEInvalidaContrasena(prueba *testing.T) {
	repositorio := newMockUsuarioRepository()
	hash, _ := services.HashearContrasena("Atacante2026#")
	victima := &domain.Usuario{ID: uuid.New(), Correo: "victima@uct.cl", PasswordHash: &hash, NombreCompleto: "Pre-registro", EstaActivo: true}
	repositorio.usuarios[victima.Correo] = victima
	servicio := services.NuevoServicioAuth(repositorio, func(context.Context, string, string) (*idtoken.Payload, error) {
		return googlePayload("google-victima", "victima@uct.cl"), nil
	}, clienteGoogleDePrueba, plazoGoogleDePrueba, nil)

	usuario, errorOperacion := servicio.LoginGoogle(context.Background(), "token")
	if errorOperacion != nil {
		prueba.Fatal(errorOperacion)
	}
	if usuario.GoogleID == nil || *usuario.GoogleID != "google-victima" || usuario.PasswordHash != nil {
		prueba.Fatal("la cuenta no quedó vinculada con la contraseña invalidada")
	}
	if victima.PasswordHash != nil || victima.GoogleID == nil {
		prueba.Fatal("la vinculación no se persistió")
	}
	if _, errorObtenido := servicio.Login(context.Background(), "victima@uct.cl", "Atacante2026#"); !errors.Is(errorObtenido, services.ErrCredencialesInvalidas) {
		prueba.Fatal("la contraseña pre-registrada sigue sirviendo:", errorObtenido)
	}
	// Logins posteriores con la misma identidad no vuelven a escribir.
	if _, errorObtenido := servicio.LoginGoogle(context.Background(), "token"); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
}

func TestGoogleRechazaOtraIdentidadVinculada(prueba *testing.T) {
	repositorio := newMockUsuarioRepository()
	vinculado := "google-original"
	hash, _ := services.HashearContrasena("Password123#")
	usuario := &domain.Usuario{ID: uuid.New(), Correo: "linked@uct.cl", GoogleID: &vinculado, PasswordHash: &hash, NombreCompleto: "Linked", EstaActivo: true}
	repositorio.usuarios[usuario.Correo] = usuario
	servicio := services.NuevoServicioAuth(repositorio, func(context.Context, string, string) (*idtoken.Payload, error) {
		return googlePayload("google-otro", "linked@uct.cl"), nil
	}, clienteGoogleDePrueba, plazoGoogleDePrueba, nil)
	if _, errorObtenido := servicio.LoginGoogle(context.Background(), "token"); !errors.Is(errorObtenido, services.ErrCredencialesInvalidas) {
		prueba.Fatal("se aceptó otra identidad Google:", errorObtenido)
	}
	if usuario.PasswordHash == nil || *usuario.GoogleID != vinculado {
		prueba.Fatal("un rechazo no debe modificar la cuenta")
	}
}

func TestGoogleAvatarFitsPostgresWithoutBreakingLogin(prueba *testing.T) {
	for _, caso := range []struct {
		name    string
		picture any
		keep    bool
	}{
		{"normal URL", "https://example.test/avatar.jpg", true},
		{"500 ASCII", strings.Repeat("a", 500), true},
		{"501 ASCII", strings.Repeat("a", 501), false},
		{"500 Unicode characters", strings.Repeat("ñ", 500), true},
		{"501 Unicode characters", strings.Repeat("ñ", 501), false},
		{"wrong type", 42, false},
		{"missing", nil, false},
		{"invalid UTF8", "https://example.test/\xff", false},
	} {
		prueba.Run(caso.name, func(prueba *testing.T) {
			repositorio := newMockUsuarioRepository()
			servicio := services.NuevoServicioAuth(repositorio, func(context.Context, string, string) (*idtoken.Payload, error) {
				return &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "fixture-subject", Claims: map[string]any{
					"email": "avatar@example.test", "email_verified": true, "name": "Fixture", "picture": caso.picture,
				}}, nil
			}, "fixture-client", plazoGoogleDePrueba, nil)
			usuario, errorObtenido := servicio.LoginGoogle(prueba.Context(), "fixture")
			if errorObtenido != nil || usuario == nil || repositorio.usuarios[usuario.Correo] != usuario {
				prueba.Fatalf("login or persistence failed: %v", errorObtenido)
			}
			if (usuario.URLAvatar != nil) != caso.keep {
				prueba.Fatalf("unexpected avatar: %v", usuario.URLAvatar)
			}
			if caso.keep && *usuario.URLAvatar != caso.picture.(string) {
				prueba.Fatal("avatar URL was truncated")
			}
		})
	}
}

// validadorQueRechaza simula un id_token que Google no reconoce.
func validadorQueRechaza(context.Context, string, string) (*idtoken.Payload, error) {
	return nil, errors.New("firma inválida")
}

// validadorQueAcepta devuelve claims válidos para el correo indicado.
func validadorQueAcepta(direccionCorreo string) services.ValidadorGoogle {
	return func(context.Context, string, string) (*idtoken.Payload, error) {
		return googlePayload("google-123", direccionCorreo), nil
	}
}

func TestGoogleErrorClassificationDoesNotExposeStorage(prueba *testing.T) {
	repositorioConInactiva := newMockUsuarioRepository()
	repositorioConInactiva.usuarios["inactiva@example.test"] = &domain.Usuario{ID: uuid.New(), Correo: "inactiva@example.test", NombreCompleto: "Inactiva"}
	for _, caso := range []struct {
		name        string
		repositorio domain.RepositorioUsuarios
		validador   services.ValidadorGoogle
		cliente     string
		status      int
	}{
		{"token rejected", newMockUsuarioRepository(), validadorQueRechaza, clienteGoogleDePrueba, 401},
		{"inactive", repositorioConInactiva, validadorQueAcepta("inactiva@example.test"), clienteGoogleDePrueba, 401},
		{"configuration", newMockUsuarioRepository(), validadorQueAcepta("fixture@example.test"), "", 503},
		{"database", usuariosQueFallan{newMockUsuarioRepository()}, validadorQueAcepta("fixture@example.test"), clienteGoogleDePrueba, 500},
	} {
		prueba.Run(caso.name, func(prueba *testing.T) {
			servicio := services.NuevoServicioAuth(caso.repositorio, caso.validador, caso.cliente, plazoGoogleDePrueba, nil)
			enrutador := gin.New()
			enrutador.POST("/google", api.NuevoManejadorAuth(servicio, false, claveJWTDePrueba).LoginGoogleUsuario)
			escritor := httptest.NewRecorder()
			peticionHTTP := httptest.NewRequest("POST", "/google", strings.NewReader(`{"id_token":"fixture"}`))
			peticionHTTP.Header.Set("Content-Type", "application/json")
			enrutador.ServeHTTP(escritor, peticionHTTP)
			if escritor.Code != caso.status || strings.Contains(escritor.Body.String(), "detalle") || strings.Contains(escritor.Body.String(), "conexión") {
				prueba.Fatal(escritor.Code, escritor.Body)
			}
		})
	}
}

// jwksFixture responde las claves públicas de Google sin salir a la red.
type jwksFixture struct{ body string }

// RoundTrip entrega el JWKS de prueba para cualquier petición.
func (respuestaJWKS jwksFixture) RoundTrip(peticion *http.Request) (*http.Response, error) {
	if peticion.URL.String() != "https://www.googleapis.com/oauth2/v3/certs" {
		return nil, io.ErrUnexpectedEOF
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Cache-Control": []string{"max-age=3600"}}, Body: io.NopCloser(strings.NewReader(respuestaJWKS.body)), Request: peticion}, nil
}

// Usar el validador real de Google y firmas RSA; sólo la respuesta de JWKS es
// una fixture. Nunca se cambia el transporte TLS de la aplicación.
func TestGoogleCryptographicValidation(prueba *testing.T) {
	clave, errorObtenido := rsa.GenerateKey(rand.Reader, 2048)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	otro, errorObtenido := rsa.GenerateKey(rand.Reader, 2048)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	jwks, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"alg": "RS256", "kid": "fixture-key", "kty": "RSA", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(clave.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(clave.E)).Bytes())}}})
	validador, errorObtenido := idtoken.NewValidator(prueba.Context(), option.WithHTTPClient(&http.Client{Transport: jwksFixture{string(jwks)}}))
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	for _, caso := range []struct {
		name, aud, issuer string
		exp               int64
		signingKey        *rsa.PrivateKey
		verified          bool
		wantsError        bool
	}{
		{"valid", "fixture-google-client", "https://accounts.google.com", time.Now().Add(time.Hour).Unix(), clave, true, false},
		{"wrong audience", "wrong-client", "https://accounts.google.com", time.Now().Add(time.Hour).Unix(), clave, true, true},
		{"wrong issuer", "fixture-google-client", "https://evil.example", time.Now().Add(time.Hour).Unix(), clave, true, true},
		{"wrong signature", "fixture-google-client", "https://accounts.google.com", time.Now().Add(time.Hour).Unix(), otro, true, true},
		{"expired", "fixture-google-client", "https://accounts.google.com", time.Now().Add(-time.Minute).Unix(), clave, true, true},
		{"unverified email", "fixture-google-client", "https://accounts.google.com", time.Now().Add(time.Hour).Unix(), clave, false, true},
	} {
		prueba.Run(caso.name, func(prueba *testing.T) {
			datosToken := jwt.MapClaims{"iss": caso.issuer, "aud": caso.aud, "sub": "google-fixture", "exp": caso.exp, "iat": time.Now().Unix(), "email": "crypto@example.test", "email_verified": caso.verified, "name": "Crypto Fixture"}
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, datosToken)
			token.Header["kid"] = "fixture-key"
			codificado, errorOperacion := token.SignedString(caso.signingKey)
			if errorOperacion != nil {
				prueba.Fatal(errorOperacion)
			}
			repositorio := newMockUsuarioRepository()
			servicio := services.NuevoServicioAuth(repositorio, func(contexto context.Context, token, audiencia string) (*idtoken.Payload, error) {
				return validador.Validate(contexto, token, audiencia)
			}, clienteGoogleDePrueba, plazoGoogleDePrueba, nil)
			usuario, errorOperacion := servicio.LoginGoogle(prueba.Context(), codificado)
			if (errorOperacion != nil) != caso.wantsError {
				prueba.Fatalf("%v %v", usuario, errorOperacion)
			}
			if caso.wantsError && len(repositorio.usuarios) != 0 {
				prueba.Fatal("token rechazado persistió usuario")
			}
		})
	}
}

// ===== Asignación de rol con ADMIN_EMAILS =====

// TestGoogleAsignaRolAdminYRechazaNoVerificado comprueba la asignación de rol al
// crear usuarios con Google, insensibilidad a mayúsculas y rechazo de correo no verificado.
func TestGoogleAsignaRolAdminYRechazaNoVerificado(prueba *testing.T) {
	correosAdmin := map[string]struct{}{"ADMIN@ejemplo.cl": {}}
	casos := []struct {
		nombre     string
		correo     string
		verificado bool
		esperaErr  bool
		rolFinal   string
	}{
		{"nuevo admin", "admin@ejemplo.cl", true, false, domain.RolAdmin},
		{"nuevo registrado", "usuario@ejemplo.cl", true, false, domain.RolRegistrado},
		{"case insensitive", "Admin@Ejemplo.CL", true, false, domain.RolAdmin},
		{"email_verified false rechazado", "admin@ejemplo.cl", false, true, ""},
	}

	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			repositorio := newMockUsuarioRepository()
			servicio := services.NuevoServicioAuth(repositorio, func(context.Context, string, string) (*idtoken.Payload, error) {
				return &idtoken.Payload{
					Issuer:  "https://accounts.google.com",
					Subject: "google-sub-1",
					Claims:  map[string]any{"email": caso.correo, "email_verified": caso.verificado, "name": "Persona"},
				}, nil
			}, clienteGoogleDePrueba, plazoGoogleDePrueba, correosAdmin)

			usuario, errorObtenido := servicio.LoginGoogle(prueba.Context(), "token-fixture")
			if caso.esperaErr {
				if !errors.Is(errorObtenido, services.ErrCredencialesInvalidas) {
					prueba.Fatalf("esperaba ErrCredencialesInvalidas, obtuve %v", errorObtenido)
				}
				if len(repositorio.usuarios) != 0 {
					prueba.Fatalf("un correo no verificado no debe crear usuario")
				}
				return
			}
			if errorObtenido != nil {
				prueba.Fatalf("error inesperado en LoginGoogle: %v", errorObtenido)
			}
			if usuario.Rol != caso.rolFinal {
				prueba.Fatalf("esperaba rol %q, obtuve %q", caso.rolFinal, usuario.Rol)
			}
			guardado := repositorio.usuarios[strings.ToLower(caso.correo)]
			if guardado == nil || guardado.Rol != caso.rolFinal {
				prueba.Fatalf("el rol persistido no coincide: %+v", guardado)
			}
		})
	}
}

// TestGooglePromueveExistenteYSinDegradarOtrosRoles verifica que un usuario
// registrado listado en ADMIN_EMAILS se promueva a admin (incluyendo JWT) y que
// nunca se degraden admins existentes ni se modifiquen colaboradores.
func TestGooglePromueveExistenteYSinDegradarOtrosRoles(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	casos := []struct {
		nombre       string
		correo       string
		rolInicial   string
		correosAdmin map[string]struct{}
		rolEsperado  string
	}{
		{"promueve registrado a admin", "promo@ejemplo.cl", domain.RolRegistrado, map[string]struct{}{"promo@ejemplo.cl": {}}, domain.RolAdmin},
		{"colaborador intacto aunque este en ADMIN_EMAILS", "colab@ejemplo.cl", domain.RolColaborador, map[string]struct{}{"colab@ejemplo.cl": {}}, domain.RolColaborador},
		{"colaborador intacto fuera de ADMIN_EMAILS", "colab2@ejemplo.cl", domain.RolColaborador, map[string]struct{}{"otro@ejemplo.cl": {}}, domain.RolColaborador},
		{"admin existente no se degrada si sale de ADMIN_EMAILS", "jefe@ejemplo.cl", domain.RolAdmin, map[string]struct{}{"otro@ejemplo.cl": {}}, domain.RolAdmin},
	}

	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			repositorio := newMockUsuarioRepository()
			sujetoGoogle := "google-existente"
			usuarioInicial := &domain.Usuario{
				ID:             uuid.New(),
				Correo:         caso.correo,
				NombreCompleto: "Cuenta Existente",
				GoogleID:       &sujetoGoogle,
				Rol:            caso.rolInicial,
				EstaActivo:     true,
			}
			repositorio.usuarios[caso.correo] = usuarioInicial

			servicio := services.NuevoServicioAuth(repositorio, func(context.Context, string, string) (*idtoken.Payload, error) {
				return googlePayload(sujetoGoogle, caso.correo), nil
			}, clienteGoogleDePrueba, plazoGoogleDePrueba, caso.correosAdmin)

			enrutador := gin.New()
			enrutador.POST("/api/v1/auth/google", api.NuevoManejadorAuth(servicio, false, claveJWTDePrueba).LoginGoogleUsuario)

			escritor := httptest.NewRecorder()
			peticion := httptest.NewRequest(http.MethodPost, "/api/v1/auth/google", strings.NewReader(`{"id_token":"fixture"}`))
			peticion.Header.Set("Content-Type", "application/json")
			enrutador.ServeHTTP(escritor, peticion)

			if escritor.Code != http.StatusOK {
				prueba.Fatalf("esperaba 200, obtuve %d: %s", escritor.Code, escritor.Body.String())
			}

			var respuesta api.RespuestaLogin
			if errorLectura := json.Unmarshal(escritor.Body.Bytes(), &respuesta); errorLectura != nil {
				prueba.Fatalf("respuesta JSON inválida: %v", errorLectura)
			}
			if respuesta.Rol != caso.rolEsperado {
				prueba.Fatalf("rol en respuesta: esperaba %q, obtuve %q", caso.rolEsperado, respuesta.Rol)
			}
			if repositorio.usuarios[caso.correo].Rol != caso.rolEsperado {
				prueba.Fatalf("rol en BD: esperaba %q, obtuve %q", caso.rolEsperado, repositorio.usuarios[caso.correo].Rol)
			}

			var cookieJWT *http.Cookie
			for _, cookie := range escritor.Result().Cookies() {
				if cookie.Name == "jwt" {
					cookieJWT = cookie
					break
				}
			}
			if cookieJWT == nil {
				prueba.Fatal("no se emitió cookie jwt")
			}
			tokenValidado, errorToken := api.ValidarToken(cookieJWT.Value, claveJWTDePrueba)
			if errorToken != nil {
				prueba.Fatalf("token inválido: %v", errorToken)
			}
			datosToken, valido := tokenValidado.Claims.(*api.DatosSesion)
			if !valido || datosToken.Rol != caso.rolEsperado {
				prueba.Fatalf("rol en JWT: esperaba %q, obtuve %+v", caso.rolEsperado, tokenValidado.Claims)
			}
		})
	}
}
