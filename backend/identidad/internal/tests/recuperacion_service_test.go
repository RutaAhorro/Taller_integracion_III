package tests

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

func (t *tokensFalsos) SolicitudesRecientes(context.Context, uuid.UUID, time.Duration, time.Duration) (int, bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.enVentana, t.enEspera, nil
}

func (t *tokensFalsos) EmitirToken(_ context.Context, usuario uuid.UUID, hash []byte, vigencia time.Duration) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.emitidos = append(t.emitidos, tokenEmitido{usuario, hash, vigencia})
	return nil
}

type correoFalso struct {
	mu       sync.Mutex
	enviados []mailer.Mensaje
	err      error
	bloqueo  chan struct{}
}

func (c *correoFalso) Enviar(ctx context.Context, m mailer.Mensaje) error {
	if c.bloqueo != nil {
		select {
		case <-c.bloqueo:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enviados = append(c.enviados, m)
	return c.err
}

type usuariosQueFallan struct{ *mockUsuarioRepository }

func (usuariosQueFallan) FindByEmail(context.Context, string) (*domain.Usuario, error) {
	return nil, errors.New("conexión rechazada")
}

func usuarioDePrueba(repo *mockUsuarioRepository, correo string, activo bool, conClave bool) *domain.Usuario {
	u := &domain.Usuario{ID: uuid.New(), Correo: correo, NombreCompleto: "Ana <b>Pérez</b>", EstaActivo: activo}
	if conClave {
		hash := "$2a$10$hash-de-prueba"
		u.PasswordHash = &hash
	}
	repo.usuarios[correo] = u
	return u
}

func procesarPendientes(t *testing.T, s services.RecuperacionService) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Cerrar(ctx); err != nil {
		t.Fatalf("la cola no terminó: %v", err)
	}
}

var tokenEnEnlace = regexp.MustCompile(`token=([A-Za-z0-9_-]{43})`)

func TestRecuperacionEmiteTokenHasheadoYEnviaCorreo(t *testing.T) {
	repo, tokens, correo := newMockUsuarioRepository(), &tokensFalsos{}, &correoFalso{}
	ana := usuarioDePrueba(repo, "ana@example.com", true, true)
	s := services.NewRecuperacionService(repo, tokens, correo, urlRestablecer, nil)
	if err := s.Solicitar("  ANA@Example.com "); err != nil {
		t.Fatal(err)
	}
	procesarPendientes(t, s)

	if len(tokens.emitidos) != 1 || tokens.emitidos[0].usuario != ana.ID || tokens.emitidos[0].vigencia != 30*time.Minute {
		t.Fatalf("token no emitido como se esperaba: %+v", tokens.emitidos)
	}
	if len(correo.enviados) != 1 || correo.enviados[0].Para != "ana@example.com" || !strings.Contains(correo.enviados[0].Asunto, "Restablece") {
		t.Fatalf("correo inesperado: %+v", correo.enviados)
	}
	m := correo.enviados[0]
	coincidencia := tokenEnEnlace.FindStringSubmatch(m.Texto)
	if coincidencia == nil || !strings.Contains(m.Texto, urlRestablecer+"?token=") {
		t.Fatalf("el texto no trae el enlace: %q", m.Texto)
	}
	if !bytes.Equal(security.HashTokenRecuperacion(coincidencia[1]), tokens.emitidos[0].hash) {
		t.Fatal("el hash guardado no corresponde al token enviado")
	}
	if bytes.Contains(tokens.emitidos[0].hash, []byte(coincidencia[1])) {
		t.Fatal("se guardó el token en claro")
	}
	if !strings.Contains(m.HTML, "Ana &lt;b&gt;Pérez&lt;/b&gt;") || strings.Contains(m.HTML, "<b>Pérez") || !strings.Contains(m.Texto, "30 minutos") {
		t.Fatalf("plantilla sin escapar o sin vigencia: %q", m.HTML)
	}
}

func TestRecuperacionNoRevelaCuentasNiProveedor(t *testing.T) {
	repo, tokens, correo := newMockUsuarioRepository(), &tokensFalsos{}, &correoFalso{}
	usuarioDePrueba(repo, "inactiva@example.com", false, true)
	google := usuarioDePrueba(repo, "google@example.com", true, false)
	s := services.NewRecuperacionService(repo, tokens, correo, urlRestablecer, nil)
	for _, c := range []string{"nadie@example.com", "inactiva@example.com", "google@example.com"} {
		if err := s.Solicitar(c); err != nil {
			t.Fatalf("%s: la respuesta debe ser igual para toda cuenta: %v", c, err)
		}
	}
	procesarPendientes(t, s)
	// La cuenta Google demuestra controlar el correo: recibe el enlace igual que una local.
	if len(correo.enviados) != 1 || correo.enviados[0].Para != "google@example.com" || len(tokens.emitidos) != 1 || tokens.emitidos[0].usuario != google.ID {
		t.Fatalf("sólo la cuenta activa debía recibir el enlace: %+v", correo.enviados)
	}
}

func TestRecuperacionValidaFormatoSinEncolar(t *testing.T) {
	repo, tokens, correo := newMockUsuarioRepository(), &tokensFalsos{}, &correoFalso{}
	s := services.NewRecuperacionService(repo, tokens, correo, urlRestablecer, nil)
	for _, c := range []string{"", "   ", "no-es-correo", "a@b", "ana@example.com\r\nBcc: x@example.com"} {
		if err := s.Solicitar(c); !errors.Is(err, services.ErrCorreoInvalido) {
			t.Errorf("%q: esperaba ErrCorreoInvalido, obtuvo %v", c, err)
		}
	}
	procesarPendientes(t, s)
	if len(tokens.emitidos) != 0 || len(correo.enviados) != 0 {
		t.Fatal("un correo inválido no debe procesarse")
	}
}

func TestRecuperacionLimitaSolicitudesPorCuenta(t *testing.T) {
	for nombre, tokens := range map[string]*tokensFalsos{"espera de un minuto": {enEspera: true}, "cinco por hora": {enVentana: 5}} {
		t.Run(nombre, func(t *testing.T) {
			repo, correo := newMockUsuarioRepository(), &correoFalso{}
			usuarioDePrueba(repo, "ana@example.com", true, true)
			s := services.NewRecuperacionService(repo, tokens, correo, urlRestablecer, nil)
			if err := s.Solicitar("ana@example.com"); err != nil {
				t.Fatal(err)
			}
			procesarPendientes(t, s)
			if len(tokens.emitidos) != 0 || len(correo.enviados) != 0 {
				t.Fatal("el límite por cuenta no se aplicó")
			}
		})
	}
}

func TestRecuperacionFallasQuedanEnLogSinCorreoDelUsuario(t *testing.T) {
	var salida bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&salida, nil))
	s := services.NewRecuperacionService(usuariosQueFallan{newMockUsuarioRepository()}, &tokensFalsos{}, &correoFalso{}, urlRestablecer, log)
	_ = s.Solicitar("secreta@example.com")
	procesarPendientes(t, s)

	repo, tokens := newMockUsuarioRepository(), &tokensFalsos{}
	usuarioDePrueba(repo, "secreta@example.com", true, true)
	s = services.NewRecuperacionService(repo, tokens, &correoFalso{err: errors.New("smtp autenticacion: 535")}, urlRestablecer, log)
	_ = s.Solicitar("secreta@example.com")
	procesarPendientes(t, s)

	logs := salida.String()
	if !strings.Contains(logs, "recuperacion_fallida") || !strings.Contains(logs, "recuperacion_correo_no_enviado") {
		t.Fatalf("faltan los diagnósticos: %s", logs)
	}
	if strings.Contains(logs, "secreta@example.com") || strings.Contains(logs, "token=") {
		t.Fatalf("los logs exponen el correo o el token: %s", logs)
	}
}

func TestRecuperacionApagadaSinTransporteOURL(t *testing.T) {
	repo, tokens := newMockUsuarioRepository(), &tokensFalsos{}
	usuarioDePrueba(repo, "ana@example.com", true, true)
	for _, s := range []services.RecuperacionService{
		services.NewRecuperacionService(repo, tokens, nil, urlRestablecer, nil),
		services.NewRecuperacionService(repo, tokens, &correoFalso{}, "", nil),
	} {
		if err := s.Solicitar("ana@example.com"); err != nil {
			t.Fatal(err)
		}
		if err := s.Solicitar("no-es-correo"); !errors.Is(err, services.ErrCorreoInvalido) {
			t.Fatal("el modo apagado debe seguir validando el formato")
		}
		procesarPendientes(t, s)
	}
	if len(tokens.emitidos) != 0 {
		t.Fatal("el modo apagado no debe emitir tokens")
	}
}

func TestRecuperacionColaLlenaNoBloqueaYCerrarRespetaPlazo(t *testing.T) {
	repo, tokens := newMockUsuarioRepository(), &tokensFalsos{}
	usuarioDePrueba(repo, "ana@example.com", true, true)
	correo := &correoFalso{bloqueo: make(chan struct{})}
	defer close(correo.bloqueo)
	s := services.NewRecuperacionService(repo, tokens, correo, urlRestablecer, nil)
	inicio := time.Now()
	for i := 0; i < 200; i++ {
		if err := s.Solicitar("ana@example.com"); err != nil {
			t.Fatal(err)
		}
	}
	if time.Since(inicio) > time.Second {
		t.Fatal("Solicitar se bloqueó con la cola llena")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := s.Cerrar(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Cerrar debía respetar el plazo del apagado: %v", err)
	}
}

func TestTokenRecuperacionAleatorioYHashEstable(t *testing.T) {
	vistos := map[string]bool{}
	for i := 0; i < 50; i++ {
		token, hash, err := security.GenerarTokenRecuperacion()
		if err != nil || len(token) != 43 || len(hash) != 32 || vistos[token] {
			t.Fatalf("token inválido o repetido: %q %v", token, err)
		}
		if !bytes.Equal(hash, security.HashTokenRecuperacion(token)) {
			t.Fatal("el hash no es reproducible")
		}
		vistos[token] = true
	}
}
