// Package auth valida el JWT que emite Identidad (HS256, en la cookie HttpOnly `jwt`).
//
// Este servicio solo VERIFICA tokens: no los emite, no consulta usuarios y no
// lee api.usuarios. Todo lo que sabe del usuario viene dentro del token.
package auth

import (
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// =============================================================================
// Constantes y errores
// =============================================================================

// LargoMaximoToken acota el trabajo de parseo si llega una cookie enorme.
const LargoMaximoToken = 4096

// ErrTokenInvalido cubre cualquier fallo (firma, algoritmo, expiración, claims).
// Es un único error a propósito: el cliente no necesita saber cuál falló.
var ErrTokenInvalido = errors.New("token inválido o expirado")

// =============================================================================
// Modelos
// =============================================================================

// Sesion es lo único que el resto del servicio conoce del usuario autenticado.
type Sesion struct {
	UsuarioID string // UUID del usuario en Identidad, normalizado
	Rol       string // admin, colaborador o registrado
	Proveedor string // origen de la cuenta (por ejemplo, local)
}

// claims es la forma del token tal como lo firma Identidad (nombres en inglés
// porque así viajan en el JSON del JWT).
type claims struct {
	UserID   string `json:"user_id"`
	Rol      string `json:"rol"`
	Provider string `json:"provider"`
	jwt.RegisteredClaims
}

// =============================================================================
// Validación
// =============================================================================

// Validar comprueba el token y devuelve la sesión. Exige:
//   - algoritmo HS256 (rechaza "none" y otros algoritmos);
//   - firma con el secreto compartido;
//   - claim `exp` presente y vigente;
//   - user_id con formato UUID, y rol y provider no vacíos.
func Validar(token, secreto string) (Sesion, error) {
	if token == "" || len(token) > LargoMaximoToken {
		return Sesion{}, ErrTokenInvalido
	}

	// --- Firma, algoritmo y expiración ---------------------------------------
	var claimsToken claims
	entregarSecreto := func(*jwt.Token) (any, error) { return []byte(secreto), nil }
	_, errorObtenido := jwt.ParseWithClaims(token, &claimsToken, entregarSecreto,
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if errorObtenido != nil {
		return Sesion{}, ErrTokenInvalido
	}

	// --- Claims propios de Identidad -----------------------------------------
	faltaUsuario := strings.TrimSpace(claimsToken.UserID) == ""
	faltaRol := strings.TrimSpace(claimsToken.Rol) == ""
	faltaProveedor := strings.TrimSpace(claimsToken.Provider) == ""
	if faltaUsuario || faltaRol || faltaProveedor {
		return Sesion{}, ErrTokenInvalido
	}
	usuarioID, errorObtenido := uuid.Parse(claimsToken.UserID)
	if errorObtenido != nil {
		return Sesion{}, ErrTokenInvalido
	}

	return Sesion{UsuarioID: usuarioID.String(), Rol: claimsToken.Rol, Proveedor: claimsToken.Provider}, nil
}
