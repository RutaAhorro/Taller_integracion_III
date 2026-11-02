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

// plazoEnvio acota un envío cuyo contexto no trae deadline.
const plazoEnvio = 30 * time.Second

// SMTP envía por STARTTLS (587) o TLS implícito (465) y nunca autentica sin cifrado.
type SMTP struct {
	direccion, servidor string
	implicito           bool
	usuario, clave      string
	remitente           *mail.Address
	cfgTLS              *tls.Config
	ahora               func() time.Time
}

// ErrorSMTP nombra la etapa que falló: "conexion" apunta a salida bloqueada hacia el
// puerto; "autenticacion", a la App Password.
type ErrorSMTP struct {
	Etapa string
	Err   error
}

func (e *ErrorSMTP) Error() string { return "smtp " + e.Etapa + ": " + e.Err.Error() }
func (e *ErrorSMTP) Unwrap() error { return e.Err }

func NuevoSMTP(cfg config.Correo) (*SMTP, error) {
	remitente, err := mail.ParseAddress(cfg.Remitente)
	if err != nil {
		return nil, errors.New("SMTP_FROM requiere una dirección de correo válida")
	}
	return &SMTP{
		direccion: net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Puerto)),
		servidor:  cfg.Host,
		implicito: cfg.TLS == config.TLSImplicito,
		usuario:   cfg.Usuario,
		clave:     cfg.Clave,
		remitente: remitente,
		cfgTLS:    &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12},
		ahora:     time.Now,
	}, nil
}

// Enviar sigue el diálogo SMTP paso a paso; cada error nombra la etapa en que falló.
func (s *SMTP) Enviar(ctx context.Context, m Mensaje) error {
	para, err := validar(m)
	if err != nil {
		return err
	}
	datos, err := construir(s.remitente, para, m, s.ahora())
	if err != nil {
		return err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, plazoEnvio)
		defer cancel()
	}
	var dialer net.Dialer
	crudo, err := dialer.DialContext(ctx, "tcp", s.direccion)
	if err != nil {
		return &ErrorSMTP{"conexion", err}
	}
	defer crudo.Close()
	// net/smtp no recibe contexto: el deadline del socket y AfterFunc cortan un servidor colgado.
	deadline, _ := ctx.Deadline()
	_ = crudo.SetDeadline(deadline)
	defer context.AfterFunc(ctx, func() { _ = crudo.Close() })()
	conn := crudo
	if s.implicito {
		seguro := tls.Client(conn, s.cfgTLS)
		if err := seguro.HandshakeContext(ctx); err != nil {
			return &ErrorSMTP{"tls", err}
		}
		conn = seguro
	}
	c, err := smtp.NewClient(conn, s.servidor)
	if err != nil {
		return &ErrorSMTP{"saludo", err}
	}
	defer c.Close()
	if err := c.Hello(nombreLocal()); err != nil {
		return &ErrorSMTP{"saludo", err}
	}
	if !s.implicito {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return &ErrorSMTP{"starttls", errors.New("el servidor no ofrece STARTTLS; no se envían credenciales sin cifrar")}
		}
		if err := c.StartTLS(s.cfgTLS); err != nil {
			return &ErrorSMTP{"starttls", err}
		}
	}
	if err := c.Auth(smtp.PlainAuth("", s.usuario, s.clave, s.servidor)); err != nil {
		return &ErrorSMTP{"autenticacion", err}
	}
	if err := c.Mail(s.remitente.Address); err != nil {
		return &ErrorSMTP{"remitente", err}
	}
	if err := c.Rcpt(para.Address); err != nil {
		return &ErrorSMTP{"destinatario", err}
	}
	w, err := c.Data()
	if err != nil {
		return &ErrorSMTP{"envio", err}
	}
	if _, err := w.Write(datos); err != nil {
		return &ErrorSMTP{"envio", err}
	}
	if err := w.Close(); err != nil {
		return &ErrorSMTP{"envio", err}
	}
	// El servidor ya aceptó el mensaje; un QUIT fallido no lo deshace.
	_ = c.Quit()
	return nil
}

// nombreLocal identifica al pod en EHLO en lugar del genérico "localhost".
func nombreLocal() string {
	if nombre, err := os.Hostname(); err == nil && nombre != "" && !strings.ContainsAny(nombre, " \t\r\n") {
		return nombre
	}
	return "localhost"
}
