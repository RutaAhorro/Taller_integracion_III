// Contrato de acceso a api.usuarios y errores de negocio compartidos.

package domain

import (
	"context"
	"errors"
)

// UsuarioRepository define el contrato para operaciones sobre la tabla api.usuarios
type UsuarioRepository interface {
	FindByEmail(contexto context.Context, direccionCorreo string) (*Usuario, error)
	FindByID(contexto context.Context, id string) (*Usuario, error)
	Create(contexto context.Context, usuario *Usuario) error
	Actualizar(contexto context.Context, id string, datos map[string]interface{}) error
}

var ErrCorreoDuplicado = errors.New("el correo electrónico ya se encuentra registrado")

// ErrUltimoAdmin impide desactivar o degradar al último administrador activo.
var ErrUltimoAdmin = errors.New("no se puede dejar el sistema sin administradores activos")

// ErrAutoModificacionAdmin impide que un admin se desactive o degrade a sí mismo.
var ErrAutoModificacionAdmin = errors.New("un administrador no puede desactivar ni degradar su propia cuenta; la acción debe ser ejecutada por otro administrador")
