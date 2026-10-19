package config

import (
	"strings"
	"testing"
)

func valid(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{"IDENTITY_DB_URL": "postgres://app_identity:fixture-private@db/test", "JWT_SECRET": strings.Repeat("j", 32), "REDIS_URL": "redis://redis:6379/0", "PORT": "", "IDENTIDAD_COOKIE_SECURE": "", "IDENTIDAD_TRUSTED_PROXIES": "172.30.0.2", "CORS_ALLOWED_ORIGINS": ""} {
		t.Setenv(key, value)
	}
	sinCorreo(t)
}
func TestValidDefaults(t *testing.T) {
	valid(t)
	c, err := Load()
	if err != nil || !c.CookieSecure || c.ListenAddr != ":8080" || len(c.TrustedProxies) != 1 {
		t.Fatalf("%+v %v", c, err)
	}
}
func TestInvalidConfigFailsWithoutSecrets(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"IDENTITY_DB_URL", ""}, {"IDENTITY_DB_URL", "postgres://postgres:fixture-private@db/test"}, {"IDENTITY_DB_URL", "postgres://app_scraping:fixture-private@db/test"}, {"IDENTITY_DB_URL", "postgres://app_identity@db/test"},
		{"JWT_SECRET", ""}, {"JWT_SECRET", "too-short"}, {"REDIS_URL", ""}, {"PORT", "0"}, {"PORT", "65536"}, {"PORT", "invalid"}, {"IDENTIDAD_COOKIE_SECURE", "maybe"},
		{"REDIS_URL", "redis://user:fixture-private@bad host"},
		{"IDENTIDAD_TRUSTED_PROXIES", ""},
		{"IDENTIDAD_TRUSTED_PROXIES", "0.0.0.0/0"},
		{"IDENTIDAD_TRUSTED_PROXIES", "::/0"},
		{"IDENTIDAD_TRUSTED_PROXIES", "*"}, {"IDENTIDAD_TRUSTED_PROXIES", "172.30.0.2,"}, {"CORS_ALLOWED_ORIGINS", "*"}, {"CORS_ALLOWED_ORIGINS", "null"}, {"CORS_ALLOWED_ORIGINS", "https://frontend.example/path"}, {"CORS_ALLOWED_ORIGINS", "https://user:fixture-private@frontend.example"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			valid(t)
			t.Setenv(tc.key, tc.value)
			_, err := Load()
			if err == nil || strings.Contains(err.Error(), "fixture-private") {
				t.Fatalf("config aceptada o secretos expuestos: %v", err)
			}
		})
	}
}
func TestExplicitLocalAndProxyConfig(t *testing.T) {
	valid(t)
	t.Setenv("IDENTIDAD_COOKIE_SECURE", "false")
	t.Setenv("IDENTIDAD_TRUSTED_PROXIES", "172.30.0.2,10.0.0.0/24")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://frontend.example,http://localhost:3000")
	c, err := Load()
	if err != nil || c.CookieSecure || len(c.TrustedProxies) != 2 || len(c.AllowedOrigins) != 2 {
		t.Fatalf("%+v %v", c, err)
	}
}
