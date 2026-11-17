package integration_test

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
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/mailer"
	"io"
	"log/slog"
	"math/big"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// ejecutarSMTPAislado mantiene las raíces efímeras fuera del proceso de otras pruebas.
// x509.SetFallbackRoots solo puede llamarse una vez por proceso. El hijo ejecuta
// un único caso y usa su certificado local sin acceder a campos privados de SMTP.
func ejecutarSMTPAislado(prueba *testing.T) bool {
	prueba.Helper()
	const variableCaso = "SUP347_CASO_SMTP"
	if os.Getenv(variableCaso) == prueba.Name() {
		return true
	}
	contexto, cancelar := context.WithTimeout(prueba.Context(), 20*time.Second)
	defer cancelar()
	proceso := exec.CommandContext(contexto, os.Args[0], "-test.run=^"+regexp.QuoteMeta(prueba.Name())+"$", "-test.count=1")
	depuracion := os.Getenv("GODEBUG")
	if depuracion != "" {
		depuracion += ","
	}
	proceso.Env = append(os.Environ(), variableCaso+"="+prueba.Name(), "GODEBUG="+depuracion+"x509usefallbackroots=1")
	if salida, errorProceso := proceso.CombinedOutput(); errorProceso != nil {
		prueba.Fatalf("prueba SMTP aislada: %v\n%s", errorProceso, salida)
	}
	return false
}

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

func certificadoLocal(pruebaLocal *testing.T) (*tls.Config, *x509.CertPool) {
	pruebaLocal.Helper()
	clave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		pruebaLocal.Fatal(err)
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
		pruebaLocal.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		pruebaLocal.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: clave, Leaf: cert}}}, pool
}

func (servicio *servidorFalso) iniciar(pruebaLocal *testing.T) int {
	pruebaLocal.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		pruebaLocal.Fatal(err)
	}
	pruebaLocal.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go servicio.atender(conn)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

func (servicio *servidorFalso) atender(conn net.Conn) {
	defer conn.Close()
	servicio.mu.Lock()
	servicio.conexiones++
	servicio.mu.Unlock()
	if servicio.colgado {
		_, _ = io.Copy(io.Discard, conn)
		return
	}
	cifrado := servicio.implicito
	if servicio.implicito {
		seguro := tls.Server(conn, servicio.cfgTLS)
		if seguro.Handshake() != nil {
			return
		}
		conn = seguro
	}
	peticion := bufio.NewReader(conn)
	responder := func(lineas ...string) {
		for _, lineaLocal := range lineas {
			_, _ = io.WriteString(conn, lineaLocal+"\r\n")
		}
	}
	responder("220 smtp-prueba ESMTP")
	for {
		linea, err := peticion.ReadString('\n')
		if err != nil {
			return
		}
		linea = strings.TrimRight(linea, "\r\n")
		mayus := strings.ToUpper(linea)
		switch {
		case strings.HasPrefix(mayus, "EHLO"):
			extensiones := []string{"250-smtp-prueba"}
			if servicio.starttls && !cifrado {
				extensiones = append(extensiones, "250-STARTTLS")
			}
			responder(append(extensiones, "250 AUTH PLAIN")...)
		case mayus == "STARTTLS":
			responder("220 listo para TLS")
			seguro := tls.Server(conn, servicio.cfgTLS)
			if seguro.Handshake() != nil {
				return
			}
			conn, cifrado, peticion = seguro, true, bufio.NewReader(seguro)
		case strings.HasPrefix(mayus, "AUTH PLAIN "):
			crudo, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(linea[len("AUTH PLAIN "):]))
			usuario, clave := "", ""
			if partes := strings.SplitN(string(crudo), "\x00", 3); len(partes) == 3 {
				usuario, clave = partes[1], partes[2]
			}
			servicio.mu.Lock()
			servicio.usuarios = append(servicio.usuarios, usuario)
			servicio.mu.Unlock()
			if usuario == "emisor@example.com" && clave == claveFalsa {
				responder("235 2.7.0 aceptado")
			} else {
				responder("535 5.7.8 Username and Password not accepted")
			}
		case strings.HasPrefix(mayus, "MAIL FROM:"):
			servicio.mu.Lock()
			servicio.desde = linea[len("MAIL FROM:"):]
			servicio.mu.Unlock()
			responder("250 ok")
		case strings.HasPrefix(mayus, "RCPT TO:"):
			servicio.mu.Lock()
			servicio.para = append(servicio.para, linea[len("RCPT TO:"):])
			servicio.mu.Unlock()
			responder("250 ok")
		case mayus == "DATA":
			responder("354 terminar con punto")
			var buffer strings.Builder
			for {
				lineaLocal, err := peticion.ReadString('\n')
				if err != nil {
					return
				}
				if lineaLocal == ".\r\n" {
					break
				}
				buffer.WriteString(strings.TrimPrefix(lineaLocal, "."))
			}
			servicio.mu.Lock()
			servicio.datos = buffer.String()
			servicio.mu.Unlock()
			responder("250 ok en cola")
		case mayus == "QUIT":
			responder("221 adiós")
			return
		default:
			responder("502 no implementado")
		}
	}
}

func nuevoSMTPDePrueba(pruebaLocal *testing.T, puerto int, modoTLS string, clave string, pool *x509.CertPool) *mailer.SMTP {
	pruebaLocal.Helper()
	servicio, err := mailer.NuevoSMTP(config.Correo{Modo: config.CorreoSMTP, Host: "127.0.0.1", Puerto: puerto, TLS: modoTLS, Usuario: "emisor@example.com", Clave: clave, Remitente: "RutaAhorro <emisor@example.com>"})
	if err != nil {
		pruebaLocal.Fatal(err)
	}
	// Las raíces se configuran por la API pública de x509 en un proceso aislado.
	if pool == nil {
		pool = x509.NewCertPool()
	}
	x509.SetFallbackRoots(pool)
	return servicio
}

func etapa(err error) string {
	var entrada *mailer.ErrorSMTP
	if errors.As(err, &entrada) {
		return entrada.Etapa
	}
	return ""
}

func TestSMTPEntregaPorSTARTTLSConTildes(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	cfgTLS, pool := certificadoLocal(pruebaLocal)
	srv := &servidorFalso{cfgTLS: cfgTLS, starttls: true}
	servicio := nuevoSMTPDePrueba(pruebaLocal, srv.iniciar(pruebaLocal), config.TLSStartTLS, claveFalsa, pool)
	mensaje := mailer.Mensaje{Para: "Destino <destino@example.com>", Asunto: "Recuperación de contraseña", Texto: "Hola, ñandú.\n.línea que empieza con punto\nFin."}
	if err := servicio.Enviar(context.Background(), mensaje); err != nil {
		pruebaLocal.Fatal(err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.usuarios) != 1 || srv.desde != "<emisor@example.com>" || len(srv.para) != 1 || srv.para[0] != "<destino@example.com>" {
		pruebaLocal.Fatalf("sobre SMTP inesperado: %v %q %v", srv.usuarios, srv.desde, srv.para)
	}
	msg, err := mail.ReadMessage(strings.NewReader(srv.datos))
	if err != nil {
		pruebaLocal.Fatal(err)
	}
	asunto, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil || asunto != mensaje.Asunto {
		pruebaLocal.Fatalf("asunto %q %v", asunto, err)
	}
	cuerpo, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if err != nil || strings.TrimRight(string(cuerpo), "\r\n") != "Hola, ñandú.\r\n.línea que empieza con punto\r\nFin." {
		pruebaLocal.Fatalf("cuerpo %q %v", cuerpo, err)
	}
	if !strings.HasSuffix(msg.Header.Get("Message-ID"), "@example.com>") || msg.Header.Get("Date") == "" {
		pruebaLocal.Fatalf("cabeceras incompletas: %v", msg.Header)
	}
}

func TestSMTPEntregaConTLSImplicito(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	cfgTLS, pool := certificadoLocal(pruebaLocal)
	srv := &servidorFalso{cfgTLS: cfgTLS, implicito: true}
	servicio := nuevoSMTPDePrueba(pruebaLocal, srv.iniciar(pruebaLocal), config.TLSImplicito, claveFalsa, pool)
	if err := servicio.Enviar(context.Background(), mensajeDePrueba()); err != nil {
		pruebaLocal.Fatal(err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if !strings.Contains(srv.datos, "Identidad puede enviar mensajes") {
		pruebaLocal.Fatalf("mensaje de prueba no entregado: %q", srv.datos)
	}
}

func TestSMTPNoEnviaCredencialesSinSTARTTLS(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	cfgTLS, pool := certificadoLocal(pruebaLocal)
	srv := &servidorFalso{cfgTLS: cfgTLS}
	servicio := nuevoSMTPDePrueba(pruebaLocal, srv.iniciar(pruebaLocal), config.TLSStartTLS, claveFalsa, pool)
	err := servicio.Enviar(context.Background(), mensajeDePrueba())
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if etapa(err) != "starttls" || len(srv.usuarios) != 0 {
		pruebaLocal.Fatalf("debió abortar antes de AUTH: %v, usuarios=%v", err, srv.usuarios)
	}
}

func TestSMTPRechazaCertificadoNoConfiable(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	cfgTLS, _ := certificadoLocal(pruebaLocal)
	srv := &servidorFalso{cfgTLS: cfgTLS, starttls: true}
	servicio := nuevoSMTPDePrueba(pruebaLocal, srv.iniciar(pruebaLocal), config.TLSStartTLS, claveFalsa, nil)
	err := servicio.Enviar(context.Background(), mensajeDePrueba())
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if etapa(err) != "starttls" || len(srv.usuarios) != 0 {
		pruebaLocal.Fatalf("aceptó un certificado sin validar: %v", err)
	}
}

func TestSMTPCredencialesRechazadasNoExponenClave(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	cfgTLS, pool := certificadoLocal(pruebaLocal)
	srv := &servidorFalso{cfgTLS: cfgTLS, starttls: true}
	servicio := nuevoSMTPDePrueba(pruebaLocal, srv.iniciar(pruebaLocal), config.TLSStartTLS, "fixture-private", pool)
	err := servicio.Enviar(context.Background(), mensajeDePrueba())
	if etapa(err) != "autenticacion" || strings.Contains(err.Error(), "fixture-private") {
		pruebaLocal.Fatalf("esperaba fallo de autenticación sin la clave: %v", err)
	}
}

func TestSMTPServidorColgadoRespetaDeadline(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	srv := &servidorFalso{colgado: true}
	servicio := nuevoSMTPDePrueba(pruebaLocal, srv.iniciar(pruebaLocal), config.TLSStartTLS, claveFalsa, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	inicio := time.Now()
	err := servicio.Enviar(ctx, mensajeDePrueba())
	if etapa(err) != "saludo" || time.Since(inicio) > 3*time.Second {
		pruebaLocal.Fatalf("el envío no respetó el deadline: %v tras %v", err, time.Since(inicio))
	}
}

func TestSMTPConexionRechazadaIndicaEtapa(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		pruebaLocal.Fatal(err)
	}
	puerto := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	servicio := nuevoSMTPDePrueba(pruebaLocal, puerto, config.TLSStartTLS, claveFalsa, nil)
	if err := servicio.Enviar(context.Background(), mensajeDePrueba()); etapa(err) != "conexion" {
		pruebaLocal.Fatalf("esperaba etapa conexion: %v", err)
	}
}

func TestMensajesInvalidosNoAbrenConexion(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	cfgTLS, pool := certificadoLocal(pruebaLocal)
	srv := &servidorFalso{cfgTLS: cfgTLS, starttls: true}
	servicio := nuevoSMTPDePrueba(pruebaLocal, srv.iniciar(pruebaLocal), config.TLSStartTLS, claveFalsa, pool)
	for nombre, mensaje := range map[string]mailer.Mensaje{
		"destinatario vacío":    {Asunto: "a", Texto: "b"},
		"destinatario inválido": {Para: "no-es-correo", Asunto: "a", Texto: "b"},
		"inyección en para":     {Para: "a@example.com\r\nBcc: b@example.com", Asunto: "a", Texto: "b"},
		"inyección en asunto":   {Para: "a@example.com", Asunto: "hola\r\nBcc: b@example.com", Texto: "b"},
		"sin cuerpo":            {Para: "a@example.com", Asunto: "a", Texto: "  "},
	} {
		if err := servicio.Enviar(context.Background(), mensaje); !errors.Is(err, mailer.ErrMensajeInvalido) {
			pruebaLocal.Errorf("%s: esperaba mailer.ErrMensajeInvalido, obtuvo %v", nombre, err)
		}
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.conexiones != 0 {
		pruebaLocal.Fatalf("mensajes inválidos abrieron %d conexiones", srv.conexiones)
	}
}

func TestConstruirAlternativaTextoYHTML(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	cfgTLS, pool := certificadoLocal(pruebaLocal)
	srv := &servidorFalso{cfgTLS: cfgTLS, starttls: true}
	transporte := nuevoSMTPDePrueba(pruebaLocal, srv.iniciar(pruebaLocal), config.TLSStartTLS, claveFalsa, pool)
	if err := transporte.Enviar(pruebaLocal.Context(), mailer.Mensaje{Para: "destino@example.com", Asunto: "Verificación", Texto: "Código: ñ", HTML: "<p>Código: ñ</p>"}); err != nil {
		pruebaLocal.Fatal(err)
	}
	srv.mu.Lock()
	datos := srv.datos
	srv.mu.Unlock()
	msg, err := mail.ReadMessage(strings.NewReader(datos))
	if err != nil {
		pruebaLocal.Fatal(err)
	}
	tipo, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || tipo != "multipart/alternative" {
		pruebaLocal.Fatalf("%s %v", tipo, err)
	}
	lector := multipart.NewReader(msg.Body, params["boundary"])
	for _, esperado := range []struct{ tipo, cuerpo string }{{"text/plain", "Código: ñ"}, {"text/html", "<p>Código: ñ</p>"}} {
		parte, err := lector.NextPart()
		if err != nil {
			pruebaLocal.Fatal(err)
		}
		// multipart.Reader decodifica quoted-printable y retira la cabecera de transferencia.
		cuerpo, _ := io.ReadAll(parte)
		if !strings.HasPrefix(parte.Header.Get("Content-Type"), esperado.tipo) || string(cuerpo) != esperado.cuerpo {
			pruebaLocal.Fatalf("parte %q: %q", parte.Header.Get("Content-Type"), cuerpo)
		}
	}
}

func TestNuevoSegunModo(pruebaLocal *testing.T) {
	var salida bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&salida, nil))
	mensaje, err := mailer.Nuevo(config.Correo{Modo: config.CorreoDeshabilitado}, log)
	if err != nil || !errors.Is(mensaje.Enviar(context.Background(), mensajeDePrueba()), mailer.ErrDeshabilitado) {
		pruebaLocal.Fatalf("deshabilitado: %v", err)
	}
	mensaje, err = mailer.Nuevo(config.Correo{Modo: config.CorreoLog}, log)
	if err != nil || mensaje.Enviar(context.Background(), mailer.Mensaje{Para: "a@example.com", Asunto: "Verificación", Texto: "https://app/verificar?token=abc"}) != nil {
		pruebaLocal.Fatalf("log: %v", err)
	}
	if !strings.Contains(salida.String(), "correo_modo_log") || !strings.Contains(salida.String(), "token=abc") {
		pruebaLocal.Fatalf("el modo log debe mostrar el mensaje en desarrollo: %s", salida.String())
	}
	mensaje, err = mailer.Nuevo(config.Correo{Modo: config.CorreoSMTP, Host: "smtp.example.com", Puerto: 587, TLS: config.TLSStartTLS, Usuario: "a@example.com", Clave: "x", Remitente: "a@example.com"}, log)
	if _, ok := mensaje.(*mailer.SMTP); err != nil || !ok {
		pruebaLocal.Fatalf("smtp: %T %v", mensaje, err)
	}
}

// mensajeDePrueba es un correo mínimo válido para ejercitar el transporte.
func mensajeDePrueba() mailer.Mensaje {
	return mailer.Mensaje{Para: "destino@example.com", Asunto: "Prueba SMTP de Identidad", Texto: "Este correo confirma que Identidad puede enviar mensajes.\n"}
}
