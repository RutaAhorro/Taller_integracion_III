// Transporte SMTP con STARTTLS o TLS implícito; nunca autentica sin cifrado.

package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
)

// ===== Transporte =====

// SMTP envía por STARTTLS (587) o TLS implícito (465) y nunca autentica sin cifrado.
type SMTP struct {
	direccion, servidor string
	implicito           bool
	usuario, clave      string
	remitente           *mail.Address
	cfgTLS              *tls.Config
	ahora               func() time.Time
	plazoEnvio          time.Duration
}

// ErrorSMTP nombra la etapa que falló: "conexion" apunta a salida bloqueada hacia el
// puerto; "autenticacion", a la App Password.
type ErrorSMTP struct {
	Etapa string
	Err   error
}

// Error antepone la etapa fallida al error original.
func (errorSMTP *ErrorSMTP) Error() string {
	return "smtp " + errorSMTP.Etapa + ": " + errorSMTP.Err.Error()
}

// Unwrap permite usar errors.Is/As sobre el error original.
func (errorSMTP *ErrorSMTP) Unwrap() error { return errorSMTP.Err }

// NuevoSMTP prepara el transporte a partir de la configuración validada;
// no abre conexiones hasta el primer envío.
func NuevoSMTP(configuracion config.Correo) (*SMTP, error) {
	plazoEnvio := configuracion.Timeout
	if plazoEnvio == 0 {
		plazoEnvio = config.TimeoutSMTPPredeterminado
	}
	remitente, errorObtenido := mail.ParseAddress(configuracion.Remitente)
	if errorObtenido != nil {
		return nil, errors.New("SMTP_FROM requiere una dirección de correo válida")
	}
	return &SMTP{
		direccion:  net.JoinHostPort(configuracion.Host, strconv.Itoa(configuracion.Puerto)),
		servidor:   configuracion.Host,
		implicito:  configuracion.TLS == config.TLSImplicito,
		usuario:    configuracion.Usuario,
		clave:      configuracion.Clave,
		remitente:  remitente,
		cfgTLS:     &tls.Config{ServerName: configuracion.Host, MinVersion: tls.VersionTLS12},
		ahora:      time.Now,
		plazoEnvio: plazoEnvio,
	}, nil
}

// ===== Envío =====

// Enviar sigue el diálogo SMTP paso a paso; cada error nombra la etapa en que falló.
func (transporte *SMTP) Enviar(contexto context.Context, mensaje Mensaje) error {
	para, errorObtenido := validar(mensaje)
	if errorObtenido != nil {
		return errorObtenido
	}
	datos, errorObtenido := construir(transporte.remitente, para, mensaje, transporte.ahora())
	if errorObtenido != nil {
		return errorObtenido
	}
	if _, valido := contexto.Deadline(); !valido {
		var cancelar context.CancelFunc
		contexto, cancelar = context.WithTimeout(contexto, transporte.plazoEnvio)
		defer cancelar()
	}
	var marcador net.Dialer
	crudo, errorObtenido := marcador.DialContext(contexto, "tcp", transporte.direccion)
	if errorObtenido != nil {
		return &ErrorSMTP{"conexion", errorObtenido}
	}
	defer crudo.Close()
	// net/smtp no recibe contexto: el deadline del socket y AfterFunc cortan un servidor colgado.
	plazo, _ := contexto.Deadline()
	_ = crudo.SetDeadline(plazo)
	defer context.AfterFunc(contexto, func() { _ = crudo.Close() })()
	conexion := crudo
	if transporte.implicito {
		seguro := tls.Client(conexion, transporte.cfgTLS)
		if errorOperacion := seguro.HandshakeContext(contexto); errorOperacion != nil {
			return &ErrorSMTP{"tls", errorOperacion}
		}
		conexion = seguro
	}
	clienteSMTP, errorObtenido := smtp.NewClient(conexion, transporte.servidor)
	if errorObtenido != nil {
		return &ErrorSMTP{"saludo", errorObtenido}
	}
	defer clienteSMTP.Close()
	if errorOperacion := clienteSMTP.Hello(nombreLocal()); errorOperacion != nil {
		return &ErrorSMTP{"saludo", errorOperacion}
	}
	if !transporte.implicito {
		if valido, _ := clienteSMTP.Extension("STARTTLS"); !valido {
			return &ErrorSMTP{"starttls", errors.New("el servidor no ofrece STARTTLS; no se envían credenciales sin cifrar")}
		}
		if errorOperacion := clienteSMTP.StartTLS(transporte.cfgTLS); errorOperacion != nil {
			return &ErrorSMTP{"starttls", errorOperacion}
		}
	}
	if errorOperacion := clienteSMTP.Auth(smtp.PlainAuth("", transporte.usuario, transporte.clave, transporte.servidor)); errorOperacion != nil {
		return &ErrorSMTP{"autenticacion", errorOperacion}
	}
	if errorOperacion := clienteSMTP.Mail(transporte.remitente.Address); errorOperacion != nil {
		return &ErrorSMTP{"remitente", errorOperacion}
	}
	if errorOperacion := clienteSMTP.Rcpt(para.Address); errorOperacion != nil {
		return &ErrorSMTP{"destinatario", errorOperacion}
	}
	escritor, errorObtenido := clienteSMTP.Data()
	if errorObtenido != nil {
		return &ErrorSMTP{"envio", errorObtenido}
	}
	if _, errorOperacion := escritor.Write(datos); errorOperacion != nil {
		return &ErrorSMTP{"envio", errorOperacion}
	}
	if errorOperacion := escritor.Close(); errorOperacion != nil {
		return &ErrorSMTP{"envio", errorOperacion}
	}
	// El servidor ya aceptó el mensaje; un QUIT fallido no lo deshace.
	_ = clienteSMTP.Quit()
	return nil
}

// nombreLocal identifica al pod en EHLO en lugar del genérico "localhost".
func nombreLocal() string {
	if nombre, errorObtenido := os.Hostname(); errorObtenido == nil && nombre != "" && !strings.ContainsAny(nombre, " \t\r\n") {
		return nombre
	}
	return "localhost"
}
