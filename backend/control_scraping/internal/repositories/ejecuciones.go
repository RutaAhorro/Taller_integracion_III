// Ejecuciones durables: outbox SQL en scraper.entregas_trabajos y su
// publicación recuperable en Redis Streams, que consume Scraper Go.
package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ===== Contrato de Redis Streams =====

// Claves compartidas con Scraper Go; cambiarlas rompe el worker.
const (
	streamTrabajos        = "scraper:jobs:stream"
	indiceTrabajos        = "scraper:jobs:index"
	prefijoTrabajoCerrado = "scraper:jobs:closed:"
)

// publicarTrabajo agrega la ejecución al stream una sola vez. Devuelve false si
// el worker ya cerró el trabajo (marca de cierre) y reutiliza la entrada
// anterior si todavía existe en el stream. El campo "payload" es el contrato.
var publicarTrabajo = redis.NewScript(`
if redis.call('EXISTS', KEYS[3]) == 1 then return false end
local anterior = redis.call('HGET', KEYS[2], ARGV[1])
if anterior and #redis.call('XRANGE', KEYS[1], anterior, anterior) > 0 then return anterior end
local identificador = redis.call('XADD', KEYS[1], '*', 'payload', ARGV[2])
redis.call('HSET', KEYS[2], ARGV[1], identificador)
return identificador
`)

// ===== Límites del outbox =====

const (
	// limiteRetiroOutbox acota cada limpieza de outboxes de trabajos terminados.
	limiteRetiroOutbox = 100
	// limitePendientesOutbox acota cuántas ejecuciones se publican por ciclo.
	limitePendientesOutbox = 20
	// segundosRepublicacion espera antes de volver a publicar la misma ejecución.
	segundosRepublicacion = 30
	// plazoPublicacion cubre la lectura SQL y el comando Redis de una ejecución.
	plazoPublicacion = 3 * time.Second
)

// ===== Outbox SQL =====

// repositorioEjecuciones implementa domain.EjecucionRepository con PostgreSQL.
type repositorioEjecuciones struct{ baseDatos *gorm.DB }

// NuevoRepositorioEjecuciones crea el outbox SQL de ejecuciones.
func NuevoRepositorioEjecuciones(baseDatos *gorm.DB) domain.EjecucionRepository {
	return &repositorioEjecuciones{baseDatos: baseDatos}
}

// RegistrarEjecucion guarda de forma idempotente el payload de un trabajo en
// progreso. Otro payload para el mismo trabajo es un conflicto de idempotencia.
func (repositorio *repositorioEjecuciones) RegistrarEjecucion(contexto context.Context, ejecucion domain.Ejecucion) error {
	return repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		var trabajo domain.TrabajoScraper
		if errorObtenido := transaccion.Clauses(clause.Locking{Strength: "UPDATE"}).First(&trabajo, "id=?", ejecucion.TrabajoID).Error; errorObtenido != nil {
			if errors.Is(errorObtenido, gorm.ErrRecordNotFound) {
				return domain.ErrTrabajoNoEncontrado
			}
			return errorObtenido
		}
		if trabajo.Estado != domain.EstadoEnProgreso {
			return domain.ErrTrabajoYaFinalizado
		}
		if errorObtenido := transaccion.Exec(`INSERT INTO scraper.entregas_trabajos (trabajo_id,payload_hash,payload) VALUES (?,?,?::jsonb) ON CONFLICT (trabajo_id) DO NOTHING`, ejecucion.TrabajoID, ejecucion.PayloadHash, ejecucion.Payload).Error; errorObtenido != nil {
			return errorObtenido
		}
		var huella string
		if errorObtenido := transaccion.Raw(`SELECT payload_hash FROM scraper.entregas_trabajos WHERE trabajo_id=?`, ejecucion.TrabajoID).Scan(&huella).Error; errorObtenido != nil {
			return errorObtenido
		}
		if huella != ejecucion.PayloadHash {
			return domain.ErrIdempotencia
		}
		return nil
	})
}

// Pendientes retira outboxes de trabajos terminados y obtiene las ejecuciones
// de trabajos en progreso cuya próxima publicación ya venció.
func (repositorio *repositorioEjecuciones) Pendientes(contexto context.Context) ([]domain.Ejecucion, error) {
	if errorObtenido := repositorio.baseDatos.WithContext(contexto).Exec(`DELETE FROM scraper.entregas_trabajos WHERE trabajo_id IN (
 SELECT et.trabajo_id FROM scraper.entregas_trabajos et JOIN scraper.trabajos_scraper t ON t.id=et.trabajo_id
 WHERE t.estado IN (?,?) LIMIT ?)`, domain.EstadoCompletado, domain.EstadoFallido, limiteRetiroOutbox).Error; errorObtenido != nil {
		return nil, errorObtenido
	}
	var resultado []domain.Ejecucion
	errorObtenido := repositorio.baseDatos.WithContext(contexto).Raw(`SELECT et.trabajo_id,et.payload_hash,et.payload::text FROM scraper.entregas_trabajos et
 JOIN scraper.trabajos_scraper t ON t.id=et.trabajo_id WHERE t.estado=?
 AND et.proxima_publicacion<=now() ORDER BY et.proxima_publicacion LIMIT ?`, domain.EstadoEnProgreso, limitePendientesOutbox).Scan(&resultado).Error
	return resultado, errorObtenido
}

// PosponerPublicacion programa el siguiente intento de publicación del trabajo.
func (repositorio *repositorioEjecuciones) PosponerPublicacion(contexto context.Context, trabajoID uuid.UUID) error {
	return repositorio.baseDatos.WithContext(contexto).Exec(`UPDATE scraper.entregas_trabajos SET proxima_publicacion=now()+make_interval(secs => ?) WHERE trabajo_id=?`, segundosRepublicacion, trabajoID).Error
}

// ===== Publicación en Redis Streams =====

// PublicadorRedis implementa domain.PublicadorTrabajos con un cliente Redis
// propio: un comando bloqueado no agota el pool que usa el probe de readiness.
type PublicadorRedis struct {
	clienteRedis *redis.Client
	baseDatos    *gorm.DB
}

// NuevoPublicadorRedis copia las opciones del cliente principal con los plazos
// del servicio. Sin cliente Redis cada publicación falla y queda en el outbox.
func NuevoPublicadorRedis(clienteRedis *redis.Client, baseDatos *gorm.DB) *PublicadorRedis {
	publicador := &PublicadorRedis{baseDatos: baseDatos}
	if clienteRedis != nil {
		opciones := *clienteRedis.Options()
		publicador.clienteRedis = redis.NewClient(limitarOpcionesRedis(&opciones))
	}
	return publicador
}

// Cerrar libera el cliente Redis dedicado del publicador.
func (publicador *PublicadorRedis) Cerrar() error {
	if publicador.clienteRedis == nil {
		return nil
	}
	return publicador.clienteRedis.Close()
}

// Publicar comprueba que el trabajo siga en progreso y lo agrega al stream.
// Lee SQL sin transacción ni FOR UPDATE para no retener locks durante Redis;
// la marca de cierre del script evita republicar si el trabajo termina entre
// la lectura y la publicación.
func (publicador *PublicadorRedis) Publicar(contexto context.Context, ejecucion domain.Ejecucion) error {
	if publicador.clienteRedis == nil {
		return errors.New("Redis no está disponible")
	}
	intento, cancelar := context.WithTimeout(contexto, plazoPublicacion)
	defer cancelar()
	var trabajo domain.TrabajoScraper
	encontrado := publicador.baseDatos.WithContext(intento).Select("id").Where("id=? AND estado=?", ejecucion.TrabajoID, domain.EstadoEnProgreso).Limit(1).Find(&trabajo)
	if encontrado.Error != nil {
		return encontrado.Error
	}
	if encontrado.RowsAffected == 0 {
		return nil
	}
	claves := []string{streamTrabajos, indiceTrabajos, prefijoTrabajoCerrado + ejecucion.TrabajoID.String()}
	errorObtenido := publicarTrabajo.Run(intento, publicador.clienteRedis, claves, ejecucion.TrabajoID.String(), ejecucion.Payload).Err()
	if errors.Is(errorObtenido, redis.Nil) {
		return nil // El worker ya cerró el trabajo.
	}
	return errorObtenido
}
