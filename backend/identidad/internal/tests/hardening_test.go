package tests

import (
	"context"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"google.golang.org/api/idtoken"
	"strings"
	"testing"
	"time"
)

func TestJWTRejectsUnsafeTokens(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			token, err := jwt.NewWithClaims(tc.method, tc.claims).SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = security.ValidarToken(token); err == nil {
				t.Fatal("token inseguro aceptado")
			}
		})
	}
	t.Run("no fallback", func(t *testing.T) {
		t.Setenv("JWT_SECRET", "")
		if _, err := security.GenerarToken("user", "registrado", "local"); err == nil {
			t.Fatal("emisión sin clave")
		}
		if _, err := security.ValidarToken("token"); err == nil {
			t.Fatal("validación sin clave")
		}
	})
}
func TestGoogleClaimsAndInactiveAccounts(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "fixture-google-client")
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
		t.Run(tc.name, func(t *testing.T) {
			repo := newMockUsuarioRepository()
			s := services.NewAuthServiceWithGoogleValidator(repo, func(ctx context.Context, token, audience string) (*idtoken.Payload, error) {
				if audience != "fixture-google-client" {
					t.Fatal("audience perdido")
				}
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 5*time.Second {
					t.Fatal("Google sin deadline")
				}
				return tc.payload, nil
			})
			user, err := s.GoogleLogin(t.Context(), "fixture")
			if (err != nil) != tc.wantError {
				t.Fatalf("%v %v", user, err)
			}
			if !tc.wantError {
				if user.PasswordHash != nil {
					t.Fatal("SSO con password local")
				}
				if _, err = s.LoginWithContext(t.Context(), user.Correo, "anything"); !errors.Is(err, services.ErrCredencialesInvalidas) {
					t.Fatal("SSO provocó panic o aceptó password")
				}
			}
		})
	}
	for _, name := range []string{"inactive", "different google subject"} {
		t.Run(name, func(t *testing.T) {
			repo := newMockUsuarioRepository()
			subject := "another-subject"
			user := &domain.Usuario{ID: uuid.New(), Correo: "fixture@example.test", EstaActivo: true}
			if name == "inactive" {
				user.EstaActivo = false
			} else {
				user.GoogleID = &subject
			}
			repo.usuarios[user.Correo] = user
			s := services.NewAuthServiceWithGoogleValidator(repo, func(context.Context, string, string) (*idtoken.Payload, error) {
				return &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "google-123", Claims: map[string]any{"email": user.Correo, "email_verified": true}}, nil
			})
			if _, err := s.GoogleLogin(t.Context(), "fixture"); err == nil {
				t.Fatal("cuenta no autorizada aceptada")
			}
		})
	}
}
func TestPasswordBeyondBcryptLimitIsValidationError(t *testing.T) {
	s := services.NewAuthService(newMockUsuarioRepository())
	_, err := s.Registrar(t.Context(), services.RegistroDTO{Correo: "fixture@example.test", NombreCompleto: "Fixture", Password: "A1!" + strings.Repeat("a", 70)})
	if !errors.Is(err, services.ErrPasswordInvalido) {
		t.Fatal(err)
	}
}

// El conflicto concurrente de SSO no equivale a autorización: volver a leer
// y comprobar estado/subject, incluso cuando otro request creó la cuenta.
type googleRaceRepository struct {
	*mockUsuarioRepository
	competing *domain.Usuario
	created   bool
}

func (r *googleRaceRepository) FindByEmail(ctx context.Context, email string) (*domain.Usuario, error) {
	if !r.created {
		return nil, nil
	}
	return r.competing, nil
}
func (r *googleRaceRepository) Create(ctx context.Context, user *domain.Usuario) error {
	r.created = true
	return domain.ErrCorreoDuplicado
}
func TestGoogleConcurrentCreationRechecksAccount(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "fixture-google-client")
	for _, name := range []string{"same account", "inactive competitor", "different subject"} {
		t.Run(name, func(t *testing.T) {
			subject := "google-123"
			if name == "different subject" {
				subject = "other-google"
			}
			competing := &domain.Usuario{ID: uuid.New(), Correo: "race@example.test", GoogleID: &subject, EstaActivo: name != "inactive competitor"}
			repo := &googleRaceRepository{mockUsuarioRepository: newMockUsuarioRepository(), competing: competing}
			s := services.NewAuthServiceWithGoogleValidator(repo, func(context.Context, string, string) (*idtoken.Payload, error) {
				return &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "google-123", Claims: map[string]any{"email": "race@example.test", "email_verified": true}}, nil
			})
			user, err := s.GoogleLogin(t.Context(), "fixture")
			if name == "same account" {
				if err != nil || user.ID != competing.ID {
					t.Fatalf("%v %v", user, err)
				}
			} else if err == nil {
				t.Fatal("conflicto autorizó cuenta incorrecta")
			}
		})
	}
}
