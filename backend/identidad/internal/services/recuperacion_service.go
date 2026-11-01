package services

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/mailer"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
)

// VigenciaTokenRecuperacion es la duración del enlace (SUP-184 pide entre 15 y 30 minutos).
const VigenciaTokenRecuperacion = 30 * time.Minute

// Límites por cuenta, además del límite por IP del endpoint (SUP-233): protegen la
// bandeja del usuario y la cuota diaria de Gmail ante solicitudes repetidas.
const (
	esperaEntreSolicitudes = time.Minute
	ventanaSolicitudes     = time.Hour
	maximoPorVentana       = 5
)

// Límites de la cola en segundo plano.
const (
	capacidadCola     = 64               // solicitudes en espera; las demás se descartan
	plazoPorSolicitud = 45 * time.Second // búsqueda + token + envío SMTP
)

// RecuperacionService emite el token de restablecimiento y despacha su correo.
type RecuperacionService interface {
	// Solicitar valida el formato y encola el correo. La búsqueda de la cuenta, el token
	// y el envío ocurren en segundo plano: la respuesta y su tiempo no revelan si la
	// cuenta existe, está inactiva o usa Google. Sólo devuelve ErrCorreoInvalido.
	Solicitar(correo string) error
	// Cerrar procesa lo encolado y termina; se llama al apagar Identidad.
	Cerrar(ctx context.Context) error
}

// recuperacionService atiende las solicitudes con un único worker (trabajar) que lee
// de una cola acotada. Así el tiempo de respuesta HTTP no depende de la cuenta y el
// trabajo pendiente se vacía ordenadamente al apagar.
type recuperacionService struct {
	usuarios       domain.UsuarioRepository
	tokens         domain.RecuperacionRepository
	correo         mailer.Mailer
	urlRestablecer string
	log            *slog.Logger

	cola    chan string   // correos pendientes de procesar
	detener chan struct{} // se cierra al apagar: no se aceptan más solicitudes
	termino chan struct{} // el worker lo cierra al terminar de vaciar la cola
	cerrar  sync.Once     // evita cerrar detener dos veces
}

// NewRecuperacionService inicia el despacho en segundo plano. Sin transporte de correo o
// sin URL de restablecimiento devuelve un servicio apagado que sólo valida el formato.
func NewRecuperacionService(usuarios domain.UsuarioRepository, tokens domain.RecuperacionRepository, correo mailer.Mailer, urlRestablecer string, log *slog.Logger) RecuperacionService {
	if correo == nil || urlRestablecer == "" || usuarios == nil || tokens == nil {
		return recuperacionApagada{}
	}
	if log == nil {
		log = slog.Default()
	}
	s := &recuperacionService{
		usuarios:       usuarios,
		tokens:         tokens,
		correo:         correo,
		urlRestablecer: urlRestablecer,
		log:            log,
		cola:           make(chan string, capacidadCola),
		detener:        make(chan struct{}),
		termino:        make(chan struct{}),
	}
	go s.trabajar()
	return s
}

func normalizarCorreoRecuperacion(correo string) (string, error) {
	correo = normalizarCorreo(correo)
	if !validarEmail(correo) {
		return "", ErrCorreoInvalido
	}
	return correo, nil
}

func (s *recuperacionService) Solicitar(correo string) error {
	correo, err := normalizarCorreoRecuperacion(correo)
	if err != nil {
		return err
	}
	// Nunca bloquea la petición HTTP: si el servicio se está apagando o la cola
	// está llena, la solicitud se descarta y la respuesta es la misma.
	select {
	case <-s.detener:
	case s.cola <- correo:
	default:
		s.log.Warn("recuperacion_cola_llena", "capacidad", capacidadCola)
	}
	return nil
}

func (s *recuperacionService) Cerrar(ctx context.Context) error {
	s.cerrar.Do(func() { close(s.detener) })
	select {
	case <-s.termino:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// trabajar procesa la cola hasta que se pide detener; entonces vacía lo que
// quedaba encolado y avisa por termino.
func (s *recuperacionService) trabajar() {
	defer close(s.termino)
	for {
		select {
		case correo := <-s.cola:
			s.procesar(correo)
		case <-s.detener:
			for { // vaciado final sin esperar nuevas solicitudes
				select {
				case correo := <-s.cola:
					s.procesar(correo)
				default:
					return
				}
			}
		}
	}
}

// procesar emite el token y envía el correo. Las cuentas inexistentes o inactivas se
// ignoran en silencio. Nunca registra el correo: los logs usan el UUID de la cuenta.
func (s *recuperacionService) procesar(correo string) {
	ctx, cancel := context.WithTimeout(context.Background(), plazoPorSolicitud)
	defer cancel()
	usuario, err := s.usuarios.FindByEmail(ctx, correo)
	if err != nil {
		s.log.Error("recuperacion_fallida", "etapa", "cuenta", "error", err)
		return
	}
	if usuario == nil || !usuario.EstaActivo {
		return
	}
	enVentana, enEspera, err := s.tokens.SolicitudesRecientes(ctx, usuario.ID, ventanaSolicitudes, esperaEntreSolicitudes)
	if err != nil {
		s.log.Error("recuperacion_fallida", "etapa", "limite", "usuario_id", usuario.ID, "error", err)
		return
	}
	if enEspera || enVentana >= maximoPorVentana {
		s.log.Info("recuperacion_limitada", "usuario_id", usuario.ID)
		return
	}
	token, hash, err := security.GenerarTokenRecuperacion()
	if err != nil {
		s.log.Error("recuperacion_fallida", "etapa", "token", "usuario_id", usuario.ID, "error", err)
		return
	}
	if err := s.tokens.EmitirToken(ctx, usuario.ID, hash, VigenciaTokenRecuperacion); err != nil {
		s.log.Error("recuperacion_fallida", "etapa", "guardar_token", "usuario_id", usuario.ID, "error", err)
		return
	}
	mensaje, err := mensajeRecuperacion(usuario, s.urlRestablecer, token)
	if err == nil {
		err = s.correo.Enviar(ctx, mensaje)
	}
	if err != nil {
		s.log.Warn("recuperacion_correo_no_enviado", "usuario_id", usuario.ID, "error", err)
		return
	}
	s.log.Info("recuperacion_enviada", "usuario_id", usuario.ID)
}

// recuperacionApagada se usa cuando no hay transporte de correo o URL: valida el
// formato para responder igual que el servicio real, pero no envía nada.
type recuperacionApagada struct{}

func (recuperacionApagada) Solicitar(correo string) error {
	_, err := normalizarCorreoRecuperacion(correo)
	return err
}

func (recuperacionApagada) Cerrar(context.Context) error { return nil }

var plantillaRecuperacion = template.Must(template.New("recuperacion").Parse(`<!doctype html>
<html lang="es"><body style="font-family:Arial,sans-serif;color:#1f2933;line-height:1.5">
<p>{{.Saludo}}:</p>
<p>Recibimos una solicitud para restablecer la contraseña de tu cuenta de RutaAhorro.</p>
<p><a href="{{.Enlace}}" style="display:inline-block;padding:10px 18px;background:#15803d;color:#ffffff;text-decoration:none;border-radius:6px">Elegir una nueva contraseña</a></p>
<p>El enlace vence en {{.Minutos}} minutos y sirve una sola vez. Si el botón no funciona, copia esta dirección en el navegador:<br>{{.Enlace}}</p>
<p>Si no lo pediste, ignora este correo: tu contraseña no cambia.</p>
</body></html>`))

func mensajeRecuperacion(usuario *domain.Usuario, urlRestablecer, token string) (mailer.Mensaje, error) {
	enlace := urlRestablecer + "?token=" + url.QueryEscape(token)
	saludo := "Hola"
	if nombre := strings.TrimSpace(usuario.NombreCompleto); nombre != "" {
		saludo += ", " + nombre
	}
	minutos := int(VigenciaTokenRecuperacion.Minutes())
	var html bytes.Buffer
	// html/template escapa el nombre y filtra esquemas peligrosos en el href.
	if err := plantillaRecuperacion.Execute(&html, map[string]any{"Saludo": saludo, "Enlace": enlace, "Minutos": minutos}); err != nil {
		return mailer.Mensaje{}, err
	}
	texto := fmt.Sprintf("%s:\n\nRecibimos una solicitud para restablecer la contraseña de tu cuenta de RutaAhorro.\n"+
		"Abre este enlace para elegir una nueva. Vence en %d minutos y sirve una sola vez:\n\n%s\n\n"+
		"Si no lo pediste, ignora este correo: tu contraseña no cambia.\n", saludo, minutos, enlace)
	return mailer.Mensaje{Para: usuario.Correo, Asunto: "Restablece tu contraseña de RutaAhorro", Texto: texto, HTML: html.String()}, nil
}
