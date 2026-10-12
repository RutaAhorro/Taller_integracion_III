// Package auth valida el contrato JWT emitido por Identidad; no emite tokens
// ni consulta usuarios. No depende de la implementación del módulo de Identidad.
package auth

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	UserID   string `json:"user_id"`
	Rol      string `json:"rol"`
	Provider string `json:"provider"`
	jwt.RegisteredClaims
}

func ValidarToken(tokenString, secret string) (*jwt.Token, error) {
	if secret == "" {
		return nil, errors.New("clave de verificación JWT no configurada")
	}
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, fmt.Errorf("token inválido: %w", err)
	}
	return token, nil
}
