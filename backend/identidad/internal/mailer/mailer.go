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

// ===== Contrato =====

// Mensaje es un correo para un único destinatario. Texto es obligatorio; HTML, opcional.
type Mensaje struct {
	Para, Asunto, Texto, HTML string
}

// Mailer es lo único que conocen los casos de uso: el transporte se elige en Nuevo.
type Mailer interface {
	Enviar(contexto context.Context, mensaje Mensaje) error
}

var (
	ErrDeshabilitado   = errors.New("correo de Identidad deshabilitado")
	ErrMensajeInvalido = errors.New("mensaje de correo inválido")
)

// Nuevo elige el transporte según configuracion.Modo. Sin modo conocido, el correo queda deshabilitado.
func Nuevo(configuracion config.Correo, registrador *slog.Logger) (Mailer, error) {
	if registrador == nil {
		registrador = slog.Default()
	}
	switch configuracion.Modo {
	case config.CorreoSMTP:
		servicio, errorObtenido := NuevoSMTP(configuracion)
		if errorObtenido != nil {
			// No retornar s directamente: un *SMTP nil dentro de la interfaz
			// Mailer no es igual a nil y el llamador no detectaría el error.
			return nil, errorObtenido
		}
		return servicio, nil
	case config.CorreoLog:
		return registro{log: registrador}, nil
	default:
		return deshabilitado{}, nil
	}
}

// ===== Transportes sin red =====

// deshabilitado es el transporte sin credenciales: Identidad arranca igual
// y cada envío devuelve ErrDeshabilitado.
type deshabilitado struct{}

// Enviar rechaza el mensaje porque el correo está deshabilitado.
func (deshabilitado) Enviar(context.Context, Mensaje) error { return ErrDeshabilitado }

// registro imprime el correo completo, con sus enlaces y tokens: sólo para desarrollo local.
type registro struct{ log *slog.Logger }

// Enviar valida el mensaje y lo escribe completo en el log.
func (registroCorreo registro) Enviar(contexto context.Context, mensaje Mensaje) error {
	if _, errorObtenido := validar(mensaje); errorObtenido != nil {
		return errorObtenido
	}
	registroCorreo.log.WarnContext(contexto, "correo_modo_log", "para", mensaje.Para, "asunto", mensaje.Asunto, "texto", mensaje.Texto)
	return nil
}

// ===== Construcción del mensaje =====

// validar rechaza saltos de línea en destinatario y asunto: permitirían inyectar
// cabeceras SMTP (por ejemplo, un Bcc oculto).
func validar(mensaje Mensaje) (*mail.Address, error) {
	para, errorObtenido := mail.ParseAddress(mensaje.Para)
	if errorObtenido != nil || strings.ContainsAny(mensaje.Para, "\r\n") {
		return nil, fmt.Errorf("%w: destinatario", ErrMensajeInvalido)
	}
	if strings.TrimSpace(mensaje.Asunto) == "" || strings.ContainsAny(mensaje.Asunto, "\r\n") {
		return nil, fmt.Errorf("%w: asunto", ErrMensajeInvalido)
	}
	if strings.TrimSpace(mensaje.Texto) == "" {
		return nil, fmt.Errorf("%w: cuerpo", ErrMensajeInvalido)
	}
	return para, nil
}

// construir arma el mensaje RFC 5322 en UTF-8 y quoted-printable, para que tildes y ñ
// lleguen intactas y ninguna línea supere el largo que admite SMTP.
func construir(remitente, para *mail.Address, mensaje Mensaje, ahora time.Time) ([]byte, error) {
	var buffer bytes.Buffer
	cabecera := func(nombre, valor string) { buffer.WriteString(nombre + ": " + valor + "\r\n") }
	cabecera("From", remitente.String())
	cabecera("To", para.String())
	cabecera("Subject", mime.QEncoding.Encode("utf-8", mensaje.Asunto))
	cabecera("Date", ahora.Format(time.RFC1123Z))
	cabecera("Message-ID", messageID(remitente.Address))
	cabecera("MIME-Version", "1.0")
	if mensaje.HTML == "" {
		cabecera("Content-Type", "text/plain; charset=UTF-8")
		cabecera("Content-Transfer-Encoding", "quoted-printable")
		buffer.WriteString("\r\n")
		if errorObtenido := escribirQP(&buffer, mensaje.Texto); errorObtenido != nil {
			return nil, errorObtenido
		}
		return buffer.Bytes(), nil
	}
	partes := multipart.NewWriter(&buffer)
	cabecera("Content-Type", "multipart/alternative; boundary="+partes.Boundary())
	buffer.WriteString("\r\n")
	for _, parametros := range []struct{ tipo, cuerpo string }{{"text/plain", mensaje.Texto}, {"text/html", mensaje.HTML}} {
		escritor, errorObtenido := partes.CreatePart(textproto.MIMEHeader{"Content-Type": {parametros.tipo + "; charset=UTF-8"}, "Content-Transfer-Encoding": {"quoted-printable"}})
		if errorObtenido != nil {
			return nil, errorObtenido
		}
		if errorOperacion := escribirQP(escritor, parametros.cuerpo); errorOperacion != nil {
			return nil, errorOperacion
		}
	}
	if errorObtenido := partes.Close(); errorObtenido != nil {
		return nil, errorObtenido
	}
	return buffer.Bytes(), nil
}

// escribirQP codifica el texto en quoted-printable para mantener líneas
// cortas y caracteres UTF-8 intactos.
func escribirQP(escritor io.Writer, texto string) error {
	codificador := quotedprintable.NewWriter(escritor)
	if _, errorObtenido := io.WriteString(codificador, texto); errorObtenido != nil {
		return errorObtenido
	}
	return codificador.Close()
}

// messageID genera un Message-ID único con el dominio del remitente.
func messageID(remitente string) string {
	dominio := "identidad.local"
	if posicion := strings.LastIndex(remitente, "@"); posicion >= 0 && posicion < len(remitente)-1 {
		dominio = remitente[posicion+1:]
	}
	var id [16]byte
	_, _ = rand.Read(id[:])
	return "<" + hex.EncodeToString(id[:]) + "@" + dominio + ">"
}
