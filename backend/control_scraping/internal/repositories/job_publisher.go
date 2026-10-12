package repositories

import (
	"context"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"time"
)

const JobStream = "scraper:jobs:stream"
const JobIndex = "scraper:jobs:index"
const JobClosedPrefix = "scraper:jobs:closed:"

var publishJob = redis.NewScript(`
if redis.call('EXISTS', KEYS[3]) == 1 then return false end
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
	// Leer SQL sin transacción ni FOR UPDATE; nunca retener locks durante Redis.
	// El ACK instala una marca de cierre en Redis atómicamente. Esa marca evita
	// republicar si el trabajo termina entre esta lectura y la publicación.
	attempt, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var job domain.TrabajoScraper
	found := p.db.WithContext(attempt).Select("id").Where("id=? AND estado='en_progreso'", e.TrabajoID).Limit(1).Find(&job)
	if found.Error != nil {
		return found.Error
	}
	if found.RowsAffected == 0 {
		return nil
	}
	err := publishJob.Run(attempt, p.rdb, []string{JobStream, JobIndex, JobClosedPrefix + e.TrabajoID.String()}, e.TrabajoID.String(), e.Payload).Err()
	if errors.Is(err, redis.Nil) { // trabajo ya cerrado por el worker
		return nil
	}
	return err
}
