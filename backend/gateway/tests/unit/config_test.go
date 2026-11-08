package unit_test

import (
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	c, err := config.Load(func(key string) string {
		if key == "BACKEND_URL" {
			return "http://127.0.0.1:8080"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenAddr != ":8082" || c.RequestTimeout != 30*time.Second || c.ReadinessTimeout != 2*time.Second || c.KeepAliveTimeout != 30*time.Second || len(c.TrustedProxies) != 0 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if len(c.CatalogPrefixes) != 2 || len(c.IdentityPrefixes) != 2 || len(c.RoutesPrefixes) != 1 || len(c.ScrapingPrefixes) != 2 {
		t.Fatalf("unexpected default route prefixes: %+v", c)
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
		{"IDENTIDAD_URL", "ftp://identidad:8080"},
		{"IDENTIDAD_URL", "http://user:password@identidad:8080"},
		{"IDENTIDAD_URL", "http://identidad:8080/api/v1/auth"},
		{"IDENTIDAD_URL", "http://identidad:70000"},
		{"SCRAPING_URL", "ftp://control:8080"},
		{"SCRAPING_URL", "http://user:password@control:8080"},
		{"SCRAPING_URL", "http://control:8080/api"},
		{"SCRAPING_URL", "http://control:8080?token=private"},
		{"SCRAPING_URL", "http://control:8080#fragment"},
		{"SCRAPING_URL", "http://control:70000"},
		{"CATALOGO_URL", "ftp://catalogo:8080"},
		{"CATALOGO_URL", "http://user:password@catalogo:8080"},
		{"CATALOGO_URL", "http://catalogo:8080/api"},
		{"CATALOGO_URL", "http://catalogo:8080?token=private"},
		{"CATALOGO_URL", "http://catalogo:8080#fragment"},
		{"CATALOGO_URL", "http://catalogo:70000"},
		{"RUTAS_URL", "ftp://motor-rutas:8001"},
		{"RUTAS_URL", "http://user:password@motor-rutas:8001"},
		{"RUTAS_URL", "http://motor-rutas:8001/api/v1/listas"},
		{"RUTAS_URL", "http://motor-rutas:8001?token=private"},
		{"GATEWAY_LISTEN_ADDR", "8082"},
		{"GATEWAY_LISTEN_ADDR", ":0"},
		{"GATEWAY_REQUEST_TIMEOUT", "-1s"},
		{"GATEWAY_DIAL_TIMEOUT", "0s"},
		{"GATEWAY_RESPONSE_HEADER_TIMEOUT", "soon"},
		{"GATEWAY_READINESS_TIMEOUT", "0s"},
		{"GATEWAY_KEEPALIVE_TIMEOUT", "-5s"},
		{"GATEWAY_TRUSTED_PROXIES", "localhost"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			_, err := config.Load(func(key string) string {
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

func TestFinalTopologyDoesNotRequireLegacyBackend(t *testing.T) {
	env := map[string]string{"SCRAPING_URL": "http://control-scraping:8080", "IDENTIDAD_URL": "http://identidad:8080", "CATALOGO_URL": "http://catalogo:8080", "RUTAS_URL": "http://motor-rutas:8001"}
	c, err := config.Load(func(key string) string { return env[key] })
	if err != nil || c.Backend != nil || c.Scraping == nil || c.Identidad == nil || c.Catalogo == nil || c.Rutas == nil {
		t.Fatalf("topología final inválida: %v %v", c, err)
	}
}

func TestLoadOverrides(t *testing.T) {
	env := map[string]string{
		"BACKEND_URL": "https://[::1]:8443/", "GATEWAY_LISTEN_ADDR": "127.0.0.1:9000",
		"CATALOGO_URL":            "http://catalogo:8080",
		"GATEWAY_REQUEST_TIMEOUT": "2s", "GATEWAY_TRUSTED_PROXIES": "127.0.0.1,10.0.0.0/24,::1",
		"GATEWAY_KEEPALIVE_TIMEOUT": "15s",
		"GATEWAY_CATALOGO_PREFIXES": "/api/v1/custom-productos",
	}
	c, err := config.Load(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenAddr != env["GATEWAY_LISTEN_ADDR"] || c.Catalogo.String() != env["CATALOGO_URL"] || c.RequestTimeout != 2*time.Second || c.KeepAliveTimeout != 15*time.Second || len(c.TrustedProxies) != 3 {
		t.Fatalf("overrides not applied: %+v", c)
	}
	if len(c.CatalogPrefixes) != 1 || c.CatalogPrefixes[0] != "/api/v1/custom-productos" {
		t.Fatalf("custom catalog prefix not applied: %+v", c.CatalogPrefixes)
	}
}
