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
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/google/uuid"
)

const urlRestablecer = "https://frontend.example/restablecer-contrasena"

type tokenEmitido struct {
	usuario  uuid.UUID
	hash     []byte
	vigencia time.Duration
}

type tokensFalsos struct {
	mu        sync.Mutex
	enVentana int
	enEspera  bool
	emitidos  []tokenEmitido
}

func (prueba *tokensFalsos) SolicitudesRecientes(context.Context, uuid.UUID, time.Duration, time.Duration) (int, bool, error) {
	prueba.mu.Lock()
	defer prueba.mu.Unlock()
	return prueba.enVentana, prueba.enEspera, nil
}

func (prueba *tokensFalsos) EmitirToken(_ context.Context, usuario uuid.UUID, hash []byte, vigencia time.Duration) error {
	prueba.mu.Lock()
	defer prueba.mu.Unlock()
	prueba.emitidos = append(prueba.emitidos, tokenEmitido{usuario, hash, vigencia})
	return nil
}

type correoFalso struct {
	mu       sync.Mutex
	enviados []mailer.Mensaje
	err      error
	bloqueo  chan struct{}
}

func (contexto *correoFalso) Enviar(ctx context.Context, mensaje mailer.Mensaje) error {
	if contexto.bloqueo != nil {
		select {
		case <-contexto.bloqueo:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	contexto.mu.Lock()
	defer contexto.mu.Unlock()
	contexto.enviados = append(contexto.enviados, mensaje)
	return contexto.err
}

type usuariosQueFallan struct{ *mockUsuarioRepository }

func (usuariosQueFallan) FindByEmail(context.Context, string) (*domain.Usuario, error) {
	return nil, errors.New("conexión rechazada")
}

func usuarioDePrueba(repo *mockUsuarioRepository, correo string, activo bool, conClave bool) *domain.Usuario {
	usuarioLocal := &domain.Usuario{ID: uuid.New(), Correo: correo, NombreCompleto: "Ana <b>Pérez</b>", EstaActivo: activo}
	if conClave {
		hash := "$2a$10$hash-de-prueba"
		usuarioLocal.PasswordHash = &hash
	}
	repo.usuarios[correo] = usuarioLocal
	return usuarioLocal
}

func procesarPendientes(prueba *testing.T, servicio services.RecuperacionService) {
	prueba.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := servicio.Cerrar(ctx); err != nil {
		prueba.Fatalf("los envíos en curso no terminaron: %v", err)
	}
}

var tokenEnEnlace = regexp.MustCompile(`token=([A-Za-z0-9_-]{43})`)

func TestRecuperacionEmiteTokenHasheadoYEnviaCorreo(prueba *testing.T) {
	repo, tokens, correo := newMockUsuarioRepository(), &tokensFalsos{}, &correoFalso{}
	ana := usuarioDePrueba(repo, "ana@example.com", true, true)
	servicio := services.NewRecuperacionService(repo, tokens, correo, urlRestablecer, nil, config.AjustesPredeterminados())
	if err := servicio.Solicitar("  ANA@Example.com "); err != nil {
		prueba.Fatal(err)
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
	if !bytes.Equal(security.HashTokenRecuperacion(coincidencia[1]), tokens.emitidos[0].hash) {
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
	repo, tokens, correo := newMockUsuarioRepository(), &tokensFalsos{}, &correoFalso{}
	usuarioDePrueba(repo, "inactiva@example.com", false, true)
	google := usuarioDePrueba(repo, "google@example.com", true, false)
	servicio := services.NewRecuperacionService(repo, tokens, correo, urlRestablecer, nil, config.AjustesPredeterminados())
	for _, contexto := range []string{"nadie@example.com", "inactiva@example.com", "google@example.com"} {
		if err := servicio.Solicitar(contexto); err != nil {
			prueba.Fatalf("%s: la respuesta debe ser igual para toda cuenta: %v", contexto, err)
		}
	}
	procesarPendientes(prueba, servicio)
	// La cuenta Google demuestra controlar el correo: recibe el enlace igual que una local.
	if len(correo.enviados) != 1 || correo.enviados[0].Para != "google@example.com" || len(tokens.emitidos) != 1 || tokens.emitidos[0].usuario != google.ID {
		prueba.Fatalf("sólo la cuenta activa debía recibir el enlace: %+v", correo.enviados)
	}
}

func TestRecuperacionValidaFormatoSinProcesar(prueba *testing.T) {
	repo, tokens, correo := newMockUsuarioRepository(), &tokensFalsos{}, &correoFalso{}
	servicio := services.NewRecuperacionService(repo, tokens, correo, urlRestablecer, nil, config.AjustesPredeterminados())
	for _, contexto := range []string{"", "   ", "no-es-correo", "a@b", "ana@example.com\r\nBcc: x@example.com"} {
		if err := servicio.Solicitar(contexto); !errors.Is(err, services.ErrCorreoInvalido) {
			prueba.Errorf("%q: esperaba ErrCorreoInvalido, obtuvo %v", contexto, err)
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
			repo, correo := newMockUsuarioRepository(), &correoFalso{}
			usuarioDePrueba(repo, "ana@example.com", true, true)
			servicio := services.NewRecuperacionService(repo, tokens, correo, urlRestablecer, nil, config.AjustesPredeterminados())
			if err := servicio.Solicitar("ana@example.com"); err != nil {
				prueba.Fatal(err)
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
	log := slog.New(slog.NewJSONHandler(&salida, nil))
	servicio := services.NewRecuperacionService(usuariosQueFallan{newMockUsuarioRepository()}, &tokensFalsos{}, &correoFalso{}, urlRestablecer, log, config.AjustesPredeterminados())
	_ = servicio.Solicitar("secreta@example.com")
	procesarPendientes(prueba, servicio)

	repo, tokens := newMockUsuarioRepository(), &tokensFalsos{}
	usuarioDePrueba(repo, "secreta@example.com", true, true)
	servicio = services.NewRecuperacionService(repo, tokens, &correoFalso{err: errors.New("smtp autenticacion: 535")}, urlRestablecer, log, config.AjustesPredeterminados())
	_ = servicio.Solicitar("secreta@example.com")
	procesarPendientes(prueba, servicio)

	logs := salida.String()
	if !strings.Contains(logs, "recuperacion_fallida") || !strings.Contains(logs, "recuperacion_correo_no_enviado") {
		prueba.Fatalf("faltan los diagnósticos: %s", logs)
	}
	if strings.Contains(logs, "secreta@example.com") || strings.Contains(logs, "token=") {
		prueba.Fatalf("los logs exponen el correo o el token: %s", logs)
	}
}

func TestRecuperacionApagadaSinTransporteOURL(prueba *testing.T) {
	repo, tokens := newMockUsuarioRepository(), &tokensFalsos{}
	usuarioDePrueba(repo, "ana@example.com", true, true)
	for _, servicio := range []services.RecuperacionService{
		services.NewRecuperacionService(repo, tokens, nil, urlRestablecer, nil, config.AjustesPredeterminados()),
		services.NewRecuperacionService(repo, tokens, &correoFalso{}, "", nil, config.AjustesPredeterminados()),
	} {
		if err := servicio.Solicitar("ana@example.com"); err != nil {
			prueba.Fatal(err)
		}
		if err := servicio.Solicitar("no-es-correo"); !errors.Is(err, services.ErrCorreoInvalido) {
			prueba.Fatal("el modo apagado debe seguir validando el formato")
		}
		procesarPendientes(prueba, servicio)
	}
	if len(tokens.emitidos) != 0 {
		prueba.Fatal("el modo apagado no debe emitir tokens")
	}
}

func TestRecuperacionNoBloqueaYCerrarRespetaPlazo(prueba *testing.T) {
	repo, tokens := newMockUsuarioRepository(), &tokensFalsos{}
	usuarioDePrueba(repo, "ana@example.com", true, true)
	correo := &correoFalso{bloqueo: make(chan struct{})}
	defer close(correo.bloqueo)
	servicio := services.NewRecuperacionService(repo, tokens, correo, urlRestablecer, nil, config.AjustesPredeterminados())
	inicio := time.Now()
	for indice := 0; indice < 200; indice++ {
		if err := servicio.Solicitar("ana@example.com"); err != nil {
			prueba.Fatal(err)
		}
	}
	if time.Since(inicio) > time.Second {
		prueba.Fatal("Solicitar se bloqueó con envíos pendientes")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := servicio.Cerrar(ctx); !errors.Is(err, context.DeadlineExceeded) {
		prueba.Fatalf("Cerrar debía respetar el plazo del apagado: %v", err)
	}
}

func TestTokenRecuperacionAleatorioYHashEstable(prueba *testing.T) {
	vistos := map[string]bool{}
	for indice := 0; indice < 50; indice++ {
		token, hash, err := security.GenerarTokenRecuperacion()
		if err != nil || len(token) != 43 || len(hash) != 32 || vistos[token] {
			prueba.Fatalf("token inválido o repetido: %q %v", token, err)
		}
		if !bytes.Equal(hash, security.HashTokenRecuperacion(token)) {
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
	servicio := services.NewRecuperacionService(repositorio, tokens, correo, urlRestablecer, nil, ajustes)
	if errorSolicitud := servicio.Solicitar("ana@example.com"); errorSolicitud != nil {
		prueba.Fatal(errorSolicitud)
	}
	procesarPendientes(prueba, servicio)
	if len(tokens.emitidos) != 1 || tokens.emitidos[0].vigencia != 15*time.Minute || len(correo.enviados) != 1 || !strings.Contains(correo.enviados[0].Texto, "15 minutos") || !strings.Contains(correo.enviados[0].HTML, "15 minutos") {
		prueba.Fatal("el token y el correo no respetan la vigencia configurada")
	}
}
