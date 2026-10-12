package tests

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"
)

type jwksFixture struct{ body string }

func (f jwksFixture) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.String() != "https://www.googleapis.com/oauth2/v3/certs" {
		return nil, io.ErrUnexpectedEOF
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Cache-Control": []string{"max-age=3600"}}, Body: io.NopCloser(strings.NewReader(f.body)), Request: r}, nil
}

// Usar el validador real de Google y firmas RSA; sólo la respuesta de JWKS es
// una fixture. Nunca se cambia el transporte TLS de la aplicación.
func TestGoogleCryptographicValidation(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "fixture-google-client")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"alg": "RS256", "kid": "fixture-key", "kty": "RSA", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
	validator, err := idtoken.NewValidator(t.Context(), option.WithHTTPClient(&http.Client{Transport: jwksFixture{string(jwks)}}))
	if err != nil {
		t.Fatal(err)
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
		t.Run(tc.name, func(t *testing.T) {
			claims := jwt.MapClaims{"iss": tc.issuer, "aud": tc.aud, "sub": "google-fixture", "exp": tc.exp, "iat": time.Now().Unix(), "email": "crypto@example.test", "email_verified": tc.verified, "name": "Crypto Fixture"}
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			token.Header["kid"] = "fixture-key"
			encoded, err := token.SignedString(tc.signingKey)
			if err != nil {
				t.Fatal(err)
			}
			repo := newMockUsuarioRepository()
			s := services.NewAuthServiceWithGoogleValidator(repo, func(ctx context.Context, token, audience string) (*idtoken.Payload, error) {
				return validator.Validate(ctx, token, audience)
			})
			user, err := s.GoogleLogin(t.Context(), encoded)
			if (err != nil) != tc.wantsError {
				t.Fatalf("%v %v", user, err)
			}
			if tc.wantsError && len(repo.usuarios) != 0 {
				t.Fatal("token rechazado persistió usuario")
			}
		})
	}
}
