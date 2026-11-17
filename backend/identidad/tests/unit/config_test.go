package unit_test

import (
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
)

func valid(prueba *testing.T) {
	prueba.Helper()
	for key, value := range map[string]string{"IDENTITY_DB_URL": "postgres://app_identity:fixture-private@db/test", "JWT_SECRET": strings.Repeat("j", 32), "REDIS_URL": "redis://redis:6379/0", "PORT": "", "IDENTIDAD_COOKIE_SECURE": "", "IDENTIDAD_TRUSTED_PROXIES": "172.30.0.2", "CORS_ALLOWED_ORIGINS": ""} {
		prueba.Setenv(key, value)
	}
	sinCorreo(prueba)
	prueba.Setenv("IDENTIDAD_URL_RESTABLECER", "")
}
func TestURLRestablecer(prueba *testing.T) {
	valid(prueba)
	prueba.Setenv("IDENTIDAD_URL_RESTABLECER", "https://frontend.example/restablecer-contrasena")
	if contexto, err := config.Load(); err != nil || contexto.URLRestablecer != "https://frontend.example/restablecer-contrasena" {
		prueba.Fatalf("%q %v", contexto.URLRestablecer, err)
	}
	for _, invalida := range []string{"ftp://frontend.example/r", "/restablecer", "https://frontend.example/r?token=x", "https://frontend.example/r#x", "https://user:fixture-private@frontend.example/r", "https://frontend.example/r x"} {
		prueba.Setenv("IDENTIDAD_URL_RESTABLECER", invalida)
		if _, err := config.Load(); err == nil || strings.Contains(err.Error(), "fixture-private") {
			prueba.Fatalf("%q aceptada o con secreto en el error: %v", invalida, err)
		}
	}
}
func TestValidDefaults(prueba *testing.T) {
	valid(prueba)
	contexto, err := config.Load()
	if err != nil || !contexto.CookieSecure || contexto.ListenAddr != ":8080" || len(contexto.TrustedProxies) != 1 {
		prueba.Fatalf("%+v %v", contexto, err)
	}
}
func TestInvalidConfigFailsWithoutSecrets(prueba *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"IDENTITY_DB_URL", ""}, {"IDENTITY_DB_URL", "postgres://postgres:fixture-private@db/test"}, {"IDENTITY_DB_URL", "postgres://app_scraping:fixture-private@db/test"}, {"IDENTITY_DB_URL", "postgres://app_identity@db/test"},
		{"JWT_SECRET", ""}, {"JWT_SECRET", "too-short"}, {"REDIS_URL", ""}, {"PORT", "0"}, {"PORT", "65536"}, {"PORT", "invalid"}, {"IDENTIDAD_COOKIE_SECURE", "maybe"},
		{"REDIS_URL", "redis://user:fixture-private@bad host"},
		{"IDENTIDAD_TRUSTED_PROXIES", ""},
		{"IDENTIDAD_TRUSTED_PROXIES", "0.0.0.0/0"},
		{"IDENTIDAD_TRUSTED_PROXIES", "::/0"},
		{"IDENTIDAD_TRUSTED_PROXIES", "*"}, {"IDENTIDAD_TRUSTED_PROXIES", "172.30.0.2,"}, {"CORS_ALLOWED_ORIGINS", "*"}, {"CORS_ALLOWED_ORIGINS", "null"}, {"CORS_ALLOWED_ORIGINS", "https://frontend.example/path"}, {"CORS_ALLOWED_ORIGINS", "https://user:fixture-private@frontend.example"},
	} {
		prueba.Run(tc.key+tc.value, func(prueba *testing.T) {
			valid(prueba)
			prueba.Setenv(tc.key, tc.value)
			_, err := config.Load()
			if err == nil || strings.Contains(err.Error(), "fixture-private") {
				prueba.Fatalf("config aceptada o secretos expuestos: %v", err)
			}
		})
	}
}
func TestExplicitLocalAndProxyConfig(prueba *testing.T) {
	valid(prueba)
	prueba.Setenv("IDENTIDAD_COOKIE_SECURE", "false")
	prueba.Setenv("IDENTIDAD_TRUSTED_PROXIES", "172.30.0.2,10.0.0.0/24")
	prueba.Setenv("CORS_ALLOWED_ORIGINS", "https://frontend.example,http://localhost:3000")
	contexto, err := config.Load()
	if err != nil || contexto.CookieSecure || len(contexto.TrustedProxies) != 2 || len(contexto.AllowedOrigins) != 2 {
		prueba.Fatalf("%+v %v", contexto, err)
	}
}

// TestAjustesConservanDefaults confirma que Load entrega los valores del servicio.
func TestAjustesConservanDefaults(prueba *testing.T) {
	valid(prueba)
	configuracion, errorCarga := config.Load()
	if errorCarga != nil {
		prueba.Fatal(errorCarga)
	}
	if configuracion.Ajustes != config.AjustesPredeterminados() || configuracion.Correo.Timeout != 30*time.Second {
		prueba.Fatal("los defaults operativos cambiaron")
	}
}

// TestClienteGoogleSeCargaDelEntorno comprueba que Load entrega el client ID.
func TestClienteGoogleSeCargaDelEntorno(prueba *testing.T) {
	valid(prueba)
	prueba.Setenv("GOOGLE_CLIENT_ID", "cliente-de-prueba")
	configuracion, errorCarga := config.Load()
	if errorCarga != nil || configuracion.GoogleClientID != "cliente-de-prueba" {
		prueba.Fatalf("client ID de Google no cargado: %q %v", configuracion.GoogleClientID, errorCarga)
	}
}

// TestAjustesNoSeLeenDelEntorno fija el acuerdo del barrido: plazos y límites
// son constantes del servicio y una variable heredada no los altera.
func TestAjustesNoSeLeenDelEntorno(prueba *testing.T) {
	valid(prueba)
	prueba.Setenv("IDENTIDAD_LOGIN_MAX", "999")
	prueba.Setenv("IDENTIDAD_SMTP_TIMEOUT", "1s")
	configuracion, errorCarga := config.Load()
	if errorCarga != nil || configuracion.Ajustes != config.AjustesPredeterminados() || configuracion.Correo.Timeout != config.TimeoutSMTPPredeterminado {
		prueba.Fatal("los ajustes operativos no deben depender del entorno")
	}
}
