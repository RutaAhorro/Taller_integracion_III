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

func skipIfRedisUnavailable(prueba *testing.T, rawURL string, err error) {
	prueba.Helper()
	if os.Getenv("REDIS_TEST_URL") != "" {
		prueba.Fatalf("Redis configurado en REDIS_TEST_URL=%s no está disponible: %v", rawURL, err)
	}
	prueba.Skipf("Redis no disponible en %s: %v", rawURL, err)
}

func newFixture(prueba *testing.T, consumer string) *fixture {
	prueba.Helper()
	rawURL := os.Getenv("REDIS_TEST_URL")
	if rawURL == "" {
		rawURL = "redis://localhost:6379/15"
	}
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		prueba.Fatalf("REDIS_TEST_URL inválida: %v", err)
	}
	options.DialTimeout = 500 * time.Millisecond
	options.ReadTimeout = 2 * time.Second
	options.WriteTimeout = 2 * time.Second
	client := redis.NewClient(options)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		skipIfRedisUnavailable(prueba, rawURL, err)
	}

	prefix := "scraper:test:" + uuid.NewString()
	accesorio := &fixture{
		client:       client,
		stream:       prefix + ":stream",
		index:        prefix + ":index",
		dead:         prefix + ":dead",
		closedPrefix: prefix + ":closed:",
		workID:       uuid.NewString(),
	}
	accesorio.queue, err = queue.New(client, queue.Options{
		Stream:       accesorio.stream,
		Group:        prefix + ":group",
		Lease:        10 * time.Second,
		Consumer:     consumer,
		IndexKey:     accesorio.index,
		DeadLetter:   accesorio.dead,
		ClosedPrefix: accesorio.closedPrefix,
	})
	if err != nil {
		prueba.Fatalf("queue.New() error = %v", err)
	}
	if err := accesorio.queue.EnsureGroup(context.Background()); err != nil {
		prueba.Fatalf("EnsureGroup() error = %v", err)
	}
	prueba.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cleanupCancel()
		_ = client.Del(cleanupCtx,
			accesorio.stream,
			accesorio.index,
			accesorio.dead,
			accesorio.closedPrefix+accesorio.workID,
		).Err()
		_ = client.Close()
	})
	return accesorio
}

func (accesorio *fixture) addJob(prueba *testing.T, payload string) string {
	prueba.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	messageID, err := accesorio.client.XAdd(ctx, &redis.XAddArgs{
		Stream: accesorio.stream,
		Values: map[string]any{"payload": payload},
	}).Result()
	if err != nil {
		prueba.Fatalf("XAdd() error = %v", err)
	}
	if err := accesorio.client.HSet(ctx, accesorio.index, accesorio.workID, messageID).Err(); err != nil {
		prueba.Fatalf("HSet() error = %v", err)
	}
	return messageID
}

func TestAckOwnershipAndDeadLetter(prueba *testing.T) {
	accesorio := newFixture(prueba, "consumer-a")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	raw := `{"trabajo_id":"` + accesorio.workID + `","spider":"jumbo_rsc"}`
	messageID := accesorio.addJob(prueba, raw)

	has, err := accesorio.queue.HasMessage(ctx, accesorio.workID)
	if err != nil || !has {
		prueba.Fatalf("HasMessage() = %v, %v; want true", has, err)
	}
	message, err := accesorio.queue.Receive(ctx)
	if err != nil {
		prueba.Fatalf("Receive() error = %v", err)
	}
	if message == nil || message.ID != messageID {
		prueba.Fatalf("message = %+v, want %s", message, messageID)
	}
	renewed, err := accesorio.queue.Renew(ctx, messageID)
	if err != nil || !renewed {
		prueba.Fatalf("Renew() = %v, %v; want true", renewed, err)
	}

	other, err := queue.New(accesorio.client, queue.Options{
		Stream:       accesorio.stream,
		Group:        accesorio.queue.Group(),
		Lease:        10 * time.Second,
		Consumer:     "consumer-b",
		IndexKey:     accesorio.index,
		DeadLetter:   accesorio.dead,
		ClosedPrefix: accesorio.closedPrefix,
	})
	if err != nil {
		prueba.Fatalf("queue.New(other) error = %v", err)
	}
	if ok, _ := other.Renew(ctx, messageID); ok {
		prueba.Fatal("otro consumidor no debe poder renovar la lease")
	}
	if ok, _ := other.Ack(ctx, messageID, raw, accesorio.workID, ""); ok {
		prueba.Fatal("otro consumidor no debe poder hacer ACK")
	}

	acked, err := accesorio.queue.Ack(ctx, messageID, raw, accesorio.workID, "attempts_exhausted")
	if err != nil || !acked {
		prueba.Fatalf("Ack() = %v, %v; want true", acked, err)
	}
	if has, _ := accesorio.queue.HasMessage(ctx, accesorio.workID); has {
		prueba.Fatal("el índice debe limpiarse después del ACK")
	}
	if exists, _ := accesorio.client.Exists(ctx, accesorio.closedPrefix+accesorio.workID).Result(); exists != 1 {
		prueba.Errorf("marcador de cierre existe = %d, want 1", exists)
	}
	if length, _ := accesorio.client.XLen(ctx, accesorio.stream).Result(); length != 0 {
		prueba.Errorf("stream length = %d, want 0", length)
	}

	dead, err := accesorio.client.XRange(ctx, accesorio.dead, "-", "+").Result()
	if err != nil {
		prueba.Fatalf("XRange(dead) error = %v", err)
	}
	if len(dead) != 1 {
		prueba.Fatalf("dead letter = %d mensajes, want 1", len(dead))
	}
	if dead[0].Values["reason"] != "attempts_exhausted" {
		prueba.Errorf("reason = %v", dead[0].Values["reason"])
	}
	if dead[0].Values["source_id"] != messageID {
		prueba.Errorf("source_id = %v, want %s", dead[0].Values["source_id"], messageID)
	}
	if dead[0].Values["payload"] != raw {
		prueba.Errorf("payload = %v", dead[0].Values["payload"])
	}
}

func TestAckSuccessDoesNotCreateDeadLetter(prueba *testing.T) {
	accesorio := newFixture(prueba, "consumer-a")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	raw := `{"trabajo_id":"` + accesorio.workID + `","spider":"jumbo_rsc"}`
	messageID := accesorio.addJob(prueba, raw)
	message, err := accesorio.queue.Receive(ctx)
	if err != nil || message == nil {
		prueba.Fatalf("Receive() = %+v, %v", message, err)
	}
	acked, err := accesorio.queue.Ack(ctx, messageID, raw, accesorio.workID, "")
	if err != nil || !acked {
		prueba.Fatalf("Ack() = %v, %v; want true", acked, err)
	}
	if exists, _ := accesorio.client.Exists(ctx, accesorio.dead).Result(); exists != 0 {
		prueba.Errorf("dead letter existe = %d, want 0", exists)
	}
	if exists, _ := accesorio.client.Exists(ctx, accesorio.closedPrefix+accesorio.workID).Result(); exists != 1 {
		prueba.Errorf("marcador de cierre existe = %d, want 1", exists)
	}
}

func TestRejectsShortLease(prueba *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:0"})
	defer client.Close()
	if _, err := queue.New(client, queue.Options{Lease: time.Second}); err == nil {
		prueba.Fatal("New() debió rechazar una lease menor a 10 s")
	}
}
