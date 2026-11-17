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

func (errorSMTP *ErrorSMTP) Error() string {
	return "smtp " + errorSMTP.Etapa + ": " + errorSMTP.Err.Error()
}
func (errorSMTP *ErrorSMTP) Unwrap() error { return errorSMTP.Err }

func NuevoSMTP(cfg config.Correo) (*SMTP, error) {
	plazoEnvio := cfg.Timeout
	if plazoEnvio == 0 {
		plazoEnvio = config.TimeoutSMTPPredeterminado
	}
	remitente, err := mail.ParseAddress(cfg.Remitente)
	if err != nil {
		return nil, errors.New("SMTP_FROM requiere una dirección de correo válida")
	}
	return &SMTP{
		direccion:  net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Puerto)),
		servidor:   cfg.Host,
		implicito:  cfg.TLS == config.TLSImplicito,
		usuario:    cfg.Usuario,
		clave:      cfg.Clave,
		remitente:  remitente,
		cfgTLS:     &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12},
		ahora:      time.Now,
		plazoEnvio: plazoEnvio,
	}, nil
}

// Enviar sigue el diálogo SMTP paso a paso; cada error nombra la etapa en que falló.
func (transporte *SMTP) Enviar(ctx context.Context, mensaje Mensaje) error {
	para, err := validar(mensaje)
	if err != nil {
		return err
	}
	datos, err := construir(transporte.remitente, para, mensaje, transporte.ahora())
	if err != nil {
		return err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, transporte.plazoEnvio)
		defer cancel()
	}
	var dialer net.Dialer
	crudo, err := dialer.DialContext(ctx, "tcp", transporte.direccion)
	if err != nil {
		return &ErrorSMTP{"conexion", err}
	}
	defer crudo.Close()
	// net/smtp no recibe contexto: el deadline del socket y AfterFunc cortan un servidor colgado.
	deadline, _ := ctx.Deadline()
	_ = crudo.SetDeadline(deadline)
	defer context.AfterFunc(ctx, func() { _ = crudo.Close() })()
	conn := crudo
	if transporte.implicito {
		seguro := tls.Client(conn, transporte.cfgTLS)
		if err := seguro.HandshakeContext(ctx); err != nil {
			return &ErrorSMTP{"tls", err}
		}
		conn = seguro
	}
	clienteSMTP, err := smtp.NewClient(conn, transporte.servidor)
	if err != nil {
		return &ErrorSMTP{"saludo", err}
	}
	defer clienteSMTP.Close()
	if err := clienteSMTP.Hello(nombreLocal()); err != nil {
		return &ErrorSMTP{"saludo", err}
	}
	if !transporte.implicito {
		if ok, _ := clienteSMTP.Extension("STARTTLS"); !ok {
			return &ErrorSMTP{"starttls", errors.New("el servidor no ofrece STARTTLS; no se envían credenciales sin cifrar")}
		}
		if err := clienteSMTP.StartTLS(transporte.cfgTLS); err != nil {
			return &ErrorSMTP{"starttls", err}
		}
	}
	if err := clienteSMTP.Auth(smtp.PlainAuth("", transporte.usuario, transporte.clave, transporte.servidor)); err != nil {
		return &ErrorSMTP{"autenticacion", err}
	}
	if err := clienteSMTP.Mail(transporte.remitente.Address); err != nil {
		return &ErrorSMTP{"remitente", err}
	}
	if err := clienteSMTP.Rcpt(para.Address); err != nil {
		return &ErrorSMTP{"destinatario", err}
	}
	escritor, err := clienteSMTP.Data()
	if err != nil {
		return &ErrorSMTP{"envio", err}
	}
	if _, err := escritor.Write(datos); err != nil {
		return &ErrorSMTP{"envio", err}
	}
	if err := escritor.Close(); err != nil {
		return &ErrorSMTP{"envio", err}
	}
	// El servidor ya aceptó el mensaje; un QUIT fallido no lo deshace.
	_ = clienteSMTP.Quit()
	return nil
}

// nombreLocal identifica al pod en EHLO en lugar del genérico "localhost".
func nombreLocal() string {
	if nombre, err := os.Hostname(); err == nil && nombre != "" && !strings.ContainsAny(nombre, " \t\r\n") {
		return nombre
	}
	return "localhost"
}
