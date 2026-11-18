// Casos de uso del panel de administración: listado, métricas y cambios de cuenta.

package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ===== Errores =====

var (
	// ErrRolInvalido se emite cuando se intenta asignar un rol fuera de api.usuarios.
	ErrRolInvalido = errors.New("rol inválido: sólo se admite admin, colaborador o registrado")
	// ErrPlanInvalido se emite cuando el plan comercial no es basico ni premium.
	ErrPlanInvalido = errors.New("plan inválido: sólo se admite basico o premium")
	// ErrSinCambiosAdmin se emite cuando la actualización administrativa no trae campos.
	ErrSinCambiosAdmin = errors.New("no se enviaron cambios para actualizar la cuenta")
	// ErrIDUsuarioInvalido se emite cuando el identificador no es un UUID válido.
	ErrIDUsuarioInvalido = errors.New("identificador de usuario inválido")
	// ErrAutoModificacionAdmin impide que un admin se desactive o se degrade a sí mismo.
	ErrAutoModificacionAdmin = domain.ErrAutoModificacionAdmin
	// ErrUltimoAdmin impide desactivar o degradar al último administrador activo.
	ErrUltimoAdmin = domain.ErrUltimoAdmin
)

// ===== Contrato del servicio =====

// AdminUsuarioService define los casos de uso del panel de administración de cuentas.
type AdminUsuarioService interface {
	Listar(contexto context.Context, filtro domain.FiltroUsuariosAdminDTO) (*domain.PaginaUsuariosAdminDTO, error)
	Metricas(contexto context.Context) (*domain.MetricasUsuariosDTO, error)
	Actualizar(contexto context.Context, id string, entrada domain.ActualizarUsuarioAdminDTO, actorID string) (*domain.UsuarioAdminDTO, error)
}

// adminUsuarioService implementa AdminUsuarioService.
type adminUsuarioService struct {
	usuarioRepo domain.AdminUsuarioRepository
}

// NewAdminUsuarioService inicializa el servicio administrativo con su repositorio.
func NewAdminUsuarioService(repositorio domain.AdminUsuarioRepository) AdminUsuarioService {
	return &adminUsuarioService{usuarioRepo: repositorio}
}

// ===== Casos de uso =====

// Listar devuelve una página de cuentas con los filtros recibidos, sin exponer credenciales.
func (servicio *adminUsuarioService) Listar(contexto context.Context, filtro domain.FiltroUsuariosAdminDTO) (*domain.PaginaUsuariosAdminDTO, error) {
	if filtro.Rol != "" {
		rol, errorObtenido := normalizarRol(filtro.Rol)
		if errorObtenido != nil {
			return nil, errorObtenido
		}
		filtro.Rol = rol
	}
	if filtro.Page < 1 {
		filtro.Page = 1
	}
	if filtro.Limit < 1 {
		filtro.Limit = domain.UsuariosAdminPorPagina
	}
	if filtro.Offset < 0 {
		filtro.Offset = 0
	}

	usuarios, total, errorOperacion := servicio.usuarioRepo.Listar(contexto, filtro)
	if errorOperacion != nil {
		return nil, errorOperacion
	}

	resultado := make([]domain.UsuarioAdminDTO, 0, len(usuarios))
	for indice := range usuarios {
		resultado = append(resultado, mapearUsuarioAdmin(&usuarios[indice]))
	}

	totalPaginas := 0
	if total > 0 {
		totalPaginas = int((total + int64(filtro.Limit) - 1) / int64(filtro.Limit))
	}
	return &domain.PaginaUsuariosAdminDTO{
		Data: resultado,
		Paginacion: domain.MetadatosPaginacionDTO{
			TotalRegistros: total,
			PaginaActual:   filtro.Page,
			TotalPaginas:   totalPaginas,
			Limite:         filtro.Limit,
		},
	}, nil
}

// Metricas devuelve el total de cuentas y las activas para el panel.
func (servicio *adminUsuarioService) Metricas(contexto context.Context) (*domain.MetricasUsuariosDTO, error) {
	total, activos, errorObtenido := servicio.usuarioRepo.Contar(contexto)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	return &domain.MetricasUsuariosDTO{TotalUsuarios: total, UsuariosActivos: activos}, nil
}

// Actualizar aplica esta_activo y/o rol sobre una cuenta existente. La guarda
// del último admin y la auditoría se ejecutan juntas en el repositorio.
func (servicio *adminUsuarioService) Actualizar(contexto context.Context, id string, entrada domain.ActualizarUsuarioAdminDTO, actorID string) (*domain.UsuarioAdminDTO, error) {
	id = strings.TrimSpace(id)
	if _, errorObtenido := uuid.Parse(id); errorObtenido != nil {
		return nil, ErrIDUsuarioInvalido
	}
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return nil, ErrIDUsuarioInvalido
	}

	datos := make(map[string]interface{})
	if entrada.Rol != nil {
		rol, errorObtenido := normalizarRol(*entrada.Rol)
		if errorObtenido != nil {
			return nil, errorObtenido
		}
		datos["rol"] = rol
	}
	if entrada.EstaActivo != nil {
		datos["esta_activo"] = *entrada.EstaActivo
	}
	if entrada.Plan != nil {
		plan, errorObtenido := normalizarPlan(*entrada.Plan)
		if errorObtenido != nil {
			return nil, errorObtenido
		}
		datos["plan"] = plan
	}
	if len(datos) == 0 {
		return nil, ErrSinCambiosAdmin
	}

	datos["actualizado_el"] = time.Now()
	if errorObtenido := servicio.usuarioRepo.ActualizarConGuarda(contexto, id, datos, actorID); errorObtenido != nil {
		if errors.Is(errorObtenido, gorm.ErrRecordNotFound) {
			return nil, ErrUsuarioNoEncontrado
		}
		if errors.Is(errorObtenido, domain.ErrUltimoAdmin) {
			return nil, ErrUltimoAdmin
		}
		return nil, errorObtenido
	}

	actualizado, errorOperacion := servicio.usuarioRepo.FindByID(contexto, id)
	if errorOperacion != nil {
		return nil, errorOperacion
	}
	if actualizado == nil {
		return nil, ErrUsuarioNoEncontrado
	}

	dto := mapearUsuarioAdmin(actualizado)
	return &dto, nil
}

// ===== Normalización y mapeo =====

// normalizarRol valida y normaliza un rol antes de persistirlo.
func normalizarRol(rol string) (string, error) {
	normalizado, valido := domain.NormalizarRol(rol)
	if !valido {
		return "", ErrRolInvalido
	}
	return normalizado, nil
}

// normalizarPlan valida y normaliza el plan comercial antes de persistirlo.
func normalizarPlan(plan string) (string, error) {
	normalizado, valido := domain.NormalizarPlan(plan)
	if !valido {
		return "", ErrPlanInvalido
	}
	return normalizado, nil
}

// mapearUsuarioAdmin convierte la entidad privada en su DTO de administración.
func mapearUsuarioAdmin(usuario *domain.Usuario) domain.UsuarioAdminDTO {
	rol, valido := domain.NormalizarRol(usuario.Rol)
	if !valido {
		rol = usuario.Rol
	}
	plan, valido := domain.NormalizarPlan(usuario.Plan)
	if !valido {
		plan = usuario.Plan
	}
	return domain.UsuarioAdminDTO{
		ID:             usuario.ID.String(),
		Correo:         usuario.Correo,
		NombreCompleto: usuario.NombreCompleto,
		Telefono:       usuario.Telefono,
		Direccion:      usuario.Direccion,
		URLAvatar:      usuario.URLAvatar,
		Rol:            rol,
		Plan:           plan,
		CuotaTokensIA:  usuario.CuotaTokensIA,
		EstaActivo:     usuario.EstaActivo,
		UltimoAcceso:   usuario.UltimoAcceso,
		CreadoEl:       usuario.CreadoEl,
		ActualizadoEl:  usuario.ActualizadoEl,
	}
}
