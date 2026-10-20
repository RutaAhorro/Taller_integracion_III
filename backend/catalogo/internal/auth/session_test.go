package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPValidatorRevalidaCookie(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		wantErr error
		role    string
	}{
		{"activa", http.StatusOK, `{"user_id":"u-1","rol":"admin"}`, nil, "admin"},
		{"inactiva", http.StatusUnauthorized, `{}`, ErrSesionInactiva, ""},
		{"sin permiso", http.StatusForbidden, `{}`, ErrSesionInactiva, ""},
		{"identidad caída", http.StatusBadGateway, `{}`, ErrSesionNoDisponible, ""},
		{"respuesta inválida", http.StatusOK, `{`, ErrSesionNoDisponible, ""},
		{"sin rol", http.StatusOK, `{"user_id":"u-1"}`, ErrSesionNoDisponible, ""},
		{"sin user_id", http.StatusOK, `{"rol":"admin"}`, ErrSesionNoDisponible, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotCookie string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotCookie = r.Header.Get("Cookie")
				if r.URL.Path != "/api/v1/auth/me" {
					t.Errorf("ruta inesperada: %s", r.URL.Path)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			validator, err := NewHTTPValidator(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			session, err := validator.Validate(context.Background(), "jwt=fixture")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error %v; esperado %v", err, tc.wantErr)
			}
			if gotCookie != "jwt=fixture" {
				t.Fatalf("cookie no reenviada: %q", gotCookie)
			}
			if tc.role != "" && (session == nil || session.Rol != tc.role) {
				t.Fatalf("rol inesperado: %#v", session)
			}
		})
	}
}

func TestNewHTTPValidatorRejectsInvalidOrigin(t *testing.T) {
	for _, origin := range []string{"", "ftp://identidad", "http://identidad/api", "http://user@identidad", "http://identidad?x=1", "http://identidad#x"} {
		if _, err := NewHTTPValidator(origin); err == nil {
			t.Errorf("se aceptó origen inválido: %q", origin)
		}
	}
}
