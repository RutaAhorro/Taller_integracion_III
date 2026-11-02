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
if ARGV[6] ~= '' then redis.call('SET', KEYS[4], '1', 'EX', 86400) end
return 1
`

type Options struct {
	Stream       string
	Group        string
	Lease        time.Duration
	Consumer     string
	IndexKey     string
	DeadLetter   string
	ClosedPrefix string
}

type Queue struct {
	client       *redis.Client
	stream       string
	group        string
	consumer     string
	lease        time.Duration
	indexKey     string
	deadLetter   string
	closedPrefix string
	cursor       string
	renewScript  *redis.Script
	ackScript    *redis.Script
}

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
	return &Queue{
		client:       client,
		stream:       opts.Stream,
		group:        opts.Group,
		consumer:     opts.Consumer,
		lease:        opts.Lease,
		indexKey:     opts.IndexKey,
		deadLetter:   opts.DeadLetter,
		closedPrefix: opts.ClosedPrefix,
		cursor:       "0-0",
		renewScript:  redis.NewScript(renewScriptSource),
		ackScript:    redis.NewScript(ackScriptSource),
	}, nil
}

func (q *Queue) Consumer() string {
	return q.consumer
}

func (q *Queue) Stream() string {
	return q.stream
}

func (q *Queue) Group() string {
	return q.group
}

func (q *Queue) EnsureGroup(ctx context.Context) error {
	err := q.client.XGroupCreateMkStream(ctx, q.stream, q.group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

// Receive reclama primero una lease vencida; si hay mensajes pendientes pero
// todavía no vencidos, espera para no adelantar trabajo ajeno. Recién después
// toma un mensaje nuevo.
func (q *Queue) Receive(ctx context.Context) (*redis.XMessage, error) {
	messages, next, err := q.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   q.stream,
		Group:    q.group,
		Consumer: q.consumer,
		MinIdle:  q.lease,
		Start:    q.cursor,
		Count:    1,
	}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	if next != "" {
		q.cursor = next
	}
	if len(messages) > 0 {
		return &messages[0], nil
	}

	pending, err := q.client.XPending(ctx, q.stream, q.group).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	if pending != nil && pending.Count > 0 {
		return nil, nil
	}

	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.group,
		Consumer: q.consumer,
		Streams:  []string{q.stream, ">"},
		Count:    1,
		Block:    time.Second,
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

func (q *Queue) Renew(ctx context.Context, messageID string) (bool, error) {
	result, err := q.renewScript.Run(ctx, q.client, []string{q.stream}, q.group, messageID, q.consumer).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (q *Queue) Ack(ctx context.Context, messageID, rawJob, workID, reason string) (bool, error) {
	keys := []string{q.stream, q.indexKey, q.deadLetter, q.closedPrefix + workID}
	result, err := q.ackScript.Run(
		ctx, q.client, keys,
		q.group, messageID, q.consumer, rawJob, reason, workID,
	).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (q *Queue) HasMessage(ctx context.Context, workID string) (bool, error) {
	messageID, err := q.client.HGet(ctx, q.indexKey, workID).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	messages, err := q.client.XRange(ctx, q.stream, messageID, messageID).Result()
	if err != nil {
		return false, err
	}
	return len(messages) > 0, nil
}

// Heartbeat renueva la lease mientras el trabajo está en curso. El canal lost
// se cierra si otro consumidor tomó el mensaje; el worker debe abandonar el
// trabajo sin hacer ACK.
func (q *Queue) Heartbeat(parent context.Context, messageID string) (
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

	ok, err := q.Renew(ctx, messageID)
	if err != nil {
		cancel()
		return ctx, cancel, lost, err
	}
	if !ok {
		markLost()
		return ctx, cancel, lost, nil
	}

	go func() {
		ticker := time.NewTicker(q.lease / 3)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ok, err := q.Renew(ctx, messageID)
				if err != nil || !ok {
					markLost()
					return
				}
			}
		}
	}()

	return ctx, cancel, lost, nil
}

func IsLost(lost <-chan struct{}) bool {
	select {
	case <-lost:
		return true
	default:
		return false
	}
}
