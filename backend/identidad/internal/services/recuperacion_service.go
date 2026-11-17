// Recuperación de contraseña (SUP-184): emite el token de restablecimiento y
// despacha su correo. El endpoint que la expone es SUP-233.
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

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/mailer"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
)

// ===== Servicio =====

// RecuperacionService emite el token de restablecimiento y despacha su correo.
type RecuperacionService interface {
	// Solicitar valida el formato y despacha el resto en segundo plano: la
	// respuesta y su tiempo no revelan si la cuenta existe, está inactiva o usa
	// Google. Sólo devuelve ErrCorreoInvalido.
	Solicitar(correo string) error
	// Cerrar espera los envíos en curso; se llama al apagar Identidad.
	Cerrar(ctx context.Context) error
}

// recuperacionService procesa cada solicitud en su propia goroutine. No hay
// cola: el límite de carga lo ponen el rate limit del endpoint y el límite por
// cuenta de procesar.
type recuperacionService struct {
	usuarios       domain.UsuarioRepository
	tokens         domain.RecuperacionRepository
	correo         mailer.Mailer
	urlRestablecer string
	log            *slog.Logger
	ajustes        config.Ajustes
	// activa es false sin transporte de correo o sin URL de restablecimiento:
	// Solicitar sólo valida el formato y responde igual que el servicio activo.
	activa bool
	// enCurso cuenta los envíos pendientes para no perderlos al apagar.
	enCurso sync.WaitGroup
}

// NewRecuperacionService arma el servicio. Sin transporte de correo o sin URL de
// restablecimiento queda apagado: valida el formato pero no emite tokens ni correos.
func NewRecuperacionService(usuarios domain.UsuarioRepository, tokens domain.RecuperacionRepository, correo mailer.Mailer, urlRestablecer string, log *slog.Logger, ajustes config.Ajustes) RecuperacionService {
	if log == nil {
		log = slog.Default()
	}
	return &recuperacionService{
		usuarios:       usuarios,
		tokens:         tokens,
		correo:         correo,
		urlRestablecer: urlRestablecer,
		log:            log,
		ajustes:        ajustes,
		activa:         correo != nil && urlRestablecer != "" && usuarios != nil && tokens != nil,
	}
}

// Solicitar nunca bloquea la petición HTTP: valida el formato y delega el resto.
func (servicio *recuperacionService) Solicitar(correo string) error {
	correo = normalizarCorreo(correo)
	if !validarEmail(correo) {
		return ErrCorreoInvalido
	}
	if !servicio.activa {
		return nil
	}
	servicio.enCurso.Add(1)
	go func() {
		defer servicio.enCurso.Done()
		servicio.procesar(correo)
	}()
	return nil
}

// Cerrar espera los envíos en curso hasta que venza el plazo del apagado.
func (servicio *recuperacionService) Cerrar(ctx context.Context) error {
	terminado := make(chan struct{})
	go func() {
		servicio.enCurso.Wait()
		close(terminado)
	}()
	select {
	case <-terminado:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// procesar emite el token y envía el correo. Las cuentas inexistentes o inactivas se
// ignoran en silencio. Nunca registra el correo: los logs usan el UUID de la cuenta.
func (servicio *recuperacionService) procesar(correo string) {
	ctx, cancel := context.WithTimeout(context.Background(), servicio.ajustes.RecuperacionTimeout)
	defer cancel()
	usuario, err := servicio.usuarios.FindByEmail(ctx, correo)
	if err != nil {
		servicio.log.Error("recuperacion_fallida", "etapa", "cuenta", "error", err)
		return
	}
	if usuario == nil || !usuario.EstaActivo {
		return
	}
	enVentana, enEspera, err := servicio.tokens.SolicitudesRecientes(ctx, usuario.ID, servicio.ajustes.RecuperacionVentana, servicio.ajustes.RecuperacionEspera)
	if err != nil {
		servicio.log.Error("recuperacion_fallida", "etapa", "limite", "usuario_id", usuario.ID, "error", err)
		return
	}
	if enEspera || enVentana >= servicio.ajustes.RecuperacionMax {
		servicio.log.Info("recuperacion_limitada", "usuario_id", usuario.ID)
		return
	}
	token, hash, err := security.GenerarTokenRecuperacion()
	if err != nil {
		servicio.log.Error("recuperacion_fallida", "etapa", "token", "usuario_id", usuario.ID, "error", err)
		return
	}
	if err := servicio.tokens.EmitirToken(ctx, usuario.ID, hash, servicio.ajustes.RecuperacionVigencia); err != nil {
		servicio.log.Error("recuperacion_fallida", "etapa", "guardar_token", "usuario_id", usuario.ID, "error", err)
		return
	}
	mensaje, err := mensajeRecuperacion(usuario, servicio.urlRestablecer, token, servicio.ajustes.RecuperacionVigencia)
	if err == nil {
		err = servicio.correo.Enviar(ctx, mensaje)
	}
	if err != nil {
		servicio.log.Warn("recuperacion_correo_no_enviado", "usuario_id", usuario.ID, "error", err)
		return
	}
	servicio.log.Info("recuperacion_enviada", "usuario_id", usuario.ID)
}

// ===== Correo =====

var plantillaRecuperacion = template.Must(template.New("recuperacion").Parse(`<!doctype html>
<html lang="es"><body style="font-family:Arial,sans-serif;color:#1f2933;line-height:1.5">
<p>{{.Saludo}}:</p>
<p>Recibimos una solicitud para restablecer la contraseña de tu cuenta de RutaAhorro.</p>
<p><a href="{{.Enlace}}" style="display:inline-block;padding:10px 18px;background:#15803d;color:#ffffff;text-decoration:none;border-radius:6px">Elegir una nueva contraseña</a></p>
<p>El enlace vence en {{.Minutos}} minutos y sirve una sola vez. Si el botón no funciona, copia esta dirección en el navegador:<br>{{.Enlace}}</p>
<p>Si no lo pediste, ignora este correo: tu contraseña no cambia.</p>
</body></html>`))

func mensajeRecuperacion(usuario *domain.Usuario, urlRestablecer, token string, vigencia time.Duration) (mailer.Mensaje, error) {
	enlace := urlRestablecer + "?token=" + url.QueryEscape(token)
	saludo := "Hola"
	if nombre := strings.TrimSpace(usuario.NombreCompleto); nombre != "" {
		saludo += ", " + nombre
	}
	minutos := int(vigencia.Minutes())
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
