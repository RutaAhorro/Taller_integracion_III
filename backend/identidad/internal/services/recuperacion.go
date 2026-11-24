// Caso de uso de restablecimiento de contraseña: consume un token de un solo
// uso y reutiliza la política de contraseñas del cambio voluntario (SUP-281).

package services

import (
	"context"
	"crypto/sha256"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
)

// ===== Restablecimiento =====

// ServicioRecuperacion valida y consume tokens de restablecimiento.
type ServicioRecuperacion struct {
	tokens domain.RepositorioRecuperacion
}

// NuevoServicioRecuperacion recibe el repositorio de tokens ya construido.
func NuevoServicioRecuperacion(tokens domain.RepositorioRecuperacion) *ServicioRecuperacion {
	return &ServicioRecuperacion{tokens: tokens}
}

// hashTokenRestablecimiento calcula el SHA-256 que se guardó al emitir el token
// (SUP-184). El token en claro nunca se persiste ni se registra.
func hashTokenRestablecimiento(token string) []byte {
	suma := sha256.Sum256([]byte(token))
	return suma[:]
}

// Restablecer comprueba primero el token, sin gastar bcrypt en tokens
// inválidos. Después valida la clave nueva con la misma política de PUT /auth/me
// y finalmente consume el token, inutiliza los demás enlaces pendientes del
// usuario y guarda el hash en una sola transacción, para que dos peticiones
// concurrentes no puedan reutilizarlo.
func (servicio *ServicioRecuperacion) Restablecer(contexto context.Context, token, contrasenaNueva string) error {
	if strings.TrimSpace(token) == "" {
		return domain.ErrTokenRecuperacionInvalido
	}

	hashToken := hashTokenRestablecimiento(token)
	if errorValidacion := servicio.tokens.Validar(contexto, hashToken); errorValidacion != nil {
		return errorValidacion
	}

	// Una clave inválida no debe inutilizar un token legítimo: se rechaza antes
	// de escribir en base y el enlace sigue sirviendo para el siguiente intento.
	hashContrasena, errorPreparacion := prepararHashContrasena(contrasenaNueva)
	if errorPreparacion != nil {
		return errorPreparacion
	}

	return servicio.tokens.Consumir(contexto, hashToken, hashContrasena, time.Now())
}
