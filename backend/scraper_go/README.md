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
| Santa Isabel | Implementado: renderData + fallback JSON-LD, paginación y filtro por seller |
| Cugat | Implementado: tarjetas WooCommerce, JSON-LD, modo producto y detección de bloqueo ajustada |
| Acuenta | Implementado: RSC, promociones, referencias EAN/imagen, tarjetas y CA intermedia |
| Lider | Implementado: `__NEXT_DATA__`, tarjetas, paginación, HTTP/1.1 y cookies |

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
        ├── internal/spiders   registro de spiders y extracción por supermercado
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

`JUMBO_CATEGORY_URLS`, `SANTA_ISABEL_CATEGORY_URLS`, `CUGAT_CATEGORY_URLS`,
`ACUENTA_CATEGORY_URLS` y `LIDER_CATEGORY_URLS` permiten agregar categorías
separadas por coma sin editar el archivo. El worker también acepta `add_url`
en el payload, pero el endpoint
`POST /api/v1/scraper/trabajos/{id}/ejecutar` de Control todavía no lo expone;
para agregar categorías desde la API o el bot falta ese campo. Por ahora se
usa el archivo o las variables de entorno.

La fuente de verdad de las categorías es `backend/scraper_go/categories/`.
Python las sigue leyendo desde `/app/research` mediante un montaje de compose;
en el host, `backend/scraper/research/*_categories.txt` son enlaces simbólicos
temporales a esa carpeta para no romper el rollback en Compose. Al borrar
`backend/scraper`, los enlaces desaparecen pero las categorías quedan en Go.

Una imagen Python reconstruida desde `backend/scraper` no incluye los TXT
reales, porque los symlinks apuntan fuera del contexto de build. Para un
rollback, usar la imagen Python ya construida o restaurar copias reales en
`backend/scraper/research` antes de reconstruirla.

## Desarrollo y pruebas

Go no se instala en el host: las pruebas se ejecutan en Docker. Desde la raíz
del repositorio.

### Suite completa

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e HOME=/tmp -e GOCACHE=/tmp/gocache -e GOMODCACHE=/tmp/gomodcache \
  -v "$PWD/backend/scraper_go:/src" -w /src golang:1.26-alpine \
  sh -c 'gofmt -w . && go test ./... && go vet ./... && go build -o /tmp/scraper-worker ./cmd/worker'
```

La imagen usa el contexto de build en la raíz para tomar las categorías de
`backend/scraper_go/categories`:

```bash
docker build -f backend/scraper_go/Dockerfile -t scraper-go:local .
```

### Extracción y paginación sin red

Los tests de `internal/spiders` usan HTML de fixtures y validan JSON-LD, RSC,
`__NEXT_DATA__`, paginación, modo producto y normalización. No consultan los
sitios reales:

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e HOME=/tmp -e GOCACHE=/tmp/gocache -e GOMODCACHE=/tmp/gomodcache \
  -v "$PWD/backend/scraper_go:/src" -w /src golang:1.26-alpine \
  sh -c 'go test -v -count=1 ./internal/spiders'
```

### Extracción directa de URLs sin API

`cmd/extract` ejecuta un spider contra URLs concretas y escribe un producto por
línea (JSONL) en `stdout`. No usa Redis, la API ni el spool, así que sirve para
probar el HTML real sin crear un trabajo. Primero construye la imagen local:

```bash
docker build -f backend/scraper_go/Dockerfile -t scraper-go:local .
```

Probar una categoría de Jumbo (limita a la primera página para que sea rápido):

```bash
docker run --rm --network host \
  --entrypoint /app/scraper-extract scraper-go:local \
  -spider jumbo_rsc \
  -url 'https://www.jumbo.cl/frutas-y-verduras' \
  -max-pages 1 \
  > /tmp/jumbo.jsonl
```

Probar una ficha de producto. Sólo Jumbo y Cugat tienen modo producto:

```bash
docker run --rm --network host \
  --entrypoint /app/scraper-extract scraper-go:local \
  -spider jumbo_rsc -product \
  -url 'https://www.jumbo.cl/palta-hass-jumbo-malla-1-kg/p' \
  > /tmp/producto.jsonl
```

Para varias categorías se puede repetir `-url` o usar un archivo:

```bash
docker run --rm --network host \
  -v "$PWD/mis_urls.txt:/urls.txt:ro" \
  --entrypoint /app/scraper-extract scraper-go:local \
  -spider acuenta_rsc -file /urls.txt \
  > /tmp/acuenta.jsonl
```

El archivo acepta una URL por línea y comentarios con `#`. Si omites `-url` y
`-file`, el spider usa todas las categorías de
`backend/scraper_go/categories/<cadena>_categories.txt`. Para Acuenta, Santa
Isabel y Lider el modo producto no existe: se prueban por categoría. Revisa los
resultados con:

```bash
wc -l /tmp/jumbo.jsonl
jq -s 'length' /tmp/jumbo.jsonl
jq -s '.[0]' /tmp/jumbo.jsonl
```

Si el archivo queda vacío, revisa el `stderr` del comando: ahí aparecen
`*_category_without_products`, `*_product_not_found`, `*_product_without_data`,
`*_category_robots_disallowed` y los bloqueos `403/429/503/504`.

### Validación contra sitios reales (26/10/2026)

Con la imagen local y una página por cadena (`-max-pages 1`, `-delay 0`) los
cinco spiders extraen productos del HTML vigente:

| Cadena | Spider | Productos |
| --- | --- | ---: |
| Jumbo | `jumbo_rsc` | 40 |
| Santa Isabel | `santa_isabel_rsc` | 40 |
| Cugat | `cugat_rsc` | 25 |
| Acuenta | `acuenta_rsc` | 43 |
| Lider | `lider_rsc` | 44 |

También se validó el modo producto de Jumbo y Cugat (1 producto cada uno).
Esta prueba cubre una página por cadena, no el catálogo completo. Los cambios
que hicieron posible la paridad fueron:

- Cugat: se dejó de tratar cualquier aparición de `captcha` como bloqueo. La
  página normal incluye el CSS de reCAPTCHA y antes se descartaba completa.
- Santa Isabel: el sitio dejó de publicar `window.__renderData`. Se conserva
  ese parser y se agregó un fallback que lee el `ItemList` de JSON-LD.
- Acuenta: el servidor no envía el certificado intermedio de GlobalSign. Se
  embebe el intermedio público en `internal/crawler/certs/` y se agrega al
  pool de confianza; no se desactiva la verificación TLS.
- Lider: se usa HTTP/1.1 y un cookie jar como Scrapy. La página sigue detrás de
  PerimeterX; si devuelve `Robot or human?` hay que reintentar más tarde, no
  evadir el challenge.
- Todas: el cliente HTTP conserva cookies entre `robots.txt` y la página.

Cugat puede registrar `scraper_robots_unavailable` porque su `robots.txt`
redirige en bucle; el aviso no impide la extracción.

### Gate de RAM

El pipeline real de spool, lotes y entrega se mide con un catálogo sintético.
El test lee `VmHWM` de `/proc/self/status` y falla si el RSS pico supera el
límite. En CI se ejecutan 50.000 productos con un tope de 150 MB:

```bash
docker run --rm --network host --user "$(id -u):$(id -g)" \
  -e HOME=/tmp -e GOCACHE=/tmp/gocache -e GOMODCACHE=/tmp/gomodcache \
  -e SCRAPER_MEMORY_GATE=1 \
  -e SCRAPER_MEMORY_ITEMS=50000 \
  -e SCRAPER_MEMORY_LIMIT_MB=150 \
  -v "$PWD/backend/scraper_go:/src" -w /src golang:1.26-alpine \
  sh -c 'go test -run TestMemoryGate -count=1 -timeout 10m -v ./internal/worker'
```

El gate no corre en la suite normal para no mezclar la medición de RSS con
`-race`; el workflow `scraper-go-checks` lo ejecuta como paso separado.

### Extracción real con Compose

Con el stack base levantado y el token de `.env` exportado:

```bash
export SCRAPING_SERVICE_TOKEN='...'
docker compose stop scraper_worker
docker compose --profile scraper-go up -d --build scraper_worker_go
docker logs -f go_scraper_worker
```

Crear un trabajo y ejecutar un producto puntual. Reemplaza `cadena_id` por el
ID real de la cadena y la URL por un producto vigente. Jumbo y Cugat aceptan
`producto_url`; para las demás cadenas se ejecuta la categoría completa:

```bash
JOB=$(curl -sS -X POST http://localhost:8080/api/v1/scraper/trabajos \
  -H "Authorization: Bearer $SCRAPING_SERVICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"cadena_id":1}' | jq -r .id)

curl -sS -X POST \
  "http://localhost:8080/api/v1/scraper/trabajos/$JOB/ejecutar" \
  -H "Authorization: Bearer $SCRAPING_SERVICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"spider":"jumbo_rsc","producto_url":"https://www.jumbo.cl/p/URL-DE-UN-PRODUCTO"}'
```

Consultar el avance y el total extraído:

```bash
watch -n 5 "curl -sS \
  -H 'Authorization: Bearer $SCRAPING_SERVICE_TOKEN' \
  http://localhost:8080/api/v1/scraper/trabajos/$JOB \
  | jq '{estado, elementos_extraidos, registro_errores}'"
```

Un resultado correcto termina con `estado: "completado"` y
`elementos_extraidos` mayor que cero. Si queda en cero, revisar en los logs
`*_category_without_products`, `*_product_not_found`, `*_product_without_data`,
`*_category_robots_disallowed` y los bloqueos `403/429/503/504`.

Para probar una categoría completa, omitir `producto_url` y usar por ejemplo
`{"spider":"jumbo_rsc"}`. Para Acuenta, Santa Isabel y Lider no hay modo
producto; se prueba por categoría.

### RAM del contenedor en vivo

Durante un trabajo real:

```bash
docker stats --no-stream go_scraper_worker
PID=$(docker inspect -f '{{.State.Pid}}' go_scraper_worker)
grep -E 'VmRSS|VmHWM' /proc/$PID/status
```

`VmHWM` es el pico desde que arrancó el proceso. El límite de Compose es de
200 MB y el gate de CI usa 150 MB.

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
todos los spiders tengan pruebas de paridad y una validación contra el HTML
real de producción.

## Pendientes

- Validar los cinco spiders contra HTML real de producción, no sólo fixtures.
- Agregar un smoke de compose que ejecute un trabajo real contra Redis y una
  API simulada, incluyendo bloqueo 403/429 y recuperación de lease.
- Crear el manifiesto K8s para la imagen Go y publicar esa imagen; el manifiesto
  actual `infrastructure/k8s/scraper/deployment.yaml` sigue apuntando a Python.
- Exponer `add_url` y `enrich_ean` en el endpoint de Control si se necesitan
  desde la API o el bot.
- Confirmar el umbral de frescura y los identificadores `SUP-XXX` de la tarjeta.
