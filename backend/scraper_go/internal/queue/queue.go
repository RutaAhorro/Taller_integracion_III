// Package queue consume el stream de Redis con lease, heartbeat y ACK
// condicionado al consumidor para garantizar procesamiento confiable.
package queue

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
)

// ===== Constantes del Stream =====

const (
	DefaultStream     = "scraper:jobs:stream"
	DefaultGroup      = "scraper-workers"
	JobIndex          = "scraper:jobs:index"
	JobClosedPrefix   = "scraper:jobs:closed:"
	DefaultDeadLetter = "scraper:jobs:dead"
)

const renewScriptSource = `
local p = redis.call('XPENDING', KEYS[1], ARGV[1], ARGV[2], ARGV[2], 1)
if #p == 0 or p[1][2] ~= ARGV[3] then return 0 end
redis.call('XCLAIM', KEYS[1], ARGV[1], ARGV[3], 0, ARGV[2], 'JUSTID')
return 1
`

const ackScriptSource = `
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
// índice, dead-letter y cierre. New completa los valores vacíos.
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
	client       *redis.Client
	stream       string
	group        string
	consumer     string
	lease        time.Duration
	block        time.Duration
	closedTTL    time.Duration
	indexKey     string
	deadLetter   string
	closedPrefix string
	cursor       string
	renewScript  *redis.Script
	ackScript    *redis.Script
}

// ===== Constructor =====

// New valida la lease mínima de 10 s y completa los nombres y claves que no
// vengan en Options.
func New(cliente *redis.Client, opciones Options) (*Queue, error) {
	if opciones.Lease < 10*time.Second {
		return nil, fmt.Errorf("SCRAPER_LEASE_MS debe ser al menos 10000")
	}
	if opciones.Stream == "" {
		opciones.Stream = DefaultStream
	}
	if opciones.Group == "" {
		opciones.Group = DefaultGroup
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
		opciones.Block = time.Second
	}
	if opciones.ClosedTTL <= 0 {
		opciones.ClosedTTL = 24 * time.Hour
	}
	return &Queue{
		client:       cliente,
		stream:       opciones.Stream,
		group:        opciones.Group,
		consumer:     opciones.Consumer,
		lease:        opciones.Lease,
		block:        opciones.Block,
		closedTTL:    opciones.ClosedTTL,
		indexKey:     opciones.IndexKey,
		deadLetter:   opciones.DeadLetter,
		closedPrefix: opciones.ClosedPrefix,
		cursor:       "0-0",
		renewScript:  redis.NewScript(renewScriptSource),
		ackScript:    redis.NewScript(ackScriptSource),
	}, nil
}

// ===== Accesores =====

// Consumer devuelve el nombre con el que este proceso figura en el grupo.
func (cola *Queue) Consumer() string {
	return cola.consumer
}

// Stream devuelve el nombre del stream de trabajos.
func (cola *Queue) Stream() string {
	return cola.stream
}

// Group devuelve el nombre del grupo de consumo.
func (cola *Queue) Group() string {
	return cola.group
}

// ===== Recepción y Lease =====

// EnsureGroup crea el grupo de consumo si no existe. Es seguro llamarlo en
// cada arranque.
func (cola *Queue) EnsureGroup(contexto context.Context) error {
	errorObtenido := cola.client.XGroupCreateMkStream(contexto, cola.stream, cola.group, "0").Err()
	if errorObtenido != nil && !strings.Contains(errorObtenido.Error(), "BUSYGROUP") {
		return errorObtenido
	}
	return nil
}

// Receive reclama primero una lease vencida; si hay mensajes pendientes pero
// todavía no vencidos, espera para no adelantar trabajo ajeno. Recién después
// toma un mensaje nuevo.
func (cola *Queue) Receive(contexto context.Context) (*redis.XMessage, error) {
	mensajes, siguiente, errorObtenido := cola.client.XAutoClaim(contexto, &redis.XAutoClaimArgs{
		Stream:   cola.stream,
		Group:    cola.group,
		Consumer: cola.consumer,
		MinIdle:  cola.lease,
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

	pendiente, errorObtenido := cola.client.XPending(contexto, cola.stream, cola.group).Result()
	if errorObtenido != nil && !errors.Is(errorObtenido, redis.Nil) {
		return nil, errorObtenido
	}
	if pendiente != nil && pendiente.Count > 0 {
		return nil, nil
	}

	flujos, errorObtenido := cola.client.XReadGroup(contexto, &redis.XReadGroupArgs{
		Group:    cola.group,
		Consumer: cola.consumer,
		Streams:  []string{cola.stream, ">"},
		Count:    1,
		Block:    cola.block,
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
	resultado, errorObtenido := cola.renewScript.Run(contexto, cola.client, []string{cola.stream}, cola.group, mensajeID, cola.consumer).Int64()
	if errorObtenido != nil {
		return false, errorObtenido
	}
	return resultado == 1, nil
}

// ===== Confirmación y Heartbeat =====

// Ack confirma el mensaje. Si reason no está vacío, primero copia el payload
// al dead-letter; en ambos casos limpia el índice y marca el trabajo cerrado.
func (cola *Queue) Ack(contexto context.Context, mensajeID, trabajoCrudo, trabajoID, motivo string) (bool, error) {
	claves := []string{cola.stream, cola.indexKey, cola.deadLetter, cola.closedPrefix + trabajoID}
	resultado, errorObtenido := cola.ackScript.Run(
		contexto, cola.client, claves,
		cola.group, mensajeID, cola.consumer, trabajoCrudo, motivo, trabajoID,
		int64(cola.closedTTL/time.Second),
	).Int64()
	if errorObtenido != nil {
		return false, errorObtenido
	}
	return resultado == 1, nil
}

// HasMessage indica si el trabajo todavía tiene un mensaje pendiente en el
// stream. El recolector lo usa para no borrar un spool con entrega en curso.
func (cola *Queue) HasMessage(contexto context.Context, trabajoID string) (bool, error) {
	mensajeID, errorObtenido := cola.client.HGet(contexto, cola.indexKey, trabajoID).Result()
	if errors.Is(errorObtenido, redis.Nil) {
		return false, nil
	}
	if errorObtenido != nil {
		return false, errorObtenido
	}
	mensajes, errorObtenido := cola.client.XRange(contexto, cola.stream, mensajeID, mensajeID).Result()
	if errorObtenido != nil {
		return false, errorObtenido
	}
	return len(mensajes) > 0, nil
}

// Heartbeat renueva la lease mientras el trabajo está en curso. El canal lost
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
		temporizador := time.NewTicker(cola.lease / 3)
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
