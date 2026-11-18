// Hash y verificación de contraseñas con bcrypt.

package security

import "golang.org/x/crypto/bcrypt"

// Límites de la política vigente y del formato bcrypt; no se alteran en el barrido.
const (
	MinimoBytesPassword = 8
	MaximoBytesPassword = 72
)

// HashPassword genera un hash seguro a partir de una contraseña en texto plano utilizando bcrypt
func HashPassword(contrasena string) (string, error) {
	cantidadBytes, errorObtenido := bcrypt.GenerateFromPassword([]byte(contrasena), bcrypt.DefaultCost)
	if errorObtenido != nil {
		return "", errorObtenido
	}
	return string(cantidadBytes), nil
}

// CheckPasswordHash compara una contraseña en texto plano contra un hash bcrypt almacenado
func CheckPasswordHash(contrasena, hash string) bool {
	errorObtenido := bcrypt.CompareHashAndPassword([]byte(hash), []byte(contrasena))
	return errorObtenido == nil
}
