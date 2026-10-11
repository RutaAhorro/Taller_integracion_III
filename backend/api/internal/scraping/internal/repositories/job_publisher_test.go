package repositories

import (
	"context"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping/internal/domain"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// El harness ejecuta este escenario con PostgreSQL real y un Redis detenido
// durante su handshake. Con el FOR UPDATE anterior el cierre vence su timeout.
func TestPublisherStalledRedisAllowsJobFinalization(t *testing.T) {
	url := os.Getenv("SUP270_PUBLISHER_TEST_DB_URL")
	if url == "" {
		t.Skip("ejecutado por smoke_scraping_recovery_compose.sh con DB efímera")
	}
	db, err := gorm.Open(postgres.Open(url), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("no se pudo conectar a PostgreSQL de prueba")
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	job := domain.TrabajoScraper{ID: uuid.New(), CadenaID: 1, Estado: "en_progreso", IniciadoEl: time.Now()}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		if _, err = io.ReadFull(connection, make([]byte, 1)); err != nil {
			return
		}
		close(started)
		<-release
	}()
	client := redis.NewClient(&redis.Options{Addr: listener.Addr().String()})
	defer client.Close()
	publisher := NewJobPublisher(client, db)
	defer publisher.Close()
	finished := make(chan error, 1)
	go func() {
		finished <- publisher.Publicar(context.Background(), domain.Ejecucion{TrabajoID: job.ID, Payload: `{}`})
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("publisher no llegó al I/O Redis")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err = NewTrabajoRepository(db).FinalizarTrabajo(ctx, job.ID, "fallido", nil, nil)
	if err != nil {
		t.Fatalf("Redis lento bloqueó el cierre SQL: %v", err)
	}
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("Redis detenido debería fallar")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("publisher no respetó su timeout")
	}
}
