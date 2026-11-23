// Acceso SQL a api.tokens_recuperacion con el rol app_identity. El consumo es
// atómico: marcar el token y cambiar la contraseña van en la misma transacción.

package repositories

import (
	"context"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"gorm.io/gorm"
)

// ===== Tokens de restablecimiento =====

// repositorioRecuperacion implementa domain.RepositorioRecuperacion sobre GORM.
type repositorioRecuperacion struct {
	baseDatos *gorm.DB
}

// NuevoRepositorioRecuperacion inicializa el repositorio de tokens con la
// conexión de Identidad, sin permisos fuera de api.tokens_recuperacion.
func NuevoRepositorioRecuperacion(baseDatos *gorm.DB) domain.RepositorioRecuperacion {
	return &repositorioRecuperacion{baseDatos: baseDatos}
}

// Validar comprueba que el hash exista, no haya expirado y siga sin usarse.
// Usa el reloj de PostgreSQL, igual que la emisión de tokens (SUP-184), para
// que la expiración no dependa de la zona horaria del proceso.
func (repositorio *repositorioRecuperacion) Validar(contexto context.Context, hashToken []byte) error {
	var vigente bool
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Raw(`SELECT EXISTS (
		SELECT 1 FROM api.tokens_recuperacion
		WHERE token_hash = ? AND usado_el IS NULL AND expira_el > now()
	)`, hashToken).Scan(&vigente).Error
	if errorObtenido != nil {
		return errorObtenido
	}
	if !vigente {
		return domain.ErrTokenRecuperacionInvalido
	}
	return nil
}

// Consumir marca el token como usado y actualiza la contraseña en una sola
// transacción. El UPDATE condicional es el candado de un solo uso: dos
// peticiones concurrentes compiten por la misma fila y sólo una obtiene el
// usuario; la otra ve cero filas y se revierte. Si falla la escritura del
// usuario, la transacción se revierte y el token queda disponible.
func (repositorio *repositorioRecuperacion) Consumir(contexto context.Context, hashToken []byte, hashContrasena string, actualizadoEl time.Time) error {
	return repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		var usuarioID string
		resultado := transaccion.Raw(`UPDATE api.tokens_recuperacion
			SET usado_el = now()
			WHERE token_hash = ? AND usado_el IS NULL AND expira_el > now()
			RETURNING usuario_id::text`, hashToken).Scan(&usuarioID)
		if resultado.Error != nil {
			return resultado.Error
		}
		if usuarioID == "" {
			return domain.ErrTokenRecuperacionInvalido
		}

		actualizacion := transaccion.Model(&domain.Usuario{}).
			Where("id = ?", usuarioID).
			Updates(map[string]interface{}{
				"password_hash":  hashContrasena,
				"actualizado_el": actualizadoEl,
			})
		if actualizacion.Error != nil {
			return actualizacion.Error
		}
		if actualizacion.RowsAffected == 0 {
			return domain.ErrTokenRecuperacionInvalido
		}
		return nil
	})
}
