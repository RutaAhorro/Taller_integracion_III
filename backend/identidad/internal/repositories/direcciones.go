// Persistencia de api.direcciones_usuario con el rol app_identity. Cada
// operación filtra por el usuario dueño y las que cambian la dirección
// principal se serializan por usuario dentro de una transacción.

package repositories

import (
	"context"
	"errors"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// ===== Construcción =====

const (
	// espacioBloqueoDirecciones es el primer entero del advisory lock de dos
	// claves: aísla este uso del adminGuardLockKey y de futuros bloqueos. El
	// segundo entero es el hash del usuario, así que sólo se serializan las
	// escrituras del mismo usuario.
	espacioBloqueoDirecciones int32 = 0x53553233

	// nombreIndicePrincipal es el índice único parcial de la migración 11 que
	// garantiza una sola principal por usuario.
	nombreIndicePrincipal = "uq_direcciones_usuario_principal"

	// ordenDirecciones fija el orden del listado y de la promoción: principal
	// primero y luego la más antigua. El id desempata filas con la misma fecha.
	ordenDirecciones = "es_principal DESC, creado_el ASC, id ASC"
)

// repositorioDirecciones implementa domain.RepositorioDirecciones con GORM.
type repositorioDirecciones struct {
	baseDatos *gorm.DB
}

// NuevoRepositorioDirecciones inicializa el repositorio con la conexión de Identidad.
func NuevoRepositorioDirecciones(baseDatos *gorm.DB) domain.RepositorioDirecciones {
	return &repositorioDirecciones{baseDatos: baseDatos}
}

// ===== Consultas =====

// Listar devuelve las direcciones del usuario en el orden documentado.
func (repositorio *repositorioDirecciones) Listar(contexto context.Context, usuarioID string) ([]domain.DireccionUsuario, error) {
	var direcciones []domain.DireccionUsuario
	errorObtenido := repositorio.baseDatos.WithContext(contexto).
		Where("usuario_id = ?", usuarioID).
		Order(ordenDirecciones).
		Find(&direcciones).Error
	return direcciones, errorObtenido
}

// ===== Escrituras =====

// Crear inserta la dirección respetando el límite y la regla de una sola
// principal. El bloqueo por usuario hace atómicos el conteo, el desmarcado de
// la principal anterior y el insert.
func (repositorio *repositorioDirecciones) Crear(contexto context.Context, usuarioID string, direccion *domain.DireccionUsuario, limite int) error {
	duenoID, errorObtenido := uuid.Parse(usuarioID)
	if errorObtenido != nil {
		return errors.New("usuario de la dirección inválido")
	}
	errorObtenido = repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		if errorBloqueo := bloquearDireccionesDeUsuario(transaccion, usuarioID); errorBloqueo != nil {
			return errorBloqueo
		}
		var existentes int64
		if errorConteo := transaccion.Model(&domain.DireccionUsuario{}).Where("usuario_id = ?", usuarioID).Count(&existentes).Error; errorConteo != nil {
			return errorConteo
		}
		if int(existentes) >= limite {
			return domain.ErrLimiteDirecciones
		}
		// La primera dirección es la principal; si no, sólo lo es si se pidió.
		if existentes == 0 {
			direccion.EsPrincipal = true
		}
		if direccion.EsPrincipal && existentes > 0 {
			if errorDesmarcado := desmarcarPrincipal(transaccion, usuarioID); errorDesmarcado != nil {
				return errorDesmarcado
			}
		}
		direccion.ID = uuid.Nil
		direccion.UsuarioID = duenoID
		return transaccion.Create(direccion).Error
	})
	return clasificarErrorDireccion(errorObtenido)
}

// Actualizar reemplaza los datos editables de la dirección y, si se pide,
// cambia la principal en la misma transacción. Una dirección ajena no se
// encuentra: el filtro por usuario_id hace que se comporte como inexistente.
func (repositorio *repositorioDirecciones) Actualizar(contexto context.Context, usuarioID, direccionID string, datos domain.DireccionUsuario, esPrincipal *bool) (*domain.DireccionUsuario, error) {
	var actualizada domain.DireccionUsuario
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		if errorBloqueo := bloquearDireccionesDeUsuario(transaccion, usuarioID); errorBloqueo != nil {
			return errorBloqueo
		}
		if errorBusqueda := transaccion.Where("id = ? AND usuario_id = ?", direccionID, usuarioID).First(&actualizada).Error; errorBusqueda != nil {
			return errorBusqueda
		}
		if esPrincipal != nil && !*esPrincipal && actualizada.EsPrincipal {
			return domain.ErrPrincipalObligatoria
		}
		hacerPrincipal := esPrincipal != nil && *esPrincipal && !actualizada.EsPrincipal
		if hacerPrincipal {
			if errorDesmarcado := desmarcarPrincipal(transaccion, usuarioID); errorDesmarcado != nil {
				return errorDesmarcado
			}
		}
		// Un mapa (no la entidad) para que las coordenadas ausentes se
		// escriban como NULL en lugar de omitirse por ser el valor cero.
		cambios := map[string]any{
			"etiqueta":        punteroComoValor(datos.Etiqueta),
			"texto_direccion": datos.TextoDireccion,
			"comuna":          datos.Comuna,
			"lat":             punteroComoValor(datos.Lat),
			"lon":             punteroComoValor(datos.Lon),
		}
		if hacerPrincipal {
			cambios["es_principal"] = true
		}
		if errorEscritura := transaccion.Model(&domain.DireccionUsuario{}).
			Where("id = ? AND usuario_id = ?", direccionID, usuarioID).
			Updates(cambios).Error; errorEscritura != nil {
			return errorEscritura
		}
		// Se relee para devolver exactamente lo persistido.
		return transaccion.Where("id = ? AND usuario_id = ?", direccionID, usuarioID).First(&actualizada).Error
	})
	if errorObtenido != nil {
		return nil, clasificarErrorDireccion(errorObtenido)
	}
	return &actualizada, nil
}

// Eliminar borra la dirección y, si era la principal, promueve a la más
// antigua de las restantes: así un usuario con direcciones siempre tiene principal.
func (repositorio *repositorioDirecciones) Eliminar(contexto context.Context, usuarioID, direccionID string) error {
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		if errorBloqueo := bloquearDireccionesDeUsuario(transaccion, usuarioID); errorBloqueo != nil {
			return errorBloqueo
		}
		var existente domain.DireccionUsuario
		if errorBusqueda := transaccion.Where("id = ? AND usuario_id = ?", direccionID, usuarioID).First(&existente).Error; errorBusqueda != nil {
			return errorBusqueda
		}
		if errorBorrado := transaccion.Where("id = ? AND usuario_id = ?", direccionID, usuarioID).Delete(&domain.DireccionUsuario{}).Error; errorBorrado != nil {
			return errorBorrado
		}
		if !existente.EsPrincipal {
			return nil
		}
		// Subconsulta con ORDER BY y LIMIT 1: la candidata más antigua.
		return transaccion.Exec(
			"UPDATE api.direcciones_usuario SET es_principal = true WHERE id = (SELECT id FROM api.direcciones_usuario WHERE usuario_id = ? ORDER BY creado_el ASC, id ASC LIMIT 1)",
			usuarioID,
		).Error
	})
	return clasificarErrorDireccion(errorObtenido)
}

// ===== Utilidades SQL =====

// bloquearDireccionesDeUsuario toma un advisory lock transaccional por usuario.
// Serializa el conteo, el cambio de principal y el borrado de un mismo usuario
// sin bloquear a los demás; se libera solo al terminar la transacción.
func bloquearDireccionesDeUsuario(transaccion *gorm.DB, usuarioID string) error {
	return transaccion.Exec("SELECT pg_advisory_xact_lock(?::int, hashtext(?))", espacioBloqueoDirecciones, usuarioID).Error
}

// desmarcarPrincipal quita la marca a la principal vigente del usuario.
func desmarcarPrincipal(transaccion *gorm.DB, usuarioID string) error {
	return transaccion.Model(&domain.DireccionUsuario{}).
		Where("usuario_id = ? AND es_principal", usuarioID).
		Update("es_principal", false).Error
}

// punteroComoValor devuelve nil sin tipo para un puntero nulo, de modo que el
// controlador escriba NULL, y el valor apuntado en caso contrario.
func punteroComoValor[T any](valor *T) any {
	if valor == nil {
		return nil
	}
	return *valor
}

// clasificarErrorDireccion traduce los errores de GORM y PostgreSQL a errores de
// dominio: fila ausente a no encontrada y violación del índice de principal a
// conflicto. No reutiliza clasificarErrorEscritura, que interpreta toda
// unicidad como correo duplicado.
func clasificarErrorDireccion(errorObtenido error) error {
	if errorObtenido == nil {
		return nil
	}
	if errors.Is(errorObtenido, gorm.ErrRecordNotFound) {
		return domain.ErrDireccionNoEncontrada
	}
	var errorPostgres *pgconn.PgError
	if errors.As(errorObtenido, &errorPostgres) && errorPostgres.Code == codigoViolacionUnicidad && errorPostgres.ConstraintName == nombreIndicePrincipal {
		return domain.ErrConflictoPrincipal
	}
	return errorObtenido
}
