package repositories

import (
	"context"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping/internal/domain"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

const JobStream = "scraper:jobs:stream"
const JobIndex = "scraper:jobs:index"

var publishJob = redis.NewScript(`
local previous = redis.call('HGET', KEYS[2], ARGV[1])
if previous and #redis.call('XRANGE', KEYS[1], previous, previous) > 0 then return previous end
local id = redis.call('XADD', KEYS[1], '*', 'payload', ARGV[2])
redis.call('HSET', KEYS[2], ARGV[1], id)
return id
`)

type jobPublisher struct {
	rdb *redis.Client
	db  *gorm.DB
}

func NewJobPublisher(rdb *redis.Client, db *gorm.DB) *jobPublisher {
	var client *redis.Client
	if rdb != nil {
		options := *rdb.Options()
		options.MaxRetries = -1
		options.ContextTimeoutEnabled = true
		options.DialTimeout = time.Second
		options.ReadTimeout = time.Second
		options.WriteTimeout = time.Second
		client = redis.NewClient(&options)
	}
	return &jobPublisher{client, db}
}

func (p *jobPublisher) Close() error {
	if p.rdb != nil {
		return p.rdb.Close()
	}
	return nil
}
func (p *jobPublisher) Publicar(ctx context.Context, e domain.Ejecucion) error {
	if p.rdb == nil {
		return errors.New("Redis no está disponible")
	}
	// Serializar publicación y cierre con la misma fila de coordinación.
	// Un publisher que leyó una entrada antigua no la recrea después del ACK.
	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job domain.TrabajoScraper
		found := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND estado='en_progreso'", e.TrabajoID).Limit(1).Find(&job)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected == 0 {
			return nil
		}
		return publishJob.Run(ctx, p.rdb, []string{JobStream, JobIndex}, e.TrabajoID.String(), e.Payload).Err()
	})
}
