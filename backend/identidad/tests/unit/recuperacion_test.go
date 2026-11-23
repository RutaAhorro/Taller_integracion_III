// Pruebas unitarias del restablecimiento por token: validación del token,
// complejidad compartida con SUP-281 y consumo de un solo uso.

package unit_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
)

// ===== Doble de repositorio =====

// tokensFalsos registra lo que el servicio pidió persistir, para comprobar que
// el token se hashó y que la clave se guardó con el hash de la política vigente.
type tokensFalsos struct {
	tokenValido   string
	errorValidar  error
	errorConsumir error

	vecesValidar           int
	vecesConsumir          int
	hashRecibido           []byte
	hashContrasenaRecibido string
	actualizadoEl          time.Time
}

var _ domain.RepositorioRecuperacion = (*tokensFalsos)(nil)

// Validar simula la consulta del token vigente en PostgreSQL.
func (tokens *tokensFalsos) Validar(_ context.Context, hashToken []byte) error {
	tokens.vecesValidar++
	tokens.hashRecibido = bytes.Clone(hashToken)
	if tokens.errorValidar != nil {
		return tokens.errorValidar
	}
	suma := sha256.Sum256([]byte(tokens.tokenValido))
	if tokens.tokenValido == "" || !bytes.Equal(hashToken, suma[:]) {
		return domain.ErrTokenRecuperacionInvalido
	}
	return nil
}

// Consumir simula la transacción atómica de consumo y cambio de contraseña.
func (tokens *tokensFalsos) Consumir(_ context.Context, hashToken []byte, hashContrasena string, actualizadoEl time.Time) error {
	tokens.vecesConsumir++
	tokens.hashRecibido = bytes.Clone(hashToken)
	tokens.hashContrasenaRecibido = hashContrasena
	tokens.actualizadoEl = actualizadoEl
	return tokens.errorConsumir
}

// ===== Casos =====

// TestServicioRecuperacionTokenInvalido comprueba que un token vacío o ajeno
// se rechaza sin llegar a consumir nada.
func TestServicioRecuperacionTokenInvalido(prueba *testing.T) {
	casos := []struct {
		nombre string
		token  string
	}{
		{"token vacío", ""},
		{"token con espacios", "   "},
		{"token inexistente", "token-ajeno"},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			tokens := &tokensFalsos{tokenValido: "token-valido"}
			servicio := services.NuevoServicioRecuperacion(tokens)

			errorObtenido := servicio.Restablecer(context.Background(), caso.token, "ClaveFuerte1!")

			if !errors.Is(errorObtenido, domain.ErrTokenRecuperacionInvalido) {
				subprueba.Fatalf("error=%v, esperado token inválido", errorObtenido)
			}
			if tokens.vecesConsumir != 0 {
				subprueba.Fatalf("se intentó consumir un token inválido: %d", tokens.vecesConsumir)
			}
		})
	}
}

// TestServicioRecuperacionTokenExpiradoOUsado cubre las dos respuestas que el
// repositorio devuelve como inválidas: vencimiento y uso previo.
func TestServicioRecuperacionTokenExpiradoOUsado(prueba *testing.T) {
	for _, nombre := range []string{"expirado", "usado"} {
		prueba.Run(nombre, func(subprueba *testing.T) {
			tokens := &tokensFalsos{errorValidar: domain.ErrTokenRecuperacionInvalido}
			servicio := services.NuevoServicioRecuperacion(tokens)

			errorObtenido := servicio.Restablecer(context.Background(), "token-vigente", "ClaveFuerte1!")

			if !errors.Is(errorObtenido, domain.ErrTokenRecuperacionInvalido) {
				subprueba.Fatalf("error=%v, esperado token inválido", errorObtenido)
			}
			if tokens.vecesConsumir != 0 {
				subprueba.Fatalf("se consumió un token %s", nombre)
			}
		})
	}
}

// TestServicioRecuperacionClaveDebilNoConsumeToken verifica que la política de
// SUP-281 se aplica antes de escribir y que una clave rechazada no inutiliza el enlace.
func TestServicioRecuperacionClaveDebilNoConsumeToken(prueba *testing.T) {
	tokens := &tokensFalsos{tokenValido: "token-valido"}
	servicio := services.NuevoServicioRecuperacion(tokens)

	errorObtenido := servicio.Restablecer(context.Background(), "token-valido", "clave-corta")

	if !errors.Is(errorObtenido, services.ErrContrasenaInvalida) {
		prueba.Fatalf("error=%v, esperada contraseña inválida", errorObtenido)
	}
	if tokens.vecesValidar != 1 || tokens.vecesConsumir != 0 {
		prueba.Fatalf("validaciones=%d consumos=%d", tokens.vecesValidar, tokens.vecesConsumir)
	}
}

// TestServicioRecuperacionExitoso comprueba el hash del token, el hash bcrypt
// guardado y la fecha de actualización enviada al repositorio.
func TestServicioRecuperacionExitoso(prueba *testing.T) {
	const token = "token-valido"
	const contrasena = "ClaveFuerte1!"
	tokens := &tokensFalsos{tokenValido: token}
	servicio := services.NuevoServicioRecuperacion(tokens)

	errorObtenido := servicio.Restablecer(context.Background(), token, contrasena)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}

	if tokens.vecesValidar != 1 || tokens.vecesConsumir != 1 {
		prueba.Fatalf("validaciones=%d consumos=%d", tokens.vecesValidar, tokens.vecesConsumir)
	}
	suma := sha256.Sum256([]byte(token))
	if !bytes.Equal(tokens.hashRecibido, suma[:]) {
		prueba.Fatal("el repositorio no recibió el SHA-256 del token")
	}
	if !services.ContrasenaCoincide(contrasena, tokens.hashContrasenaRecibido) {
		prueba.Fatal("el repositorio no recibió un hash bcrypt de la clave nueva")
	}
	if tokens.actualizadoEl.IsZero() {
		prueba.Fatal("no se envió la fecha de actualización")
	}
}

// TestServicioRecuperacionTokenReutilizado simula una segunda petición que
// pierde la carrera por el token: el repositorio devuelve el error de un solo uso.
func TestServicioRecuperacionTokenReutilizado(prueba *testing.T) {
	tokens := &tokensFalsos{
		tokenValido:   "token-valido",
		errorConsumir: domain.ErrTokenRecuperacionInvalido,
	}
	servicio := services.NuevoServicioRecuperacion(tokens)

	errorObtenido := servicio.Restablecer(context.Background(), "token-valido", "ClaveFuerte1!")

	if !errors.Is(errorObtenido, domain.ErrTokenRecuperacionInvalido) {
		prueba.Fatalf("error=%v, esperado token ya usado", errorObtenido)
	}
	if tokens.vecesConsumir != 1 {
		prueba.Fatalf("consumos=%d", tokens.vecesConsumir)
	}
}
