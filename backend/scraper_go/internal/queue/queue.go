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
func New(client *redis.Client, opts Options) (*Queue, error) {
	if opts.Lease < 10*time.Second {
		return nil, fmt.Errorf("SCRAPER_LEASE_MS debe ser al menos 10000")
	}
	if opts.Stream == "" {
		opts.Stream = DefaultStream
	}
	if opts.Group == "" {
		opts.Group = DefaultGroup
	}
	if opts.Consumer == "" {
		hostname, _ := os.Hostname()
		opts.Consumer = hostname + "-" + uuid.NewString()
	}
	if opts.IndexKey == "" {
		opts.IndexKey = JobIndex
	}
	if opts.DeadLetter == "" {
		opts.DeadLetter = DefaultDeadLetter
	}
	if opts.ClosedPrefix == "" {
		opts.ClosedPrefix = JobClosedPrefix
	}
	if opts.Block <= 0 {
		opts.Block = time.Second
	}
	if opts.ClosedTTL <= 0 {
		opts.ClosedTTL = 24 * time.Hour
	}
	return &Queue{
		client:       client,
		stream:       opts.Stream,
		group:        opts.Group,
		consumer:     opts.Consumer,
		lease:        opts.Lease,
		block:        opts.Block,
		closedTTL:    opts.ClosedTTL,
		indexKey:     opts.IndexKey,
		deadLetter:   opts.DeadLetter,
		closedPrefix: opts.ClosedPrefix,
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
func (cola *Queue) EnsureGroup(ctx context.Context) error {
	err := cola.client.XGroupCreateMkStream(ctx, cola.stream, cola.group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

// Receive reclama primero una lease vencida; si hay mensajes pendientes pero
// todavía no vencidos, espera para no adelantar trabajo ajeno. Recién después
// toma un mensaje nuevo.
func (cola *Queue) Receive(ctx context.Context) (*redis.XMessage, error) {
	messages, next, err := cola.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   cola.stream,
		Group:    cola.group,
		Consumer: cola.consumer,
		MinIdle:  cola.lease,
		Start:    cola.cursor,
		Count:    1,
	}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	if next != "" {
		cola.cursor = next
	}
	if len(messages) > 0 {
		return &messages[0], nil
	}

	pending, err := cola.client.XPending(ctx, cola.stream, cola.group).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	if pending != nil && pending.Count > 0 {
		return nil, nil
	}

	streams, err := cola.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    cola.group,
		Consumer: cola.consumer,
		Streams:  []string{cola.stream, ">"},
		Count:    1,
		Block:    cola.block,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}
	return &streams[0].Messages[0], nil
}

// Renew extiende la lease sólo si este consumidor sigue siendo el dueño del
// mensaje. Devuelve false cuando otro consumidor ya lo reclamó.
func (cola *Queue) Renew(ctx context.Context, messageID string) (bool, error) {
	result, err := cola.renewScript.Run(ctx, cola.client, []string{cola.stream}, cola.group, messageID, cola.consumer).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// ===== Confirmación y Heartbeat =====

// Ack confirma el mensaje. Si reason no está vacío, primero copia el payload
// al dead-letter; en ambos casos limpia el índice y marca el trabajo cerrado.
func (cola *Queue) Ack(ctx context.Context, messageID, rawJob, workID, reason string) (bool, error) {
	keys := []string{cola.stream, cola.indexKey, cola.deadLetter, cola.closedPrefix + workID}
	result, err := cola.ackScript.Run(
		ctx, cola.client, keys,
		cola.group, messageID, cola.consumer, rawJob, reason, workID,
		int64(cola.closedTTL/time.Second),
	).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// HasMessage indica si el trabajo todavía tiene un mensaje pendiente en el
// stream. El recolector lo usa para no borrar un spool con entrega en curso.
func (cola *Queue) HasMessage(ctx context.Context, workID string) (bool, error) {
	messageID, err := cola.client.HGet(ctx, cola.indexKey, workID).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	messages, err := cola.client.XRange(ctx, cola.stream, messageID, messageID).Result()
	if err != nil {
		return false, err
	}
	return len(messages) > 0, nil
}

// Heartbeat renueva la lease mientras el trabajo está en curso. El canal lost
// se cierra si otro consumidor tomó el mensaje; el worker debe abandonar el
// trabajo sin hacer ACK.
func (cola *Queue) Heartbeat(parent context.Context, messageID string) (
	context.Context,
	context.CancelFunc,
	<-chan struct{},
	error,
) {
	ctx, cancel := context.WithCancel(parent)
	lost := make(chan struct{})
	var once sync.Once
	markLost := func() {
		once.Do(func() { close(lost) })
		cancel()
	}

	ok, err := cola.Renew(ctx, messageID)
	if err != nil {
		cancel()
		return ctx, cancel, lost, err
	}
	if !ok {
		markLost()
		return ctx, cancel, lost, nil
	}

	go func() {
		ticker := time.NewTicker(cola.lease / 3)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ok, err := cola.Renew(ctx, messageID)
				if err != nil || !ok {
					markLost()
					return
				}
			}
		}
	}()

	return ctx, cancel, lost, nil
}

// ===== Utilidades =====

// IsLost informa si el canal de pérdida de lease ya se cerró.
func IsLost(lost <-chan struct{}) bool {
	select {
	case <-lost:
		return true
	default:
		return false
	}
}
