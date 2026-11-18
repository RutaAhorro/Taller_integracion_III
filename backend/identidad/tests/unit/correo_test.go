// Configuración de correo: selección de modo, SMTP y validaciones sin secretos en errores.

package unit_test

import (
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"strings"
	"testing"
)

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
