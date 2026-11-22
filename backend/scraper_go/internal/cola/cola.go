// Package cola gestiona el consumo del stream de Redis con lease, el spool
// local en SQLite, la retención de directorios y el cliente HTTP hacia
// Control de Scraping.
package cola

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
)

// ===== Constantes del Stream =====

const (
	// JobIndex identifica el hash de Redis que indexa trabajo_id -> mensaje_id.
	JobIndex = "scraper:jobs:index"
	// JobClosedPrefix define el prefijo de las claves TTL de trabajos finalizados.
	JobClosedPrefix = "scraper:jobs:closed:"
	// DefaultDeadLetter define el stream de mensajes descartados o rechazados.
	DefaultDeadLetter = "scraper:jobs:dead"
)

const scriptRenovacion = `
local p = redis.call('XPENDING', KEYS[1], ARGV[1], ARGV[2], ARGV[2], 1)
if #p == 0 or p[1][2] ~= ARGV[3] then return 0 end
redis.call('XCLAIM', KEYS[1], ARGV[1], ARGV[3], 0, ARGV[2], 'JUSTID')
return 1
`

const scriptConfirmacion = `
local p = redis.call('XPENDING', KEYS[1], ARGV[1], ARGV[2], ARGV[2], 1)
if #p == 0 or p[1][2] ~= ARGV[3] then return 0 end
if ARGV[5] ~= '' then
  redis.call('XADD', KEYS[3], '*', 'payload', ARGV[4], 'reason', ARGV[5], 'source_id', ARGV[2])
end
redis.call('XACK', KEYS[1], ARGV[1], ARGV[2])
redis.call('XDEL', KEYS[1], ARGV[2])
if redis.call('HGET', KEYS[2], ARGV[6]) == ARGV[2] then redis.call('HDEL', KEYS[2], ARGV[6]) end
if ARGV[6] ~= '' then redis.call('SET', KEYS[4], '1', 'EX', ARGV[7]) end
return 1
`

// ===== Opciones =====

// Options configura el stream, el grupo, la lease y las claves auxiliares de
// índice, dead-letter y cierre. NewQueue completa los valores vacíos.
type Options struct {
	Stream       string
	Group        string
	Lease        time.Duration
	Block        time.Duration
	ClosedTTL    time.Duration
	Consumer     string
	IndexKey     string
	DeadLetter   string
	ClosedPrefix string
}

// ===== Estructura de Cola =====

// Queue consume el stream con lease exclusiva por consumidor: sólo el dueño
// puede renovar o confirmar un mensaje.
type Queue struct {
	cliente       *redis.Client
	flujo         string
	grupo         string
	consumidor    string
	arriendo      time.Duration
	bloqueo       time.Duration
	vidaCierre    time.Duration
	claveIndice   string
	colaMuerta    string
	prefijoCierre string
	cursor        string
	scriptRenovar *redis.Script
	scriptAck     *redis.Script
}

// ===== Constructor =====

// NewQueue valida la lease mínima de 10 s y completa los nombres y claves que no
// vengan en Options.
func NewQueue(cliente *redis.Client, opciones Options) (*Queue, error) {
	configuracionPredeterminada := config.Default()
	if opciones.Lease < config.MinLeaseDuration {
		return nil, fmt.Errorf("SCRAPER_LEASE_MS debe ser al menos %d", config.MinLeaseDuration.Milliseconds())
	}
	if opciones.Stream == "" {
		opciones.Stream = configuracionPredeterminada.QueueName
	}
	if opciones.Group == "" {
		opciones.Group = configuracionPredeterminada.Group
	}
	if opciones.Consumer == "" {
		nombreServidor, _ := os.Hostname()
		opciones.Consumer = nombreServidor + "-" + uuid.NewString()
	}
	if opciones.IndexKey == "" {
		opciones.IndexKey = JobIndex
	}
	if opciones.DeadLetter == "" {
		opciones.DeadLetter = DefaultDeadLetter
	}
	if opciones.ClosedPrefix == "" {
		opciones.ClosedPrefix = JobClosedPrefix
	}
	if opciones.Block <= 0 {
		opciones.Block = configuracionPredeterminada.QueueBlock
	}
	if opciones.ClosedTTL <= 0 {
		opciones.ClosedTTL = configuracionPredeterminada.QueueClosedTTL
	}
	return &Queue{
		cliente:       cliente,
		flujo:         opciones.Stream,
		grupo:         opciones.Group,
		consumidor:    opciones.Consumer,
		arriendo:      opciones.Lease,
		bloqueo:       opciones.Block,
		vidaCierre:    opciones.ClosedTTL,
		claveIndice:   opciones.IndexKey,
		colaMuerta:    opciones.DeadLetter,
		prefijoCierre: opciones.ClosedPrefix,
		cursor:        "0-0",
		scriptRenovar: redis.NewScript(scriptRenovacion),
		scriptAck:     redis.NewScript(scriptConfirmacion),
	}, nil
}

// ===== Accesores =====

// Stream devuelve el nombre del stream de trabajos.
func (cola *Queue) Stream() string {
	return cola.flujo
}

// Group devuelve el consumer group usado por el worker.
func (cola *Queue) Group() string {
	return cola.grupo
}

// Consumer devuelve el identificador único de este consumidor.
func (cola *Queue) Consumer() string {
	return cola.consumidor
}

// ===== Operaciones del Stream =====

// EnsureGroup crea el stream y el consumer group si todavía no existen.
// Ignora BUSYGROUP para que varios workers puedan llamarlo de forma segura en
// cada arranque.
func (cola *Queue) EnsureGroup(contexto context.Context) error {
	errorObtenido := cola.cliente.XGroupCreateMkStream(contexto, cola.flujo, cola.grupo, "0").Err()
	if errorObtenido != nil && !strings.Contains(errorObtenido.Error(), "BUSYGROUP") {
		return errorObtenido
	}
	return nil
}

// Receive reclama primero una lease vencida; si hay mensajes pendientes pero
// todavía no vencidos, espera para no adelantar trabajo ajeno. Recién después
// toma un mensaje nuevo.
func (cola *Queue) Receive(contexto context.Context) (*redis.XMessage, error) {
	mensajes, siguiente, errorObtenido := cola.cliente.XAutoClaim(contexto, &redis.XAutoClaimArgs{
		Stream:   cola.flujo,
		Group:    cola.grupo,
		Consumer: cola.consumidor,
		MinIdle:  cola.arriendo,
		Start:    cola.cursor,
		Count:    1,
	}).Result()
	if errorObtenido != nil && !errors.Is(errorObtenido, redis.Nil) {
		return nil, errorObtenido
	}
	if siguiente != "" {
		cola.cursor = siguiente
	}
	if len(mensajes) > 0 {
		return &mensajes[0], nil
	}

	pendiente, errorObtenido := cola.cliente.XPending(contexto, cola.flujo, cola.grupo).Result()
	if errorObtenido != nil && !errors.Is(errorObtenido, redis.Nil) {
		return nil, errorObtenido
	}
	if pendiente != nil && pendiente.Count > 0 {
		return nil, nil
	}

	flujos, errorObtenido := cola.cliente.XReadGroup(contexto, &redis.XReadGroupArgs{
		Group:    cola.grupo,
		Consumer: cola.consumidor,
		Streams:  []string{cola.flujo, ">"},
		Count:    1,
		Block:    cola.bloqueo,
	}).Result()
	if errors.Is(errorObtenido, redis.Nil) {
		return nil, nil
	}
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if len(flujos) == 0 || len(flujos[0].Messages) == 0 {
		return nil, nil
	}
	return &flujos[0].Messages[0], nil
}

// Renew extiende la lease sólo si este consumidor sigue siendo el dueño del
// mensaje. Devuelve false cuando otro consumidor ya lo reclamó.
func (cola *Queue) Renew(contexto context.Context, mensajeID string) (bool, error) {
	resultado, errorObtenido := cola.scriptRenovar.Run(contexto, cola.cliente, []string{cola.flujo}, cola.grupo, mensajeID, cola.consumidor).Int64()
	if errorObtenido != nil {
		return false, errorObtenido
	}
	return resultado == 1, nil
}

// ===== Confirmación y Heartbeat =====

// Ack confirma el mensaje. Si motivo no está vacío, primero copia el payload
// al dead-letter; en ambos casos limpia el índice y marca el trabajo cerrado.
func (cola *Queue) Ack(contexto context.Context, mensajeID, trabajoCrudo, trabajoID, motivo string) (bool, error) {
	claves := []string{cola.flujo, cola.claveIndice, cola.colaMuerta, cola.prefijoCierre + trabajoID}
	resultado, errorObtenido := cola.scriptAck.Run(
		contexto, cola.cliente, claves,
		cola.grupo, mensajeID, cola.consumidor, trabajoCrudo, motivo, trabajoID,
		int64(cola.vidaCierre/time.Second),
	).Int64()
	if errorObtenido != nil {
		return false, errorObtenido
	}
	return resultado == 1, nil
}

// HasMessage indica si el trabajo todavía tiene un mensaje pendiente en el
// stream. El recolector lo usa para no borrar un spool con entrega en curso.
func (cola *Queue) HasMessage(contexto context.Context, trabajoID string) (bool, error) {
	mensajeID, errorObtenido := cola.cliente.HGet(contexto, cola.claveIndice, trabajoID).Result()
	if errors.Is(errorObtenido, redis.Nil) {
		return false, nil
	}
	if errorObtenido != nil {
		return false, errorObtenido
	}
	mensajes, errorObtenido := cola.cliente.XRange(contexto, cola.flujo, mensajeID, mensajeID).Result()
	if errorObtenido != nil {
		return false, errorObtenido
	}
	return len(mensajes) > 0, nil
}

// Heartbeat renueva la lease mientras el trabajo está en curso. El canal perdido
// se cierra si otro consumidor tomó el mensaje; el worker debe abandonar el
// trabajo sin hacer ACK.
func (cola *Queue) Heartbeat(padre context.Context, mensajeID string) (
	context.Context,
	context.CancelFunc,
	<-chan struct{},
	error,
) {
	contexto, cancelar := context.WithCancel(padre)
	perdido := make(chan struct{})
	var unaVez sync.Once
	marcarPerdido := func() {
		unaVez.Do(func() { close(perdido) })
		cancelar()
	}

	valido, errorObtenido := cola.Renew(contexto, mensajeID)
	if errorObtenido != nil {
		cancelar()
		return contexto, cancelar, perdido, errorObtenido
	}
	if !valido {
		marcarPerdido()
		return contexto, cancelar, perdido, nil
	}

	go func() {
		temporizador := time.NewTicker(cola.arriendo / 3)
		defer temporizador.Stop()
		for {
			select {
			case <-contexto.Done():
				return
			case <-temporizador.C:
				valido, errorObtenido := cola.Renew(contexto, mensajeID)
				if errorObtenido != nil || !valido {
					marcarPerdido()
					return
				}
			}
		}
	}()

	return contexto, cancelar, perdido, nil
}

// ===== Utilidades =====

// IsLost informa si el canal de pérdida de lease ya se cerró.
func IsLost(perdido <-chan struct{}) bool {
	select {
	case <-perdido:
		return true
	default:
		return false
	}
}
