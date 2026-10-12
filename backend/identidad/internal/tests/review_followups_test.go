package tests

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/middleware"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/api/idtoken"
)

// W4 (#99): con una cookie robada no se puede cambiar correo ni contraseña.
func TestActualizarPerfilExigeContrasenaActual(t *testing.T) {
	repo := newMockUsuarioRepository()
	service := services.NewAuthService(repo)
	hash, _ := security.HashPassword("Password123#")
	local := &domain.Usuario{ID: uuid.New(), Correo: "local@uct.cl", PasswordHash: &hash, NombreCompleto: "Local", EstaActivo: true}
	googleID := "google-only"
	sso := &domain.Usuario{ID: uuid.New(), Correo: "sso@uct.cl", GoogleID: &googleID, NombreCompleto: "SSO", EstaActivo: true}
	repo.usuarios[local.Correo] = local
	repo.usuarios[sso.Correo] = sso

	nueva := "OtraClaveSegura2026!"
	correo := "nuevo@uct.cl"
	mismoCorreo := " LOCAL@uct.cl "
	incorrecta := "NoEsLaClave123#"
	correcta := "Password123#"
	for _, tc := range []struct {
		name    string
		user    *domain.Usuario
		input   domain.ActualizarPerfilDTO
		wantErr error
	}{
		{"contraseña sin actual", local, domain.ActualizarPerfilDTO{Password: &nueva}, services.ErrReautenticacionRequerida},
		{"correo sin actual", local, domain.ActualizarPerfilDTO{Correo: &correo}, services.ErrReautenticacionRequerida},
		{"actual incorrecta", local, domain.ActualizarPerfilDTO{Password: &nueva, PasswordActual: &incorrecta}, services.ErrReautenticacionRequerida},
		{"cuenta Google sin contraseña", sso, domain.ActualizarPerfilDTO{Password: &nueva, PasswordActual: &correcta}, services.ErrReautenticacionRequerida},
		{"mismo correo no es un cambio", local, domain.ActualizarPerfilDTO{Correo: &mismoCorreo}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			antes := tc.user.PasswordHash
			_, err := service.ActualizarPerfil(context.Background(), tc.user.ID.String(), tc.input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error %v; esperado %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && tc.user.PasswordHash != antes {
				t.Fatal("la contraseña cambió sin reautenticación")
			}
		})
	}
	if repo.usuarios["local@uct.cl"] == nil {
		t.Fatal("el correo cambió sin reautenticación")
	}
}

func googlePayload(subject, email string) *idtoken.Payload {
	return &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: subject,
		Claims: map[string]any{"email": email, "email_verified": true, "name": "Fixture"}}
}

// W1 (#99), medida provisoria: la primera vinculación con Google invalida la
// contraseña local, porque el registro local nunca verificó el correo.
func TestGoogleVinculaCuentaLocalEInvalidaContrasena(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "fixture-google-client")
	repo := newMockUsuarioRepository()
	hash, _ := security.HashPassword("Atacante2026#")
	victima := &domain.Usuario{ID: uuid.New(), Correo: "victima@uct.cl", PasswordHash: &hash, NombreCompleto: "Pre-registro", EstaActivo: true}
	repo.usuarios[victima.Correo] = victima
	s := services.NewAuthServiceWithGoogleValidator(repo, func(context.Context, string, string) (*idtoken.Payload, error) {
		return googlePayload("google-victima", "victima@uct.cl"), nil
	})

	user, err := s.GoogleLogin(context.Background(), "token")
	if err != nil {
		t.Fatal(err)
	}
	if user.GoogleID == nil || *user.GoogleID != "google-victima" || user.PasswordHash != nil {
		t.Fatal("la cuenta no quedó vinculada con la contraseña invalidada")
	}
	if victima.PasswordHash != nil || victima.GoogleID == nil {
		t.Fatal("la vinculación no se persistió")
	}
	if _, err := s.LoginWithContext(context.Background(), "victima@uct.cl", "Atacante2026#"); !errors.Is(err, services.ErrCredencialesInvalidas) {
		t.Fatal("la contraseña pre-registrada sigue sirviendo:", err)
	}
	// Logins posteriores con la misma identidad no vuelven a escribir.
	if _, err := s.GoogleLogin(context.Background(), "token"); err != nil {
		t.Fatal(err)
	}
}

func TestGoogleRechazaOtraIdentidadVinculada(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "fixture-google-client")
	repo := newMockUsuarioRepository()
	vinculado := "google-original"
	hash, _ := security.HashPassword("Password123#")
	user := &domain.Usuario{ID: uuid.New(), Correo: "linked@uct.cl", GoogleID: &vinculado, PasswordHash: &hash, NombreCompleto: "Linked", EstaActivo: true}
	repo.usuarios[user.Correo] = user
	s := services.NewAuthServiceWithGoogleValidator(repo, func(context.Context, string, string) (*idtoken.Payload, error) {
		return googlePayload("google-otro", "linked@uct.cl"), nil
	})
	if _, err := s.GoogleLogin(context.Background(), "token"); !errors.Is(err, services.ErrCredencialesInvalidas) {
		t.Fatal("se aceptó otra identidad Google:", err)
	}
	if user.PasswordHash == nil || *user.GoogleID != vinculado {
		t.Fatal("un rechazo no debe modificar la cuenta")
	}
}

// W3 (#99): los logins exitosos no consumen intentos; los fallidos sí.
// Requiere Redis real: IDENTIDAD_TEST_REDIS_URL (p. ej. redis://localhost:6379/15).
func TestRateLimitLoginSoloCuentaFallidos(t *testing.T) {
	url := os.Getenv("IDENTIDAD_TEST_REDIS_URL")
	if url == "" {
		t.Skip("IDENTIDAD_TEST_REDIS_URL no definido")
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := "rate_limit:login:192.0.2.10"
	if err := rdb.Del(ctx, key).Err(); err != nil {
		t.Skip("Redis no disponible:", err)
	}
	defer rdb.Del(context.Background(), key)

	gin.SetMode(gin.TestMode)
	status := http.StatusOK
	r := gin.New()
	r.POST("/login", middleware.RateLimitLogin(rdb), func(c *gin.Context) { c.Status(status) })
	login := func() int {
		req := httptest.NewRequest("POST", "/login", strings.NewReader("{}"))
		req.RemoteAddr = "192.0.2.10:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	for i := 0; i < 20; i++ {
		if code := login(); code != http.StatusOK {
			t.Fatalf("login exitoso %d bloqueado con %d", i+1, code)
		}
	}
	status = http.StatusUnauthorized
	for i := 0; i < 5; i++ {
		if code := login(); code != http.StatusUnauthorized {
			t.Fatalf("fallo %d: %d", i+1, code)
		}
	}
	if code := login(); code != http.StatusTooManyRequests {
		t.Fatalf("el sexto fallo debe bloquear: %d", code)
	}
	status = http.StatusOK
	if code := login(); code != http.StatusTooManyRequests {
		t.Fatalf("con el límite alcanzado tampoco entra un login correcto: %d", code)
	}
	if ttl := rdb.PTTL(ctx, key).Val(); ttl <= 0 {
		t.Fatal("la clave quedó sin TTL")
	}
}
