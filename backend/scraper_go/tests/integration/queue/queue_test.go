package queue_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/queue"
)

type fixture struct {
	client       *redis.Client
	queue        *queue.Queue
	stream       string
	index        string
	dead         string
	closedPrefix string
	workID       string
}

func skipIfRedisUnavailable(t *testing.T, rawURL string, err error) {
	t.Helper()
	if os.Getenv("REDIS_TEST_URL") != "" {
		t.Fatalf("Redis configurado en REDIS_TEST_URL=%s no está disponible: %v", rawURL, err)
	}
	t.Skipf("Redis no disponible en %s: %v", rawURL, err)
}

func newFixture(t *testing.T, consumer string) *fixture {
	t.Helper()
	rawURL := os.Getenv("REDIS_TEST_URL")
	if rawURL == "" {
		rawURL = "redis://localhost:6379/15"
	}
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		t.Fatalf("REDIS_TEST_URL inválida: %v", err)
	}
	options.DialTimeout = 500 * time.Millisecond
	options.ReadTimeout = 2 * time.Second
	options.WriteTimeout = 2 * time.Second
	client := redis.NewClient(options)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		skipIfRedisUnavailable(t, rawURL, err)
	}

	prefix := "scraper:test:" + uuid.NewString()
	f := &fixture{
		client:       client,
		stream:       prefix + ":stream",
		index:        prefix + ":index",
		dead:         prefix + ":dead",
		closedPrefix: prefix + ":closed:",
		workID:       uuid.NewString(),
	}
	f.queue, err = queue.New(client, queue.Options{
		Stream:       f.stream,
		Group:        prefix + ":group",
		Lease:        10 * time.Second,
		Consumer:     consumer,
		IndexKey:     f.index,
		DeadLetter:   f.dead,
		ClosedPrefix: f.closedPrefix,
	})
	if err != nil {
		t.Fatalf("queue.New() error = %v", err)
	}
	if err := f.queue.EnsureGroup(context.Background()); err != nil {
		t.Fatalf("EnsureGroup() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cleanupCancel()
		_ = client.Del(cleanupCtx,
			f.stream,
			f.index,
			f.dead,
			f.closedPrefix+f.workID,
		).Err()
		_ = client.Close()
	})
	return f
}

func (f *fixture) addJob(t *testing.T, payload string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	messageID, err := f.client.XAdd(ctx, &redis.XAddArgs{
		Stream: f.stream,
		Values: map[string]any{"payload": payload},
	}).Result()
	if err != nil {
		t.Fatalf("XAdd() error = %v", err)
	}
	if err := f.client.HSet(ctx, f.index, f.workID, messageID).Err(); err != nil {
		t.Fatalf("HSet() error = %v", err)
	}
	return messageID
}

func TestAckOwnershipAndDeadLetter(t *testing.T) {
	f := newFixture(t, "consumer-a")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	raw := `{"trabajo_id":"` + f.workID + `","spider":"jumbo_rsc"}`
	messageID := f.addJob(t, raw)

	has, err := f.queue.HasMessage(ctx, f.workID)
	if err != nil || !has {
		t.Fatalf("HasMessage() = %v, %v; want true", has, err)
	}
	message, err := f.queue.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if message == nil || message.ID != messageID {
		t.Fatalf("message = %+v, want %s", message, messageID)
	}
	renewed, err := f.queue.Renew(ctx, messageID)
	if err != nil || !renewed {
		t.Fatalf("Renew() = %v, %v; want true", renewed, err)
	}

	other, err := queue.New(f.client, queue.Options{
		Stream:       f.stream,
		Group:        f.queue.Group(),
		Lease:        10 * time.Second,
		Consumer:     "consumer-b",
		IndexKey:     f.index,
		DeadLetter:   f.dead,
		ClosedPrefix: f.closedPrefix,
	})
	if err != nil {
		t.Fatalf("queue.New(other) error = %v", err)
	}
	if ok, _ := other.Renew(ctx, messageID); ok {
		t.Fatal("otro consumidor no debe poder renovar la lease")
	}
	if ok, _ := other.Ack(ctx, messageID, raw, f.workID, ""); ok {
		t.Fatal("otro consumidor no debe poder hacer ACK")
	}

	acked, err := f.queue.Ack(ctx, messageID, raw, f.workID, "attempts_exhausted")
	if err != nil || !acked {
		t.Fatalf("Ack() = %v, %v; want true", acked, err)
	}
	if has, _ := f.queue.HasMessage(ctx, f.workID); has {
		t.Fatal("el índice debe limpiarse después del ACK")
	}
	if exists, _ := f.client.Exists(ctx, f.closedPrefix+f.workID).Result(); exists != 1 {
		t.Errorf("marcador de cierre existe = %d, want 1", exists)
	}
	if length, _ := f.client.XLen(ctx, f.stream).Result(); length != 0 {
		t.Errorf("stream length = %d, want 0", length)
	}

	dead, err := f.client.XRange(ctx, f.dead, "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange(dead) error = %v", err)
	}
	if len(dead) != 1 {
		t.Fatalf("dead letter = %d mensajes, want 1", len(dead))
	}
	if dead[0].Values["reason"] != "attempts_exhausted" {
		t.Errorf("reason = %v", dead[0].Values["reason"])
	}
	if dead[0].Values["source_id"] != messageID {
		t.Errorf("source_id = %v, want %s", dead[0].Values["source_id"], messageID)
	}
	if dead[0].Values["payload"] != raw {
		t.Errorf("payload = %v", dead[0].Values["payload"])
	}
}

func TestAckSuccessDoesNotCreateDeadLetter(t *testing.T) {
	f := newFixture(t, "consumer-a")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	raw := `{"trabajo_id":"` + f.workID + `","spider":"jumbo_rsc"}`
	messageID := f.addJob(t, raw)
	message, err := f.queue.Receive(ctx)
	if err != nil || message == nil {
		t.Fatalf("Receive() = %+v, %v", message, err)
	}
	acked, err := f.queue.Ack(ctx, messageID, raw, f.workID, "")
	if err != nil || !acked {
		t.Fatalf("Ack() = %v, %v; want true", acked, err)
	}
	if exists, _ := f.client.Exists(ctx, f.dead).Result(); exists != 0 {
		t.Errorf("dead letter existe = %d, want 0", exists)
	}
	if exists, _ := f.client.Exists(ctx, f.closedPrefix+f.workID).Result(); exists != 1 {
		t.Errorf("marcador de cierre existe = %d, want 1", exists)
	}
}

func TestRejectsShortLease(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:0"})
	defer client.Close()
	if _, err := queue.New(client, queue.Options{Lease: time.Second}); err == nil {
		t.Fatal("New() debió rechazar una lease menor a 10 s")
	}
}
