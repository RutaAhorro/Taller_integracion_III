// Recuperación de contraseña: token hasheado, correo, límites por cuenta,
// modo apagado y logs sin datos personales.

package unit_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/mailer"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/google/uuid"
)

const urlRestablecer = "https://frontend.example/restablecer-contrasena"

// tokenEmitido registra lo que el servicio pidió guardar.
type tokenEmitido struct {
	usuario  uuid.UUID
	hash     []byte
	vigencia time.Duration
}

// tokensFalsos simula el repositorio de tokens y sus límites por cuenta.
type tokensFalsos struct {
	mu        sync.Mutex
	enVentana int
	enEspera  bool
	emitidos  []tokenEmitido
}

// SolicitudesRecientes devuelve los límites configurados en la prueba.
func (tokens *tokensFalsos) SolicitudesRecientes(context.Context, uuid.UUID, time.Duration, time.Duration) (int, bool, error) {
	tokens.mu.Lock()
	defer tokens.mu.Unlock()
	return tokens.enVentana, tokens.enEspera, nil
}

// EmitirToken guarda la emisión para las aserciones.
func (tokens *tokensFalsos) EmitirToken(_ context.Context, usuario uuid.UUID, hash []byte, vigencia time.Duration) error {
	tokens.mu.Lock()
	defer tokens.mu.Unlock()
	tokens.emitidos = append(tokens.emitidos, tokenEmitido{usuario, hash, vigencia})
	return nil
}

// correoFalso registra los mensajes y puede bloquearse o fallar a pedido.
type correoFalso struct {
	mu            sync.Mutex
	enviados      []mailer.Mensaje
	errorDevuelto error
	bloqueo       chan struct{}
}

// Enviar espera el desbloqueo o el plazo y registra el mensaje.
func (correo *correoFalso) Enviar(contextoOperacion context.Context, mensaje mailer.Mensaje) error {
	if correo.bloqueo != nil {
		select {
		case <-correo.bloqueo:
		case <-contextoOperacion.Done():
			return contextoOperacion.Err()
		}
	}
	correo.mu.Lock()
	defer correo.mu.Unlock()
	correo.enviados = append(correo.enviados, mensaje)
	return correo.errorDevuelto
}

// usuariosQueFallan simula una base caída al buscar la cuenta.
type usuariosQueFallan struct{ *mockUsuarioRepository }

// BuscarPorCorreo siempre falla.
func (usuariosQueFallan) BuscarPorCorreo(context.Context, string) (*domain.Usuario, error) {
	return nil, errors.New("conexión rechazada")
}

// usuarioDePrueba agrega una cuenta con nombre que requiere escape HTML.
func usuarioDePrueba(repositorio *mockUsuarioRepository, correo string, activo bool, conClave bool) *domain.Usuario {
	usuarioLocal := &domain.Usuario{ID: uuid.New(), Correo: correo, NombreCompleto: "Ana <b>Pérez</b>", EstaActivo: activo}
	if conClave {
		hash := "$2a$10$hash-de-prueba"
		usuarioLocal.PasswordHash = &hash
	}
	repositorio.usuarios[correo] = usuarioLocal
	return usuarioLocal
}

// procesarPendientes espera a que terminen los envíos en segundo plano.
func procesarPendientes(prueba *testing.T, servicio *services.ServicioRecuperacion) {
	prueba.Helper()
	contexto, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if errorObtenido := servicio.Cerrar(contexto); errorObtenido != nil {
		prueba.Fatalf("los envíos en curso no terminaron: %v", errorObtenido)
	}
}

var tokenEnEnlace = regexp.MustCompile(`token=([A-Za-z0-9_-]{43})`)

func TestRecuperacionEmiteTokenHasheadoYEnviaCorreo(prueba *testing.T) {
	repositorio, tokens, correo := newMockUsuarioRepository(), &tokensFalsos{}, &correoFalso{}
	ana := usuarioDePrueba(repositorio, "ana@example.com", true, true)
	servicio := services.NuevoServicioRecuperacion(repositorio, tokens, correo, urlRestablecer, nil, config.AjustesPredeterminados())
	if errorObtenido := servicio.Solicitar("  ANA@Example.com "); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	procesarPendientes(prueba, servicio)

	if len(tokens.emitidos) != 1 || tokens.emitidos[0].usuario != ana.ID || tokens.emitidos[0].vigencia != 30*time.Minute {
		prueba.Fatalf("token no emitido como se esperaba: %+v", tokens.emitidos)
	}
	if len(correo.enviados) != 1 || correo.enviados[0].Para != "ana@example.com" || !strings.Contains(correo.enviados[0].Asunto, "Restablece") {
		prueba.Fatalf("correo inesperado: %+v", correo.enviados)
	}
	mensaje := correo.enviados[0]
	coincidencia := tokenEnEnlace.FindStringSubmatch(mensaje.Texto)
	if coincidencia == nil || !strings.Contains(mensaje.Texto, urlRestablecer+"?token=") {
		prueba.Fatalf("el texto no trae el enlace: %q", mensaje.Texto)
	}
	if !bytes.Equal(services.HashTokenRecuperacion(coincidencia[1]), tokens.emitidos[0].hash) {
		prueba.Fatal("el hash guardado no corresponde al token enviado")
	}
	if bytes.Contains(tokens.emitidos[0].hash, []byte(coincidencia[1])) {
		prueba.Fatal("se guardó el token en claro")
	}
	if !strings.Contains(mensaje.HTML, "Ana &lt;b&gt;Pérez&lt;/b&gt;") || strings.Contains(mensaje.HTML, "<b>Pérez") || !strings.Contains(mensaje.Texto, "30 minutos") {
		prueba.Fatalf("plantilla sin escapar o sin vigencia: %q", mensaje.HTML)
	}
}

func TestRecuperacionNoRevelaCuentasNiProveedor(prueba *testing.T) {
	repositorio, tokens, correo := newMockUsuarioRepository(), &tokensFalsos{}, &correoFalso{}
	usuarioDePrueba(repositorio, "inactiva@example.com", false, true)
	google := usuarioDePrueba(repositorio, "google@example.com", true, false)
	servicio := services.NuevoServicioRecuperacion(repositorio, tokens, correo, urlRestablecer, nil, config.AjustesPredeterminados())
	for _, direccion := range []string{"nadie@example.com", "inactiva@example.com", "google@example.com"} {
		if errorObtenido := servicio.Solicitar(direccion); errorObtenido != nil {
			prueba.Fatalf("%s: la respuesta debe ser igual para toda cuenta: %v", direccion, errorObtenido)
		}
	}
	procesarPendientes(prueba, servicio)
	// La cuenta Google demuestra controlar el correo: recibe el enlace igual que una local.
	if len(correo.enviados) != 1 || correo.enviados[0].Para != "google@example.com" || len(tokens.emitidos) != 1 || tokens.emitidos[0].usuario != google.ID {
		prueba.Fatalf("sólo la cuenta activa debía recibir el enlace: %+v", correo.enviados)
	}
}

func TestRecuperacionValidaFormatoSinProcesar(prueba *testing.T) {
	repositorio, tokens, correo := newMockUsuarioRepository(), &tokensFalsos{}, &correoFalso{}
	servicio := services.NuevoServicioRecuperacion(repositorio, tokens, correo, urlRestablecer, nil, config.AjustesPredeterminados())
	for _, direccion := range []string{"", "   ", "no-es-correo", "a@b", "ana@example.com\r\nBcc: x@example.com"} {
		if errorObtenido := servicio.Solicitar(direccion); !errors.Is(errorObtenido, services.ErrCorreoInvalido) {
			prueba.Errorf("%q: esperaba ErrCorreoInvalido, obtuvo %v", direccion, errorObtenido)
		}
	}
	procesarPendientes(prueba, servicio)
	if len(tokens.emitidos) != 0 || len(correo.enviados) != 0 {
		prueba.Fatal("un correo inválido no debe procesarse")
	}
}

func TestRecuperacionLimitaSolicitudesPorCuenta(prueba *testing.T) {
	for nombre, tokens := range map[string]*tokensFalsos{"espera de un minuto": {enEspera: true}, "cinco por hora": {enVentana: 5}} {
		prueba.Run(nombre, func(prueba *testing.T) {
			repositorio, correo := newMockUsuarioRepository(), &correoFalso{}
			usuarioDePrueba(repositorio, "ana@example.com", true, true)
			servicio := services.NuevoServicioRecuperacion(repositorio, tokens, correo, urlRestablecer, nil, config.AjustesPredeterminados())
			if errorObtenido := servicio.Solicitar("ana@example.com"); errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
			procesarPendientes(prueba, servicio)
			if len(tokens.emitidos) != 0 || len(correo.enviados) != 0 {
				prueba.Fatal("el límite por cuenta no se aplicó")
			}
		})
	}
}

func TestRecuperacionFallasQuedanEnLogSinCorreoDelUsuario(prueba *testing.T) {
	var salida bytes.Buffer
	registrador := slog.New(slog.NewJSONHandler(&salida, nil))
	servicio := services.NuevoServicioRecuperacion(usuariosQueFallan{newMockUsuarioRepository()}, &tokensFalsos{}, &correoFalso{}, urlRestablecer, registrador, config.AjustesPredeterminados())
	_ = servicio.Solicitar("secreta@example.com")
	procesarPendientes(prueba, servicio)

	repositorio, tokens := newMockUsuarioRepository(), &tokensFalsos{}
	usuarioDePrueba(repositorio, "secreta@example.com", true, true)
	servicio = services.NuevoServicioRecuperacion(repositorio, tokens, &correoFalso{errorDevuelto: errors.New("smtp autenticacion: 535")}, urlRestablecer, registrador, config.AjustesPredeterminados())
	_ = servicio.Solicitar("secreta@example.com")
	procesarPendientes(prueba, servicio)

	registros := salida.String()
	if !strings.Contains(registros, "recuperacion_fallida") || !strings.Contains(registros, "recuperacion_correo_no_enviado") {
		prueba.Fatalf("faltan los diagnósticos: %s", registros)
	}
	if strings.Contains(registros, "secreta@example.com") || strings.Contains(registros, "token=") {
		prueba.Fatalf("los logs exponen el correo o el token: %s", registros)
	}
}

func TestRecuperacionApagadaSinTransporteOURL(prueba *testing.T) {
	repositorio, tokens := newMockUsuarioRepository(), &tokensFalsos{}
	usuarioDePrueba(repositorio, "ana@example.com", true, true)
	for _, servicio := range []*services.ServicioRecuperacion{
		services.NuevoServicioRecuperacion(repositorio, tokens, nil, urlRestablecer, nil, config.AjustesPredeterminados()),
		services.NuevoServicioRecuperacion(repositorio, tokens, &correoFalso{}, "", nil, config.AjustesPredeterminados()),
	} {
		if errorObtenido := servicio.Solicitar("ana@example.com"); errorObtenido != nil {
			prueba.Fatal(errorObtenido)
		}
		if errorObtenido := servicio.Solicitar("no-es-correo"); !errors.Is(errorObtenido, services.ErrCorreoInvalido) {
			prueba.Fatal("el modo apagado debe seguir validando el formato")
		}
		procesarPendientes(prueba, servicio)
	}
	if len(tokens.emitidos) != 0 {
		prueba.Fatal("el modo apagado no debe emitir tokens")
	}
}

func TestRecuperacionNoBloqueaYCerrarRespetaPlazo(prueba *testing.T) {
	repositorio, tokens := newMockUsuarioRepository(), &tokensFalsos{}
	usuarioDePrueba(repositorio, "ana@example.com", true, true)
	correo := &correoFalso{bloqueo: make(chan struct{})}
	defer close(correo.bloqueo)
	servicio := services.NuevoServicioRecuperacion(repositorio, tokens, correo, urlRestablecer, nil, config.AjustesPredeterminados())
	inicio := time.Now()
	for indice := 0; indice < 200; indice++ {
		if errorObtenido := servicio.Solicitar("ana@example.com"); errorObtenido != nil {
			prueba.Fatal(errorObtenido)
		}
	}
	if time.Since(inicio) > time.Second {
		prueba.Fatal("Solicitar se bloqueó con envíos pendientes")
	}
	contexto, cancelar := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelar()
	if errorObtenido := servicio.Cerrar(contexto); !errors.Is(errorObtenido, context.DeadlineExceeded) {
		prueba.Fatalf("Cerrar debía respetar el plazo del apagado: %v", errorObtenido)
	}
}

func TestTokenRecuperacionAleatorioYHashEstable(prueba *testing.T) {
	vistos := map[string]bool{}
	for indice := 0; indice < 50; indice++ {
		token, hash, errorObtenido := services.GenerarTokenRecuperacion()
		if errorObtenido != nil || len(token) != 43 || len(hash) != 32 || vistos[token] {
			prueba.Fatalf("token inválido o repetido: %q %v", token, errorObtenido)
		}
		if !bytes.Equal(hash, services.HashTokenRecuperacion(token)) {
			prueba.Fatal("el hash no es reproducible")
		}
		vistos[token] = true
	}
}

// TestRecuperacionUsaVigenciaConfigurada mantiene coherencia entre BD y correo.
func TestRecuperacionUsaVigenciaConfigurada(prueba *testing.T) {
	repositorio, tokens, correo := newMockUsuarioRepository(), &tokensFalsos{}, &correoFalso{}
	usuarioDePrueba(repositorio, "ana@example.com", true, true)
	ajustes := config.AjustesPredeterminados()
	ajustes.RecuperacionVigencia = 15 * time.Minute
	servicio := services.NuevoServicioRecuperacion(repositorio, tokens, correo, urlRestablecer, nil, ajustes)
	if errorSolicitud := servicio.Solicitar("ana@example.com"); errorSolicitud != nil {
		prueba.Fatal(errorSolicitud)
	}
	procesarPendientes(prueba, servicio)
	if len(tokens.emitidos) != 1 || tokens.emitidos[0].vigencia != 15*time.Minute || len(correo.enviados) != 1 || !strings.Contains(correo.enviados[0].Texto, "15 minutos") || !strings.Contains(correo.enviados[0].HTML, "15 minutos") {
		prueba.Fatal("el token y el correo no respetan la vigencia configurada")
	}
}
