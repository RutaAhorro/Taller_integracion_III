// DTO del panel de administración de cuentas y registro de auditoría de sus cambios.

package domain

import (
	"time"

	"github.com/google/uuid"
)

// ===== DTO del panel =====

// FiltroUsuariosAdminDTO contiene los criterios opcionales del listado administrativo.
type FiltroUsuariosAdminDTO struct {
	Query      string
	Rol        string
	EstaActivo *bool
	Page       int
	Limit      int
	// Offset lo calcula el handler a partir de Page y Limit; no es un parámetro HTTP.
	Offset int
}

// ActualizarUsuarioAdminDTO define los campos administrables de una cuenta.
type ActualizarUsuarioAdminDTO struct {
	EstaActivo *bool   `json:"esta_activo,omitempty"`
	Rol        *string `json:"rol,omitempty"`
	Plan       *string `json:"plan,omitempty"`
}

// UsuarioAdminDTO es la representación pública de una cuenta en el panel de administración.
type UsuarioAdminDTO struct {
	ID             string     `json:"id"`
	Correo         string     `json:"correo"`
	NombreCompleto string     `json:"nombre_completo"`
	Telefono       *string    `json:"telefono,omitempty"`
	Direccion      *string    `json:"direccion,omitempty"`
	URLAvatar      *string    `json:"url_avatar,omitempty"`
	Rol            string     `json:"rol"`
	Plan           string     `json:"plan"`
	CuotaTokensIA  int        `json:"cuota_tokens_ia"`
	EstaActivo     bool       `json:"esta_activo"`
	UltimoAcceso   *time.Time `json:"ultimo_acceso,omitempty"`
	CreadoEl       time.Time  `json:"creado_el"`
	ActualizadoEl  time.Time  `json:"actualizado_el"`
}

// MetadatosPaginacionDTO describe la página solicitada y el total de cuentas.
type MetadatosPaginacionDTO struct {
	TotalRegistros int64 `json:"total_registros"`
	PaginaActual   int   `json:"pagina_actual"`
	TotalPaginas   int   `json:"total_paginas"`
	Limite         int   `json:"limite"`
}

// PaginaUsuariosAdminDTO es la respuesta paginada del listado administrativo.
type PaginaUsuariosAdminDTO struct {
	Data       []UsuarioAdminDTO      `json:"data"`
	Paginacion MetadatosPaginacionDTO `json:"paginacion"`
}

// MetricasUsuariosDTO resume el conteo de cuentas para el panel de administración.
type MetricasUsuariosDTO struct {
	TotalUsuarios   int64 `json:"total_usuarios"`
	UsuariosActivos int64 `json:"usuarios_activos"`
}

// ===== Auditoría =====

// AuditoriaAdminUsuario registra quién cambió el estado o rol de una cuenta.
// Se escribe dentro de la misma transacción que la actualización.
type AuditoriaAdminUsuario struct {
	ID         int64                  `gorm:"primaryKey" json:"id"`
	ActorID    uuid.UUID              `gorm:"type:uuid;not null" json:"actor_id"`
	ObjetivoID uuid.UUID              `gorm:"type:uuid;not null" json:"objetivo_id"`
	Cambios    map[string]interface{} `gorm:"serializer:json;type:jsonb;not null" json:"cambios"`
	CreadoEl   time.Time              `gorm:"default:now();not null" json:"creado_el"`
}

// TableName fija la tabla con esquema, porque GORM no lo infiere.
func (AuditoriaAdminUsuario) TableName() string {
	return "api.auditoria_admin_usuarios"
}
