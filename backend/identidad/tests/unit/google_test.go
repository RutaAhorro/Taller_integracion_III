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

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
)

func TestGoogleClaimsAndInactiveAccounts(prueba *testing.T) {
	for _, tc := range []struct {
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
		prueba.Run(tc.name, func(prueba *testing.T) {
			repo := newMockUsuarioRepository()
			servicio := services.NewAuthService(repo, func(ctx context.Context, token, audience string) (*idtoken.Payload, error) {
				if audience != "fixture-google-client" {
					prueba.Fatal("audience perdido")
				}
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 5*time.Second {
					prueba.Fatal("Google sin deadline")
				}
				return tc.payload, nil
			}, clienteGoogleDePrueba, plazoGoogleDePrueba)
			user, err := servicio.GoogleLogin(prueba.Context(), "fixture")
			if (err != nil) != tc.wantError {
				prueba.Fatalf("%v %v", user, err)
			}
			if !tc.wantError {
				if user.PasswordHash != nil {
					prueba.Fatal("SSO con password local")
				}
				if _, err = servicio.LoginWithContext(prueba.Context(), user.Correo, "anything"); !errors.Is(err, services.ErrCredencialesInvalidas) {
					prueba.Fatal("SSO provocó panic o aceptó password")
				}
			}
		})
	}
	for _, name := range []string{"inactive", "different google subject"} {
		prueba.Run(name, func(prueba *testing.T) {
			repo := newMockUsuarioRepository()
			subject := "another-subject"
			user := &domain.Usuario{ID: uuid.New(), Correo: "fixture@example.test", EstaActivo: true}
			if name == "inactive" {
				user.EstaActivo = false
			} else {
				user.GoogleID = &subject
			}
			repo.usuarios[user.Correo] = user
			servicio := services.NewAuthService(repo, func(context.Context, string, string) (*idtoken.Payload, error) {
				return &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "google-123", Claims: map[string]any{"email": user.Correo, "email_verified": true}}, nil
			}, clienteGoogleDePrueba, plazoGoogleDePrueba)
			if _, err := servicio.GoogleLogin(prueba.Context(), "fixture"); err == nil {
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

func (peticion *googleRaceRepository) FindByEmail(ctx context.Context, email string) (*domain.Usuario, error) {
	if !peticion.created {
		return nil, nil
	}
	return peticion.competing, nil
}

func (peticion *googleRaceRepository) Create(ctx context.Context, user *domain.Usuario) error {
	peticion.created = true
	return domain.ErrCorreoDuplicado
}

func TestGoogleConcurrentCreationRechecksAccount(prueba *testing.T) {
	for _, name := range []string{"same account", "inactive competitor", "different subject"} {
		prueba.Run(name, func(prueba *testing.T) {
			subject := "google-123"
			if name == "different subject" {
				subject = "other-google"
			}
			competing := &domain.Usuario{ID: uuid.New(), Correo: "race@example.test", GoogleID: &subject, EstaActivo: name != "inactive competitor"}
			repo := &googleRaceRepository{mockUsuarioRepository: newMockUsuarioRepository(), competing: competing}
			servicio := services.NewAuthService(repo, func(context.Context, string, string) (*idtoken.Payload, error) {
				return &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "google-123", Claims: map[string]any{"email": "race@example.test", "email_verified": true}}, nil
			}, clienteGoogleDePrueba, plazoGoogleDePrueba)
			user, err := servicio.GoogleLogin(prueba.Context(), "fixture")
			if name == "same account" {
				if err != nil || user.ID != competing.ID {
					prueba.Fatalf("%v %v", user, err)
				}
			} else if err == nil {
				prueba.Fatal("conflicto autorizó cuenta incorrecta")
			}
		})
	}
}

func googlePayload(subject, email string) *idtoken.Payload {
	return &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: subject,
		Claims: map[string]any{"email": email, "email_verified": true, "name": "Fixture"}}
}

// W1 (#99), medida provisoria: la primera vinculación con Google invalida la
// contraseña local, porque el registro local nunca verificó el correo.
func TestGoogleVinculaCuentaLocalEInvalidaContrasena(prueba *testing.T) {
	repo := newMockUsuarioRepository()
	hash, _ := security.HashPassword("Atacante2026#")
	victima := &domain.Usuario{ID: uuid.New(), Correo: "victima@uct.cl", PasswordHash: &hash, NombreCompleto: "Pre-registro", EstaActivo: true}
	repo.usuarios[victima.Correo] = victima
	servicio := services.NewAuthService(repo, func(context.Context, string, string) (*idtoken.Payload, error) {
		return googlePayload("google-victima", "victima@uct.cl"), nil
	}, clienteGoogleDePrueba, plazoGoogleDePrueba)

	user, err := servicio.GoogleLogin(context.Background(), "token")
	if err != nil {
		prueba.Fatal(err)
	}
	if user.GoogleID == nil || *user.GoogleID != "google-victima" || user.PasswordHash != nil {
		prueba.Fatal("la cuenta no quedó vinculada con la contraseña invalidada")
	}
	if victima.PasswordHash != nil || victima.GoogleID == nil {
		prueba.Fatal("la vinculación no se persistió")
	}
	if _, err := servicio.LoginWithContext(context.Background(), "victima@uct.cl", "Atacante2026#"); !errors.Is(err, services.ErrCredencialesInvalidas) {
		prueba.Fatal("la contraseña pre-registrada sigue sirviendo:", err)
	}
	// Logins posteriores con la misma identidad no vuelven a escribir.
	if _, err := servicio.GoogleLogin(context.Background(), "token"); err != nil {
		prueba.Fatal(err)
	}
}

func TestGoogleRechazaOtraIdentidadVinculada(prueba *testing.T) {
	repo := newMockUsuarioRepository()
	vinculado := "google-original"
	hash, _ := security.HashPassword("Password123#")
	user := &domain.Usuario{ID: uuid.New(), Correo: "linked@uct.cl", GoogleID: &vinculado, PasswordHash: &hash, NombreCompleto: "Linked", EstaActivo: true}
	repo.usuarios[user.Correo] = user
	servicio := services.NewAuthService(repo, func(context.Context, string, string) (*idtoken.Payload, error) {
		return googlePayload("google-otro", "linked@uct.cl"), nil
	}, clienteGoogleDePrueba, plazoGoogleDePrueba)
	if _, err := servicio.GoogleLogin(context.Background(), "token"); !errors.Is(err, services.ErrCredencialesInvalidas) {
		prueba.Fatal("se aceptó otra identidad Google:", err)
	}
	if user.PasswordHash == nil || *user.GoogleID != vinculado {
		prueba.Fatal("un rechazo no debe modificar la cuenta")
	}
}

func TestGoogleAvatarFitsPostgresWithoutBreakingLogin(prueba *testing.T) {
	for _, tc := range []struct {
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
		prueba.Run(tc.name, func(prueba *testing.T) {
			repo := newMockUsuarioRepository()
			servicio := services.NewAuthService(repo, func(context.Context, string, string) (*idtoken.Payload, error) {
				return &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "fixture-subject", Claims: map[string]any{
					"email": "avatar@example.test", "email_verified": true, "name": "Fixture", "picture": tc.picture,
				}}, nil
			}, "fixture-client", plazoGoogleDePrueba)
			user, err := servicio.GoogleLogin(prueba.Context(), "fixture")
			if err != nil || user == nil || repo.usuarios[user.Correo] != user {
				prueba.Fatalf("login or persistence failed: %v", err)
			}
			if (user.URLAvatar != nil) != tc.keep {
				prueba.Fatalf("unexpected avatar: %v", user.URLAvatar)
			}
			if tc.keep && *user.URLAvatar != tc.picture.(string) {
				prueba.Fatal("avatar URL was truncated")
			}
		})
	}
}

type googleErrorService struct {
	services.AuthService
	err error
}

func (servicio googleErrorService) GoogleLogin(context.Context, string) (*domain.Usuario, error) {
	return nil, servicio.err
}

func TestGoogleErrorClassificationDoesNotExposeStorage(prueba *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{{"token rejected", services.ErrCredencialesInvalidas, 401}, {"inactive", services.ErrCuentaInactiva, 401}, {"configuration", services.ErrGoogleNoConfigurado, 503}, {"database", errors.New("private SQL database table and constraint"), 500}} {
		prueba.Run(tc.name, func(prueba *testing.T) {
			peticion := gin.New()
			peticion.POST("/google", handlers.NewAuthHandler(googleErrorService{err: tc.err}, false, claveJWTDePrueba).GoogleLoginUsuario)
			escritor := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/google", strings.NewReader(`{"id_token":"fixture"}`))
			req.Header.Set("Content-Type", "application/json")
			peticion.ServeHTTP(escritor, req)
			if escritor.Code != tc.status || strings.Contains(escritor.Body.String(), "detalle") || strings.Contains(escritor.Body.String(), "private") {
				prueba.Fatal(escritor.Code, escritor.Body)
			}
		})
	}
}

type jwksFixture struct{ body string }

func (archivo jwksFixture) RoundTrip(peticion *http.Request) (*http.Response, error) {
	if peticion.URL.String() != "https://www.googleapis.com/oauth2/v3/certs" {
		return nil, io.ErrUnexpectedEOF
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Cache-Control": []string{"max-age=3600"}}, Body: io.NopCloser(strings.NewReader(archivo.body)), Request: peticion}, nil
}

// Usar el validador real de Google y firmas RSA; sólo la respuesta de JWKS es
// una fixture. Nunca se cambia el transporte TLS de la aplicación.
func TestGoogleCryptographicValidation(prueba *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		prueba.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		prueba.Fatal(err)
	}
	jwks, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"alg": "RS256", "kid": "fixture-key", "kty": "RSA", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
	validator, err := idtoken.NewValidator(prueba.Context(), option.WithHTTPClient(&http.Client{Transport: jwksFixture{string(jwks)}}))
	if err != nil {
		prueba.Fatal(err)
	}
	for _, tc := range []struct {
		name, aud, issuer string
		exp               int64
		signingKey        *rsa.PrivateKey
		verified          bool
		wantsError        bool
	}{
		{"valid", "fixture-google-client", "https://accounts.google.com", time.Now().Add(time.Hour).Unix(), key, true, false},
		{"wrong audience", "wrong-client", "https://accounts.google.com", time.Now().Add(time.Hour).Unix(), key, true, true},
		{"wrong issuer", "fixture-google-client", "https://evil.example", time.Now().Add(time.Hour).Unix(), key, true, true},
		{"wrong signature", "fixture-google-client", "https://accounts.google.com", time.Now().Add(time.Hour).Unix(), other, true, true},
		{"expired", "fixture-google-client", "https://accounts.google.com", time.Now().Add(-time.Minute).Unix(), key, true, true},
		{"unverified email", "fixture-google-client", "https://accounts.google.com", time.Now().Add(time.Hour).Unix(), key, false, true},
	} {
		prueba.Run(tc.name, func(prueba *testing.T) {
			claims := jwt.MapClaims{"iss": tc.issuer, "aud": tc.aud, "sub": "google-fixture", "exp": tc.exp, "iat": time.Now().Unix(), "email": "crypto@example.test", "email_verified": tc.verified, "name": "Crypto Fixture"}
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			token.Header["kid"] = "fixture-key"
			encoded, err := token.SignedString(tc.signingKey)
			if err != nil {
				prueba.Fatal(err)
			}
			repo := newMockUsuarioRepository()
			servicio := services.NewAuthService(repo, func(ctx context.Context, token, audience string) (*idtoken.Payload, error) {
				return validator.Validate(ctx, token, audience)
			}, clienteGoogleDePrueba, plazoGoogleDePrueba)
			user, err := servicio.GoogleLogin(prueba.Context(), encoded)
			if (err != nil) != tc.wantsError {
				prueba.Fatalf("%v %v", user, err)
			}
			if tc.wantsError && len(repo.usuarios) != 0 {
				prueba.Fatal("token rechazado persistió usuario")
			}
		})
	}
}
