# Scraper Go (`backend/scraper_go`)

Microservicio trabajador de scraping en Go. Consume trabajos desde Redis Streams, extrae productos de cinco cadenas de supermercados chilenos (Jumbo, Santa Isabel, Cugat, A Cuenta y Lider), almacena el progreso y los lotes en un spool SQLite local por trabajo y entrega los resultados por HTTP a Control de Scraping.

### Qué hace y qué NO hace

- **Sí hace:** consumo de trabajos con lease exclusiva en Redis Streams (`internal/cola/cola.go`), extracción HTTP con soporte de `robots.txt`, throttle por dominio, reintentos y detección de bloqueos antibot (`internal/extraccion/crawler.go`), normalización y recorte de campos según los límites del esquema de destino (`internal/dominio/normalizacion.go`), persistencia local en SQLite antes del envío (`internal/cola/spool.go`), entrega idempotente por lotes y cierre de trabajos por HTTP (`internal/cola/cliente_control.go`), y recolección de spools terminales según retención y presión de disco (`internal/cola/retencion.go`).
- **NO hace:** no expone ningún servidor HTTP público ni puerto de escucha (`cmd/worker/main.go`), no se conecta a PostgreSQL ni ejecuta consultas SQL sobre la base de datos central (`go.mod` sólo incluye el driver `modernc.org/sqlite`), y no recibe `JWT_SECRET` ni tokens privados de Catálogo (`internal/config/config.go`).

---

## Estructura del servicio

El servicio se organiza en 2 puntos de entrada bajo `cmd/`, 5 paquetes internos bajo `internal/` y pruebas separadas bajo `tests/`:

### Puntos de entrada (`cmd/`)

- `cmd/worker/main.go`: demonio principal del worker (`scraper-worker`); carga la configuración desde el entorno, verifica la conexión a Redis, construye el crawler HTTP, el registro de spiders, la cola Redis y el cliente de Control, y ejecuta el bucle de trabajo.
- `cmd/extract/main.go`: CLI de extracción directa (`scraper-extract`) que ejecuta un spider contra URLs de categorías o de ficha de producto y emite un producto por línea (JSONL) en `stdout`, sin usar Redis, spool SQLite ni la API de Control.

### Paquetes internos (`internal/`)

- **`internal/config`**
  - `config.go`: lectura, valores por defecto y validación de todas las variables de entorno del servicio (`Config`, `Default()`, `FromEnv()`, `Validate()`) y recolección dinámica de variables `*_CATEGORY_URLS`.
- **`internal/dominio`**
  - `producto.go`: modelos de datos compartidos (`Product` sin `omitempty` en campos opcionales para preservar `null` en JSON, `Item` con el nombre del supermercado, `Job`) y constructores de punteros (`StringPtr`, `BoolPtr`).
  - `normalizacion.go`: normalización de precios en pesos chilenos con redondeo `HALF_UP` (`PrecioCLP`), limpieza de códigos `EAN/GTIN` (`EAN`), colapso de espacios (`Texto`), truncado por runas UTF-8 (`Truncar`, `LimitarProducto`) y derivación de SKU desde la URL de la ficha (`SKUFromURL`).
- **`internal/extraccion`**
  - `catalogo.go`: carga y validación del catálogo declarativo de sitios (`Catalog`, `Site`, `URLRule`, `Pagination`, `CategoryFrom`), embebiendo `spiders.json` por defecto o leyendo un archivo externo.
  - `spiders.json`: especificación declarativa de los cinco spiders (`jumbo_rsc`, `santa_isabel_rsc`, `cugat_rsc`, `acuenta_rsc`, `lider_rsc`), sus hosts permitidos, reglas de URL, paginación, selectores CSS, ajustes, límites y listas.
  - `crawler.go`: cliente HTTP (`Client`) con pool TLS que incorpora el certificado intermedio embebido (`certs/globalsign-rsa-ov-ssl-ca-2018.pem`), cookie jar, caché positiva y negativa de `robots.txt`, pausa (`delay`) por host, reintentos con backoff exponencial y clasificación de bloqueos (`BlockedError`).
  - `spiders.go`: contratos `Spider`, `Fetcher` y `Emitter`, registro de spiders (`Registry`), ciclo base de recorrido por categorías o producto individual (`spiderBase`), `CompleteItem` y lectura/escritura de archivos de categorías (`CollectURLs`).
  - `parseo_comun.go`: utilidades compartidas entre spiders para construir URLs paginadas (`PageURL`, `StartPage`), derivar el nombre de categoría desde la URL (`CategoryFromURL`), extraer objetos JSON-LD y consultar el DOM con `goquery`.
  - `jumbo.go`: spider `jumbo_rsc`; parsea bloques JSON-LD de categorías y fichas, deriva SKU con prefijo `JUMBO-` cuando falta y soporta enriquecimiento opcional de EAN visitando la ficha del producto (`enrich_ean`).
  - `santa_isabel.go`: spider `santa_isabel_rsc`; extrae productos desde `window.__renderData` con respaldo en `ItemList` de JSON-LD y filtra ofertas por `seller`.
  - `cugat.go`: spider `cugat_rsc`; extrae tarjetas HTML de WooCommerce en listados de categoría y bloques JSON-LD en fichas de producto.
  - `acuenta.go`: spider `acuenta_rsc`; extrae productos combinando enlaces del DOM con payloads de React Server Components (RSC) o tarjetas HTML de respaldo.
  - `lider.go`: spider `lider_rsc`; extrae productos desde `script#__NEXT_DATA__`, bloques JSON-LD o tarjetas HTML de respaldo sobre HTTP/1.1 con cookie jar.
- **`internal/cola`**
  - `cola.go`: consumidor de Redis Streams (`Queue`) con creación idempotente de grupo (`EnsureGroup`), reclamo de leases vencidas (`XAUTOCLAIM`), guarda de mensajes pendientes (`XPENDING`), lectura bloqueante (`XREADGROUP`), renovación y confirmación atómica mediante scripts Lua (`Renew`, `Ack`) y latido periódico (`Heartbeat`).
  - `spool.go`: outbox SQLite local por trabajo (`Spool`) con tablas `items`, `batches` y `progress`, deduplicación por huella SHA-256 (`huellaProducto`), sellado de lotes sin mezclar supermercados (`Seal`) y reenvío ordenado en memoria acotada (`Replay`).
  - `cliente_control.go`: cliente HTTP hacia Control de Scraping (`Client`) para ingesta idempotente de lotes (`IngestBatch`), consulta de estado (`GetWork`) y cierre de trabajos (`FinalizeWork`), sin seguir redirecciones.
  - `retencion.go`: verificación de espacio libre en el volumen (`EnsureCapacity`, `FreeBytes`), borrado seguro respetando `flock` (`CleanupSpool`) y recolector rotativo de spools terminales (`Collector`).
- **`internal/worker`**
  - `worker.go`: orquestador principal (`Worker.Run`, `HandleMessage`, `procesarTrabajo`) que coordina la lease de Redis, el bloqueo de archivo `execution.lock`, el spool SQLite, la ejecución del spider, la entrega de lotes, la finalización en Control y el ACK o derivación a dead-letter.

### Pruebas (`tests/`)

- **`tests/unit/config`**: `config_test.go` (valores por defecto, lectura de entorno, sufijo `_CATEGORY_URLS`, fallos ante enteros mal formados o rangos inválidos).
- **`tests/unit/dominio`**: `normalizacion_test.go` (precios CLP, EAN, texto, truncado por runas UTF-8 y derivación de SKU desde URL).
- **`tests/unit/extraccion`**: `catalogo_test.go`, `crawler_test.go`, `spiders_test.go`, `parseo_comun_test.go`, `helpers_test.go`, `jumbo_test.go`, `santa_isabel_test.go`, `cugat_test.go`, `acuenta_test.go` y `lider_test.go` (validación del catálogo, crawler HTTP, ciclo común y parsers HTML/JSON-LD/RSC/NextData con fixtures locales sin red).
- **`tests/unit/cola`**: `cliente_control_test.go` (contrato HTTP con Control, recibos de lote, `202 entrega_pendiente`, errores permanentes y redirecciones) y `retencion_test.go` (espacio libre y `CleanupSpool` con `flock`).
- **`tests/unit/worker`**: `worker_test.go` (validación de dependencias obligatorias y configuración en `worker.New`).
- **`tests/integration/cola`**: `cola_test.go` (lease, renovación, ACK, dead-letter en Redis real y `Collector.Collect`) y `spool_test.go` (deduplicación, aislamiento por supermercado en `Seal`, persistencia y reenvío idempotente en SQLite).
- **`tests/integration/worker`**: `helpers_test.go`, `worker_test.go` y `recovery_test.go` (flujo extremo a extremo contra Redis real y API simulada, bloqueos, crawls vacíos, reanudación con `crawl_done=1`, límite de intentos y respeto de `execution.lock`).
- **`tests/performance/worker`**: `memory_test.go` (`TestMemoryGate`: prueba de consumo de memoria RSS/`RssAnon` y cgroup con 200.000 productos sintéticos a través del pipeline real de spool y entrega).

---

## Flujo de un trabajo

1. **Consumo en Redis Streams (`internal/cola/cola.go`):**
   - El worker asegura el stream `scraper:jobs:stream` (`config.DefaultQueueName`) y el consumer group `scraper-workers` (`config.DefaultQueueGroup`) mediante `XGROUP CREATE ... MKSTREAM`.
   - Claves auxiliares en Redis: hash de índice `scraper:jobs:index` (`cola.JobIndex`, mapea `trabajo_id -> mensaje_id`), prefijo de trabajos cerrados `scraper:jobs:closed:<trabajo_id>` (`cola.JobClosedPrefix`, con TTL `SCRAPER_JOB_CLOSED_TTL_SECONDS`, por defecto `86400` s) y stream dead-letter `scraper:jobs:dead` (`cola.DefaultDeadLetter`).
   - `Queue.Receive` intenta primero reclamar un mensaje cuya lease haya expirado (`XAUTOCLAIM` con `MinIdle = SCRAPER_LEASE_MS`). Si no hay mensajes vencidos pero `XPENDING` reporta mensajes pendientes en el grupo, espera sin leer mensajes nuevos. Sólo cuando no hay pendientes lee un mensaje nuevo con `XREADGROUP` (`Streams: [stream, ">"]`, `Count: 1`, `Block: SCRAPER_QUEUE_BLOCK_MS`).
2. **Validación del payload y lease/heartbeat (`internal/worker/worker.go`, `internal/cola/cola.go`):**
   - `HandleMessage` deserializa el campo `payload` del mensaje Redis (`trabajo_id` UUID obligatorio, `spider` registrado —por defecto `jumbo_rsc` si viene vacío—, `product_id`, `product_url`, `add_url`, `enrich_ean`). Si `product_id` viene definido sin `product_url` o el payload es inválido, confirma el mensaje enviándolo a `scraper:jobs:dead` con `reason = "invalid_payload"`.
   - `Queue.Heartbeat` valida la titularidad inicial con `Renew` (script Lua que comprueba en `XPENDING` que el consumidor actual siga siendo el dueño antes de ejecutar `XCLAIM ... JUSTID`) y lanza un ticker cada `lease / 3` (`20 s` para la lease por defecto de `60000 ms`). Si otro consumidor reclama el mensaje o falla la renovación, se cierra el canal `perdido` y se cancela el `context.Context` del trabajo.
3. **Verificación previa y apertura del spool SQLite (`internal/worker/worker.go`, `internal/cola/spool.go`):**
   - `procesarTrabajo` consulta `GET /trabajos/{id}` en Control. Si el trabajo ya está en estado terminal (`completado` o `fallido`), retorna ese estado para confirmar el mensaje en Redis sin abrir el spool ni ejecutar el spider.
   - `cola.Open` valida que `trabajo_id` sea un UUID, verifica `EnsureCapacity` sobre `SCRAPER_STATE_DIR` y abre `<SCRAPER_STATE_DIR>/<trabajo_id>/outbox.sqlite` (permisos `0600` sobre el archivo y `0700` sobre el directorio) con `_pragma=journal_mode(DELETE)&_pragma=synchronous(FULL)&_pragma=busy_timeout(30000)` y `SetMaxOpenConns(1)`.
   - El spool contiene tres tablas:
     - `items (id INTEGER PRIMARY KEY, fingerprint TEXT UNIQUE NOT NULL, market TEXT, product TEXT NOT NULL, batch_id TEXT)`
     - `batches (id TEXT PRIMARY KEY, payload TEXT NOT NULL, delivered INTEGER NOT NULL DEFAULT 0)`
     - `progress (key TEXT PRIMARY KEY, value INTEGER NOT NULL)`
   - A continuación toma un bloqueo exclusivo no bloqueante (`flock` `LOCK_EX|LOCK_NB`) sobre `<SCRAPER_STATE_DIR>/<trabajo_id>/execution.lock` y vuelve a consultar `GET /trabajos/{id}` tras adquirir el lock.
   - Antes de iniciar o reanudar el crawl, ejecuta `Seal(BatchSize)` y `Replay(ctx, api)` para despachar cualquier lote que hubiera quedado pendiente en una ejecución previa interrumpida.
4. **Huella de deduplicación del spool (`internal/cola/spool.go:110-143`):**
   - Antes de insertar en `items`, `CompleteItem` aplica `dominio.LimitarProducto` para derivar el `SKU` desde `url_producto` si falta y truncar por runas UTF-8 a los límites de `scraper.productos_crudos` (`producto` y `mecanica_promocion`: 255; `sku`, `marca`, `categoria` y `formato_crudo`: 100; `ean_gtin`: 50).
   - `Spool.Add` serializa `dominio.Product` con `json.Marshal(producto)`, verifica espacio libre (`minimoLibre + len(datos)*4`) y calcula la huella con `huellaProducto(mercado, datos)`:
     $$\text{fingerprint} = \text{hex}\!\left(\text{SHA-256}\!\left(\text{bytes}(\text{mercado}) \;\Vert\; \text{0x00} \;\Vert\; \text{json.Marshal}(\text{producto})\right)\right)$$
     donde si `mercado == nil` no se escriben bytes de mercado antes del byte nulo `0x00`. La columna `fingerprint TEXT UNIQUE NOT NULL` con `INSERT OR IGNORE INTO items` descarta duplicados dentro del mismo trabajo.
5. **Crawl y progreso (`internal/worker/worker.go:326-377`):**
   - Si `progress["crawl_done"] == 0`, el worker lee `progress["crawl_attempts"]`. Si `intentos >= SCRAPER_CRAWL_ATTEMPTS` (por defecto `3`), aborta con `ErrCrawlExhausted` (error permanente que cierra el trabajo como `fallido` con motivo `attempts_exhausted` y conserva el spool para revisión). Si aún quedan intentos, incrementa `crawl_attempts` en SQLite y ejecuta `spider.Run`.
   - Cada vez que el spider emite un `dominio.Item`, el callback lo normaliza con `CompleteItem`, lo guarda con `almacen.Add` y, al acumular `SCRAPER_API_BATCH_SIZE` elementos (por defecto `100`), invoca `Seal` y `Replay`.
   - Si el spider devuelve `*extraccion.BlockedError`, el worker persiste la cantidad de respuestas bloqueadas en `progress["blocked_responses"]` y retorna el error. Si el spider termina sin error, marca `progress["crawl_done"] = 1`.
6. **Sellado (`Seal`) y entrega (`Replay`) a Control (`internal/cola/spool.go:158-331`):**
   - `Seal(tamanoLote)` agrupa las filas de `items` con `batch_id IS NULL` seleccionando primero un `market` (`primerMercadoSinLote`) y tomando hasta `tamanoLote` filas de ese mismo supermercado (`filasSinLote`). **Nunca mezcla supermercados distintos en un mismo lote.** Por cada lote genera un UUID v4 (`loteID`), persiste en `batches(id, payload)` el JSON `{"products":[...],"supermarket":...}` y asigna `batch_id = loteID` a los `items` dentro de una misma transacción SQLite.
   - `Replay(contexto, cliente)` lee de a un lote por vez (`SELECT id, payload FROM batches WHERE delivered = 0 ORDER BY rowid LIMIT 1`) para mantener acotado el uso de RAM, llama a `cliente.IngestBatch` con `Idempotency-Key: <loteID>` y `Authorization: Bearer <SCRAPING_SERVICE_TOKEN>`, y sólo marca `UPDATE batches SET delivered = 1 WHERE id = ?` cuando `IngestBatch` confirma el recibo del lote.
   - **Semántica ante `202 Accepted` (`entrega_pendiente`):** si la API responde con `"codigo": "entrega_pendiente"`, `IngestBatch` devuelve un `*cola.Error` con `Status: 202` y `Code: "entrega_pendiente"`, cuyo método `Permanent()` es `false` (`internal/cola/cliente_control.go:47-57,146-152`). `Replay` no marca `delivered = 1`, el worker registra `worker_delivery_pending` y retorna **sin finalizar el trabajo y sin hacer ACK** en Redis para reintentar el mismo lote en el siguiente ciclo.
7. **Finalización (`FinalizeWork`) y confirmación (`Ack`) (`internal/worker/worker.go:192-282,379-403`):**
   - **Lease perdida:** si en cualquier punto `cola.IsLost(perdido)` es verdadero o se recibe `ErrLeaseLost` (o el contexto del heartbeat se cancela), el worker registra `worker_lease_lost` y aborta de inmediato **sin llamar a `FinalizeWork` ni a `Ack`**.
   - **Guardarraíl de trabajo vacío:** tras el `Seal` y `Replay` finales, el worker verifica `almacen.Count()`. Si el crawl terminó con `0` productos en `items`, devuelve `ErrNoProducts`, que se procesa como fallo permanente (`estado = "fallido"`, `motivo = "no_products"`, `registro_errores = "sin productos: el crawl no extrajo elementos; no se ingirió un lote vacío"`).
   - **Cierre exitoso:** si hay productos y todos los lotes fueron entregados, llama a `PUT /trabajos/{id}/finalizar` con `{"estado": "completado"}`. Sólo después de que Control confirma el mismo `id` y `estado`, el worker ejecuta `clienteCola.Ack(..., motivo="")` (que en un script Lua verifica la propiedad en `XPENDING`, ejecuta `XACK` y `XDEL` sobre el stream, borra la entrada en `scraper:jobs:index` y fija `scraper:jobs:closed:<trabajo_id>` con `EX 86400`) y elimina el directorio del spool con `CleanupSpool`.
   - **Fallos permanentes (`ErrCrawlExhausted`, `ErrNoProducts`, `*extraccion.BlockedError` o `*cola.Error` con `Permanent() == true`):** si el error de la API no es `404 Not Found`, el worker llama primero a `PUT /trabajos/{id}/finalizar` con `estado: "fallido"`, `registro_errores` y (en caso de bloqueo) `respuestas_bloqueadas`. Si `FinalizeWork` tiene éxito, confirma el mensaje con `Ack(..., motivo)` (`"blocked"`, `"no_products"`, `"attempts_exhausted"` o `"delivery_rejected"`), lo que copia el mensaje a `scraper:jobs:dead` (`payload`, `reason`, `source_id`) antes de hacer `XACK`/`XDEL`. El spool de un trabajo fallido se conserva en disco hasta que venza `SCRAPER_FAILED_RETENTION_SECONDS` o exista presión de disco.

---

## Contratos con Control de Scraping

El cliente HTTP (`internal/cola/cliente_control.go`) deriva la URL base eliminando el sufijo `/productos` de `SCRAPER_API_URL` (`urlBase()`). Todas las peticiones envían `Accept: application/json` y `Authorization: Bearer <SCRAPING_SERVICE_TOKEN>`, no siguen redirecciones HTTP (`http.ErrUseLastResponse`) y limitan la lectura de respuesta a `4 MiB` (`MaxResponseBytes = 4 << 20`).

### 1. Ingesta de lote de productos

- **Ruta:** `POST <urlBase>/trabajos/{trabajo_id}/productos` (si `trabajo_id` estuviera vacío, usa `<SCRAPER_API_URL>` asegurando el sufijo `/productos`).
- **Headers:** `Content-Type: application/json`, `Accept: application/json`, `Authorization: Bearer <SCRAPING_SERVICE_TOKEN>`, `Idempotency-Key: <lote_id>` (UUID del lote en la tabla `batches`).
- **Cuerpo de solicitud (`cargaIngesta` y `dominio.Product`):**
  ```json
  {
    "productos": [
      {
        "producto": "Leche Entera 1 L",
        "precio": 1190,
        "precio_normal": 1390,
        "precio_oferta": 1190,
        "ean_gtin": "7802900000000",
        "sku": "123456",
        "marca": "Marca",
        "formato_crudo": "1 L",
        "mecanica_promocion": null,
        "en_stock": true,
        "url_producto": "https://www.jumbo.cl/leche-entera-1-l/p",
        "imagen": "https://www.jumbo.cl/img/123456.jpg",
        "categoria": "lacteos"
      }
    ],
    "supermercado": "Jumbo"
  }
  ```
  *(Nota: `cargaIngesta` también declara `sucursal_id` y `codigo_sucursal` con `omitempty`; en `dominio.Product` ningún campo puntero lleva `omitempty`, por lo que los valores ausentes se envían siempre como `null`).*
- **Validación de respuesta:**
  - Si el cuerpo contiene `"codigo": "entrega_pendiente"`, se trata como transitorio (`Status: 202`, `Permanent() == false`).
  - Cuando se envía `Idempotency-Key`, la respuesta `200 OK` no puede ser vacía y debe incluir números enteros tales que `total_recibidos == len(productos)` y `precios_registrados == insertados + actualizados`.
  - Se clasifican como errores permanentes (`Permanent() == true`) los estados HTTP `400`, `404` y `413`, o los códigos de error `"idempotency_conflict"`, `"entrega_fallida"`, `"entregas_fallidas"` y `"lote_rechazado"`.

### 2. Cierre de trabajo

- **Ruta:** `PUT <urlBase>/trabajos/{trabajo_id}/finalizar`
- **Headers:** `Content-Type: application/json`, `Accept: application/json`, `Authorization: Bearer <SCRAPING_SERVICE_TOKEN>`.
- **Cuerpo de solicitud:**
  - Obligatorio: `"estado"` (`"completado"` o `"fallido"`).
  - Opcionales (se incluyen sólo cuando están presentes): `"elementos_extraidos"` (`int`), `"respuestas_bloqueadas"` (`int`), `"registro_errores"` (`string`).
- **Validación de respuesta:** exige un JSON con `id == trabajo_id` y `estado == estado_enviado`.

### 3. Consulta de estado de trabajo

- **Ruta:** `GET <urlBase>/trabajos/{trabajo_id}`
- **Headers:** `Accept: application/json`, `Authorization: Bearer <SCRAPING_SERVICE_TOKEN>`.
- **Validación de respuesta:** exige un JSON con `id == trabajo_id` y `estado` perteneciente a `{"en_progreso", "completado", "fallido"}`.

### Antes / Ahora de contratos

En la compactación del servicio (SUP-388) **no hubo cambios de contrato**: se mantienen intactos los endpoints HTTP, cabeceras (`Authorization: Bearer`, `Idempotency-Key`), campos JSON de productos/lotes/trabajos, claves y scripts Lua de Redis Streams, y el esquema SQLite (`items`, `batches`, `progress`) junto con la función de huella `huellaProducto`.

---

## Configuración

Todas las variables de entorno se leen y validan al arrancar en `internal/config/config.go` (`FromEnv()` y `Validate()`).
- Una variable vacía equivale a no definirla y conserva el valor por defecto (salvo `SCRAPING_SERVICE_TOKEN`, que es obligatorio para construir el worker en `worker.New`).
- **Fallo rápido:** cualquier variable numérica con un entero mal formado (`"<CLAVE> debe ser un número entero"`) o fuera del rango permitido por `Validate()` hace fallar el arranque inmediatamente.
- **Categorías adicionales por entorno (`*_CATEGORY_URLS`):** `leerURLsCategoriaEntorno()` (`internal/config/config.go:268-282`) inspecciona todas las variables de entorno que terminan en el sufijo `_CATEGORY_URLS` (`config.SufijoURLsCategoria`), separa sus valores por coma y las indexa por nombre de variable en `Config.CategoryURLs`. Al construir el registro (`extraccion.NewRegistryFromConfig`), cada sitio toma las URLs de la variable declarada en su campo `category_env` de `spiders.json`.

| Variable | Valor por defecto en código (`config.Default()`) | ¿Compartida? (`GUIA_CONFIGURACION.md`) | Propósito y restricciones de validación |
| --- | --- | :---: | --- |
| `REDIS_URL` | `redis://localhost:6379/0` | **Sí (§3)** | Cadena de conexión al broker Redis Streams. |
| `SCRAPER_API_URL` | `http://localhost:8082/api/v1/scraper/productos` | **Sí (§4 exc.)** | Endpoint HTTP de ingesta en Control (vía Gateway); de él se deriva la raíz `/trabajos/{id}`. |
| `SCRAPING_SERVICE_TOKEN` | `""` (vacío) | **Sí (§2)** | Token Bearer M2M hacia Control de Scraping. Obligatorio en `worker.New`. |
| `SCRAPER_QUEUE_NAME` | `scraper:jobs:stream` | No | Nombre del stream de Redis de donde se consumen los trabajos. |
| `SCRAPER_GROUP` | `scraper-workers` | No | Nombre del consumer group de Redis Streams. |
| `SCRAPER_STATE_DIR` | `/var/lib/scraper` | No | Directorio base donde se crean los spools SQLite (`<trabajo_id>/outbox.sqlite`). |
| `SCRAPER_STATE_MIN_FREE_BYTES` | `67108864` (64 MiB) | No | Reserva mínima de espacio libre en el volumen del spool. `Validate()` exige `>= 1048576` (1 MiB). |
| `SCRAPER_FAILED_RETENTION_SECONDS` | `86400` (24 h) | No | Segundos que se conserva el spool de un trabajo `fallido` sin presión de disco. `Validate()` exige `>= 0`. |
| `SCRAPER_LEASE_MS` | `60000` (60 s) | No | Duración de la lease de un mensaje en Redis (en ms). `Validate()` exige `>= 10000` (10 s). |
| `SCRAPER_API_TIMEOUT` | `30` (30 s) | No | Timeout HTTP (en segundos) de cada petición a Control de Scraping. `Validate()` exige `> 0`. |
| `SCRAPER_API_BATCH_SIZE` | `100` | No | Cantidad de productos por lote sellado y enviado. `Validate()` exige entre `1` y `1000`. |
| `SCRAPER_CATEGORY_DIR` | `/app/research` | No | Directorio donde se leen y actualizan los archivos `<cadena>_categories.txt`. |
| `SCRAPER_SPIDERS_FILE` | `""` (usa embebido) | No | Ruta opcional a un archivo JSON externo para reemplazar el catálogo embebido `internal/extraccion/spiders.json`. |
| `<SPIDER>_CATEGORY_URLS` (`JUMBO_CATEGORY_URLS`, `SANTA_ISABEL_CATEGORY_URLS`, `CUGAT_CATEGORY_URLS`, `ACUENTA_CATEGORY_URLS`, `LIDER_CATEGORY_URLS`) | vacío | No | Listas opcionales de URLs de categoría separadas por coma. Se detectan por el sufijo `_CATEGORY_URLS` y se enlazan mediante `category_env` de `spiders.json`. |
| `LOG_LEVEL` | `INFO` | No | Nivel de logging estructurado (`DEBUG`, `INFO`, `WARN`/`WARNING`, `ERROR`). |
| `SCRAPER_USER_AGENT` | `TallerIntegracionIII/1.0 (contacto del proyecto)` | No | Cabecera `User-Agent` enviada por el crawler HTTP y usada al evaluar `robots.txt`. |
| `SCRAPER_DOWNLOAD_DELAY_MS` | `2000` (2 s) | No | Pausa mínima en milisegundos entre peticiones consecutivas al mismo host. `Validate()` exige `>= 0`. |
| `SCRAPER_DOWNLOAD_TIMEOUT` | `30` (30 s) | No | Timeout HTTP por petición del crawler (en segundos). `Validate()` exige `> 0`. |
| `SCRAPER_DOWNLOAD_MAXSIZE` | `5242880` (5 MiB) | No | Tamaño máximo en bytes aceptado para el cuerpo de una respuesta HTTP. `Validate()` exige `> 0`. |
| `SCRAPER_RETRY_TIMES` | `3` | No | Número máximo de reintentos HTTP ante errores transitorios o bloqueos. `Validate()` exige `>= 0`. |
| `SCRAPER_MAX_PAGES` | `100` | No | Límite máximo de páginas a recorrer por cada categoría. `Validate()` exige `>= 1`. |
| `SCRAPER_ROBOTS_CACHE_TTL_SECONDS` | `3600` (1 h) | No | TTL en segundos de la caché en memoria de `robots.txt` descargados con éxito. `Validate()` exige `> 0`. |
| `SCRAPER_ROBOTS_FAILURE_TTL_SECONDS` | `600` (10 min) | No | TTL en segundos de la caché negativa cuando `robots.txt` falla (modo fail-open). `Validate()` exige `> 0`. |
| `SCRAPER_ROBOTS_MAX_BYTES` | `524288` (512 KiB) | No | Tamaño máximo en bytes leído al descargar `robots.txt`. `Validate()` exige `> 0`. |
| `SCRAPER_BACKOFF_MIN_MS` | `10` (10 ms) | No | Límite inferior en milisegundos del jitter exponencial entre reintentos HTTP. `Validate()` exige `> 0`. |
| `SCRAPER_BACKOFF_MAX_SECONDS` | `120` (120 s) | No | Tope máximo en segundos de espera entre reintentos HTTP (incluido `Retry-After`). `Validate()` exige `> 0`. |
| `SCRAPER_REDIS_TIMEOUT_SECONDS` | `5` (5 s) | No | Timeout en segundos de conexión, lectura y escritura del cliente Redis. `Validate()` exige `> 0`. |
| `SCRAPER_REDIS_PING_TIMEOUT_SECONDS` | `5` (5 s) | No | Timeout en segundos del `PING` inicial a Redis al arrancar el worker. `Validate()` exige `> 0`. |
| `SCRAPER_QUEUE_BLOCK_MS` | `1000` (1 s) | No | Tiempo de bloqueo en milisegundos de `XREADGROUP` esperando mensajes nuevos. `Validate()` exige `>= 0`. |
| `SCRAPER_JOB_CLOSED_TTL_SECONDS` | `86400` (24 h) | No | TTL en segundos de la clave `scraper:jobs:closed:<trabajo_id>` tras hacer ACK. `Validate()` exige `> 0`. |
| `SCRAPER_POLL_INTERVAL_MS` | `200` (200 ms) | No | Pausa en milisegundos del bucle principal cuando la cola no entrega mensajes. `Validate()` exige `> 0`. |
| `SCRAPER_RETRY_INTERVAL_SECONDS` | `5` (5 s) | No | Espera en segundos antes de reintentar cuando Redis o el volumen del spool fallan. `Validate()` exige `> 0`. |
| `SCRAPER_COLLECT_INTERVAL_SECONDS` | `10` (10 s) | No | Intervalo en segundos entre pasadas del recolector de spools (`Collector.Collect`). `Validate()` exige `> 0`. |
| `SCRAPER_COLLECT_LIMIT` | `20` | No | Máximo de directorios de spool evaluados por cada pasada del recolector. `Validate()` exige `>= 1`. |
| `SCRAPER_COLLECT_TIMEOUT_SECONDS` | `5` (5 s) | No | Presupuesto máximo de tiempo en segundos para una pasada del recolector. `Validate()` exige `> 0`. |
| `SCRAPER_GC_TIMEOUT_SECONDS` | `2` (2 s) | No | Timeout HTTP en segundos del cliente que consulta `GET /trabajos/{id}` durante la recolección (acotado por `SCRAPER_API_TIMEOUT` si éste es menor). `Validate()` exige `> 0`. |
| `SCRAPER_CRAWL_ATTEMPTS` | `3` | No | Intentos máximos de crawl por trabajo (`progress["crawl_attempts"]`) antes de fallar con `ErrCrawlExhausted`. `Validate()` exige `>= 1`. |

*(Además, el runtime de Go reconoce `GOMEMLIMIT`, que en Kubernetes se fija en `105MiB` y se documenta como comentario opcional en `backend/scraper_go/.env.example`).*

---

## Catálogo de spiders y archivos de categorías

El catálogo declarativo vive embebido en `internal/extraccion/spiders.json` (`//go:embed spiders.json` en `internal/extraccion/catalogo.go:18-19`) y puede sustituirse en tiempo de arranque definiendo `SCRAPER_SPIDERS_FILE` con la ruta a otro JSON válido. Al cargarse (`extraccion.Load`), el servicio valida que cada sitio declare `name`, `parser`, `supermarket`, `category_file`, `allowed_hosts`, mensajes de validación de URL y todos los selectores, ajustes, límites y listas requeridos por su parser (`validarConfiguracionParser`).

| Spider (`name`) | Parser (`parser`) | Supermercado (`supermarket`) | Archivo en `categories/` (`category_file`) | Variable de categorías extra (`category_env`) | Hosts permitidos (`allowed_hosts`) |
| --- | --- | --- | --- | --- | --- |
| `jumbo_rsc` | `jumbo` | `Jumbo` | `jumbo_categories.txt` | `JUMBO_CATEGORY_URLS` | `jumbo.cl`, `www.jumbo.cl` |
| `santa_isabel_rsc` | `santa_isabel` | `Santa Isabel` | `santa_isabel_categories.txt` | `SANTA_ISABEL_CATEGORY_URLS` | `santaisabel.cl`, `www.santaisabel.cl` |
| `cugat_rsc` | `cugat` | `Cugat` | `cugat_categories.txt` | `CUGAT_CATEGORY_URLS` | `cugat.cl`, `www.cugat.cl` |
| `acuenta_rsc` | `acuenta` | `A Cuenta` | `acuenta_categories.txt` | `ACUENTA_CATEGORY_URLS` | `acuenta.cl`, `www.acuenta.cl` |
| `lider_rsc` | `lider` | `Lider` | `lider_categories.txt` | `LIDER_CATEGORY_URLS` | `super.lider.cl` |

- **Resolución de URLs de categoría (`internal/extraccion/spiders.go:260-277`):** si el `Job` no trae `CategoryURLs` explícitas, el spider lee `<SCRAPER_CATEGORY_DIR>/<category_file>` (ignorando líneas vacías y comentarios `#`), concatena las URLs de la variable `category_env`, elimina duplicados preservando el orden y, si la lista resultante queda vacía, usa `default_categories` de `spiders.json`. Todas las URLs se validan contra `ValidateCategoryURL` (HTTPS obligatorio, host en `allowed_hosts` y prefijo/sufijo/segmentos requeridos).
- **Adición dinámica (`add_url`):** si el payload del trabajo incluye `add_url`, el spider valida la URL con `ValidateCategoryURL` y la agrega al archivo `<SCRAPER_CATEGORY_DIR>/<category_file>` si no existía (`guardarURLCategoria`).

---

## Operación

### Construcción de la imagen (`backend/scraper_go/Dockerfile`)

El `Dockerfile` usa el contexto de la raíz del repositorio para compilar dos binarios estáticos (`CGO_ENABLED=0`) sobre `golang:1.26-alpine` y empaquetarlos en `alpine:3.22`:
- `/app/scraper-worker` (entrypoint por defecto de la imagen, compilado desde `./cmd/worker`).
- `/app/scraper-extract` (compilado desde `./cmd/extract`).
- Copia `backend/scraper_go/categories/*_categories.txt` en `/app/research/` y fija `ENV SCRAPER_STATE_DIR=/var/lib/scraper` y `SCRAPER_CATEGORY_DIR=/app/research`.

```bash
docker build -f backend/scraper_go/Dockerfile -t scraper-go:local .
```

### Docker Compose (`docker-compose.yml:301-329`)

En `docker-compose.yml` el servicio se define como `scraper_worker` (`container_name: scraper_worker`):
- Depende de `redis` y `gateway` (`condition: service_healthy`).
- Conecta con `REDIS_URL=redis://redis:6379/0` y `SCRAPER_API_URL=http://gateway:8080/api/v1/scraper/productos`, exigiendo `SCRAPING_SERVICE_TOKEN`.
- Monta el volumen persistente `scraperdata:/var/lib/scraper` (`SCRAPER_STATE_DIR=/var/lib/scraper`, `SCRAPER_CATEGORY_DIR=/app/research`, `SCRAPER_LEASE_MS=60000`, `SCRAPER_STATE_MIN_FREE_BYTES=67108864`, `SCRAPER_FAILED_RETENTION_SECONDS=86400`).
- Límite de recursos en Compose: `cpus: '0.5'` y `memory: 200M`.

### Kubernetes (`infrastructure/k8s/scraper-go/`)

- `deployment.yaml`: `Deployment` `scraper-go` con `replicas: 1` y estrategia `Recreate` (un único consumidor escribe sobre el PVC de spools). Toma `SCRAPING_SERVICE_TOKEN` del Secret `scraping-service-secret` (`key: token`), apunta a `REDIS_URL=redis://redis:6379/0` y `SCRAPER_API_URL=http://api:8080/api/v1/scraper/productos`, define `GOMEMLIMIT=105MiB`, `requests` de `50m` CPU / `64Mi` RAM y `limits` de `500m` CPU / `128Mi` RAM. Monta el PVC `scraper-state` en `/var/lib/scraper` y un `emptyDir` de `256Mi` en `/tmp`.
- `pvc.yaml`: `PersistentVolumeClaim` `scraper-state` (`ReadWriteOnce`, `1Gi`).

### Validación de HTML sin Redis ni API (`cmd/extract`)

El binario `/app/scraper-extract` (`cmd/extract/main.go`) permite probar los spiders directamente contra URLs o archivos locales y emite cada producto normalizado (`CompleteItem`) como una línea JSON en `stdout`, reportando `extraidos=<N>` en `stderr`.

Flags reales de `cmd/extract/main.go:55-66`:
- `-spider <nombre>`: spider a ejecutar (por defecto `jumbo_rsc`).
- `-url <url>`: URL de categoría o de producto; se puede repetir varias veces.
- `-file <ruta>`: archivo con URLs adicionales (una por línea; ignora vacías y `#`).
- `-product`: tratar la URL como ficha individual de producto (requiere exactamente una URL entre `-url` y `-file`).
- `-enrich-ean`: consultar la ficha individual para completar el EAN cuando falte en el listado (implementado en Jumbo).
- `-max-pages <int>`: máximo de páginas a recorrer por categoría (por defecto `100`, debe ser `>= 1`).
- `-category-dir <ruta>`: directorio de archivos `<cadena>_categories.txt` (si se omite, usa `SCRAPER_CATEGORY_DIR` o busca `backend/scraper_go/categories` y `categories`).
- `-delay <duration>`: pausa entre peticiones HTTP (por defecto `-1`, que conserva `SCRAPER_DOWNLOAD_DELAY_MS`).
- `-timeout <duration>`: timeout por petición HTTP (por defecto `0`, que conserva `SCRAPER_DOWNLOAD_TIMEOUT`).
- `-retries <int>`: reintentos HTTP (por defecto `-1`, que conserva `SCRAPER_RETRY_TIMES`).

Ejemplo de ejecución con la imagen local:

```bash
# Extraer la primera página de una categoría
docker run --rm --network host \
  --entrypoint /app/scraper-extract scraper-go:local \
  -spider jumbo_rsc \
  -url 'https://www.jumbo.cl/frutas-y-verduras/verduras' \
  -max-pages 1

# Extraer una ficha individual de producto
docker run --rm --network host \
  --entrypoint /app/scraper-extract scraper-go:local \
  -spider jumbo_rsc -product \
  -url 'https://www.jumbo.cl/palta-hass-jumbo-malla-1-kg/p'
```

---

## Pruebas

El workflow `.github/workflows/scraper-go-checks.yml` verifica formato (`gofmt -l .`), pruebas con detector de condiciones de carrera (`go test -race ./...`), dos pasadas del gate de memoria (`TestMemoryGate`), análisis estático (`go vet ./...`), compilación de `./cmd/worker` y construcción de la imagen Docker.

### 1. Pruebas unitarias, de integración y análisis estático

Desde `backend/scraper_go`:

```bash
gofmt -l .
go vet ./...
go test -race ./...
```

- **Integración con Redis (`REDIS_TEST_URL`):** los paquetes `tests/integration/cola` y `tests/integration/worker` se conectan a `REDIS_TEST_URL` (o a `redis://localhost:6379/15` si no está definida). Si `REDIS_TEST_URL` no está definida y no hay un Redis local disponible, esas pruebas hacen `SKIP`; si `REDIS_TEST_URL` **está** definida (como en CI: `REDIS_TEST_URL=redis://localhost:6379/15`) y Redis no responde, las pruebas fallan con `Fatalf`.

### 2. Gate de memoria (`tests/performance/worker/memory_test.go`)

`TestMemoryGate` ejecuta 200.000 productos sintéticos a través de `cola.Open`, `CompleteItem`, `Spool.Add`, `Spool.Seal`, `Spool.Replay` y `Client.FinalizeWork` contra un servidor HTTP local de prueba. Se omite en `go test ./...` normal a menos que `SCRAPER_MEMORY_GATE=1` (requiere Linux para leer `/proc/self/status`).

Variables que lee `tests/performance/worker/memory_test.go`:
- `SCRAPER_MEMORY_GATE`: debe ser `"1"` para activar la prueba (`memory_test.go:44`).
- `SCRAPER_MEMORY_ITEMS`: cantidad de productos sintéticos a procesar (por defecto `200000`, debe ser `>= 1`, `memory_test.go:51`).
- `SCRAPER_MEMORY_LIMIT_MB`: límite máximo en MB para el pico `VmHWM` (`PeakRSSMB`) y para `RssAnon` (`PeakAnonRSSMB`) muestreado cada `10 ms` (por defecto `150`, `memory_test.go:52,205`).
- `SCRAPER_MEMORY_COMPACT`: si es `"1"`, usa un producto sintético mínimo para correr sobre `tmpfs` dentro de un cgroup estricto sin agotar el cgroup por archivos en `/tmp` (`memory_test.go:53,352-357`).
- `SCRAPER_MEMORY_CGROUP_LIMIT_MB`: límite informativo del cgroup en MB; cuando es `> 0`, lee además `/sys/fs/cgroup/memory.peak` (o `memory.max_usage_in_bytes`) y lo reporta en `cgroup_peak_mb` (`memory_test.go:185-189,308-324`).
- `SCRAPER_MEMORY_RESULT`: ruta opcional donde escribe el reporte JSON con `items`, `compact`, `limit_mb`, `baseline_rss_mb`, `peak_rss_mb`, `peak_anon_rss_mb`, `cgroup_limit_mb`, `cgroup_peak_mb`, `go_version` y `measured_at` (`memory_test.go:213-245`).

Ejecución del gate en host y en cgroup de 192 MB (tal como en `.github/workflows/scraper-go-checks.yml:60-99`):

```bash
cd backend/scraper_go

# 1. Gate en host (200k productos con payload realista, tope 150 MB RSS / RssAnon)
SCRAPER_MEMORY_GATE=1 \
SCRAPER_MEMORY_ITEMS=200000 \
SCRAPER_MEMORY_LIMIT_MB=150 \
SCRAPER_MEMORY_RESULT=ram-result.json \
  go test -run TestMemoryGate -count=1 -timeout 10m ./tests/performance/worker

# 2. Gate en contenedor con cgroup de 192 MB (verifica ausencia de OOMKilled)
CGO_ENABLED=0 go test -c -o scraper-memory.test ./tests/performance/worker
docker run --rm --name scraper-ram-192 \
  --memory=192m --memory-swap=192m \
  --tmpfs /tmp:rw,size=512m \
  -v "$PWD:/src" -w /src \
  -e SCRAPER_MEMORY_GATE=1 \
  -e SCRAPER_MEMORY_ITEMS=200000 \
  -e SCRAPER_MEMORY_COMPACT=1 \
  -e SCRAPER_MEMORY_LIMIT_MB=150 \
  -e SCRAPER_MEMORY_CGROUP_LIMIT_MB=192 \
  -e SCRAPER_MEMORY_RESULT=/src/ram-result-192.json \
  alpine:3.22 \
  /src/scraper-memory.test -test.run '^TestMemoryGate$' -test.count=1 -test.timeout=10m -test.v
```
