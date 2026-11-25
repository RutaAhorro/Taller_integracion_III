// Restablecimiento contra PostgreSQL real y desechable: vigencia, un solo uso,
// complejidad y consumo atómico. Se omite sin IDENTIDAD_TEST_DB_URL
// (app_identity) e IDENTIDAD_TEST_ADMIN_URL (operador para limpiar).

package integration_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ===== Siembra y lectura =====

// sembrarTokenRestablecer inserta el SHA-256 del token con la vigencia pedida.
// El token en claro vive sólo en la prueba, igual que en el flujo por correo.
func sembrarTokenRestablecer(prueba *testing.T, base *gorm.DB, usuarioID uuid.UUID, token string, vigencia time.Duration, usado bool) []byte {
	prueba.Helper()
	suma := sha256.Sum256([]byte(token))
	consulta := `INSERT INTO api.tokens_recuperacion (usuario_id, token_hash, expira_el) VALUES (?, ?, now() + make_interval(secs => ?))`
	argumentos := []any{usuarioID, suma[:], vigencia.Seconds()}
	if usado {
		consulta = `INSERT INTO api.tokens_recuperacion (usuario_id, token_hash, expira_el, usado_el)
			VALUES (?, ?, now() + make_interval(secs => ?), now())`
	}
	if errorObtenido := base.Exec(consulta, argumentos...).Error; errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	return suma[:]
}

// cuentaConClave crea una cuenta, le pone una contraseña inicial y limpia sus
// tokens y su usuario al terminar.
func cuentaConClave(prueba *testing.T, runtime, operador *gorm.DB, prefijo, contrasenaInicial string) uuid.UUID {
	prueba.Helper()
	usuario := crearCuenta(prueba, runtime, operador, prefijo, "restablecer", domain.RolRegistrado)
	hashInicial, errorHash := services.HashearContrasena(contrasenaInicial)
	if errorHash != nil {
		prueba.Fatal(errorHash)
	}
	if errorObtenido := runtime.Exec("UPDATE api.usuarios SET password_hash = ? WHERE id = ?", hashInicial, usuario.ID).Error; errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	prueba.Cleanup(func() {
		operador.Exec("DELETE FROM api.tokens_recuperacion WHERE usuario_id = ?", usuario.ID)
	})
	return usuario.ID
}

// hashGuardado lee el hash actual de la cuenta.
func hashGuardado(prueba *testing.T, base *gorm.DB, usuarioID uuid.UUID) string {
	prueba.Helper()
	var hash string
	if errorObtenido := base.Raw("SELECT password_hash FROM api.usuarios WHERE id = ?", usuarioID).Scan(&hash).Error; errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	return hash
}

// tokenEstaUsado indica si el token quedó inutilizado.
func tokenEstaUsado(prueba *testing.T, base *gorm.DB, hashToken []byte) bool {
	prueba.Helper()
	var usado bool
	if errorObtenido := base.Raw("SELECT usado_el IS NOT NULL FROM api.tokens_recuperacion WHERE token_hash = ?", hashToken).Scan(&usado).Error; errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	return usado
}

// ===== Pruebas =====

// TestRecuperacionPostgresTokenValidoYUnSoloUso valida el camino completo:
// cambia el hash, marca el token y rechaza la segunda petición.
func TestRecuperacionPostgresTokenValidoYUnSoloUso(prueba *testing.T) {
	runtime, operador := basesDePrueba(prueba)
	contexto := context.Background()
	if errorObtenido := repositories.VerificarAlmacenamiento(contexto, runtime); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	prefijo := "recuperacion-" + uuid.NewString()[:8]
	usuarioID := cuentaConClave(prueba, runtime, operador, prefijo, "ClaveInicial1!")
	hashInicial := hashGuardado(prueba, runtime, usuarioID)
	const token = "token-valido-una-vez"
	hashToken := sembrarTokenRestablecer(prueba, runtime, usuarioID, token, 30*time.Minute, false)
	servicio := services.NuevoServicioRecuperacion(repositories.NuevoRepositorioRecuperacion(runtime))

	if errorObtenido := servicio.Restablecer(contexto, token, "ClaveNueva1!"); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}

	hashFinal := hashGuardado(prueba, runtime, usuarioID)
	if hashFinal == hashInicial || !services.ContrasenaCoincide("ClaveNueva1!", hashFinal) {
		prueba.Fatal("no se guardó el hash de la clave nueva")
	}
	if !tokenEstaUsado(prueba, runtime, hashToken) {
		prueba.Fatal("el token sigue vigente después de usarlo")
	}
	if errorObtenido := servicio.Restablecer(contexto, token, "OtraClave2!"); !errors.Is(errorObtenido, domain.ErrTokenRecuperacionInvalido) {
		prueba.Fatalf("reutilización: error=%v", errorObtenido)
	}
	if hashFinal != hashGuardado(prueba, runtime, usuarioID) {
		prueba.Fatal("la reutilización cambió la contraseña")
	}
}

// TestRecuperacionPostgresInvalidaTodosLosTokensDelUsuario comprueba que, si el
// usuario pidió varios enlaces, usar uno deja los demás sin efecto en la misma
// transacción.
func TestRecuperacionPostgresInvalidaTodosLosTokensDelUsuario(prueba *testing.T) {
	runtime, operador := basesDePrueba(prueba)
	contexto := context.Background()
	prefijo := "recuperacion-varios-" + uuid.NewString()[:8]
	usuarioID := cuentaConClave(prueba, runtime, operador, prefijo, "ClaveInicial1!")
	const tokenUsado = "token-primero"
	const tokenHermano = "token-segundo"
	hashUsado := sembrarTokenRestablecer(prueba, runtime, usuarioID, tokenUsado, 30*time.Minute, false)
	hashHermano := sembrarTokenRestablecer(prueba, runtime, usuarioID, tokenHermano, 30*time.Minute, false)
	servicio := services.NuevoServicioRecuperacion(repositories.NuevoRepositorioRecuperacion(runtime))

	if errorObtenido := servicio.Restablecer(contexto, tokenUsado, "ClaveNueva1!"); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}

	if !tokenEstaUsado(prueba, runtime, hashUsado) || !tokenEstaUsado(prueba, runtime, hashHermano) {
		prueba.Fatal("quedó un token pendiente después del restablecimiento")
	}
	hashFinal := hashGuardado(prueba, runtime, usuarioID)
	if errorObtenido := servicio.Restablecer(contexto, tokenHermano, "OtraClave2!"); !errors.Is(errorObtenido, domain.ErrTokenRecuperacionInvalido) {
		prueba.Fatalf("el token hermano sigue sirviendo: error=%v", errorObtenido)
	}
	if hashFinal != hashGuardado(prueba, runtime, usuarioID) {
		prueba.Fatal("el token hermano cambió la contraseña")
	}
}

// TestRecuperacionPostgresTokenExpirado comprueba que un token vencido no sirve
// ni se consume.
func TestRecuperacionPostgresTokenExpirado(prueba *testing.T) {
	runtime, operador := basesDePrueba(prueba)
	contexto := context.Background()
	prefijo := "recuperacion-exp-" + uuid.NewString()[:8]
	usuarioID := cuentaConClave(prueba, runtime, operador, prefijo, "ClaveInicial1!")
	hashInicial := hashGuardado(prueba, runtime, usuarioID)
	hashToken := sembrarTokenRestablecer(prueba, runtime, usuarioID, "token-expirado", -time.Minute, false)
	servicio := services.NuevoServicioRecuperacion(repositories.NuevoRepositorioRecuperacion(runtime))

	errorObtenido := servicio.Restablecer(contexto, "token-expirado", "ClaveNueva1!")

	if !errors.Is(errorObtenido, domain.ErrTokenRecuperacionInvalido) {
		prueba.Fatalf("error=%v, esperado token inválido", errorObtenido)
	}
	if hashGuardado(prueba, runtime, usuarioID) != hashInicial {
		prueba.Fatal("cambió la contraseña con un token expirado")
	}
	if tokenEstaUsado(prueba, runtime, hashToken) {
		prueba.Fatal("se marcó como usado un token expirado")
	}
}

// TestRecuperacionPostgresTokenUsado rechaza un token ya consumido.
func TestRecuperacionPostgresTokenUsado(prueba *testing.T) {
	runtime, operador := basesDePrueba(prueba)
	contexto := context.Background()
	prefijo := "recuperacion-usado-" + uuid.NewString()[:8]
	usuarioID := cuentaConClave(prueba, runtime, operador, prefijo, "ClaveInicial1!")
	hashInicial := hashGuardado(prueba, runtime, usuarioID)
	sembrarTokenRestablecer(prueba, runtime, usuarioID, "token-usado", 30*time.Minute, true)
	servicio := services.NuevoServicioRecuperacion(repositories.NuevoRepositorioRecuperacion(runtime))

	errorObtenido := servicio.Restablecer(contexto, "token-usado", "ClaveNueva1!")

	if !errors.Is(errorObtenido, domain.ErrTokenRecuperacionInvalido) {
		prueba.Fatalf("error=%v, esperado token inválido", errorObtenido)
	}
	if hashGuardado(prueba, runtime, usuarioID) != hashInicial {
		prueba.Fatal("cambió la contraseña con un token usado")
	}
}

// TestRecuperacionPostgresTokenInexistente rechaza un token que no está en la base.
func TestRecuperacionPostgresTokenInexistente(prueba *testing.T) {
	runtime, operador := basesDePrueba(prueba)
	contexto := context.Background()
	prefijo := "recuperacion-inex-" + uuid.NewString()[:8]
	usuarioID := cuentaConClave(prueba, runtime, operador, prefijo, "ClaveInicial1!")
	hashInicial := hashGuardado(prueba, runtime, usuarioID)
	servicio := services.NuevoServicioRecuperacion(repositories.NuevoRepositorioRecuperacion(runtime))

	errorObtenido := servicio.Restablecer(contexto, "token-que-nunca-se-emitió", "ClaveNueva1!")

	if !errors.Is(errorObtenido, domain.ErrTokenRecuperacionInvalido) {
		prueba.Fatalf("error=%v, esperado token inválido", errorObtenido)
	}
	if hashGuardado(prueba, runtime, usuarioID) != hashInicial {
		prueba.Fatal("cambió la contraseña con un token inexistente")
	}
}

// TestRecuperacionPostgresClaveDebilNoConsumeToken comprueba que rechazar la
// clave por la política de SUP-281 no quema el enlace.
func TestRecuperacionPostgresClaveDebilNoConsumeToken(prueba *testing.T) {
	runtime, operador := basesDePrueba(prueba)
	contexto := context.Background()
	prefijo := "recuperacion-debil-" + uuid.NewString()[:8]
	usuarioID := cuentaConClave(prueba, runtime, operador, prefijo, "ClaveInicial1!")
	hashInicial := hashGuardado(prueba, runtime, usuarioID)
	const token = "token-clave-debil"
	hashToken := sembrarTokenRestablecer(prueba, runtime, usuarioID, token, 30*time.Minute, false)
	servicio := services.NuevoServicioRecuperacion(repositories.NuevoRepositorioRecuperacion(runtime))

	if errorObtenido := servicio.Restablecer(contexto, token, "clave-corta"); !errors.Is(errorObtenido, services.ErrContrasenaInvalida) {
		prueba.Fatalf("error=%v, esperada contraseña inválida", errorObtenido)
	}
	if hashGuardado(prueba, runtime, usuarioID) != hashInicial || tokenEstaUsado(prueba, runtime, hashToken) {
		prueba.Fatal("una clave inválida consumió el token")
	}
	if errorObtenido := servicio.Restablecer(contexto, token, "ClaveValida1!"); errorObtenido != nil {
		prueba.Fatalf("el token debía seguir vigente: %v", errorObtenido)
	}
}

// TestRecuperacionPostgresConcurrencia comprueba que dos peticiones simultáneas
// con el mismo token sólo cambian la contraseña una vez.
func TestRecuperacionPostgresConcurrencia(prueba *testing.T) {
	runtime, operador := basesDePrueba(prueba)
	contexto := context.Background()
	prefijo := "recuperacion-conc-" + uuid.NewString()[:8]
	usuarioID := cuentaConClave(prueba, runtime, operador, prefijo, "ClaveInicial1!")
	const token = "token-concurrente"
	hashToken := sembrarTokenRestablecer(prueba, runtime, usuarioID, token, 30*time.Minute, false)
	servicio := services.NuevoServicioRecuperacion(repositories.NuevoRepositorioRecuperacion(runtime))

	var espera sync.WaitGroup
	resultados := make(chan error, 2)
	for indice := 0; indice < 2; indice++ {
		espera.Add(1)
		go func() {
			defer espera.Done()
			resultados <- servicio.Restablecer(contexto, token, "ClaveConcurrente1!")
		}()
	}
	espera.Wait()
	close(resultados)

	correctos, invalidos := 0, 0
	for errorObtenido := range resultados {
		switch {
		case errorObtenido == nil:
			correctos++
		case errors.Is(errorObtenido, domain.ErrTokenRecuperacionInvalido):
			invalidos++
		default:
			prueba.Fatalf("error inesperado: %v", errorObtenido)
		}
	}
	if correctos != 1 || invalidos != 1 {
		prueba.Fatalf("éxitos=%d inválidos=%d, esperado 1 y 1", correctos, invalidos)
	}
	if !services.ContrasenaCoincide("ClaveConcurrente1!", hashGuardado(prueba, runtime, usuarioID)) {
		prueba.Fatal("no se guardó la clave de la petición ganadora")
	}
	if !tokenEstaUsado(prueba, runtime, hashToken) {
		prueba.Fatal("el token quedó vigente tras la carrera")
	}
}
