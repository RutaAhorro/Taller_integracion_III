# Scraper Go

Microservicio que reemplaza de forma gradual al worker de scraping de
`backend/scraper` (Python/Scrapy). Mantiene el contrato HTTP con Control de
Scraping, el stream y el grupo de Redis, el esquema del spool SQLite y los
nombres de las variables de entorno.

El worker de Python sigue siendo el proceso activo hasta completar la marcha
paralela y el cutover. Este servicio es el camino de reemplazo y el mecanismo de
rollback mientras ambos convivan.

## Estado de la migración

| Supermercado | Estado |
| --- | --- |
| Jumbo | Implementado: JSON-LD, paginación, modo producto y enriquecimiento EAN |
| Santa Isabel | Implementado: renderData, paginación y filtro por seller |
| Cugat | Pendiente de portar |
| Acuenta | Pendiente de portar |
| Lider | Pendiente de portar |

La base del worker ya está implementada: configuración, cliente de Control de
Scraping, outbox SQLite, cola Redis con lease y heartbeat, crawler con
robots.txt/throttle/reintentos, detección de bloqueos, retención de spools y
orquestación.

## Arquitectura

```text
cmd/worker
  └── internal/worker          orquesta lease, spool, crawl, entrega y cierre
        ├── internal/queue     Redis Streams: receive, renew, ack, dead-letter
        ├── internal/spool     outbox SQLite por trabajo (items, batches, progress)
        ├── internal/spiders   registro de spiders y Jumbo
        │     └── internal/crawler   HTTP, robots.txt, throttle, reintentos
        ├── internal/controlscraping contrato HTTP e idempotencia
        ├── internal/normalize paridad de precios, EAN y texto con Python
        ├── internal/storage   capacidad, limpieza y retención
        └── internal/config    variables de entorno y límites
```

Un solo microservicio, con módulos internos. Los spiders dependen de las
interfaces `Fetcher` y `Emitter`, no de Redis ni de SQLite.

## Contrato

- Cola: `scraper:jobs:stream`, grupo `scraper-workers`, índice
  `scraper:jobs:index`, cerrados `scraper:jobs:closed:<trabajo_id>` y
  dead-letter `scraper:jobs:dead`.
- Ingesta: `POST /api/v1/scraper/trabajos/{id}/productos` con
  `Idempotency-Key`.
- Cierre: `PUT /api/v1/scraper/trabajos/{id}/finalizar`.
- Estado: `GET /api/v1/scraper/trabajos/{id}`.
- Autorización: `Authorization: Bearer $SCRAPING_SERVICE_TOKEN`.
- Spool: `items`, `batches` y `progress`, compatible con
  `backend/scraper/scraper_core/outbox.py`.

Un producto conserva los campos y los `null` del contrato de Python. El
supermercado viaja en el nivel superior del lote y la categoría dentro del
producto.

## Configuración

| Variable | Default | Uso |
| --- | --- | --- |
| `REDIS_URL` | `redis://localhost:6379/0` | Broker |
| `SCRAPER_API_URL` | `http://localhost:8082/api/v1/scraper/productos` | Ingesta |
| `SCRAPING_SERVICE_TOKEN` | vacío | Obligatorio en ejecución |
| `SCRAPER_STATE_DIR` | `/var/lib/scraper` | Spools |
| `SCRAPER_LEASE_MS` | `60000` | Lease mínima de 10 s |
| `SCRAPER_STATE_MIN_FREE_BYTES` | `67108864` | Reserva mínima de 64 MiB |
| `SCRAPER_FAILED_RETENTION_SECONDS` | `86400` | Retención de fallidos |
| `SCRAPER_API_TIMEOUT` | `30` | Timeout de ingesta en segundos |
| `SCRAPER_API_BATCH_SIZE` | `100` | Productos por lote (1..1000) |
| `SCRAPER_QUEUE_NAME` | `scraper:jobs:stream` | Stream |
| `SCRAPER_GROUP` | `scraper-workers` | Grupo de consumo |
| `SCRAPER_CATEGORY_DIR` | `/app/research` | Archivos de categorías |
| `SCRAPER_DOWNLOAD_DELAY_MS` | `2000` | Throttle por dominio |
| `SCRAPER_DOWNLOAD_TIMEOUT` | `30` | Timeout HTTP |
| `SCRAPER_DOWNLOAD_MAXSIZE` | `5242880` | Tamaño máximo de respuesta |
| `SCRAPER_RETRY_TIMES` | `3` | Reintentos HTTP |
| `LOG_LEVEL` | `INFO` | `DEBUG`, `INFO`, `WARNING`, `ERROR` |

`JUMBO_CATEGORY_URLS` permite agregar categorías separadas por coma sin editar
el archivo. El campo `add_url` de un trabajo agrega la categoría al archivo
`jumbo_categories.txt` del directorio configurado.

## Desarrollo y pruebas

Go no se instala en el host: las pruebas se ejecutan en Docker.

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e HOME=/tmp -e GOCACHE=/tmp/gocache -e GOMODCACHE=/tmp/gomodcache \
  -v "$PWD:/src" -w /src golang:1.26-alpine \
  sh -c 'gofmt -w . && go test ./... && go vet ./... && go build ./cmd/worker'
```

Desde la raíz del repositorio, con el contexto de build en la raíz para tomar
las categorías de `backend/scraper/research`:

```bash
docker build -f backend/scraper_go/Dockerfile -t scraper-go:local .
```

## Marcha paralela y cutover

El servicio de compose `scraper_worker_go` está detrás del perfil
`scraper-go` y usa por defecto el mismo stream y grupo que Python. Para una
marcha paralela, usar una cola y un grupo propios:

```bash
SCRAPER_GO_QUEUE_NAME=scraper:jobs:stream:go \
SCRAPER_GO_GROUP=scraper-workers-go \
docker compose --profile scraper-go up -d scraper_worker_go
```

Para el cutover:

1. Detener el worker de Python: `docker compose stop scraper_worker`.
2. Levantar Go con los valores por defecto (mismo stream y grupo):
   `docker compose --profile scraper-go up -d scraper_worker_go`.
3. Verificar ACK, finalización y limpieza de spools en los logs.
4. Rollback: detener Go y volver a levantar `scraper_worker`. El esquema del
   spool es el mismo.

Borrar `backend/scraper` sólo después de que el cutover esté estable y de que
los cuatro spiders restantes tengan pruebas de paridad.

## Pendientes

- Portar Cugat, Acuenta y Lider con fixtures y pruebas doradas.
- Agregar el gate de RAM en CI (objetivo de trabajo: 150 MB de RSS; el worker
  Go debería quedar en 60–100 MB).
- Agregar un smoke de compose que ejecute un trabajo real contra Redis y una
  API simulada, incluyendo bloqueo 403/429 y recuperación de lease.
- Confirmar el umbral de frescura y los identificadores `SUP-XXX` de la tarjeta.
