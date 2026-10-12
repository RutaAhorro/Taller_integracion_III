// Package control compone trabajos y entrega los lotes al servicio Catálogo.
package control

import (
	"context"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/routes"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"os"
	"sync"
	"time"
)

type Module struct {
	db      *gorm.DB
	handler *handlers.ScraperHandler
	recover func(context.Context)
	queue   *services.QueueService
	close   func() error
	wg      sync.WaitGroup
}

func New(db *gorm.DB, rdb *redis.Client) (*Module, error) {
	token := os.Getenv("CATALOGO_INTERNAL_TOKEN")
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
	rows, err := m.db.WithContext(ctx).Raw("SELECT cadena_nombre FROM scraper.trabajos_scraper LIMIT 0").Rows()
	if err != nil {
		return errors.New("aplicar 06_propiedad_permisos.sql y permisos de coordinación antes de iniciar la API")
	}
	rows.Close()
	for _, table := range []string{"scraper.trabajos_scraper", "scraper.entregas_ingesta", "scraper.entregas_trabajos"} {
		rows, err := m.db.WithContext(ctx).Raw("SELECT * FROM " + table + " LIMIT 0").Rows()
		if err != nil {
			return errors.New("aplicar 05_recuperacion_scraping.sql y permisos de coordinación antes de iniciar la API")
		}
		rows.Close()
		var permitted bool
		if err := m.db.WithContext(ctx).Raw(`SELECT
            has_table_privilege(current_user, ?, 'SELECT') AND
            has_table_privilege(current_user, ?, 'INSERT') AND
            has_table_privilege(current_user, ?, 'UPDATE')`, table, table, table).Scan(&permitted).Error; err != nil || !permitted {
			return errors.New("faltan permisos runtime de coordinación; revisar 06_propiedad_permisos.sql")
		}
	}
	var canDelete bool
	if err := m.db.WithContext(ctx).Raw("SELECT has_table_privilege(current_user, 'scraper.entregas_trabajos', 'DELETE')").Scan(&canDelete).Error; err != nil || !canDelete {
		return errors.New("conceder DELETE en scraper.entregas_trabajos al rol de coordinación antes de iniciar la API")
	}
	return nil
}

// Sólo main inicia los loops; construir el router no crea goroutines.
func (m *Module) Start(ctx context.Context) {
	m.wg.Add(2)
	go func() {
		defer m.wg.Done()
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
		defer m.wg.Done()
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

// Wait espera que terminen los dos loops y cierre el cliente del publisher.
func (m *Module) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() { m.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	routes.RegistrarRutasScraper(group, m.handler)
}
