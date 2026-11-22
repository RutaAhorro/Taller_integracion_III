// Package repositories implementa el acceso SQL de Identidad con GORM, sólo
// sobre sus tablas (rol app_identity) y sin AutoMigrate.
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

// ===== Construcción =====

// adminGuardLockKey serializa las actualizaciones administrativas entre
// instancias. El valor es un identificador estable, no un dato de negocio.
const adminGuardLockKey int64 = 0x535550323432

// repositorioUsuarios implementa los repositorios de perfil y de
// administración sobre api.usuarios.
type repositorioUsuarios struct {
	baseDatos *gorm.DB
}

// NuevoRepositorioUsuarios inicializa un repositorio de usuarios con conexión GORM.
func NuevoRepositorioUsuarios(baseDatos *gorm.DB) domain.RepositorioUsuarios {
	return &repositorioUsuarios{baseDatos: baseDatos}
}

// NuevoRepositorioAdmin expone las operaciones administrativas del mismo adaptador.
func NuevoRepositorioAdmin(baseDatos *gorm.DB) domain.RepositorioAdmin {
	return &repositorioUsuarios{baseDatos: baseDatos}
}

// ===== Consultas =====

// BuscarPorCorreo consulta si existe un usuario por su dirección de correo electrónico
func (repositorio *repositorioUsuarios) BuscarPorCorreo(contexto context.Context, direccionCorreo string) (*domain.Usuario, error) {
	var usuario domain.Usuario
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Where("correo = ?", direccionCorreo).First(&usuario).Error
	if errorObtenido != nil {
		if errors.Is(errorObtenido, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, errorObtenido
	}
	return &usuario, nil
}

// BuscarPorID consulta a un usuario por su clave primaria UUID
func (repositorio *repositorioUsuarios) BuscarPorID(contexto context.Context, id string) (*domain.Usuario, error) {
	var usuario domain.Usuario
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Where("id = ?", id).First(&usuario).Error
	if errorObtenido != nil {
		if errors.Is(errorObtenido, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, errorObtenido
	}
	return &usuario, nil
}

// Listar devuelve una página de cuentas de api.usuarios y el total de filas
// que cumplen los filtros administrativos indicados.
func (repositorio *repositorioUsuarios) Listar(contexto context.Context, filtro domain.FiltroUsuariosAdminDTO) ([]domain.Usuario, int64, error) {
	consulta := repositorio.baseDatos.WithContext(contexto).Model(&domain.Usuario{})

	if texto := strings.TrimSpace(filtro.Query); texto != "" {
		patron := "%" + escaparPatronLike(strings.ToLower(texto)) + "%"
		consulta = consulta.Where(`LOWER(correo) LIKE ? ESCAPE '!' OR LOWER(nombre_completo) LIKE ? ESCAPE '!'`, patron, patron)
	}
	// El servicio ya validó y normalizó el rol del filtro.
	if filtro.Rol != "" {
		consulta = consulta.Where("rol = ?", filtro.Rol)
	}
	if filtro.EstaActivo != nil {
		consulta = consulta.Where("esta_activo = ?", *filtro.EstaActivo)
	}

	// Count sobre una sesión independiente para no arrastrar ORDER/LIMIT.
	var total int64
	if errorObtenido := consulta.Session(&gorm.Session{}).Count(&total).Error; errorObtenido != nil {
		return nil, 0, errorObtenido
	}

	consulta = consulta.Order("creado_el DESC, id DESC")
	if filtro.Limit > 0 {
		consulta = consulta.Limit(filtro.Limit)
	}
	if desplazamiento := filtro.Desplazamiento(); desplazamiento > 0 {
		consulta = consulta.Offset(desplazamiento)
	}

	var usuarios []domain.Usuario
	if errorObtenido := consulta.Select(
		"id, correo, nombre_completo, telefono, direccion, url_avatar, rol, plan, cuota_tokens_ia, esta_activo, ultimo_acceso, creado_el, actualizado_el",
	).Find(&usuarios).Error; errorObtenido != nil {
		return nil, 0, errorObtenido
	}
	return usuarios, total, nil
}

// Contar devuelve el total de cuentas y cuántas están activas, en dos COUNT
// simples que no exponen columnas sensibles ni cargan filas en memoria.
func (repositorio *repositorioUsuarios) Contar(contexto context.Context) (int64, int64, error) {
	var total, activos int64
	consulta := repositorio.baseDatos.WithContext(contexto).Model(&domain.Usuario{})
	if errorObtenido := consulta.Session(&gorm.Session{}).Count(&total).Error; errorObtenido != nil {
		return 0, 0, errorObtenido
	}
	if errorObtenido := consulta.Session(&gorm.Session{}).Where("esta_activo = ?", true).Count(&activos).Error; errorObtenido != nil {
		return 0, 0, errorObtenido
	}
	return total, activos, nil
}

// ===== Escrituras =====

// Crear inserta un nuevo registro de usuario en la base de datos
func (repositorio *repositorioUsuarios) Crear(contexto context.Context, usuario *domain.Usuario) error {
	return clasificarErrorEscritura(repositorio.baseDatos.WithContext(contexto).Create(usuario).Error)
}

// Actualizar aplica cambios parciales dinámicamente a la tabla api.usuarios usando GORM Updates
func (repositorio *repositorioUsuarios) Actualizar(contexto context.Context, id string, datos map[string]interface{}) error {
	resultado := repositorio.baseDatos.WithContext(contexto).
		Model(&domain.Usuario{}).
		Where("id = ?", id).
		Updates(datos)
	if resultado.Error != nil {
		return clasificarErrorEscritura(resultado.Error)
	}
	if resultado.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ===== Administración con guarda del último admin =====

// ActualizarConGuarda aplica el cambio de estado/rol y su auditoría dentro de
// una transacción. El advisory lock serializa las actualizaciones entre
// instancias para que dos admins no puedan degradarse mutuamente a la vez.
func (repositorio *repositorioUsuarios) ActualizarConGuarda(contexto context.Context, id string, datos map[string]interface{}, actorID string) error {
	actorUUID, errorObtenido := uuid.Parse(strings.TrimSpace(actorID))
	if errorObtenido != nil {
		return errors.New("actor de auditoría inválido")
	}

	return repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		if errorOperacion := transaccion.Exec("SELECT pg_advisory_xact_lock(?)", adminGuardLockKey).Error; errorOperacion != nil {
			return errorOperacion
		}

		var usuario domain.Usuario
		if errorOperacion := transaccion.Where("id = ?", id).First(&usuario).Error; errorOperacion != nil {
			return errorOperacion
		}

		// usuarios_rol_check garantiza que usuario.Rol ya es un valor canónico.
		dejaDeSerAdmin := false
		if usuario.Rol == domain.RolAdmin && usuario.EstaActivo {
			if activo, valido := datos["esta_activo"].(bool); valido && !activo {
				dejaDeSerAdmin = true
			}
			if nuevoRol, valido := datos["rol"].(string); valido {
				if canonico, _ := domain.NormalizarRol(nuevoRol); canonico != domain.RolAdmin {
					dejaDeSerAdmin = true
				}
			}
		}

		if dejaDeSerAdmin {
			var otros int64
			if errorOperacion := transaccion.Model(&domain.Usuario{}).
				Where("id <> ? AND rol = ? AND esta_activo = ?",
					id, domain.RolAdmin, true).
				Count(&otros).Error; errorOperacion != nil {
				return errorOperacion
			}
			if otros == 0 {
				return domain.ErrUltimoAdmin
			}
			// Con otro admin activo, el cambio seguiría siendo peligroso si el
			// actor se lo aplica a sí mismo: la revalidación de sesión lo
			// dejaría fuera al instante. Exigir que lo haga otro administrador.
			if actorUUID == usuario.ID {
				return domain.ErrAutoModificacionAdmin
			}
		}

		resultado := transaccion.Model(&domain.Usuario{}).Where("id = ?", id).Updates(datos)
		if resultado.Error != nil {
			return clasificarErrorEscritura(resultado.Error)
		}
		if resultado.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		cambios := map[string]interface{}{}
		if nuevo, valido := datos["rol"]; valido {
			// Sólo se audita un cambio efectivo. Un PUT idempotente con el
			// mismo valor no debe dejar un registro ficticio (anterior == nuevo).
			if texto, esTexto := nuevo.(string); !esTexto || strings.TrimSpace(texto) != strings.TrimSpace(usuario.Rol) {
				cambios["rol"] = map[string]interface{}{"anterior": usuario.Rol, "nuevo": nuevo}
			}
		}
		if nuevo, valido := datos["esta_activo"]; valido {
			if activo, esBool := nuevo.(bool); !esBool || activo != usuario.EstaActivo {
				cambios["esta_activo"] = map[string]interface{}{"anterior": usuario.EstaActivo, "nuevo": nuevo}
			}
		}
		if nuevo, valido := datos["plan"]; valido {
			if texto, esTexto := nuevo.(string); !esTexto || strings.TrimSpace(texto) != strings.TrimSpace(usuario.Plan) {
				cambios["plan"] = map[string]interface{}{"anterior": usuario.Plan, "nuevo": nuevo}
			}
		}
		if len(cambios) == 0 {
			return nil
		}
		auditoria := domain.AuditoriaAdminUsuario{
			ActorID:    actorUUID,
			ObjetivoID: usuario.ID,
			Cambios:    cambios,
		}
		if errorOperacion := transaccion.Create(&auditoria).Error; errorOperacion != nil {
			return errorOperacion
		}
		return nil
	})
}

// ===== Utilidades SQL =====

// escaparPatronLike neutraliza los comodines de LIKE para que la búsqueda
// trate `%`, `_` y `!` como texto literal. Se usa `!` como carácter de escape.
func escaparPatronLike(texto string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(texto)
}

// codigoViolacionUnicidad es el SQLSTATE con que PostgreSQL rechaza un valor
// duplicado en una columna única.
const codigoViolacionUnicidad = "23505"

// clasificarErrorEscritura traduce la violación de unicidad de PostgreSQL
// al error de negocio de correo duplicado.
func clasificarErrorEscritura(errorObtenido error) error {
	var errorPostgres *pgconn.PgError
	if errors.As(errorObtenido, &errorPostgres) && errorPostgres.Code == codigoViolacionUnicidad {
		return domain.ErrCorreoDuplicado
	}
	return errorObtenido
}
