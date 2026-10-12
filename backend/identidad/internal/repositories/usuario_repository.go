package repositories

import (
	"context"
	"errors"

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

func classifyWriteError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return domain.ErrCorreoDuplicado
	}
	return err
}
