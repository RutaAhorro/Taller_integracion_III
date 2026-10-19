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
	// ErrSinCambiosAdmin se emite cuando la actualización administrativa no trae campos.
	ErrSinCambiosAdmin = errors.New("no se enviaron cambios para actualizar la cuenta")
	// ErrIDUsuarioInvalido se emite cuando el identificador no es un UUID válido.
	ErrIDUsuarioInvalido = errors.New("identificador de usuario inválido")
)

// AdminUsuarioService define los casos de uso del panel de administración de cuentas.
type AdminUsuarioService interface {
	Listar(ctx context.Context, filtro domain.FiltroUsuariosAdminDTO) ([]domain.UsuarioAdminDTO, error)
	Actualizar(ctx context.Context, id string, input domain.ActualizarUsuarioAdminDTO) (*domain.UsuarioAdminDTO, error)
}

type adminUsuarioService struct {
	usuarioRepo domain.AdminUsuarioRepository
}

// NewAdminUsuarioService inicializa el servicio administrativo con su repositorio.
func NewAdminUsuarioService(usuarioRepo domain.AdminUsuarioRepository) AdminUsuarioService {
	return &adminUsuarioService{usuarioRepo: usuarioRepo}
}

// Listar devuelve las cuentas con los filtros recibidos, sin exponer credenciales.
func (s *adminUsuarioService) Listar(ctx context.Context, filtro domain.FiltroUsuariosAdminDTO) ([]domain.UsuarioAdminDTO, error) {
	if filtro.Rol != "" {
		rol, err := normalizarRol(filtro.Rol)
		if err != nil {
			return nil, err
		}
		filtro.Rol = rol
	}

	usuarios, err := s.usuarioRepo.Listar(ctx, filtro)
	if err != nil {
		return nil, err
	}

	resultado := make([]domain.UsuarioAdminDTO, 0, len(usuarios))
	for i := range usuarios {
		resultado = append(resultado, mapearUsuarioAdmin(&usuarios[i]))
	}
	return resultado, nil
}

// Actualizar aplica esta_activo y/o rol sobre una cuenta existente.
func (s *adminUsuarioService) Actualizar(ctx context.Context, id string, input domain.ActualizarUsuarioAdminDTO) (*domain.UsuarioAdminDTO, error) {
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
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
	if len(datos) == 0 {
		return nil, ErrSinCambiosAdmin
	}

	usuario, err := s.usuarioRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if usuario == nil {
		return nil, ErrUsuarioNoEncontrado
	}

	datos["actualizado_el"] = time.Now()
	if err := s.usuarioRepo.Actualizar(ctx, id, datos); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUsuarioNoEncontrado
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
	switch strings.ToLower(strings.TrimSpace(rol)) {
	case domain.RolAdmin:
		return domain.RolAdmin, nil
	case domain.RolColaborador:
		return domain.RolColaborador, nil
	case domain.RolRegistrado:
		return domain.RolRegistrado, nil
	default:
		return "", ErrRolInvalido
	}
}

// mapearUsuarioAdmin convierte la entidad privada en su DTO de administración.
func mapearUsuarioAdmin(usuario *domain.Usuario) domain.UsuarioAdminDTO {
	return domain.UsuarioAdminDTO{
		ID:             usuario.ID.String(),
		Correo:         usuario.Correo,
		NombreCompleto: usuario.NombreCompleto,
		Telefono:       usuario.Telefono,
		Direccion:      usuario.Direccion,
		URLAvatar:      usuario.URLAvatar,
		Rol:            usuario.Rol,
		CuotaTokensIA:  usuario.CuotaTokensIA,
		EstaActivo:     usuario.EstaActivo,
		CreadoEl:       usuario.CreadoEl,
		ActualizadoEl:  usuario.ActualizadoEl,
	}
}
