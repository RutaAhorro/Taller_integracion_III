package domain

import (
	"context"
	"time"
)

// Roles válidos para api.usuarios.rol. El plan comercial no forma parte del rol.
const (
	RolAdmin       = "admin"
	RolColaborador = "colaborador"
	RolRegistrado  = "registrado"
)

// FiltroUsuariosAdminDTO contiene los criterios opcionales del listado administrativo.
type FiltroUsuariosAdminDTO struct {
	Query      string
	Rol        string
	EstaActivo *bool
	Limit      int
	Offset     int
}

// ActualizarUsuarioAdminDTO define los campos administrables de una cuenta.
type ActualizarUsuarioAdminDTO struct {
	EstaActivo *bool   `json:"esta_activo,omitempty"`
	Rol        *string `json:"rol,omitempty"`
}

// UsuarioAdminDTO es la representación pública de una cuenta en el panel de administración.
type UsuarioAdminDTO struct {
	ID             string    `json:"id"`
	Correo         string    `json:"correo"`
	NombreCompleto string    `json:"nombre_completo"`
	Telefono       *string   `json:"telefono,omitempty"`
	Direccion      *string   `json:"direccion,omitempty"`
	URLAvatar      *string   `json:"url_avatar,omitempty"`
	Rol            string    `json:"rol"`
	CuotaTokensIA  int       `json:"cuota_tokens_ia"`
	EstaActivo     bool      `json:"esta_activo"`
	CreadoEl       time.Time `json:"creado_el"`
	ActualizadoEl  time.Time `json:"actualizado_el"`
}

// AdminUsuarioRepository define las operaciones de administración sobre api.usuarios.
// Se mantiene separado de UsuarioRepository para no acoplar el perfil público.
type AdminUsuarioRepository interface {
	Listar(ctx context.Context, filtro FiltroUsuariosAdminDTO) ([]Usuario, error)
	FindByID(ctx context.Context, id string) (*Usuario, error)
	Actualizar(ctx context.Context, id string, datos map[string]interface{}) error
}
