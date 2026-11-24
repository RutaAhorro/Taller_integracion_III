// Direcciones guardadas del usuario (api.direcciones_usuario): entidad, DTO del
// contrato HTTP y contrato de persistencia. Identidad es su dueño; el motor de
// rutas sólo recibirá sus coordenadas por HTTP, nunca por SQL.

package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ===== Límites del contrato =====

// Límites de campos y de cantidad. Los máximos de texto coinciden con los
// VARCHAR de la tabla; el de cantidad evita que una cuenta acumule filas sin tope.
const (
	MaximoDireccionesPorUsuario    = 10
	MaximoCaracteresTextoDireccion = 255
	MaximoCaracteresEtiqueta       = 100
	MaximoCaracteresComuna         = 100

	// Rango geográfico válido; el mismo que imponen los CHECK de la migración 11.
	LatitudMinima  = -90.0
	LatitudMaxima  = 90.0
	LongitudMinima = -180.0
	LongitudMaxima = 180.0

	// DecimalesCoordenadas son los decimales que guarda DECIMAL(10,8)/(11,8).
	// El servicio redondea a esta precisión para que la respuesta coincida con
	// lo persistido (unos 1,1 mm: irrelevante para un origen de ruta).
	DecimalesCoordenadas = 8
)

// ===== Errores =====

var (
	// ErrDireccionNoEncontrada cubre tanto la dirección inexistente como la
	// ajena: nunca se distingue, para no revelar que existe en otra cuenta.
	ErrDireccionNoEncontrada = errors.New("dirección no encontrada")
	// ErrLimiteDirecciones se emite al intentar superar MaximoDireccionesPorUsuario.
	ErrLimiteDirecciones = errors.New("se alcanzó el máximo de direcciones guardadas por usuario")
	// ErrPrincipalObligatoria impide dejar al usuario sin principal desmarcándola
	// directamente: se cambia marcando otra como principal.
	ErrPrincipalObligatoria = errors.New("la dirección principal no se puede desmarcar; marcar otra como principal")
	// ErrConflictoPrincipal se emite si el índice único parcial detecta dos
	// principales simultáneas. El bloqueo por usuario lo evita; es la red de seguridad.
	ErrConflictoPrincipal = errors.New("otra operación cambió la dirección principal; reintentar")
)

// ===== Entidad =====

// DireccionUsuario es una fila de api.direcciones_usuario. Lat y Lon son
// punteros porque son opcionales y deben distinguirse de la coordenada 0.
type DireccionUsuario struct {
	ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UsuarioID      uuid.UUID `gorm:"type:uuid;not null"`
	Etiqueta       *string   `gorm:"type:varchar(100)"`
	TextoDireccion string    `gorm:"type:varchar(255);not null"`
	// Comuna es nullable en la base por las filas previas a la migración 11;
	// la API la exige siempre, así que una fila nueva nunca queda sin ella.
	Comuna      string    `gorm:"type:varchar(100)"`
	Lat         *float64  `gorm:"type:decimal(10,8)"`
	Lon         *float64  `gorm:"type:decimal(11,8)"`
	EsPrincipal bool      `gorm:"not null"`
	CreadoEl    time.Time `gorm:"default:now();not null"`
}

// TableName fija la tabla con esquema, porque GORM no lo infiere.
func (DireccionUsuario) TableName() string {
	return "api.direcciones_usuario"
}

// ===== DTO HTTP =====

// EntradaDireccionDTO es el cuerpo de POST y PUT. El usuario nunca viaja aquí:
// sale de la sesión. EsPrincipal es opcional; las coordenadas, ambas o ninguna.
type EntradaDireccionDTO struct {
	Etiqueta       *string  `json:"etiqueta"`
	TextoDireccion string   `json:"texto_direccion"`
	Comuna         string   `json:"comuna"`
	Lat            *float64 `json:"lat"`
	Lon            *float64 `json:"lon"`
	EsPrincipal    *bool    `json:"es_principal"`
}

// DireccionDTO es la representación pública de una dirección. Etiqueta, lat y
// lon se emiten como null cuando faltan, para que el cliente no deba inferirlo.
type DireccionDTO struct {
	ID             string    `json:"id"`
	Etiqueta       *string   `json:"etiqueta"`
	TextoDireccion string    `json:"texto_direccion"`
	Comuna         string    `json:"comuna"`
	Lat            *float64  `json:"lat"`
	Lon            *float64  `json:"lon"`
	EsPrincipal    bool      `json:"es_principal"`
	CreadoEl       time.Time `json:"creado_el"`
}

// ListaDireccionesDTO es la respuesta del listado.
type ListaDireccionesDTO struct {
	Data []DireccionDTO `json:"data"`
}

// ===== Contrato de repositorio =====

// RepositorioDirecciones define la persistencia de direcciones. Toda operación
// recibe el usuario dueño y filtra por él: una dirección ajena se comporta
// exactamente como una inexistente (ErrDireccionNoEncontrada).
type RepositorioDirecciones interface {
	// Listar devuelve las direcciones del usuario: principal primero, luego
	// de la más antigua a la más reciente (desempate por id).
	Listar(contexto context.Context, usuarioID string) ([]DireccionUsuario, error)
	// Crear inserta la dirección en una transacción. Si es la primera del usuario
	// queda principal; si viene marcada, desmarca la anterior. Devuelve
	// ErrLimiteDirecciones al superar el máximo.
	Crear(contexto context.Context, usuarioID string, direccion *DireccionUsuario, limite int) error
	// Actualizar reemplaza los datos editables de una dirección. esPrincipal nil
	// conserva el estado; true la promueve; false sobre la principal devuelve
	// ErrPrincipalObligatoria.
	Actualizar(contexto context.Context, usuarioID, direccionID string, datos DireccionUsuario, esPrincipal *bool) (*DireccionUsuario, error)
	// Eliminar borra la dirección. Si era la principal, promueve a la más
	// antigua de las restantes en la misma transacción.
	Eliminar(contexto context.Context, usuarioID, direccionID string) error
}
