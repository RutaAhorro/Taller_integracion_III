package repositories

import (
	"context"
	"errors"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type gormUsuarioRepository struct {
	db *gorm.DB
}

// NewUsuarioRepository inicializa un repositorio de usuarios con conexión GORM
func NewUsuarioRepository(db *gorm.DB) domain.UsuarioRepository {
	return &gormUsuarioRepository{db: db}
}

// NewAdminUsuarioRepository expone las operaciones administrativas del mismo adaptador.
func NewAdminUsuarioRepository(db *gorm.DB) domain.AdminUsuarioRepository {
	return &gormUsuarioRepository{db: db}
}

// FindByEmail consulta si existe un usuario por su dirección de correo electrónico
func (r *gormUsuarioRepository) FindByEmail(ctx context.Context, email string) (*domain.Usuario, error) {
	var usuario domain.Usuario
	err := r.db.WithContext(ctx).Where("correo = ?", email).First(&usuario).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &usuario, nil
}

// FindByID consulta a un usuario por su clave primaria UUID
func (r *gormUsuarioRepository) FindByID(ctx context.Context, id string) (*domain.Usuario, error) {
	var usuario domain.Usuario
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&usuario).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &usuario, nil
}

// Listar devuelve las cuentas de api.usuarios con los filtros administrativos indicados.
func (r *gormUsuarioRepository) Listar(ctx context.Context, filtro domain.FiltroUsuariosAdminDTO) ([]domain.Usuario, error) {
	query := r.db.WithContext(ctx).Model(&domain.Usuario{})

	if texto := strings.TrimSpace(filtro.Query); texto != "" {
		patron := "%" + escaparPatronLike(strings.ToLower(texto)) + "%"
		query = query.Where(`LOWER(correo) LIKE ? ESCAPE '!' OR LOWER(nombre_completo) LIKE ? ESCAPE '!'`, patron, patron)
	}
	if rol := strings.TrimSpace(filtro.Rol); rol != "" {
		query = query.Where("LOWER(rol) = ?", strings.ToLower(rol))
	}
	if filtro.EstaActivo != nil {
		query = query.Where("esta_activo = ?", *filtro.EstaActivo)
	}

	query = query.Order("creado_el DESC, id DESC")
	if filtro.Limit > 0 {
		query = query.Limit(filtro.Limit)
	}
	if filtro.Offset > 0 {
		query = query.Offset(filtro.Offset)
	}

	var usuarios []domain.Usuario
	if err := query.Find(&usuarios).Error; err != nil {
		return nil, err
	}
	return usuarios, nil
}

// Create inserta un nuevo registro de usuario en la base de datos
func (r *gormUsuarioRepository) Create(ctx context.Context, usuario *domain.Usuario) error {
	return classifyWriteError(r.db.WithContext(ctx).Create(usuario).Error)
}

// Actualizar aplica cambios parciales dinámicamente a la tabla api.usuarios usando GORM Updates
func (r *gormUsuarioRepository) Actualizar(ctx context.Context, id string, datos map[string]interface{}) error {
	res := r.db.WithContext(ctx).
		Model(&domain.Usuario{}).
		Where("id = ?", id).
		Updates(datos)
	if res.Error != nil {
		return classifyWriteError(res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ContarAdminsActivos cuenta administradores activos distintos del usuario
// indicado. El rol se compara sin distinguir mayúsculas para tolerar datos
// históricos escritos antes de normalizar el RBAC.
func (r *gormUsuarioRepository) ContarAdminsActivos(ctx context.Context, excluirID string) (int64, error) {
	query := r.db.WithContext(ctx).
		Model(&domain.Usuario{}).
		Where("LOWER(rol) = ? AND esta_activo = ?", domain.RolAdmin, true)
	if excluirID != "" {
		query = query.Where("id <> ?", excluirID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// escaparPatronLike neutraliza los comodines de LIKE para que la búsqueda
// trate `%`, `_` y `!` como texto literal. Se usa `!` como carácter de escape.
func escaparPatronLike(texto string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(texto)
}

func classifyWriteError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return domain.ErrCorreoDuplicado
	}
	return err
}
