// Configuración: entorno obligatorio, proxies y orígenes, sin secretos en los errores.

package unit_test

import (
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
)

// ===== Entorno general =====

// valid fija un entorno mínimo válido; cada prueba cambia sólo lo que evalúa.
func valid(prueba *testing.T) {
	prueba.Helper()
	for clave, valor := range map[string]string{"IDENTITY_DB_URL": "postgres://app_identity:fixture-private@db/test", "JWT_SECRET": strings.Repeat("j", 32), "REDIS_URL": "redis://redis:6379/0", "PORT": "", "IDENTIDAD_COOKIE_SECURE": "", "IDENTIDAD_TRUSTED_PROXIES": "172.30.0.2", "CORS_ALLOWED_ORIGINS": ""} {
		prueba.Setenv(clave, valor)
	}
}
func TestValidDefaults(prueba *testing.T) {
	valid(prueba)
	configuracion, errorObtenido := config.Cargar()
	if errorObtenido != nil || !configuracion.CookieSegura || configuracion.DireccionEscucha != ":8080" || len(configuracion.ProxiesConfiables) != 1 {
		prueba.Fatalf("%+v %v", configuracion, errorObtenido)
	}
}
func TestInvalidConfigFailsWithoutSecrets(prueba *testing.T) {
	for _, caso := range []struct{ key, value string }{
		{"IDENTITY_DB_URL", ""}, {"IDENTITY_DB_URL", "postgres://postgres:fixture-private@db/test"}, {"IDENTITY_DB_URL", "postgres://app_scraping:fixture-private@db/test"}, {"IDENTITY_DB_URL", "postgres://app_identity@db/test"},
		{"JWT_SECRET", ""}, {"JWT_SECRET", "too-short"}, {"REDIS_URL", ""}, {"PORT", "0"}, {"PORT", "65536"}, {"PORT", "invalid"}, {"IDENTIDAD_COOKIE_SECURE", "maybe"},
		{"REDIS_URL", "redis://user:fixture-private@bad host"},
		{"IDENTIDAD_TRUSTED_PROXIES", ""},
		{"IDENTIDAD_TRUSTED_PROXIES", "0.0.0.0/0"},
		{"IDENTIDAD_TRUSTED_PROXIES", "::/0"},
		{"IDENTIDAD_TRUSTED_PROXIES", "*"}, {"IDENTIDAD_TRUSTED_PROXIES", "172.30.0.2,"}, {"CORS_ALLOWED_ORIGINS", "*"}, {"CORS_ALLOWED_ORIGINS", "null"}, {"CORS_ALLOWED_ORIGINS", "https://frontend.example/path"}, {"CORS_ALLOWED_ORIGINS", "https://user:fixture-private@frontend.example"},
	} {
		prueba.Run(caso.key+caso.value, func(prueba *testing.T) {
			valid(prueba)
			prueba.Setenv(caso.key, caso.value)
			_, errorObtenido := config.Cargar()
			if errorObtenido == nil || strings.Contains(errorObtenido.Error(), "fixture-private") {
				prueba.Fatalf("config aceptada o secretos expuestos: %v", errorObtenido)
			}
		})
	}
}
func TestExplicitLocalAndProxyConfig(prueba *testing.T) {
	valid(prueba)
	prueba.Setenv("IDENTIDAD_COOKIE_SECURE", "false")
	prueba.Setenv("IDENTIDAD_TRUSTED_PROXIES", "172.30.0.2,10.0.0.0/24")
	prueba.Setenv("CORS_ALLOWED_ORIGINS", "https://frontend.example,http://localhost:3000")
	configuracion, errorObtenido := config.Cargar()
	if errorObtenido != nil || configuracion.CookieSegura || len(configuracion.ProxiesConfiables) != 2 || len(configuracion.OrigenesPermitidos) != 2 {
		prueba.Fatalf("%+v %v", configuracion, errorObtenido)
	}
}

// TestClienteGoogleSeCargaDelEntorno comprueba que Load entrega el client ID.
func TestClienteGoogleSeCargaDelEntorno(prueba *testing.T) {
	valid(prueba)
	prueba.Setenv("GOOGLE_CLIENT_ID", "cliente-de-prueba")
	configuracion, errorCarga := config.Cargar()
	if errorCarga != nil || configuracion.ClienteGoogle != "cliente-de-prueba" {
		prueba.Fatalf("client ID de Google no cargado: %q %v", configuracion.ClienteGoogle, errorCarga)
	}
}
