// Package scraping compone trabajos y entrega los lotes al servicio Catálogo.
package scraping

import (
	"context"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping/internal/routes"
	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"os"
	"time"
)

type Module struct {
	db      *gorm.DB
	handler *handlers.ScraperHandler
	recover func(context.Context)
	queue   *services.QueueService
	close   func() error
}

func New(db *gorm.DB, rdb *redis.Client) (*Module, error) {
	token := os.Getenv("CATALOGO_INTERNAL_TOKEN")
	if token != "" && token == os.Getenv("JWT_SECRET") {
		return nil, errors.New("CATALOGO_INTERNAL_TOKEN debe ser distinto de JWT_SECRET")
	}
	timeout := 10 * time.Second
	if raw := os.Getenv("CATALOGO_INGEST_TIMEOUT"); raw != "" {
		var err error
		timeout, err = time.ParseDuration(raw)
		if err != nil {
			return nil, errors.New("CATALOGO_INGEST_TIMEOUT debe ser una duración válida")
		}
	}
	trabajos := repositories.NewTrabajoRepository(db)
	ingesta, err := repositories.NewCatalogoClient(os.Getenv("CATALOGO_INTERNAL_URL"), token, timeout)
	if err != nil {
		return nil, err
	}
	service := services.NewScraperService(trabajos, ingesta, repositories.NewEntregaRepository(db))
	publisher := repositories.NewJobPublisher(rdb, db)
	queue := services.NewQueueService(repositories.NewEjecucionRepository(db), publisher)
	handler := handlers.NewScraperHandler(service, rdb)
	handler.SetQueue(queue)
	return &Module{db: db, handler: handler, queue: queue, close: publisher.Close, recover: service.(interface{ Recuperar(context.Context) }).Recuperar}, nil
}

func (m *Module) VerifyStorage(ctx context.Context) error {
	for _, table := range []string{"scraper.entregas_ingesta", "scraper.entregas_trabajos"} {
		rows, err := m.db.WithContext(ctx).Raw("SELECT * FROM " + table + " LIMIT 0").Rows()
		if err != nil {
			return errors.New("aplicar 05_recuperacion_scraping.sql y permisos de coordinación antes de iniciar la API")
		}
		rows.Close()
	}
	return nil
}

// Sólo main inicia los loops; construir el router no crea goroutines.
func (m *Module) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				attempt, cancel := context.WithTimeout(ctx, 65*time.Second)
				m.recover(attempt)
				cancel()
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		defer m.close()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				attempt, cancel := context.WithTimeout(ctx, 10*time.Second)
				m.queue.Recuperar(attempt)
				cancel()
			}
		}
	}()
}

func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	routes.RegistrarRutasScraper(group, m.handler)
}
