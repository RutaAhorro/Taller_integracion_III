package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/api/idtoken"
)

func TestProfileNameValidationPreservesStoredName(t *testing.T) {
	for _, name := range []string{"@@@;;;", " \t ", "@ @"} {
		t.Run(name, func(t *testing.T) {
			repo := newMockUsuarioRepository()
			user := &domain.Usuario{ID: uuid.New(), Correo: "profile@example.test", NombreCompleto: "Original"}
			repo.usuarios[user.Correo] = user
			h := handlers.NewAuthHandler(services.NewAuthService(repo))
			r := gin.New()
			r.PUT("/profile", func(c *gin.Context) { c.Set("user_id", user.ID.String()) }, h.ActualizarPerfil)
			body, _ := json.Marshal(map[string]string{"nombre_completo": name})
			request := httptest.NewRequest(http.MethodPut, "/profile", strings.NewReader(string(body)))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			r.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || user.NombreCompleto != "Original" {
				t.Fatalf("status=%d name=%q body=%s", response.Code, user.NombreCompleto, response.Body)
			}
		})
	}
}

func TestNameSanitizationPreservesUnicodeBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"multibyte boundary", strings.Repeat("a", 99) + "ñz", strings.Repeat("a", 99) + "ñ"},
		{"one hundred accents", strings.Repeat("á", 101), strings.Repeat("á", 100)},
		{"allowed alphabet", "  María Ñúñez-23  ", "María Ñúñez-23"},
		{"invalid UTF8", "A\xffB", "AB"},
		{"filtered whitespace", "@ @", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := utils.SanitizarInputBusqueda(tc.input)
			if got != tc.want || !utf8.ValidString(got) {
				t.Fatalf("got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestGoogleAvatarFitsPostgresWithoutBreakingLogin(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "fixture-client")
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
		t.Run(tc.name, func(t *testing.T) {
			repo := newMockUsuarioRepository()
			s := services.NewAuthServiceWithGoogleValidator(repo, func(context.Context, string, string) (*idtoken.Payload, error) {
				return &idtoken.Payload{Issuer: "https://accounts.google.com", Subject: "fixture-subject", Claims: map[string]any{
					"email": "avatar@example.test", "email_verified": true, "name": "Fixture", "picture": tc.picture,
				}}, nil
			})
			user, err := s.GoogleLogin(t.Context(), "fixture")
			if err != nil || user == nil || repo.usuarios[user.Correo] != user {
				t.Fatalf("login or persistence failed: %v", err)
			}
			if (user.URLAvatar != nil) != tc.keep {
				t.Fatalf("unexpected avatar: %v", user.URLAvatar)
			}
			if tc.keep && *user.URLAvatar != tc.picture.(string) {
				t.Fatal("avatar URL was truncated")
			}
		})
	}
}
