// Configuración del transporte de correo: modo, servidor SMTP, TLS y credenciales.

package config

import (
	"errors"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"
)

// ===== Constantes =====

// TimeoutSMTPPredeterminado acota cada envío por el transporte SMTP.
const TimeoutSMTPPredeterminado = 30 * time.Second

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
	Timeout                                    time.Duration
}

// ===== Carga =====

// LoadCorreo lee y valida sólo la configuración de correo; Load la incorpora al arrancar.
// Sin IDENTIDAD_CORREO_MODO, hay SMTP cuando existen credenciales: así el Deployment
// arranca con correo deshabilitado mientras el Secret identidad-smtp no exista.
func LoadCorreo() (Correo, error) {
	configuracion := Correo{
		Timeout:   TimeoutSMTPPredeterminado,
		Modo:      strings.TrimSpace(os.Getenv("IDENTIDAD_CORREO_MODO")),
		Host:      strings.TrimSpace(os.Getenv("SMTP_HOST")),
		Usuario:   os.Getenv("SMTP_USER"),
		Clave:     os.Getenv("SMTP_PASS"),
		Remitente: strings.TrimSpace(os.Getenv("SMTP_FROM")),
	}
	if configuracion.Modo == "" {
		configuracion.Modo = CorreoDeshabilitado
		if configuracion.Usuario != "" || configuracion.Clave != "" {
			configuracion.Modo = CorreoSMTP
		}
	}
	switch configuracion.Modo {
	case CorreoDeshabilitado, CorreoLog:
		return Correo{Modo: configuracion.Modo, Timeout: configuracion.Timeout}, nil
	case CorreoSMTP:
		// Continúa con las validaciones de SMTP más abajo.
	default:
		return Correo{}, errors.New("IDENTIDAD_CORREO_MODO debe ser smtp, log o deshabilitado")
	}
	if configuracion.Host == "" || strings.ContainsAny(configuracion.Host, " \t\r\n/@") {
		return Correo{}, errors.New("SMTP_HOST requiere un nombre de servidor sin esquema ni espacios")
	}
	configuracion.Puerto = 587
	if texto := strings.TrimSpace(os.Getenv("SMTP_PORT")); texto != "" {
		cantidad, errorObtenido := strconv.Atoi(texto)
		if errorObtenido != nil || cantidad < 1 || cantidad > 65535 {
			return Correo{}, errors.New("SMTP_PORT debe estar entre 1 y 65535")
		}
		configuracion.Puerto = cantidad
	}
	configuracion.TLS = strings.TrimSpace(os.Getenv("SMTP_TLS"))
	if configuracion.TLS == "" {
		configuracion.TLS = TLSStartTLS
		if configuracion.Puerto == 465 {
			configuracion.TLS = TLSImplicito
		}
	}
	if configuracion.TLS != TLSStartTLS && configuracion.TLS != TLSImplicito {
		return Correo{}, errors.New("SMTP_TLS debe ser starttls o tls")
	}
	if configuracion.Usuario == "" || configuracion.Clave == "" || strings.ContainsAny(configuracion.Usuario, "\r\n") {
		return Correo{}, errors.New("SMTP_USER y SMTP_PASS son obligatorios en modo smtp")
	}
	if configuracion.Remitente == "" {
		configuracion.Remitente = configuracion.Usuario
	}
	if _, errorObtenido := mail.ParseAddress(configuracion.Remitente); errorObtenido != nil || strings.ContainsAny(configuracion.Remitente, "\r\n") {
		return Correo{}, errors.New("SMTP_FROM requiere una dirección de correo válida")
	}
	return configuracion, nil
}
