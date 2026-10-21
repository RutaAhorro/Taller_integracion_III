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

// AdminUsuarioService define los casos de uso del panel de administración de cuentas.
type AdminUsuarioService interface {
	Listar(ctx context.Context, filtro domain.FiltroUsuariosAdminDTO) (*domain.PaginaUsuariosAdminDTO, error)
	Metricas(ctx context.Context) (*domain.MetricasUsuariosDTO, error)
	Actualizar(ctx context.Context, id string, input domain.ActualizarUsuarioAdminDTO, actorID string) (*domain.UsuarioAdminDTO, error)
}

type adminUsuarioService struct {
	usuarioRepo domain.AdminUsuarioRepository
}

// NewAdminUsuarioService inicializa el servicio administrativo con su repositorio.
func NewAdminUsuarioService(usuarioRepo domain.AdminUsuarioRepository) AdminUsuarioService {
	return &adminUsuarioService{usuarioRepo: usuarioRepo}
}

// Listar devuelve una página de cuentas con los filtros recibidos, sin exponer credenciales.
func (s *adminUsuarioService) Listar(ctx context.Context, filtro domain.FiltroUsuariosAdminDTO) (*domain.PaginaUsuariosAdminDTO, error) {
	if filtro.Rol != "" {
		rol, err := normalizarRol(filtro.Rol)
		if err != nil {
			return nil, err
		}
		filtro.Rol = rol
	}
	if filtro.Page < 1 {
		filtro.Page = 1
	}
	if filtro.Limit < 1 {
		filtro.Limit = 20
	}
	if filtro.Offset < 0 {
		filtro.Offset = 0
	}

	usuarios, total, err := s.usuarioRepo.Listar(ctx, filtro)
	if err != nil {
		return nil, err
	}

	resultado := make([]domain.UsuarioAdminDTO, 0, len(usuarios))
	for i := range usuarios {
		resultado = append(resultado, mapearUsuarioAdmin(&usuarios[i]))
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
func (s *adminUsuarioService) Metricas(ctx context.Context) (*domain.MetricasUsuariosDTO, error) {
	total, activos, err := s.usuarioRepo.Contar(ctx)
	if err != nil {
		return nil, err
	}
	return &domain.MetricasUsuariosDTO{TotalUsuarios: total, UsuariosActivos: activos}, nil
}

// Actualizar aplica esta_activo y/o rol sobre una cuenta existente. La guarda
// del último admin y la auditoría se ejecutan juntas en el repositorio.
func (s *adminUsuarioService) Actualizar(ctx context.Context, id string, input domain.ActualizarUsuarioAdminDTO, actorID string) (*domain.UsuarioAdminDTO, error) {
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrIDUsuarioInvalido
	}
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return nil, ErrIDUsuarioInvalido
	}

	datos := make(map[string]interface{})
	if input.Rol != nil {
		rol, err := normalizarRol(*input.Rol)
		if err != nil {
			return nil, err
		}
		datos["rol"] = rol
	}
	if input.EstaActivo != nil {
		datos["esta_activo"] = *input.EstaActivo
	}
	if input.Plan != nil {
		plan, err := normalizarPlan(*input.Plan)
		if err != nil {
			return nil, err
		}
		datos["plan"] = plan
	}
	if len(datos) == 0 {
		return nil, ErrSinCambiosAdmin
	}

	datos["actualizado_el"] = time.Now()
	if err := s.usuarioRepo.ActualizarConGuarda(ctx, id, datos, actorID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUsuarioNoEncontrado
		}
		if errors.Is(err, domain.ErrUltimoAdmin) {
			return nil, ErrUltimoAdmin
		}
		return nil, err
	}

	actualizado, err := s.usuarioRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if actualizado == nil {
		return nil, ErrUsuarioNoEncontrado
	}

	dto := mapearUsuarioAdmin(actualizado)
	return &dto, nil
}

// normalizarRol valida y normaliza un rol antes de persistirlo.
func normalizarRol(rol string) (string, error) {
	normalizado, ok := domain.NormalizarRol(rol)
	if !ok {
		return "", ErrRolInvalido
	}
	return normalizado, nil
}

// normalizarPlan valida y normaliza el plan comercial antes de persistirlo.
func normalizarPlan(plan string) (string, error) {
	normalizado, ok := domain.NormalizarPlan(plan)
	if !ok {
		return "", ErrPlanInvalido
	}
	return normalizado, nil
}

// mapearUsuarioAdmin convierte la entidad privada en su DTO de administración.
func mapearUsuarioAdmin(usuario *domain.Usuario) domain.UsuarioAdminDTO {
	rol, ok := domain.NormalizarRol(usuario.Rol)
	if !ok {
		rol = usuario.Rol
	}
	plan, ok := domain.NormalizarPlan(usuario.Plan)
	if !ok {
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
