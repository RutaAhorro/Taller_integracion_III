// Package scraping compone trabajos y entrega los lotes al servicio Catálogo.
package scraping

import (
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
	handler *handlers.ScraperHandler
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
	service := services.NewScraperService(trabajos, ingesta)
	return &Module{handler: handlers.NewScraperHandler(service, rdb)}, nil
}

func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	routes.RegistrarRutasScraper(group, m.handler)
}
