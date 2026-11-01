// Package mailer despacha los correos de Identidad (verificación y recuperación de cuenta).
// Los casos de uso dependen de Mailer; config.Correo elige el transporte.
package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
)

// Mensaje es un correo para un único destinatario. Texto es obligatorio; HTML, opcional.
type Mensaje struct {
	Para, Asunto, Texto, HTML string
}

// Mailer es lo único que conocen los casos de uso: el transporte se elige en Nuevo.
type Mailer interface {
	Enviar(ctx context.Context, m Mensaje) error
}

var (
	ErrDeshabilitado   = errors.New("correo de Identidad deshabilitado")
	ErrMensajeInvalido = errors.New("mensaje de correo inválido")
)

// Nuevo elige el transporte según cfg.Modo. Sin modo conocido, el correo queda deshabilitado.
func Nuevo(cfg config.Correo, log *slog.Logger) (Mailer, error) {
	if log == nil {
		log = slog.Default()
	}
	switch cfg.Modo {
	case config.CorreoSMTP:
		s, err := NuevoSMTP(cfg)
		if err != nil {
			// No retornar s directamente: un *SMTP nil dentro de la interfaz
			// Mailer no es igual a nil y el llamador no detectaría el error.
			return nil, err
		}
		return s, nil
	case config.CorreoLog:
		return registro{log: log}, nil
	default:
		return deshabilitado{}, nil
	}
}

// MensajePrueba es el correo de `identidad correo-prueba`.
func MensajePrueba(para, origen string, ahora time.Time) Mensaje {
	return Mensaje{
		Para:   para,
		Asunto: "Prueba SMTP de Identidad",
		Texto: "Este correo confirma que Identidad puede enviar mensajes.\n\n" +
			"Origen: " + origen + "\n" +
			"Fecha: " + ahora.Format(time.RFC3339) + "\n",
	}
}

type deshabilitado struct{}

func (deshabilitado) Enviar(context.Context, Mensaje) error { return ErrDeshabilitado }

// registro imprime el correo completo, con sus enlaces y tokens: sólo para desarrollo local.
type registro struct{ log *slog.Logger }

func (r registro) Enviar(ctx context.Context, m Mensaje) error {
	if _, err := validar(m); err != nil {
		return err
	}
	r.log.WarnContext(ctx, "correo_modo_log", "para", m.Para, "asunto", m.Asunto, "texto", m.Texto)
	return nil
}

// validar rechaza saltos de línea en destinatario y asunto: permitirían inyectar
// cabeceras SMTP (por ejemplo, un Bcc oculto).
func validar(m Mensaje) (*mail.Address, error) {
	para, err := mail.ParseAddress(m.Para)
	if err != nil || strings.ContainsAny(m.Para, "\r\n") {
		return nil, fmt.Errorf("%w: destinatario", ErrMensajeInvalido)
	}
	if strings.TrimSpace(m.Asunto) == "" || strings.ContainsAny(m.Asunto, "\r\n") {
		return nil, fmt.Errorf("%w: asunto", ErrMensajeInvalido)
	}
	if strings.TrimSpace(m.Texto) == "" {
		return nil, fmt.Errorf("%w: cuerpo", ErrMensajeInvalido)
	}
	return para, nil
}

// construir arma el mensaje RFC 5322 en UTF-8 y quoted-printable, para que tildes y ñ
// lleguen intactas y ninguna línea supere el largo que admite SMTP.
func construir(remitente, para *mail.Address, m Mensaje, ahora time.Time) ([]byte, error) {
	var b bytes.Buffer
	cabecera := func(nombre, valor string) { b.WriteString(nombre + ": " + valor + "\r\n") }
	cabecera("From", remitente.String())
	cabecera("To", para.String())
	cabecera("Subject", mime.QEncoding.Encode("utf-8", m.Asunto))
	cabecera("Date", ahora.Format(time.RFC1123Z))
	cabecera("Message-ID", messageID(remitente.Address))
	cabecera("MIME-Version", "1.0")
	if m.HTML == "" {
		cabecera("Content-Type", "text/plain; charset=UTF-8")
		cabecera("Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		if err := escribirQP(&b, m.Texto); err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}
	partes := multipart.NewWriter(&b)
	cabecera("Content-Type", "multipart/alternative; boundary="+partes.Boundary())
	b.WriteString("\r\n")
	for _, p := range []struct{ tipo, cuerpo string }{{"text/plain", m.Texto}, {"text/html", m.HTML}} {
		w, err := partes.CreatePart(textproto.MIMEHeader{"Content-Type": {p.tipo + "; charset=UTF-8"}, "Content-Transfer-Encoding": {"quoted-printable"}})
		if err != nil {
			return nil, err
		}
		if err := escribirQP(w, p.cuerpo); err != nil {
			return nil, err
		}
	}
	if err := partes.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func escribirQP(w io.Writer, texto string) error {
	qp := quotedprintable.NewWriter(w)
	if _, err := io.WriteString(qp, texto); err != nil {
		return err
	}
	return qp.Close()
}

func messageID(remitente string) string {
	dominio := "identidad.local"
	if at := strings.LastIndex(remitente, "@"); at >= 0 && at < len(remitente)-1 {
		dominio = remitente[at+1:]
	}
	var id [16]byte
	_, _ = rand.Read(id[:])
	return "<" + hex.EncodeToString(id[:]) + "@" + dominio + ">"
}
