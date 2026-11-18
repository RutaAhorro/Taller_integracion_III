// Configuración: entorno obligatorio, proxies, URL de restablecimiento y
// selección del transporte de correo sin secretos en los errores.

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
	sinCorreo(prueba)
	prueba.Setenv("IDENTIDAD_URL_RESTABLECER", "")
}
func TestURLRestablecer(prueba *testing.T) {
	valid(prueba)
	prueba.Setenv("IDENTIDAD_URL_RESTABLECER", "https://frontend.example/restablecer-contrasena")
	if contexto, errorObtenido := config.Load(); errorObtenido != nil || contexto.URLRestablecer != "https://frontend.example/restablecer-contrasena" {
		prueba.Fatalf("%q %v", contexto.URLRestablecer, errorObtenido)
	}
	for _, invalida := range []string{"ftp://frontend.example/r", "/restablecer", "https://frontend.example/r?token=x", "https://frontend.example/r#x", "https://user:fixture-private@frontend.example/r", "https://frontend.example/r x"} {
		prueba.Setenv("IDENTIDAD_URL_RESTABLECER", invalida)
		if _, errorObtenido := config.Load(); errorObtenido == nil || strings.Contains(errorObtenido.Error(), "fixture-private") {
			prueba.Fatalf("%q aceptada o con secreto en el error: %v", invalida, errorObtenido)
		}
	}
}
func TestValidDefaults(prueba *testing.T) {
	valid(prueba)
	contexto, errorObtenido := config.Load()
	if errorObtenido != nil || !contexto.CookieSecure || contexto.ListenAddr != ":8080" || len(contexto.TrustedProxies) != 1 {
		prueba.Fatalf("%+v %v", contexto, errorObtenido)
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
			_, errorObtenido := config.Load()
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
	contexto, errorObtenido := config.Load()
	if errorObtenido != nil || contexto.CookieSecure || len(contexto.TrustedProxies) != 2 || len(contexto.AllowedOrigins) != 2 {
		prueba.Fatalf("%+v %v", contexto, errorObtenido)
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

// ===== Correo =====

// sinCorreo vacía todas las variables de correo.
func sinCorreo(prueba *testing.T) {
	prueba.Helper()
	for _, clave := range []string{"IDENTIDAD_CORREO_MODO", "SMTP_HOST", "SMTP_PORT", "SMTP_TLS", "SMTP_USER", "SMTP_PASS", "SMTP_FROM"} {
		prueba.Setenv(clave, "")
	}
}

// conSMTP fija una configuración SMTP completa con valores de prueba.
func conSMTP(prueba *testing.T) {
	prueba.Helper()
	sinCorreo(prueba)
	prueba.Setenv("SMTP_HOST", "smtp.gmail.com")
	prueba.Setenv("SMTP_USER", "emisor@example.com")
	prueba.Setenv("SMTP_PASS", "fixture-private")
}

// El ConfigMap trae SMTP_HOST antes de que exista el Secret: Identidad debe arrancar igual.
func TestCorreoSinCredencialesQuedaDeshabilitado(prueba *testing.T) {
	sinCorreo(prueba)
	prueba.Setenv("SMTP_HOST", "smtp.gmail.com")
	contexto, errorObtenido := config.LoadCorreo()
	if errorObtenido != nil || contexto.Modo != config.CorreoDeshabilitado || contexto.Clave != "" {
		prueba.Fatalf("%+v %v", contexto, errorObtenido)
	}
}

func TestCorreoSMTPConDefaults(prueba *testing.T) {
	conSMTP(prueba)
	contexto, errorObtenido := config.LoadCorreo()
	if errorObtenido != nil || contexto.Modo != config.CorreoSMTP || contexto.Puerto != 587 || contexto.TLS != config.TLSStartTLS || contexto.Remitente != "emisor@example.com" {
		prueba.Fatalf("%+v %v", contexto, errorObtenido)
	}
	prueba.Setenv("SMTP_PORT", "465")
	prueba.Setenv("SMTP_FROM", "RutaAhorro <emisor@example.com>")
	contexto, errorObtenido = config.LoadCorreo()
	if errorObtenido != nil || contexto.Puerto != 465 || contexto.TLS != config.TLSImplicito || contexto.Remitente != "RutaAhorro <emisor@example.com>" {
		prueba.Fatalf("465 debe usar TLS implícito: %+v %v", contexto, errorObtenido)
	}
}

func TestCorreoLogNoExigeCredenciales(prueba *testing.T) {
	sinCorreo(prueba)
	prueba.Setenv("IDENTIDAD_CORREO_MODO", "log")
	if contexto, errorObtenido := config.LoadCorreo(); errorObtenido != nil || contexto.Modo != config.CorreoLog {
		prueba.Fatalf("%+v %v", contexto, errorObtenido)
	}
}

func TestCorreoInvalidoNoExponeClave(prueba *testing.T) {
	for nombre, caso := range map[string]map[string]string{
		"modo desconocido":       {"IDENTIDAD_CORREO_MODO": "consola"},
		"smtp sin credenciales":  {"SMTP_USER": "", "SMTP_PASS": "", "IDENTIDAD_CORREO_MODO": "smtp"},
		"falta usuario":          {"SMTP_USER": ""},
		"falta clave":            {"SMTP_PASS": ""},
		"falta host":             {"SMTP_HOST": ""},
		"host con esquema":       {"SMTP_HOST": "smtp://smtp.gmail.com"},
		"puerto cero":            {"SMTP_PORT": "0"},
		"puerto fuera de rango":  {"SMTP_PORT": "65536"},
		"puerto no numérico":     {"SMTP_PORT": "587/tcp"},
		"tls sin cifrar":         {"SMTP_TLS": "none"},
		"remitente inválido":     {"SMTP_FROM": "no-es-correo"},
		"remitente con cabecera": {"SMTP_FROM": "a@example.com\r\nBcc: b@example.com"},
		"usuario con cabecera":   {"SMTP_USER": "emisor@example.com\r\nX: y"},
	} {
		prueba.Run(nombre, func(prueba *testing.T) {
			conSMTP(prueba)
			for clave, valor := range caso {
				prueba.Setenv(clave, valor)
			}
			if contexto, errorObtenido := config.LoadCorreo(); errorObtenido == nil || strings.Contains(errorObtenido.Error(), "fixture-private") {
				prueba.Fatalf("config aceptada o clave expuesta: %+v %v", contexto, errorObtenido)
			}
		})
	}
}

func TestLoadIncluyeCorreoYFallaSiEsInvalido(prueba *testing.T) {
	valid(prueba)
	conSMTP(prueba)
	contexto, errorOperacion := config.Load()
	if errorOperacion != nil || contexto.Correo.Modo != config.CorreoSMTP {
		prueba.Fatalf("%+v %v", contexto.Correo.Modo, errorOperacion)
	}
	prueba.Setenv("SMTP_TLS", "none")
	if _, errorObtenido := config.Load(); errorObtenido == nil {
		prueba.Fatal("Load aceptó un transporte SMTP sin cifrado")
	}
}
