package domain

import (
	"context"
	"errors"
)

// UsuarioRepository define el contrato para operaciones sobre la tabla api.usuarios
type UsuarioRepository interface {
	FindByEmail(ctx context.Context, email string) (*Usuario, error)
	FindByID(ctx context.Context, id string) (*Usuario, error)
	Create(ctx context.Context, usuario *Usuario) error
	Actualizar(ctx context.Context, id string, datos map[string]interface{}) error
}

var ErrCorreoDuplicado = errors.New("el correo electrónico ya se encuentra registrado")

// ErrUltimoAdmin impide desactivar o degradar al último administrador activo.
var ErrUltimoAdmin = errors.New("no se puede dejar el sistema sin administradores activos")
