# Microservicio de Web Scraping (Scrapy)

Este microservicio está destinado a extraer de manera asíncrona los catálogos de productos y ofertas desde sitios web de supermercados, implementando el framework **Scrapy**. 

El microservicio está integrado en `docker-compose.yml` bajo el servicio `scraper_worker`, escuchando la cola Redis para procesar trabajos en segundo plano enviados desde el API Gateway. También permite ejecutarse de forma independiente para tareas de desarrollo y pruebas locales.

## Tecnologías y Entorno
El contenedor cuenta con las dependencias necesarias inyectadas en su `Dockerfile`:
- `Scrapy`: Framework de extracción web.
- `psycopg2-binary`: Dependencia disponible para integraciones auxiliares.
- `redis`: Cliente usado por el worker para consumir trabajos desde Redis.

La API publica trabajos en la cola Redis `scraper:jobs`. El worker consume esos trabajos, ejecuta el spider indicado y envía los productos a la API para su persistencia. La cola de refresh (`RefreshQueue`) continúa siendo local al proceso y evita duplicados durante una ejecución.

## Ejecución disparada desde la API

La API y el worker se conectan mediante Redis. Primero se inicia el worker desde `backend/scraper` y se deja escuchando:

```bash
REDIS_URL=redis://localhost:6379/0 \
SCRAPING_SERVICE_TOKEN="$SCRAPING_SERVICE_TOKEN" \
SCRAPER_API_URL=http://localhost:8080/api/v1/scraper/productos \
.venv/bin/python -m scraper_core.worker
```

En otra terminal, se registra y encola un trabajo:

```bash
curl -X POST http://localhost:8080/api/v1/scraper/trabajos \
	-H "Authorization: Bearer $SCRAPING_SERVICE_TOKEN" \
	-H "Content-Type: application/json" \
	-d '{"cadena_id":1}'
```

Con el `id` devuelto, se dispara el spider:

```bash
curl -X POST http://localhost:8080/api/v1/scraper/trabajos/{ID}/ejecutar \
	-H "Authorization: Bearer $SCRAPING_SERVICE_TOKEN" \
	-H "Content-Type: application/json" \
	-d '{"spider":"jumbo_rsc"}'
```

Control de scraping exige `SCRAPING_SERVICE_TOKEN` (el mismo valor del `.env`) como
`Authorization: Bearer` en todas las rutas `/api/v1/scraper/*`; sin él responde 401.

Los spiders permitidos son `jumbo_rsc`, `santa_isabel_rsc`, `cugat_rsc`, `acuenta_rsc` y `lider_rsc`. El estado se consulta con `GET /api/v1/scraper/trabajos/{ID}` y debe avanzar de `en_progreso` a `completado` o `fallido`. La ingesta se realiza por lotes en `POST /api/v1/scraper/trabajos/{ID}/productos`.

Para comprobar la cola pendiente:

```bash
docker exec redis_broker redis-cli LLEN scraper:jobs
```

## Documentación de Uso (Modo Desarrollo)

A diferencia de FastAPI, **Scrapy no es un servidor web permanente**. Es un entorno de ejecución de _scripts_ (Spiders). Desde la raíz del repositorio, construye la imagen actual así:

```bash
docker build -t taller-integracion-scraper:local ./backend/scraper
```

### 1. Inicializar el proyecto Scrapy
Si aún no has andamiado la estructura estándar de Scrapy, ejecuta el comando desde el directorio `backend/scraper` usando el entorno local:

```bash
scrapy startproject scraper_core .
```

### 2. Crear un nuevo "Spider" (Bot recolector)
Para generar el archivo de un bot que escanee un supermercado ficticio:

```bash
docker run --rm taller-integracion-scraper:local \
	scrapy genspider ejemplo_supermercado misupermercado.com
```

### 3. Ejecutar la recolección de datos manualmente

#### Jumbo (`jumbo_rsc`)
```bash
docker run --rm \
	-v "$PWD:/salida" \
	taller-integracion-scraper:local \
	scrapy crawl jumbo_rsc \
	-a enrich_ean=true \
	-s JOBDIR= \
	-O /salida/jumbo.json
```
La opción `-a enrich_ean=true` consulta la ficha de cada producto sin EAN en el listado.

#### Santa Isabel (`santa_isabel_rsc`)
```bash
docker run --rm \
	-v "$PWD/backend/scraper:/app" \
	-v "$PWD:/salida" \
	taller-integracion-scraper:local \
	scrapy crawl santa_isabel_rsc \
	-s JOBDIR= \
	-O /salida/santa_isabel.json
```
El spider lee el estado SSR de Santa Isabel (`window.__renderData`) y acepta categorías desde `research/santa_isabel_categories.txt` o `SANTA_ISABEL_CATEGORY_URLS`.

#### A Cuenta (`acuenta_rsc`)
```bash
docker run --rm \
	-v "$PWD/backend/scraper:/app" \
	-v "$PWD:/salida" \
	taller-integracion-scraper:local \
	scrapy crawl acuenta_rsc \
	-s JOBDIR= \
	-O /salida/acuenta.json
```
Acepta categorías desde `research/acuenta_categories.txt` o la variable `ACUENTA_CATEGORY_URLS`.

#### Cugat (`cugat_rsc`)
```bash
docker run --rm \
	-v "$PWD/backend/scraper:/app" \
	-v "$PWD:/salida" \
	taller-integracion-scraper:local \
	scrapy crawl cugat_rsc \
	-a enrich_details=true \
	-s JOBDIR= \
	-O /salida/cugat.json
```
La opción `-a enrich_details=true` consulta el JSON-LD de cada ficha para enriquecer la marca, EAN/GTIN e imagen.

#### Lider Supermercado (`lider_rsc`)
```bash
docker run --rm \
	-v "$PWD/backend/scraper:/app" \
	-v "$PWD:/salida" \
	taller-integracion-scraper:local \
	scrapy crawl lider_rsc \
	-s JOBDIR= \
	-O /salida/lider.json
```
Usa únicamente rutas públicas `/browse/` permitidas por `robots.txt` y categorías desde `research/lider_categories.txt` o `LIDER_CATEGORY_URLS`.

#### Ejecutar spiders en secuencia
Para ejecutar los spiders en secuencia desde la raíz del repositorio y guardar cada catálogo por separado:

```bash
docker run --rm \
	-v "$PWD/backend/scraper:/app" \
	-v "$PWD:/salida" \
	taller-integracion-scraper:local \
	sh -c 'scrapy crawl jumbo_rsc -a enrich_ean=true -s JOBDIR= -O /salida/jumbo.json && scrapy crawl santa_isabel_rsc -s JOBDIR= -O /salida/santa_isabel.json && scrapy crawl cugat_rsc -a enrich_details=true -s JOBDIR= -O /salida/cugat.json && scrapy crawl acuenta_rsc -s JOBDIR= -O /salida/acuenta.json && scrapy crawl lider_rsc -s JOBDIR= -O /salida/lider.json'
```

### Ejecución local sin persistencia

Desde `backend/scraper`, el runtime ejecuta el scraping masivo y entrega cada producto extraído como una línea JSON por `stdout`. Los mensajes de Scrapy se mantienen en `stderr`:

```bash
python -m scraper_core.runtime
```

Para actualizar un producto concreto, pasa su URL pública:

```bash
python -m scraper_core.runtime --product-url "https://www.jumbo.cl/ruta-del-producto"
```

El runtime captura los items en un feed JSONL temporal y retorna los objetos en `result["items"]`. Para persistirlos mediante la API se debe usar el worker y el flujo disparado desde la API descrito arriba.

Cada producto conserva los campos básicos (`producto`, `precio`, `categoria`, `imagen`) y puede incluir `ean_gtin`, `sku`, `precio_normal`, `precio_oferta`, `marca`, `formato_crudo`, `mecanica_promocion`, `en_stock` y `url_producto`. Los campos no publicados por el supermercado origen quedan como `null`.

## Política de resiliencia

Todos los spiders del proyecto comparten una política responsable configurada globalmente en `settings.py` para no bloquear los sitios de origen ni sobrepasar los recursos del servidor:

- 1 request simultáneo por dominio (`CONCURRENT_REQUESTS_PER_DOMAIN = 1`)
- Delay mínimo de 2s entre peticiones (`DOWNLOAD_DELAY = 2`)
- Throttling automático habilitado (`AUTOTHROTTLE_ENABLED = True`)
- Retries limitados solo para errores temporales (`429`, `500`, `503`, `504`)
- Backoff progresivo para evitar rebotes de carga
- Fin explícito cuando la página ya no tiene más resultados (respuestas HTTP 404 de paginación)
- Advertencias explícitas en logs para respuestas vacías o formatos inesperados

Esto mantiene una extracción cuidadosa, estable y compatible con un entorno Docker y hardware con recursos reducidos (servidor Pentium).

## Estado de Integración

El microservicio de scraping se encuentra completamente integrado con la arquitectura general del sistema:

1. **Redis Broker**: Escucha solicitudes enviadas por el API Gateway a través de la cola `scraper:jobs`.
2. **API Gateway (Go)**: Recibe lotes de productos scrapeados vía HTTP (`POST /api/v1/scraper/productos`) e ingesta la información en la base de datos PostgreSQL.
3. **Persistencia (PostgreSQL)**: Se almacena en el esquema `scraper.*` en las tablas `productos_crudos` y `capturas_precios`.

