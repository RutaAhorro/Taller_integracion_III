package tests

import (
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"strings"
	"testing"
)

// Tokens arbitrarios no pueden causar panic ni entregar una sesión incompleta.
func FuzzJWTParser(f *testing.F) {
	for _, value := range []string{"", "not-a-token", "eyJhbGciOiJub25lIn0.eyJyb2wiOiJhZG1pbiJ9.", strings.Repeat("x", 4097)} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		token, err := security.ValidarToken(value)
		if err != nil {
			return
		}
		if token == nil || !token.Valid {
			t.Fatal("token aceptado sin validez")
		}
		claims, ok := token.Claims.(*security.JWTClaims)
		if !ok || claims.ExpiresAt == nil || strings.TrimSpace(claims.UserID) == "" || strings.TrimSpace(claims.Rol) == "" || strings.TrimSpace(claims.Provider) == "" {
			t.Fatal("sesión incompleta")
		}
	})
}
