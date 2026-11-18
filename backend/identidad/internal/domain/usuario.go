// Package domain define las entidades, contratos de repositorio y DTO de
// Identidad. No depende de HTTP ni de la base de datos.
package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ===== Usuario =====

// Valores del modelo y de la política actual. Se mantienen constantes para no
// introducir cambios de plan comercial ni del esquema durante el barrido.
const (
	CuotaTokensInicial     = 1000
	MaximoCaracteresAvatar = 500
)

// Usuario representa la entidad de usuarios centralizados en api.usuarios
type Usuario struct {
	ID             uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GoogleID       *string    `gorm:"type:varchar(255);unique" json:"google_id,omitempty"`
	Correo         string     `gorm:"type:varchar(255);unique;not null" json:"correo"`
	PasswordHash   *string    `gorm:"type:varchar(255)" json:"-"`
	NombreCompleto string     `gorm:"type:varchar(255);not null" json:"nombre_completo"`
	Telefono       *string    `gorm:"type:varchar(50)" json:"telefono,omitempty"`
	Direccion      *string    `gorm:"type:varchar(255)" json:"direccion,omitempty"`
	URLAvatar      *string    `gorm:"type:varchar(500)" json:"url_avatar,omitempty"`
	Rol            string     `gorm:"type:varchar(30);default:'registrado';not null" json:"rol"`
	Plan           string     `gorm:"type:varchar(20);default:'basico';not null" json:"plan"`
	CuotaTokensIA  int        `gorm:"default:1000;not null" json:"cuota_tokens_ia"`
	EstaActivo     bool       `gorm:"default:true;not null" json:"esta_activo"`
	UltimoAcceso   *time.Time `gorm:"type:timestamptz" json:"ultimo_acceso,omitempty"`
	// TokenVerificacion existe en la tabla pero hoy no se escribe: no hay flujo
	// de verificación de correo. Se conserva para no cambiar columnas.
	TokenVerificacion *uuid.UUID `gorm:"type:uuid" json:"token_verificacion,omitempty"`
	CreadoEl          time.Time  `gorm:"default:now();not null" json:"creado_el"`
	ActualizadoEl     time.Time  `gorm:"default:now();not null" json:"actualizado_el"`
}

// TableName fija la tabla con esquema, porque GORM no lo infiere.
func (Usuario) TableName() string {
	return "api.usuarios"
}

// ===== Roles y planes =====

// Roles válidos para api.usuarios.rol. El plan comercial no forma parte del rol.
const (
	// Límites documentados del contrato HTTP de administración.
	UsuariosAdminPorPagina    = 20
	MaxUsuariosAdminPorPagina = 200
	RolAdmin                  = "admin"
	RolColaborador            = "colaborador"
	RolRegistrado             = "registrado"

	// Planes comerciales válidos. No forman parte del rol RBAC.
	PlanBasico  = "basico"
	PlanPremium = "premium"
)

// NormalizarRol devuelve el valor canónico almacenado en api.usuarios.rol.
// Sólo existen admin, colaborador y registrado: la migración 08 eliminó el
// alias histórico "administrador" de los datos.
func NormalizarRol(rol string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(rol)) {
	case RolAdmin:
		return RolAdmin, true
	case RolColaborador:
		return RolColaborador, true
	case RolRegistrado:
		return RolRegistrado, true
	default:
		return "", false
	}
}

// NormalizarPlan devuelve el valor canónico del plan comercial.
func NormalizarPlan(plan string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case PlanBasico:
		return PlanBasico, true
	case PlanPremium:
		return PlanPremium, true
	default:
		return "", false
	}
}

// ===== Perfil propio =====

// ActualizarPerfilDTO define los campos opcionales para la actualización parcial del perfil de usuario
type ActualizarPerfilDTO struct {
	NombreCompleto *string `json:"nombre_completo,omitempty"`
	Correo         *string `json:"correo,omitempty"`
	Telefono       *string `json:"telefono,omitempty"`
	Direccion      *string `json:"direccion,omitempty"`
	Password       *string `json:"password,omitempty"`
	// PasswordActual es obligatoria para cambiar correo o contraseña: una cookie
	// robada no basta para tomar la cuenta de forma permanente.
	PasswordActual *string `json:"current_password,omitempty"`
}

// PerfilUsuarioDTO representa la respuesta con los datos del usuario tras la actualización o consulta
type PerfilUsuarioDTO struct {
	ID             string  `json:"id"`
	Correo         string  `json:"correo"`
	NombreCompleto string  `json:"nombre_completo"`
	Telefono       *string `json:"telefono,omitempty"`
	Direccion      *string `json:"direccion,omitempty"`
	Rol            string  `json:"rol"`
	EstaActivo     bool    `json:"esta_activo"`
	Mensaje        string  `json:"mensaje,omitempty"`
}
