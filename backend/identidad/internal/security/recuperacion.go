// Generación de tokens de recuperación y su hash almacenable.

package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// GenerarTokenRecuperacion devuelve el token del enlace (256 bits en base64url) y su
// SHA-256, que es lo único que se guarda. El token en claro sólo viaja por correo.
func GenerarTokenRecuperacion() (token string, hash []byte, errorObtenido error) {
	var crudo [32]byte
	if _, errorOperacion := rand.Read(crudo[:]); errorOperacion != nil {
		return "", nil, errorOperacion
	}
	token = base64.RawURLEncoding.EncodeToString(crudo[:])
	return token, HashTokenRecuperacion(token), nil
}

// HashTokenRecuperacion es el cálculo que también usa el restablecimiento (SUP-185)
// para encontrar el token recibido. El token ya tiene 256 bits de azar: basta SHA-256.
func HashTokenRecuperacion(token string) []byte {
	suma := sha256.Sum256([]byte(token))
	return suma[:]
}
