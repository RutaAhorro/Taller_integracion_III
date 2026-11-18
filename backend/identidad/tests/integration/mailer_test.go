// Transporte SMTP contra un servidor local con certificados efímeros: STARTTLS,
// TLS implícito, autenticación, plazos y errores por etapa, sin red externa.

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

// certificadoLocal crea una CA y un certificado efímeros para 127.0.0.1 y
// devuelve la configuración del servidor y el pool de confianza del cliente.
func certificadoLocal(pruebaLocal *testing.T) (*tls.Config, *x509.CertPool) {
	pruebaLocal.Helper()
	clave, errorObtenido := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if errorObtenido != nil {
		pruebaLocal.Fatal(errorObtenido)
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
	der, errorObtenido := x509.CreateCertificate(rand.Reader, plantilla, plantilla, &clave.PublicKey, clave)
	if errorObtenido != nil {
		pruebaLocal.Fatal(errorObtenido)
	}
	certificado, errorObtenido := x509.ParseCertificate(der)
	if errorObtenido != nil {
		pruebaLocal.Fatal(errorObtenido)
	}
	pool := x509.NewCertPool()
	pool.AddCert(certificado)
	return &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: clave, Leaf: certificado}}}, pool
}

// iniciar escucha en un puerto libre de loopback y atiende una conexión por vez.
func (servidor *servidorFalso) iniciar(pruebaLocal *testing.T) int {
	pruebaLocal.Helper()
	escucha, errorOperacion := net.Listen("tcp", "127.0.0.1:0")
	if errorOperacion != nil {
		pruebaLocal.Fatal(errorOperacion)
	}
	pruebaLocal.Cleanup(func() { _ = escucha.Close() })
	go func() {
		for {
			conexion, errorObtenido := escucha.Accept()
			if errorObtenido != nil {
				return
			}
			go servidor.atender(conexion)
		}
	}()
	return escucha.Addr().(*net.TCPAddr).Port
}

// atender simula el diálogo SMTP mínimo y guarda lo recibido para las aserciones.
func (servidor *servidorFalso) atender(conexion net.Conn) {
	defer conexion.Close()
	servidor.mu.Lock()
	servidor.conexiones++
	servidor.mu.Unlock()
	if servidor.colgado {
		_, _ = io.Copy(io.Discard, conexion)
		return
	}
	cifrado := servidor.implicito
	if servidor.implicito {
		seguro := tls.Server(conexion, servidor.cfgTLS)
		if seguro.Handshake() != nil {
			return
		}
		conexion = seguro
	}
	peticion := bufio.NewReader(conexion)
	responder := func(lineas ...string) {
		for _, lineaLocal := range lineas {
			_, _ = io.WriteString(conexion, lineaLocal+"\r\n")
		}
	}
	responder("220 smtp-prueba ESMTP")
	for {
		linea, errorOperacion := peticion.ReadString('\n')
		if errorOperacion != nil {
			return
		}
		linea = strings.TrimRight(linea, "\r\n")
		mayus := strings.ToUpper(linea)
		switch {
		case strings.HasPrefix(mayus, "EHLO"):
			extensiones := []string{"250-smtp-prueba"}
			if servidor.starttls && !cifrado {
				extensiones = append(extensiones, "250-STARTTLS")
			}
			responder(append(extensiones, "250 AUTH PLAIN")...)
		case mayus == "STARTTLS":
			responder("220 listo para TLS")
			seguro := tls.Server(conexion, servidor.cfgTLS)
			if seguro.Handshake() != nil {
				return
			}
			conexion, cifrado, peticion = seguro, true, bufio.NewReader(seguro)
		case strings.HasPrefix(mayus, "AUTH PLAIN "):
			crudo, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(linea[len("AUTH PLAIN "):]))
			usuario, clave := "", ""
			if partes := strings.SplitN(string(crudo), "\x00", 3); len(partes) == 3 {
				usuario, clave = partes[1], partes[2]
			}
			servidor.mu.Lock()
			servidor.usuarios = append(servidor.usuarios, usuario)
			servidor.mu.Unlock()
			if usuario == "emisor@example.com" && clave == claveFalsa {
				responder("235 2.7.0 aceptado")
			} else {
				responder("535 5.7.8 Username and Password not accepted")
			}
		case strings.HasPrefix(mayus, "MAIL FROM:"):
			servidor.mu.Lock()
			servidor.desde = linea[len("MAIL FROM:"):]
			servidor.mu.Unlock()
			responder("250 ok")
		case strings.HasPrefix(mayus, "RCPT TO:"):
			servidor.mu.Lock()
			servidor.para = append(servidor.para, linea[len("RCPT TO:"):])
			servidor.mu.Unlock()
			responder("250 ok")
		case mayus == "DATA":
			responder("354 terminar con punto")
			var buffer strings.Builder
			for {
				lineaLocal, errorObtenido := peticion.ReadString('\n')
				if errorObtenido != nil {
					return
				}
				if lineaLocal == ".\r\n" {
					break
				}
				buffer.WriteString(strings.TrimPrefix(lineaLocal, "."))
			}
			servidor.mu.Lock()
			servidor.datos = buffer.String()
			servidor.mu.Unlock()
			responder("250 ok en cola")
		case mayus == "QUIT":
			responder("221 adiós")
			return
		default:
			responder("502 no implementado")
		}
	}
}

// nuevoSMTPDePrueba arma el transporte real apuntando al servidor local.
func nuevoSMTPDePrueba(pruebaLocal *testing.T, puerto int, modoTLS string, clave string, pool *x509.CertPool) *mailer.SMTP {
	pruebaLocal.Helper()
	servicio, errorObtenido := mailer.NuevoSMTP(config.Correo{Modo: config.CorreoSMTP, Host: "127.0.0.1", Puerto: puerto, TLS: modoTLS, Usuario: "emisor@example.com", Clave: clave, Remitente: "RutaAhorro <emisor@example.com>"})
	if errorObtenido != nil {
		pruebaLocal.Fatal(errorObtenido)
	}
	// Las raíces se configuran por la API pública de x509 en un proceso aislado.
	if pool == nil {
		pool = x509.NewCertPool()
	}
	x509.SetFallbackRoots(pool)
	return servicio
}

// etapa extrae la etapa de un *mailer.ErrorSMTP, o vacío si no lo es.
func etapa(errorObtenido error) string {
	var entrada *mailer.ErrorSMTP
	if errors.As(errorObtenido, &entrada) {
		return entrada.Etapa
	}
	return ""
}

func TestSMTPEntregaPorSTARTTLSConTildes(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	configuracionTLS, pool := certificadoLocal(pruebaLocal)
	servidor := &servidorFalso{cfgTLS: configuracionTLS, starttls: true}
	servicio := nuevoSMTPDePrueba(pruebaLocal, servidor.iniciar(pruebaLocal), config.TLSStartTLS, claveFalsa, pool)
	mensaje := mailer.Mensaje{Para: "Destino <destino@example.com>", Asunto: "Recuperación de contraseña", Texto: "Hola, ñandú.\n.línea que empieza con punto\nFin."}
	if errorOperacion := servicio.Enviar(context.Background(), mensaje); errorOperacion != nil {
		pruebaLocal.Fatal(errorOperacion)
	}
	servidor.mu.Lock()
	defer servidor.mu.Unlock()
	if len(servidor.usuarios) != 1 || servidor.desde != "<emisor@example.com>" || len(servidor.para) != 1 || servidor.para[0] != "<destino@example.com>" {
		pruebaLocal.Fatalf("sobre SMTP inesperado: %v %q %v", servidor.usuarios, servidor.desde, servidor.para)
	}
	mensajeError, errorObtenido := mail.ReadMessage(strings.NewReader(servidor.datos))
	if errorObtenido != nil {
		pruebaLocal.Fatal(errorObtenido)
	}
	asunto, errorObtenido := new(mime.WordDecoder).DecodeHeader(mensajeError.Header.Get("Subject"))
	if errorObtenido != nil || asunto != mensaje.Asunto {
		pruebaLocal.Fatalf("asunto %q %v", asunto, errorObtenido)
	}
	cuerpo, errorObtenido := io.ReadAll(quotedprintable.NewReader(mensajeError.Body))
	if errorObtenido != nil || strings.TrimRight(string(cuerpo), "\r\n") != "Hola, ñandú.\r\n.línea que empieza con punto\r\nFin." {
		pruebaLocal.Fatalf("cuerpo %q %v", cuerpo, errorObtenido)
	}
	if !strings.HasSuffix(mensajeError.Header.Get("Message-ID"), "@example.com>") || mensajeError.Header.Get("Date") == "" {
		pruebaLocal.Fatalf("cabeceras incompletas: %v", mensajeError.Header)
	}
}

func TestSMTPEntregaConTLSImplicito(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	configuracionTLS, pool := certificadoLocal(pruebaLocal)
	servidor := &servidorFalso{cfgTLS: configuracionTLS, implicito: true}
	servicio := nuevoSMTPDePrueba(pruebaLocal, servidor.iniciar(pruebaLocal), config.TLSImplicito, claveFalsa, pool)
	if errorObtenido := servicio.Enviar(context.Background(), mensajeDePrueba()); errorObtenido != nil {
		pruebaLocal.Fatal(errorObtenido)
	}
	servidor.mu.Lock()
	defer servidor.mu.Unlock()
	if !strings.Contains(servidor.datos, "Identidad puede enviar mensajes") {
		pruebaLocal.Fatalf("mensaje de prueba no entregado: %q", servidor.datos)
	}
}

func TestSMTPNoEnviaCredencialesSinSTARTTLS(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	configuracionTLS, pool := certificadoLocal(pruebaLocal)
	servidor := &servidorFalso{cfgTLS: configuracionTLS}
	servicio := nuevoSMTPDePrueba(pruebaLocal, servidor.iniciar(pruebaLocal), config.TLSStartTLS, claveFalsa, pool)
	errorObtenido := servicio.Enviar(context.Background(), mensajeDePrueba())
	servidor.mu.Lock()
	defer servidor.mu.Unlock()
	if etapa(errorObtenido) != "starttls" || len(servidor.usuarios) != 0 {
		pruebaLocal.Fatalf("debió abortar antes de AUTH: %v, usuarios=%v", errorObtenido, servidor.usuarios)
	}
}

func TestSMTPRechazaCertificadoNoConfiable(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	configuracionTLS, _ := certificadoLocal(pruebaLocal)
	servidor := &servidorFalso{cfgTLS: configuracionTLS, starttls: true}
	servicio := nuevoSMTPDePrueba(pruebaLocal, servidor.iniciar(pruebaLocal), config.TLSStartTLS, claveFalsa, nil)
	errorObtenido := servicio.Enviar(context.Background(), mensajeDePrueba())
	servidor.mu.Lock()
	defer servidor.mu.Unlock()
	if etapa(errorObtenido) != "starttls" || len(servidor.usuarios) != 0 {
		pruebaLocal.Fatalf("aceptó un certificado sin validar: %v", errorObtenido)
	}
}

func TestSMTPCredencialesRechazadasNoExponenClave(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	configuracionTLS, pool := certificadoLocal(pruebaLocal)
	servidor := &servidorFalso{cfgTLS: configuracionTLS, starttls: true}
	servicio := nuevoSMTPDePrueba(pruebaLocal, servidor.iniciar(pruebaLocal), config.TLSStartTLS, "fixture-private", pool)
	errorObtenido := servicio.Enviar(context.Background(), mensajeDePrueba())
	if etapa(errorObtenido) != "autenticacion" || strings.Contains(errorObtenido.Error(), "fixture-private") {
		pruebaLocal.Fatalf("esperaba fallo de autenticación sin la clave: %v", errorObtenido)
	}
}

func TestSMTPServidorColgadoRespetaDeadline(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	servidor := &servidorFalso{colgado: true}
	servicio := nuevoSMTPDePrueba(pruebaLocal, servidor.iniciar(pruebaLocal), config.TLSStartTLS, claveFalsa, nil)
	contexto, cancelar := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancelar()
	inicio := time.Now()
	errorObtenido := servicio.Enviar(contexto, mensajeDePrueba())
	if etapa(errorObtenido) != "saludo" || time.Since(inicio) > 3*time.Second {
		pruebaLocal.Fatalf("el envío no respetó el deadline: %v tras %v", errorObtenido, time.Since(inicio))
	}
}

func TestSMTPConexionRechazadaIndicaEtapa(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	escucha, errorOperacion := net.Listen("tcp", "127.0.0.1:0")
	if errorOperacion != nil {
		pruebaLocal.Fatal(errorOperacion)
	}
	puerto := escucha.Addr().(*net.TCPAddr).Port
	_ = escucha.Close()
	servicio := nuevoSMTPDePrueba(pruebaLocal, puerto, config.TLSStartTLS, claveFalsa, nil)
	if errorObtenido := servicio.Enviar(context.Background(), mensajeDePrueba()); etapa(errorObtenido) != "conexion" {
		pruebaLocal.Fatalf("esperaba etapa conexion: %v", errorObtenido)
	}
}

func TestMensajesInvalidosNoAbrenConexion(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	configuracionTLS, pool := certificadoLocal(pruebaLocal)
	servidor := &servidorFalso{cfgTLS: configuracionTLS, starttls: true}
	servicio := nuevoSMTPDePrueba(pruebaLocal, servidor.iniciar(pruebaLocal), config.TLSStartTLS, claveFalsa, pool)
	for nombre, mensaje := range map[string]mailer.Mensaje{
		"destinatario vacío":    {Asunto: "a", Texto: "b"},
		"destinatario inválido": {Para: "no-es-correo", Asunto: "a", Texto: "b"},
		"inyección en para":     {Para: "a@example.com\r\nBcc: b@example.com", Asunto: "a", Texto: "b"},
		"inyección en asunto":   {Para: "a@example.com", Asunto: "hola\r\nBcc: b@example.com", Texto: "b"},
		"sin cuerpo":            {Para: "a@example.com", Asunto: "a", Texto: "  "},
	} {
		if errorObtenido := servicio.Enviar(context.Background(), mensaje); !errors.Is(errorObtenido, mailer.ErrMensajeInvalido) {
			pruebaLocal.Errorf("%s: esperaba mailer.ErrMensajeInvalido, obtuvo %v", nombre, errorObtenido)
		}
	}
	servidor.mu.Lock()
	defer servidor.mu.Unlock()
	if servidor.conexiones != 0 {
		pruebaLocal.Fatalf("mensajes inválidos abrieron %d conexiones", servidor.conexiones)
	}
}

func TestConstruirAlternativaTextoYHTML(pruebaLocal *testing.T) {
	if !ejecutarSMTPAislado(pruebaLocal) {
		return
	}
	configuracionTLS, pool := certificadoLocal(pruebaLocal)
	servidor := &servidorFalso{cfgTLS: configuracionTLS, starttls: true}
	transporte := nuevoSMTPDePrueba(pruebaLocal, servidor.iniciar(pruebaLocal), config.TLSStartTLS, claveFalsa, pool)
	if errorObtenido := transporte.Enviar(pruebaLocal.Context(), mailer.Mensaje{Para: "destino@example.com", Asunto: "Verificación", Texto: "Código: ñ", HTML: "<p>Código: ñ</p>"}); errorObtenido != nil {
		pruebaLocal.Fatal(errorObtenido)
	}
	servidor.mu.Lock()
	datos := servidor.datos
	servidor.mu.Unlock()
	mensaje, errorOperacion := mail.ReadMessage(strings.NewReader(datos))
	if errorOperacion != nil {
		pruebaLocal.Fatal(errorOperacion)
	}
	tipo, parametros, errorOperacion := mime.ParseMediaType(mensaje.Header.Get("Content-Type"))
	if errorOperacion != nil || tipo != "multipart/alternative" {
		pruebaLocal.Fatalf("%s %v", tipo, errorOperacion)
	}
	lector := multipart.NewReader(mensaje.Body, parametros["boundary"])
	for _, esperado := range []struct{ tipo, cuerpo string }{{"text/plain", "Código: ñ"}, {"text/html", "<p>Código: ñ</p>"}} {
		parte, errorObtenido := lector.NextPart()
		if errorObtenido != nil {
			pruebaLocal.Fatal(errorObtenido)
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
	registrador := slog.New(slog.NewJSONHandler(&salida, nil))
	mensaje, errorObtenido := mailer.Nuevo(config.Correo{Modo: config.CorreoDeshabilitado}, registrador)
	if errorObtenido != nil || !errors.Is(mensaje.Enviar(context.Background(), mensajeDePrueba()), mailer.ErrDeshabilitado) {
		pruebaLocal.Fatalf("deshabilitado: %v", errorObtenido)
	}
	mensaje, errorObtenido = mailer.Nuevo(config.Correo{Modo: config.CorreoLog}, registrador)
	if errorObtenido != nil || mensaje.Enviar(context.Background(), mailer.Mensaje{Para: "a@example.com", Asunto: "Verificación", Texto: "https://app/verificar?token=abc"}) != nil {
		pruebaLocal.Fatalf("log: %v", errorObtenido)
	}
	if !strings.Contains(salida.String(), "correo_modo_log") || !strings.Contains(salida.String(), "token=abc") {
		pruebaLocal.Fatalf("el modo log debe mostrar el mensaje en desarrollo: %s", salida.String())
	}
	mensaje, errorObtenido = mailer.Nuevo(config.Correo{Modo: config.CorreoSMTP, Host: "smtp.example.com", Puerto: 587, TLS: config.TLSStartTLS, Usuario: "a@example.com", Clave: "x", Remitente: "a@example.com"}, registrador)
	if _, valido := mensaje.(*mailer.SMTP); errorObtenido != nil || !valido {
		pruebaLocal.Fatalf("smtp: %T %v", mensaje, errorObtenido)
	}
}

// mensajeDePrueba es un correo mínimo válido para ejercitar el transporte.
func mensajeDePrueba() mailer.Mensaje {
	return mailer.Mensaje{Para: "destino@example.com", Asunto: "Prueba SMTP de Identidad", Texto: "Este correo confirma que Identidad puede enviar mensajes.\n"}
}
