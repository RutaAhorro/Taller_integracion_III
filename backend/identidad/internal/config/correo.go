package config

import (
	"errors"
	"net/mail"
	"os"
	"strconv"
	"strings"
)

// Modos de envío de correo de Identidad (SUP-189).
const (
	CorreoDeshabilitado = "deshabilitado"
	CorreoSMTP          = "smtp"
	// CorreoLog imprime los mensajes completos en el log: sólo desarrollo local.
	CorreoLog = "log"
)

// Modos TLS admitidos. No existe modo sin cifrado: las credenciales nunca viajan en claro.
const (
	TLSStartTLS  = "starttls"
	TLSImplicito = "tls"
)

// Correo configura el transporte SMTP. Clave nunca se registra ni aparece en errores.
type Correo struct {
	Modo, Host, TLS, Usuario, Clave, Remitente string
	Puerto                                     int
}

// LoadCorreo lee sólo el correo, para que `identidad correo-prueba` no exija BD ni Redis.
// Sin IDENTIDAD_CORREO_MODO, hay SMTP cuando existen credenciales: así el Deployment
// arranca con correo deshabilitado mientras el Secret identidad-smtp no exista.
func LoadCorreo() (Correo, error) {
	c := Correo{
		Modo:      strings.TrimSpace(os.Getenv("IDENTIDAD_CORREO_MODO")),
		Host:      strings.TrimSpace(os.Getenv("SMTP_HOST")),
		Usuario:   os.Getenv("SMTP_USER"),
		Clave:     os.Getenv("SMTP_PASS"),
		Remitente: strings.TrimSpace(os.Getenv("SMTP_FROM")),
	}
	if c.Modo == "" {
		c.Modo = CorreoDeshabilitado
		if c.Usuario != "" || c.Clave != "" {
			c.Modo = CorreoSMTP
		}
	}
	switch c.Modo {
	case CorreoDeshabilitado, CorreoLog:
		return Correo{Modo: c.Modo}, nil
	case CorreoSMTP:
		// Continúa con las validaciones de SMTP más abajo.
	default:
		return Correo{}, errors.New("IDENTIDAD_CORREO_MODO debe ser smtp, log o deshabilitado")
	}
	if c.Host == "" || strings.ContainsAny(c.Host, " \t\r\n/@") {
		return Correo{}, errors.New("SMTP_HOST requiere un nombre de servidor sin esquema ni espacios")
	}
	c.Puerto = 587
	if raw := strings.TrimSpace(os.Getenv("SMTP_PORT")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 65535 {
			return Correo{}, errors.New("SMTP_PORT debe estar entre 1 y 65535")
		}
		c.Puerto = n
	}
	c.TLS = strings.TrimSpace(os.Getenv("SMTP_TLS"))
	if c.TLS == "" {
		c.TLS = TLSStartTLS
		if c.Puerto == 465 {
			c.TLS = TLSImplicito
		}
	}
	if c.TLS != TLSStartTLS && c.TLS != TLSImplicito {
		return Correo{}, errors.New("SMTP_TLS debe ser starttls o tls")
	}
	if c.Usuario == "" || c.Clave == "" || strings.ContainsAny(c.Usuario, "\r\n") {
		return Correo{}, errors.New("SMTP_USER y SMTP_PASS son obligatorios en modo smtp")
	}
	if c.Remitente == "" {
		c.Remitente = c.Usuario
	}
	if _, err := mail.ParseAddress(c.Remitente); err != nil || strings.ContainsAny(c.Remitente, "\r\n") {
		return Correo{}, errors.New("SMTP_FROM requiere una dirección de correo válida")
	}
	return c, nil
}
