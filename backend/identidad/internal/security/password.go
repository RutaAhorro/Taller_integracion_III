package security

import "golang.org/x/crypto/bcrypt"

// HashPassword genera un hash seguro a partir de una contraseña en texto plano utilizando bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPasswordHash compara una contraseña en texto plano contra un hash bcrypt almacenado
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
