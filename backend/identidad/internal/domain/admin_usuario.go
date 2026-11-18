// Roles, planes y contratos del panel de administración de cuentas.

package domain

import (
	"context"
	"strings"
	"time"
)

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
