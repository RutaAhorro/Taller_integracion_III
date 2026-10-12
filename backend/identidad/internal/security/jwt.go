package security

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims define los datos del payload del token JWT centralizado
type JWTClaims struct {
	UserID   string `json:"user_id"`
	Rol      string `json:"rol"`
	Provider string `json:"provider"`
	jwt.RegisteredClaims
}

// getJWTSecret nunca usa claves de respaldo.
func getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil
	}
	return []byte(secret)
}

// GenerarToken emite un JWT firmado con HS256 conteniendo estrictamente user_id, rol, provider y expiración a 24 horas
func GenerarToken(usuarioID, rol, provider string) (string, error) {
	if usuarioID == "" {
		return "", errors.New("el usuarioID no puede estar vacío")
	}

	ahora := time.Now()
	claims := JWTClaims{
		UserID:   usuarioID,
		Rol:      rol,
		Provider: provider,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(ahora.Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(ahora),
			NotBefore: jwt.NewNumericDate(ahora),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret := getJWTSecret()
	if len(secret) == 0 {
		return "", errors.New("JWT_SECRET no configurado")
	}
	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("error al firmar token JWT: %w", err)
	}

	return tokenString, nil
}

// ValidarToken parsea y verifica la firma criptográfica y expiración del JWT
func ValidarToken(tokenString string) (*jwt.Token, error) {
	if len(tokenString) > 4096 {
		return nil, errors.New("token inválido")
	}
	secret := getJWTSecret()
	if len(secret) == 0 {
		return nil, errors.New("JWT_SECRET no configurado")
	}
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())

	if err != nil {
		return nil, fmt.Errorf("token inválido: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || strings.TrimSpace(claims.UserID) == "" || strings.TrimSpace(claims.Rol) == "" || strings.TrimSpace(claims.Provider) == "" {
		return nil, errors.New("claims de identidad incompletos")
	}
	return token, nil
}
