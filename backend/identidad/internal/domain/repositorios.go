// Contratos de persistencia que implementa el paquete repositories y errores
// de negocio que esos contratos devuelven.

package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ===== Usuarios =====

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

// ===== Contrato de repositorio =====

// AdminUsuarioRepository define las operaciones de administración sobre api.usuarios.
// Se mantiene separado de UsuarioRepository para no acoplar el perfil público.
type AdminUsuarioRepository interface {
	Listar(contexto context.Context, filtro FiltroUsuariosAdminDTO) ([]Usuario, int64, error)
	FindByID(contexto context.Context, id string) (*Usuario, error)
	// Contar devuelve el total de cuentas y cuántas están activas.
	Contar(contexto context.Context) (total, activos int64, errorObtenido error)
	// ActualizarConGuarda aplica el cambio y registra la auditoría en una sola
	// transacción, impidiendo que se desactive o degrade al último admin activo.
	ActualizarConGuarda(contexto context.Context, id string, datos map[string]interface{}, actorID string) error
}

// ===== Tokens de recuperación =====

// RecuperacionRepository persiste tokens en api.tokens_recuperacion (SUP-184).
// Las comparaciones de tiempo usan el reloj de PostgreSQL, igual que el consumo (SUP-185).
type RecuperacionRepository interface {
	// SolicitudesRecientes cuenta los tokens emitidos dentro de ventana e indica si
	// alguno tiene menos de espera.
	SolicitudesRecientes(contexto context.Context, usuarioID uuid.UUID, ventana, espera time.Duration) (enVentana int, enEspera bool, errorObtenido error)
	// EmitirToken invalida los tokens pendientes del usuario y guarda el hash nuevo.
	EmitirToken(contexto context.Context, usuarioID uuid.UUID, hash []byte, vigencia time.Duration) error
}
