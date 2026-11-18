// Package security reúne la criptografía de Identidad: JWT de sesión,
// hash de contraseñas y tokens de recuperación.
package security

import (
	"errors"
	"fmt"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ===== Contrato del token =====

// maximoBytesJWT limita el formato recibido, no la duración del contrato.
const maximoBytesJWT = 4096

// JWTClaims define los datos del payload del token JWT centralizado
type JWTClaims struct {
	UserID   string `json:"user_id"`
	Rol      string `json:"rol"`
	Provider string `json:"provider"`
	jwt.RegisteredClaims
}

// ===== Emisión y validación =====

// GenerarToken emite un JWT firmado con HS256 conteniendo estrictamente user_id, rol, provider y expiración a 24 horas.
// La clave es la validada al arrancar: nunca se lee del entorno ni existe clave de respaldo.
func GenerarToken(usuarioID, rol, proveedor, clave string) (string, error) {
	if usuarioID == "" {
		return "", errors.New("el usuarioID no puede estar vacío")
	}

	ahora := time.Now()
	datosToken := JWTClaims{
		UserID:   usuarioID,
		Rol:      rol,
		Provider: proveedor,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(ahora.Add(config.DuracionSesion)),
			IssuedAt:  jwt.NewNumericDate(ahora),
			NotBefore: jwt.NewNumericDate(ahora),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, datosToken)
	if clave == "" {
		return "", errors.New("JWT_SECRET no configurado")
	}
	tokenTexto, errorObtenido := token.SignedString([]byte(clave))
	if errorObtenido != nil {
		return "", fmt.Errorf("error al firmar token JWT: %w", errorObtenido)
	}

	return tokenTexto, nil
}

// ValidarToken parsea y verifica la firma criptográfica y expiración del JWT
func ValidarToken(tokenTexto, clave string) (*jwt.Token, error) {
	if len(tokenTexto) > maximoBytesJWT {
		return nil, errors.New("token inválido")
	}
	if clave == "" {
		return nil, errors.New("JWT_SECRET no configurado")
	}
	secreto := []byte(clave)
	token, errorObtenido := jwt.ParseWithClaims(tokenTexto, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secreto, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())

	if errorObtenido != nil {
		return nil, fmt.Errorf("token inválido: %w", errorObtenido)
	}

	datosToken, valido := token.Claims.(*JWTClaims)
	if !valido || strings.TrimSpace(datosToken.UserID) == "" || strings.TrimSpace(datosToken.Rol) == "" || strings.TrimSpace(datosToken.Provider) == "" {
		return nil, errors.New("claims de identidad incompletos")
	}
	return token, nil
}
