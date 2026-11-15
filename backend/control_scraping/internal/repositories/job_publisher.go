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

var publicarTrabajo = redis.NewScript(`
if redis.call('EXISTS', KEYS[3]) == 1 then return false end
local previous = redis.call('HGET', KEYS[2], ARGV[1])
if previous and #redis.call('XRANGE', KEYS[1], previous, previous) > 0 then return previous end
local id = redis.call('XADD', KEYS[1], '*', 'payload', ARGV[2])
redis.call('HSET', KEYS[2], ARGV[1], id)
return id
`)

type jobPublisher struct {
	clienteRedis *redis.Client
	baseDatos    *gorm.DB
}

func NewJobPublisher(clienteRedis *redis.Client, baseDatos *gorm.DB) *jobPublisher {
	var cliente *redis.Client
	if clienteRedis != nil {
		opciones := *clienteRedis.Options()
		opciones.MaxRetries = -1
		opciones.ContextTimeoutEnabled = true
		opciones.DialTimeout = time.Second
		opciones.ReadTimeout = time.Second
		opciones.WriteTimeout = time.Second
		cliente = redis.NewClient(&opciones)
	}
	return &jobPublisher{clienteRedis: cliente, baseDatos: baseDatos}
}

func (publicador *jobPublisher) Close() error {
	if publicador.clienteRedis != nil {
		return publicador.clienteRedis.Close()
	}
	return nil
}
func (publicador *jobPublisher) Publicar(contexto context.Context, ejecucion domain.Ejecucion) error {
	if publicador.clienteRedis == nil {
		return errors.New("Redis no está disponible")
	}
	// Leer SQL sin transacción ni FOR UPDATE; nunca retener locks durante Redis.
	// El ACK instala una marca de cierre en Redis atómicamente. Esa marca evita
	// republicar si el trabajo termina entre esta lectura y la publicación.
	intento, cancelar := context.WithTimeout(contexto, 3*time.Second)
	defer cancelar()
	var trabajo domain.TrabajoScraper
	encontrado := publicador.baseDatos.WithContext(intento).Select("id").Where("id=? AND estado='en_progreso'", ejecucion.TrabajoID).Limit(1).Find(&trabajo)
	if encontrado.Error != nil {
		return encontrado.Error
	}
	if encontrado.RowsAffected == 0 {
		return nil
	}
	errorObtenido := publicarTrabajo.Run(intento, publicador.clienteRedis, []string{JobStream, JobIndex, JobClosedPrefix + ejecucion.TrabajoID.String()}, ejecucion.TrabajoID.String(), ejecucion.Payload).Err()
	if errors.Is(errorObtenido, redis.Nil) { // trabajo ya cerrado por el worker
		return nil
	}
	return errorObtenido
}
