package config

import (
	"strings"
	"testing"
)

func sinCorreo(t *testing.T) {
	t.Helper()
	for _, key := range []string{"IDENTIDAD_CORREO_MODO", "SMTP_HOST", "SMTP_PORT", "SMTP_TLS", "SMTP_USER", "SMTP_PASS", "SMTP_FROM"} {
		t.Setenv(key, "")
	}
}

func conSMTP(t *testing.T) {
	t.Helper()
	sinCorreo(t)
	t.Setenv("SMTP_HOST", "smtp.gmail.com")
	t.Setenv("SMTP_USER", "emisor@example.com")
	t.Setenv("SMTP_PASS", "fixture-private")
}

// El ConfigMap trae SMTP_HOST antes de que exista el Secret: Identidad debe arrancar igual.
func TestCorreoSinCredencialesQuedaDeshabilitado(t *testing.T) {
	sinCorreo(t)
	t.Setenv("SMTP_HOST", "smtp.gmail.com")
	c, err := LoadCorreo()
	if err != nil || c.Modo != CorreoDeshabilitado || c.Clave != "" {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestCorreoSMTPConDefaults(t *testing.T) {
	conSMTP(t)
	c, err := LoadCorreo()
	if err != nil || c.Modo != CorreoSMTP || c.Puerto != 587 || c.TLS != TLSStartTLS || c.Remitente != "emisor@example.com" {
		t.Fatalf("%+v %v", c, err)
	}
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_FROM", "RutaAhorro <emisor@example.com>")
	c, err = LoadCorreo()
	if err != nil || c.Puerto != 465 || c.TLS != TLSImplicito || c.Remitente != "RutaAhorro <emisor@example.com>" {
		t.Fatalf("465 debe usar TLS implícito: %+v %v", c, err)
	}
}

func TestCorreoLogNoExigeCredenciales(t *testing.T) {
	sinCorreo(t)
	t.Setenv("IDENTIDAD_CORREO_MODO", "log")
	if c, err := LoadCorreo(); err != nil || c.Modo != CorreoLog {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestCorreoInvalidoNoExponeClave(t *testing.T) {
	for nombre, tc := range map[string]map[string]string{
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
		t.Run(nombre, func(t *testing.T) {
			conSMTP(t)
			for key, value := range tc {
				t.Setenv(key, value)
			}
			if c, err := LoadCorreo(); err == nil || strings.Contains(err.Error(), "fixture-private") {
				t.Fatalf("config aceptada o clave expuesta: %+v %v", c, err)
			}
		})
	}
}

func TestLoadIncluyeCorreoYFallaSiEsInvalido(t *testing.T) {
	valid(t)
	conSMTP(t)
	c, err := Load()
	if err != nil || c.Correo.Modo != CorreoSMTP {
		t.Fatalf("%+v %v", c.Correo.Modo, err)
	}
	t.Setenv("SMTP_TLS", "none")
	if _, err := Load(); err == nil {
		t.Fatal("Load aceptó un transporte SMTP sin cifrado")
	}
}
