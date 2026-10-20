package repositories

import (
	"context"
	"errors"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// adminGuardLockKey serializa las actualizaciones administrativas entre
// instancias. El valor es un identificador estable, no un dato de negocio.
const adminGuardLockKey int64 = 0x535550323432

type gormUsuarioRepository struct {
	db *gorm.DB
}

// NewUsuarioRepository inicializa un repositorio de usuarios con conexión GORM.
// Sin conexión devuelve nil para que los routers de prueba no intenten consultar.
func NewUsuarioRepository(db *gorm.DB) domain.UsuarioRepository {
	if db == nil {
		return nil
	}
	return &gormUsuarioRepository{db: db}
}

// NewAdminUsuarioRepository expone las operaciones administrativas del mismo adaptador.
func NewAdminUsuarioRepository(db *gorm.DB) domain.AdminUsuarioRepository {
	if db == nil {
		return nil
	}
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

// Listar devuelve una página de cuentas de api.usuarios y el total de filas
// que cumplen los filtros administrativos indicados.
func (r *gormUsuarioRepository) Listar(ctx context.Context, filtro domain.FiltroUsuariosAdminDTO) ([]domain.Usuario, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.Usuario{})

	if texto := strings.TrimSpace(filtro.Query); texto != "" {
		patron := "%" + escaparPatronLike(strings.ToLower(texto)) + "%"
		query = query.Where(`LOWER(correo) LIKE ? ESCAPE '!' OR LOWER(nombre_completo) LIKE ? ESCAPE '!'`, patron, patron)
	}
	if rol := strings.ToLower(strings.TrimSpace(filtro.Rol)); rol != "" {
		if rol == domain.RolAdmin {
			query = query.Where("LOWER(TRIM(rol)) IN ?", []string{domain.RolAdmin, domain.RolAdminLegacy})
		} else {
			query = query.Where("LOWER(TRIM(rol)) = ?", rol)
		}
	}
	if filtro.EstaActivo != nil {
		query = query.Where("esta_activo = ?", *filtro.EstaActivo)
	}

	// Count sobre una sesión independiente para no arrastrar ORDER/LIMIT.
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
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
		return nil, 0, err
	}
	return usuarios, total, nil
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

// ActualizarConGuarda aplica el cambio de estado/rol y su auditoría dentro de
// una transacción. El advisory lock serializa las actualizaciones entre
// instancias para que dos admins no puedan degradarse mutuamente a la vez.
func (r *gormUsuarioRepository) ActualizarConGuarda(ctx context.Context, id string, datos map[string]interface{}, actorID string) error {
	actorUUID, err := uuid.Parse(strings.TrimSpace(actorID))
	if err != nil {
		return errors.New("actor de auditoría inválido")
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", adminGuardLockKey).Error; err != nil {
			return err
		}

		var usuario domain.Usuario
		if err := tx.Where("id = ?", id).First(&usuario).Error; err != nil {
			return err
		}

		rolActual, _ := domain.NormalizarRol(usuario.Rol)
		// Si no se está cambiando el rol, aprovechar la actualización para
		// persistir el valor canónico (p. ej. "administrador" -> "admin").
		if _, cambiaRol := datos["rol"]; !cambiaRol {
			if canonico, ok := domain.NormalizarRol(usuario.Rol); ok && strings.TrimSpace(usuario.Rol) != canonico {
				datos["rol"] = canonico
			}
		}

		dejaDeSerAdmin := false
		if rolActual == domain.RolAdmin && usuario.EstaActivo {
			if activo, ok := datos["esta_activo"].(bool); ok && !activo {
				dejaDeSerAdmin = true
			}
			if nuevoRol, ok := datos["rol"].(string); ok {
				if canonico, _ := domain.NormalizarRol(nuevoRol); canonico != domain.RolAdmin {
					dejaDeSerAdmin = true
				}
			}
		}

		if dejaDeSerAdmin {
			var otros int64
			if err := tx.Model(&domain.Usuario{}).
				Where("id <> ? AND LOWER(TRIM(rol)) IN ? AND esta_activo = ?",
					id, []string{domain.RolAdmin, domain.RolAdminLegacy}, true).
				Count(&otros).Error; err != nil {
				return err
			}
			if otros == 0 {
				return domain.ErrUltimoAdmin
			}
		}

		res := tx.Model(&domain.Usuario{}).Where("id = ?", id).Updates(datos)
		if res.Error != nil {
			return classifyWriteError(res.Error)
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		cambios := map[string]interface{}{}
		if nuevo, ok := datos["rol"]; ok {
			cambios["rol"] = map[string]interface{}{"anterior": usuario.Rol, "nuevo": nuevo}
		}
		if nuevo, ok := datos["esta_activo"]; ok {
			cambios["esta_activo"] = map[string]interface{}{"anterior": usuario.EstaActivo, "nuevo": nuevo}
		}
		if nuevo, ok := datos["plan"]; ok {
			cambios["plan"] = map[string]interface{}{"anterior": usuario.Plan, "nuevo": nuevo}
		}
		auditoria := domain.AuditoriaAdminUsuario{
			ActorID:    actorUUID,
			ObjetivoID: usuario.ID,
			Cambios:    cambios,
		}
		if err := tx.Create(&auditoria).Error; err != nil {
			return err
		}
		return nil
	})
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
