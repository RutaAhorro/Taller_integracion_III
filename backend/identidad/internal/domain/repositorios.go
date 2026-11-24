// Contratos de persistencia que implementa el paquete repositories y errores
// de negocio que esos contratos devuelven.

package domain

import (
	"context"
	"errors"
	"time"
)

// ===== Usuarios =====

// RepositorioUsuarios define el contrato para operaciones sobre la tabla api.usuarios
type RepositorioUsuarios interface {
	BuscarPorCorreo(contexto context.Context, direccionCorreo string) (*Usuario, error)
	BuscarPorID(contexto context.Context, id string) (*Usuario, error)
	Crear(contexto context.Context, usuario *Usuario) error
	Actualizar(contexto context.Context, id string, datos map[string]interface{}) error
}

var ErrCorreoDuplicado = errors.New("el correo electrónico ya se encuentra registrado")

// ErrUltimoAdmin impide desactivar o degradar al último administrador activo.
var ErrUltimoAdmin = errors.New("no se puede dejar el sistema sin administradores activos")

// ErrAutoModificacionAdmin impide que un admin se desactive o degrade a sí mismo.
var ErrAutoModificacionAdmin = errors.New("un administrador no puede desactivar ni degradar su propia cuenta; la acción debe ser ejecutada por otro administrador")

// ===== Recuperación de contraseña =====

// ErrTokenRecuperacionInvalido se devuelve cuando el token no existe, expiró o
// ya fue consumido. Un solo error evita distinguir esos casos ante quien llama.
var ErrTokenRecuperacionInvalido = errors.New("el token de restablecimiento no es válido")

// RepositorioRecuperacion persiste el consumo de tokens de api.tokens_recuperacion.
type RepositorioRecuperacion interface {
	// Validar comprueba que el token exista, no haya expirado ni esté usado,
	// sin consumirlo todavía.
	Validar(contexto context.Context, hashToken []byte) error
	// Consumir marca el token como usado, inutiliza los demás tokens pendientes
	// del usuario y guarda el nuevo hash de contraseña en la misma transacción.
	// Falla si otro intento lo consumió primero.
	Consumir(contexto context.Context, hashToken []byte, hashContrasena string, actualizadoEl time.Time) error
}

// ===== Contrato de repositorio =====

// RepositorioAdmin define las operaciones de administración sobre api.usuarios.
// Se mantiene separado de RepositorioUsuarios para no acoplar el perfil público.
type RepositorioAdmin interface {
	Listar(contexto context.Context, filtro FiltroUsuariosAdminDTO) ([]Usuario, int64, error)
	BuscarPorID(contexto context.Context, id string) (*Usuario, error)
	// Contar devuelve el total de cuentas y cuántas están activas.
	Contar(contexto context.Context) (total, activos int64, errorObtenido error)
	// ActualizarConGuarda aplica el cambio y registra la auditoría en una sola
	// transacción, impidiendo que se desactive o degrade al último admin activo.
	ActualizarConGuarda(contexto context.Context, id string, datos map[string]interface{}, actorID string) error
}
