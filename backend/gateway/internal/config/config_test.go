package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	c, err := Load(func(key string) string {
		if key == "BACKEND_URL" {
			return "http://127.0.0.1:8080"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenAddr != ":8082" || c.RequestTimeout != 30*time.Second || c.ReadinessTimeout != 2*time.Second || len(c.TrustedProxies) != 0 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"BACKEND_URL", ""},
		{"BACKEND_URL", "ftp://backend:8080"},
		{"BACKEND_URL", "http://user:password@backend:8080"},
		{"BACKEND_URL", "http://backend:8080/api"},
		{"BACKEND_URL", "http://backend:8080?token=private"},
		{"BACKEND_URL", "http://backend:8080#fragment"},
		{"BACKEND_URL", "http://backend:70000"},
		{"CATALOGO_URL", "ftp://catalogo:8080"},
		{"CATALOGO_URL", "http://user:password@catalogo:8080"},
		{"CATALOGO_URL", "http://catalogo:8080/api"},
		{"CATALOGO_URL", "http://catalogo:8080?token=private"},
		{"CATALOGO_URL", "http://catalogo:8080#fragment"},
		{"CATALOGO_URL", "http://catalogo:70000"},
		{"GATEWAY_LISTEN_ADDR", "8082"},
		{"GATEWAY_LISTEN_ADDR", ":0"},
		{"GATEWAY_REQUEST_TIMEOUT", "-1s"},
		{"GATEWAY_DIAL_TIMEOUT", "0s"},
		{"GATEWAY_RESPONSE_HEADER_TIMEOUT", "soon"},
		{"GATEWAY_READINESS_TIMEOUT", "0s"},
		{"GATEWAY_TRUSTED_PROXIES", "localhost"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			_, err := Load(func(key string) string {
				if key == tc.key {
					return tc.value
				}
				if key == "BACKEND_URL" {
					return "http://backend:8080"
				}
				return ""
			})
			if err == nil {
				t.Fatal("expected configuration error")
			}
			if strings.Contains(err.Error(), "password") || strings.Contains(err.Error(), "private") {
				t.Fatalf("error exposes supplied credentials: %v", err)
			}
		})
	}
}

func TestLoadOverrides(t *testing.T) {
	env := map[string]string{
		"BACKEND_URL": "https://[::1]:8443/", "GATEWAY_LISTEN_ADDR": "127.0.0.1:9000",
		"CATALOGO_URL":            "http://catalogo:8080",
		"GATEWAY_REQUEST_TIMEOUT": "2s", "GATEWAY_TRUSTED_PROXIES": "127.0.0.1,10.0.0.0/24,::1",
	}
	c, err := Load(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenAddr != env["GATEWAY_LISTEN_ADDR"] || c.Catalogo.String() != env["CATALOGO_URL"] || c.RequestTimeout != 2*time.Second || len(c.TrustedProxies) != 3 {
		t.Fatalf("overrides not applied: %+v", c)
	}
}
