package mailer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"io"
	"log/slog"
	"math/big"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"sync"
	"testing"
	"time"
)

const claveFalsa = "fixture-app-password"

// servidorFalso habla el subconjunto de SMTP que usa net/smtp: EHLO, STARTTLS,
// AUTH PLAIN, MAIL, RCPT, DATA y QUIT. Registra lo recibido para las aserciones.
type servidorFalso struct {
	cfgTLS    *tls.Config
	starttls  bool
	implicito bool
	colgado   bool

	mu         sync.Mutex
	conexiones int
	usuarios   []string
	desde      string
	para       []string
	datos      string
}

func certificadoLocal(t *testing.T) (*tls.Config, *x509.CertPool) {
	t.Helper()
	clave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	plantilla := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "smtp-prueba"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, plantilla, plantilla, &clave.PublicKey, clave)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: clave, Leaf: cert}}}, pool
}

func (s *servidorFalso) iniciar(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.atender(conn)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

func (s *servidorFalso) atender(conn net.Conn) {
	defer conn.Close()
	s.mu.Lock()
	s.conexiones++
	s.mu.Unlock()
	if s.colgado {
		_, _ = io.Copy(io.Discard, conn)
		return
	}
	cifrado := s.implicito
	if s.implicito {
		seguro := tls.Server(conn, s.cfgTLS)
		if seguro.Handshake() != nil {
			return
		}
		conn = seguro
	}
	r := bufio.NewReader(conn)
	responder := func(lineas ...string) {
		for _, l := range lineas {
			_, _ = io.WriteString(conn, l+"\r\n")
		}
	}
	responder("220 smtp-prueba ESMTP")
	for {
		linea, err := r.ReadString('\n')
		if err != nil {
			return
		}
		linea = strings.TrimRight(linea, "\r\n")
		mayus := strings.ToUpper(linea)
		switch {
		case strings.HasPrefix(mayus, "EHLO"):
			extensiones := []string{"250-smtp-prueba"}
			if s.starttls && !cifrado {
				extensiones = append(extensiones, "250-STARTTLS")
			}
			responder(append(extensiones, "250 AUTH PLAIN")...)
		case mayus == "STARTTLS":
			responder("220 listo para TLS")
			seguro := tls.Server(conn, s.cfgTLS)
			if seguro.Handshake() != nil {
				return
			}
			conn, cifrado, r = seguro, true, bufio.NewReader(seguro)
		case strings.HasPrefix(mayus, "AUTH PLAIN "):
			crudo, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(linea[len("AUTH PLAIN "):]))
			usuario, clave := "", ""
			if partes := strings.SplitN(string(crudo), "\x00", 3); len(partes) == 3 {
				usuario, clave = partes[1], partes[2]
			}
			s.mu.Lock()
			s.usuarios = append(s.usuarios, usuario)
			s.mu.Unlock()
			if usuario == "emisor@example.com" && clave == claveFalsa {
				responder("235 2.7.0 aceptado")
			} else {
				responder("535 5.7.8 Username and Password not accepted")
			}
		case strings.HasPrefix(mayus, "MAIL FROM:"):
			s.mu.Lock()
			s.desde = linea[len("MAIL FROM:"):]
			s.mu.Unlock()
			responder("250 ok")
		case strings.HasPrefix(mayus, "RCPT TO:"):
			s.mu.Lock()
			s.para = append(s.para, linea[len("RCPT TO:"):])
			s.mu.Unlock()
			responder("250 ok")
		case mayus == "DATA":
			responder("354 terminar con punto")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(strings.TrimPrefix(l, "."))
			}
			s.mu.Lock()
			s.datos = b.String()
			s.mu.Unlock()
			responder("250 ok en cola")
		case mayus == "QUIT":
			responder("221 adiós")
			return
		default:
			responder("502 no implementado")
		}
	}
}

func nuevoSMTPDePrueba(t *testing.T, puerto int, modoTLS string, clave string, pool *x509.CertPool) *SMTP {
	t.Helper()
	s, err := NuevoSMTP(config.Correo{Modo: config.CorreoSMTP, Host: "127.0.0.1", Puerto: puerto, TLS: modoTLS, Usuario: "emisor@example.com", Clave: clave, Remitente: "RutaAhorro <emisor@example.com>"})
	if err != nil {
		t.Fatal(err)
	}
	s.cfgTLS.RootCAs = pool
	return s
}

func etapa(err error) string {
	var e *ErrorSMTP
	if errors.As(err, &e) {
		return e.Etapa
	}
	return ""
}

func TestSMTPEntregaPorSTARTTLSConTildes(t *testing.T) {
	cfgTLS, pool := certificadoLocal(t)
	srv := &servidorFalso{cfgTLS: cfgTLS, starttls: true}
	s := nuevoSMTPDePrueba(t, srv.iniciar(t), config.TLSStartTLS, claveFalsa, pool)
	m := Mensaje{Para: "Destino <destino@example.com>", Asunto: "Recuperación de contraseña", Texto: "Hola, ñandú.\n.línea que empieza con punto\nFin."}
	if err := s.Enviar(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.usuarios) != 1 || srv.desde != "<emisor@example.com>" || len(srv.para) != 1 || srv.para[0] != "<destino@example.com>" {
		t.Fatalf("sobre SMTP inesperado: %v %q %v", srv.usuarios, srv.desde, srv.para)
	}
	msg, err := mail.ReadMessage(strings.NewReader(srv.datos))
	if err != nil {
		t.Fatal(err)
	}
	asunto, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil || asunto != m.Asunto {
		t.Fatalf("asunto %q %v", asunto, err)
	}
	cuerpo, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if err != nil || strings.TrimRight(string(cuerpo), "\r\n") != "Hola, ñandú.\r\n.línea que empieza con punto\r\nFin." {
		t.Fatalf("cuerpo %q %v", cuerpo, err)
	}
	if !strings.HasSuffix(msg.Header.Get("Message-ID"), "@example.com>") || msg.Header.Get("Date") == "" {
		t.Fatalf("cabeceras incompletas: %v", msg.Header)
	}
}

func TestSMTPEntregaConTLSImplicito(t *testing.T) {
	cfgTLS, pool := certificadoLocal(t)
	srv := &servidorFalso{cfgTLS: cfgTLS, implicito: true}
	s := nuevoSMTPDePrueba(t, srv.iniciar(t), config.TLSImplicito, claveFalsa, pool)
	if err := s.Enviar(context.Background(), MensajePrueba("destino@example.com", "pod-identidad", time.Now())); err != nil {
		t.Fatal(err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if !strings.Contains(srv.datos, "pod-identidad") {
		t.Fatalf("mensaje de prueba no entregado: %q", srv.datos)
	}
}

func TestSMTPNoEnviaCredencialesSinSTARTTLS(t *testing.T) {
	cfgTLS, pool := certificadoLocal(t)
	srv := &servidorFalso{cfgTLS: cfgTLS}
	s := nuevoSMTPDePrueba(t, srv.iniciar(t), config.TLSStartTLS, claveFalsa, pool)
	err := s.Enviar(context.Background(), MensajePrueba("destino@example.com", "pod", time.Now()))
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if etapa(err) != "starttls" || len(srv.usuarios) != 0 {
		t.Fatalf("debió abortar antes de AUTH: %v, usuarios=%v", err, srv.usuarios)
	}
}

func TestSMTPRechazaCertificadoNoConfiable(t *testing.T) {
	cfgTLS, _ := certificadoLocal(t)
	srv := &servidorFalso{cfgTLS: cfgTLS, starttls: true}
	s := nuevoSMTPDePrueba(t, srv.iniciar(t), config.TLSStartTLS, claveFalsa, nil)
	err := s.Enviar(context.Background(), MensajePrueba("destino@example.com", "pod", time.Now()))
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if etapa(err) != "starttls" || len(srv.usuarios) != 0 {
		t.Fatalf("aceptó un certificado sin validar: %v", err)
	}
}

func TestSMTPCredencialesRechazadasNoExponenClave(t *testing.T) {
	cfgTLS, pool := certificadoLocal(t)
	srv := &servidorFalso{cfgTLS: cfgTLS, starttls: true}
	s := nuevoSMTPDePrueba(t, srv.iniciar(t), config.TLSStartTLS, "fixture-private", pool)
	err := s.Enviar(context.Background(), MensajePrueba("destino@example.com", "pod", time.Now()))
	if etapa(err) != "autenticacion" || strings.Contains(err.Error(), "fixture-private") {
		t.Fatalf("esperaba fallo de autenticación sin la clave: %v", err)
	}
}

func TestSMTPServidorColgadoRespetaDeadline(t *testing.T) {
	srv := &servidorFalso{colgado: true}
	s := nuevoSMTPDePrueba(t, srv.iniciar(t), config.TLSStartTLS, claveFalsa, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	inicio := time.Now()
	err := s.Enviar(ctx, MensajePrueba("destino@example.com", "pod", time.Now()))
	if etapa(err) != "saludo" || time.Since(inicio) > 3*time.Second {
		t.Fatalf("el envío no respetó el deadline: %v tras %v", err, time.Since(inicio))
	}
}

func TestSMTPConexionRechazadaIndicaEtapa(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	puerto := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	s := nuevoSMTPDePrueba(t, puerto, config.TLSStartTLS, claveFalsa, nil)
	if err := s.Enviar(context.Background(), MensajePrueba("destino@example.com", "pod", time.Now())); etapa(err) != "conexion" {
		t.Fatalf("esperaba etapa conexion: %v", err)
	}
}

func TestMensajesInvalidosNoAbrenConexion(t *testing.T) {
	cfgTLS, pool := certificadoLocal(t)
	srv := &servidorFalso{cfgTLS: cfgTLS, starttls: true}
	s := nuevoSMTPDePrueba(t, srv.iniciar(t), config.TLSStartTLS, claveFalsa, pool)
	for nombre, m := range map[string]Mensaje{
		"destinatario vacío":    {Asunto: "a", Texto: "b"},
		"destinatario inválido": {Para: "no-es-correo", Asunto: "a", Texto: "b"},
		"inyección en para":     {Para: "a@example.com\r\nBcc: b@example.com", Asunto: "a", Texto: "b"},
		"inyección en asunto":   {Para: "a@example.com", Asunto: "hola\r\nBcc: b@example.com", Texto: "b"},
		"sin cuerpo":            {Para: "a@example.com", Asunto: "a", Texto: "  "},
	} {
		if err := s.Enviar(context.Background(), m); !errors.Is(err, ErrMensajeInvalido) {
			t.Errorf("%s: esperaba ErrMensajeInvalido, obtuvo %v", nombre, err)
		}
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.conexiones != 0 {
		t.Fatalf("mensajes inválidos abrieron %d conexiones", srv.conexiones)
	}
}

func TestConstruirAlternativaTextoYHTML(t *testing.T) {
	remitente, _ := mail.ParseAddress("RutaAhorro <emisor@example.com>")
	para, _ := mail.ParseAddress("destino@example.com")
	datos, err := construir(remitente, para, Mensaje{Asunto: "Verificación", Texto: "Código: ñ", HTML: "<p>Código: ñ</p>"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	msg, err := mail.ReadMessage(bytes.NewReader(datos))
	if err != nil {
		t.Fatal(err)
	}
	tipo, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || tipo != "multipart/alternative" {
		t.Fatalf("%s %v", tipo, err)
	}
	lector := multipart.NewReader(msg.Body, params["boundary"])
	for _, esperado := range []struct{ tipo, cuerpo string }{{"text/plain", "Código: ñ"}, {"text/html", "<p>Código: ñ</p>"}} {
		parte, err := lector.NextPart()
		if err != nil {
			t.Fatal(err)
		}
		// multipart.Reader decodifica quoted-printable y retira la cabecera de transferencia.
		cuerpo, _ := io.ReadAll(parte)
		if !strings.HasPrefix(parte.Header.Get("Content-Type"), esperado.tipo) || string(cuerpo) != esperado.cuerpo {
			t.Fatalf("parte %q: %q", parte.Header.Get("Content-Type"), cuerpo)
		}
	}
}

func TestNuevoSegunModo(t *testing.T) {
	var salida bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&salida, nil))
	m, err := Nuevo(config.Correo{Modo: config.CorreoDeshabilitado}, log)
	if err != nil || !errors.Is(m.Enviar(context.Background(), MensajePrueba("a@example.com", "pod", time.Now())), ErrDeshabilitado) {
		t.Fatalf("deshabilitado: %v", err)
	}
	m, err = Nuevo(config.Correo{Modo: config.CorreoLog}, log)
	if err != nil || m.Enviar(context.Background(), Mensaje{Para: "a@example.com", Asunto: "Verificación", Texto: "https://app/verificar?token=abc"}) != nil {
		t.Fatalf("log: %v", err)
	}
	if !strings.Contains(salida.String(), "correo_modo_log") || !strings.Contains(salida.String(), "token=abc") {
		t.Fatalf("el modo log debe mostrar el mensaje en desarrollo: %s", salida.String())
	}
	m, err = Nuevo(config.Correo{Modo: config.CorreoSMTP, Host: "smtp.example.com", Puerto: 587, TLS: config.TLSStartTLS, Usuario: "a@example.com", Clave: "x", Remitente: "a@example.com"}, log)
	if _, ok := m.(*SMTP); err != nil || !ok {
		t.Fatalf("smtp: %T %v", m, err)
	}
}
