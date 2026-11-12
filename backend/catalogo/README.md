# Catálogo y precios

Servicio Go independiente para consultar productos, precios, historial y catálogo
administrativo, y persistir lotes enviados por coordinación de scraping mediante
una API interna autenticada. No requiere Redis.

## Configuración y ejecución local

El servicio lee variables de entorno del proceso; no carga archivos `.env`
automáticamente. Para desarrollo local:

```bash
cd backend/catalogo
cp .env.example .env
# Configurar DB_URL, JWT_SECRET y CATALOGO_INTERNAL_TOKEN en .env
set -a
source .env
set +a
go run ./cmd/catalogo
```

### Variables de entorno

| Variable | Requisito / Predeterminado | Descripción |
| --- | --- | --- |
| `DB_URL` | Obligatoria | Conexión PostgreSQL con usuario `app_catalogo` |
| `JWT_SECRET` | Obligatoria | Clave HMAC HS256 compartida con Identidad para validar tokens de administración |
| `CATALOGO_INTERNAL_TOKEN` | Opcional | Clave Bearer para `/internal/v1/*`. Vacía activa el modo sólo lectura |
| `PORT` | `8080` (ejemplo `8083`) | Puerto de escucha HTTP |
| `CATALOGO_CORS_ALLOWED_ORIGINS` | Vacía | Orígenes HTTP(S) exactos separados por comas. Si está vacía se usa `CORS_ALLOWED_ORIGINS`. Vacías ambas, se rechazan las peticiones con `Origin` |
| `DB_MAX_OPEN_CONNS` | `10` | Límite de conexiones abiertas al pool de PostgreSQL |
| `DB_MAX_IDLE_CONNS` | `5` | Conexiones inactivas máximas (no puede superar `DB_MAX_OPEN_CONNS`) |
| `DB_CONN_MAX_LIFETIME` | `30m` | Tiempo máximo de vida por conexión |
| `CATALOGO_READ_HEADER_TIMEOUT` | `5s` | Lectura de cabeceras de la petición |
| `CATALOGO_READ_TIMEOUT` | `30s` | Lectura completa de la petición |
| `CATALOGO_WRITE_TIMEOUT` | `30s` | Escritura de la respuesta |
| `CATALOGO_IDLE_TIMEOUT` | `60s` | Conexión keep-alive inactiva |
| `CATALOGO_SHUTDOWN_TIMEOUT` | `15s` | Espera de peticiones activas al recibir SIGINT/SIGTERM |
| `CATALOGO_STARTUP_TIMEOUT` | `5s` | Verificación de tablas y permisos al arrancar |
| `CATALOGO_READINESS_TIMEOUT` | `2s` | Ping a PostgreSQL de `/_catalogo/ready` |
| `CATALOGO_DEFAULT_PAGE_SIZE` | `20` | Tamaño de página cuando no se envía `limit` |
| `CATALOGO_MAX_PAGE_SIZE` | `100` | Tope de `limit` en listados públicos y de administración |
| `CATALOGO_MAX_SEARCH_LENGTH` | `100` | Bytes conservados del texto de búsqueda (una letra con tilde ocupa 2) |
| `CATALOGO_MAX_BATCH_PRODUCTOS` | `1000` | Productos aceptados por lote de ingesta |
| `CATALOGO_MAX_BODY_BYTES` | `8388608` (8 MiB) | Bytes aceptados en el cuerpo JSON de la ingesta; más allá responde 413 |

Las duraciones usan el formato de Go (`500ms`, `5s`, `30m`) y deben ser positivas;
los límites, enteros positivos. Un valor inválido impide arrancar con un mensaje
que nombra la variable. `CATALOGO_DEFAULT_PAGE_SIZE` no puede superar
`CATALOGO_MAX_PAGE_SIZE`.

### CORS y seguridad de errores

`CATALOGO_CORS_ALLOWED_ORIGINS` (o `CORS_ALLOWED_ORIGINS`) contiene orígenes HTTP(S) exactos sin rutas ni comodines.
Si está vacío o el origen no coincide, las peticiones con cabecera `Origin`
reciben 403; las peticiones sin `Origin` continúan funcionando. Las cabeceras
permitidas se limitan a `Content-Type`, `Authorization` y `X-Request-ID`, y los
métodos a `GET, PUT, OPTIONS`. CORS no reemplaza autenticación ni protección CSRF.

Los errores internos 500 y recuperaciones de pánico devuelven respuestas genéricas
sin volcar trazas ni detalles SQL. El diagnóstico se registra internamente con
`slog`, ruta normalizada y el identificador `X-Request-ID` propagado o generado.

### Autenticación administrativa

`JWT_SECRET` valida de forma local la firma HS256, expiración y claim de rol
`admin` sin consultar a Identidad. Esto mantiene la disponibilidad del catálogo
ante caídas de Identidad. Una cuenta desactivada o degradada conserva acceso
hasta que expire el token (24 h); una revocación inmediata requerirá lista de
revocación desacoplada o tokens de corta duración sin reintroducir llamadas
síncronas a Identidad. No se aceptan tokens sin expiración ni claims declarados
en cabeceras arbitrarias.

## Rutas HTTP

### Rutas públicas y administrativas

| Método | Ruta | Acceso | Descripción |
| --- | --- | --- | --- |
| GET | `/api/v1/productos` | Público | Listado paginado de productos con filtros |
| GET | `/api/v1/productos/buscar` | Público | Búsqueda por texto (parámetro `q`, mínimo 3 caracteres) |
| GET | `/api/v1/productos/:id` | Público | Detalle de producto con historial de precios |
| GET | `/api/v1/admin/productos` | Admin (JWT) | Catálogo administrativo con paginación |
| GET | `/api/v1/admin/productos/` | Admin (JWT) | Variante con barra final de la ruta administrativa |
| GET | `/api/v1/admin/productos/fuentes` | Admin (JWT) | Lista de cadenas con estado de fuente (`habilitada` / `esta_activa`) |
| PUT | `/api/v1/admin/productos/fuentes/:id` | Admin (JWT) | Actualiza el flag `esta_activa` de la cadena seleccionada |

### Sondas de salud

| Método | Ruta | Acceso | Descripción |
| --- | --- | --- | --- |
| GET | `/_catalogo/live` | Público | Sonda de vida (liveness/startup); responde 200 en memoria sin consultar PostgreSQL |
| GET | `/_catalogo/ready` | Público | Sonda de preparación (readiness); verifica PostgreSQL con timeout de 2 s (503 si falla) |
| GET | `/health` | Público | Alias de preparación equivalente a `/_catalogo/ready` |

### API interna de ingesta

Rutas privadas bajo el prefijo `/internal/v1` destinadas a la coordinación de
scraping. No se exponen a través del Gateway ni en Ingress público:

| Método | Ruta | Payload JSON | Descripción |
| --- | --- | --- | --- |
| POST | `/internal/v1/cadenas/resolver` | `{"cadena_id": int, "nombre": string}` | Resuelve o registra una cadena con lock transaccional |
| POST | `/internal/v1/sucursales/resolver` | `{"cadena_id": int, "sucursal_id": *int, "codigo_sucursal": *string}` | Resuelve o registra una sucursal para la cadena |
| POST | `/internal/v1/ingestas` | `{"sucursal_id": int, "lote_id": string, "productos": [...]}` | Persistencia atómica de 1 a 1000 productos y capturas de precio |

## Ingesta interna y token privado

Las operaciones de `/internal/v1` aplican las siguientes reglas:

1. **Autenticación en tiempo constante:** requieren la cabecera
   `Authorization: Bearer <CATALOGO_INTERNAL_TOKEN>`. La comparación es segura
   frente a ataques de temporización (`subtle.ConstantTimeCompare`).
2. **Rechazo estricto de CORS:** cualquier petición a rutas internas que incluya
   cabecera `Origin` se rechaza con HTTP 403 (`internal_origin_forbidden`),
   incluso si presenta un token válido.
3. **Límites del cuerpo HTTP:** tamaño máximo `CATALOGO_MAX_BODY_BYTES` (8 MiB
   por defecto, HTTP 413 si se supera), rechazo de campos JSON desconocidos
   (`DisallowUnknownFields`), rechazo de contenido extra tras el cierre del JSON
   y lote acotado de 1 a `CATALOGO_MAX_BATCH_PRODUCTOS` productos (1000 por defecto).
4. **Modo sólo lectura:** si `CATALOGO_INTERNAL_TOKEN` está vacío, el servidor
   no registra las rutas internas y opera únicamente como lector.
5. **Seguridad del token:** debe ser una cadena aleatoria de al menos 32
   caracteres (generable con `openssl rand -hex 32`), privada y estrictamente
   distinta de `JWT_SECRET`. En Kubernetes se inyecta desde el Secret
   `catalogo-internal-secret` (clave `token`).

## Base de datos y permisos (rol `app_catalogo`)

Catálogo opera con el rol PostgreSQL `app_catalogo`, definido en
[06_propiedad_permisos.sql](../../infrastructure/db/06_propiedad_permisos.sql) y
documentado en [datos y permisos del backend](../../docs/Guias/GUIA_DATOS_Y_PERMISOS_BACKEND.md):

- **Esquema:** `USAGE` en `scraper`.
- **Lectura y escritura:**
  - `SELECT, INSERT, UPDATE` en `scraper.productos_crudos`.
  - `SELECT, INSERT` en `scraper.cadenas_supermercado`.
  - `UPDATE (esta_activa)` acotado a nivel de columna en `scraper.cadenas_supermercado`
    (administración de fuentes, SUP-243); no tiene permiso `UPDATE` sobre la tabla completa.
  - `SELECT, INSERT` en `scraper.sucursales_supermercado`.
  - `SELECT, INSERT` en `scraper.capturas_precios`.
  - `SELECT, INSERT` en `scraper.lotes_catalogo` (recibos de idempotencia).
- **Secuencias:** `USAGE` en `cadenas_supermercado_id_seq`,
  `sucursales_supermercado_id_seq` y `capturas_precios_id_seq`.
- **Límites de aislamiento:** sin permisos en esquemas `api` ni `rutas`, sin
  acceso a tablas de coordinación (`scraper.trabajos_scraper`, `scraper.entregas_ingesta`,
  `scraper.entregas_trabajos`), y sin privilegios de `DELETE` ni `TRUNCATE`.

En el arranque, `ingesta.VerifyStorage` valida estas tablas y permisos en un plazo
de 5 s antes de abrir el puerto HTTP. Si los privilegios son insuficientes, el
proceso aborta. Durante la ejecución normal, la sonda de preparación únicamente
comprueba conectividad mediante `PingContext`.

## Ingesta idempotente y consistencia

Catálogo persiste productos y precios de forma atómica:

- **Transacción única:** cada lote se ejecuta en una sola transacción PostgreSQL;
  cualquier fallo SQL revierte íntegramente las inserciones y actualizaciones.
- **Idempotencia con `lote_id`:** cuando la solicitud incluye un identificador
  de lote (UUID), Catálogo adquiere un lock transaccional
  `pg_advisory_xact_lock(2701, hashtext(lote_id))` y calcula la huella SHA-256
  (`payload_hash`) del contenido.
  - Si el lote ya existe en `scraper.lotes_catalogo` con el mismo hash, retorna
    de inmediato el resultado previo sin duplicar capturas ni actualizar contadores.
  - Si el `lote_id` ya fue usado con un contenido distinto, retorna HTTP 409
    (`idempotency_conflict`).
  - Al completar el procesamiento exitoso, persiste el recibo en
    `scraper.lotes_catalogo` (`id`, `payload_hash`, `resultado`).
- **Upsert de productos:** la tabla `scraper.productos_crudos` actualiza por
  conflicto sobre `(sucursal_id, sku)`. Los campos descriptivos vacíos o nulos
  no sobrescriben información existente previa (`COALESCE`).
- **Captura de precios y disponibilidad:** cada ítem inserta un registro en
  `scraper.capturas_precios` con el estado `esta_disponible` explícito según el
  stock informado, evitando valores predeterminados desalineados.

## Ventanas de fallo con coordinación de scraping

Catálogo y Control de scraping no comparten transacción distribuida. Las
respuestas de esta tabla las emite **Control** (`backend/control_scraping`) al
cliente que entrega resultados; Catálogo solo confirma o rechaza el lote:

| Escenario | Respuesta de Control | Comportamiento |
| --- | --- | --- |
| Catálogo no responde o error SQL | HTTP 503 (`catalogo_unavailable`) | El lote no se confirma; coordinación no incrementa el contador del trabajo |
| Plazo de conexión excedido | HTTP 504 (`catalogo_timeout`) | Plazo del cliente HTTP de Control (`CATALOGO_INGEST_TIMEOUT`, por defecto 10 s); sin reintento ciego |
| Catálogo confirma pero falla el contador del trabajo | HTTP 409 (`contador_pendiente`, `catalogo_confirmado: true`) | El lote ya fue persistido; el reintento debe usar el mismo `lote_id` para resolver por idempotencia sin duplicar precios |
| Caída del coordinador tras confirmación de Catálogo | Resultado incierto | El reintento con el mismo `lote_id` es seguro gracias a `scraper.lotes_catalogo` |

## Despliegue y sondas

En Kubernetes ([deployment.yaml](../../infrastructure/k8s/catalogo/deployment.yaml)):

- **Usuario sin privilegios:** corre bajo UID/GID `65532:65532`, sin escalada de
  privilegios y con sistema de archivos raíz de sólo lectura (`readOnlyRootFilesystem: true`).
- **Sondas configuradas:**
  - `startupProbe`: `/_catalogo/live`, periodo 2 s, umbral de fallo 30 (hasta 60 s para verificación inicial de storage).
  - `livenessProbe`: `/_catalogo/live`, periodo 10 s, timeout 2 s.
  - `readinessProbe`: `/_catalogo/ready`, periodo 5 s, timeout 3 s.
- **Secretos:** `DB_URL` proviene de `catalogo-db-secret`, `JWT_SECRET` de `db-secret`
  y `CATALOGO_INTERNAL_TOKEN` de `catalogo-internal-secret`.

## Procedimientos de rollback

### Rollback de ingesta (Docker Compose)

Si se requiere revertir el cliente HTTP de ingesta y volver temporalmente al escritor
SQL histórico en coordinación de scraping, se utiliza
[docker-compose.catalogo-ingesta-rollback.yml](../../docker-compose.catalogo-ingesta-rollback.yml).
Catálogo permanece activo como servicio de consulta:

```bash
SUP269_LEGACY_API_IMAGE=<imagen_control_anterior> \
docker compose -f docker-compose.yml -f docker-compose.catalogo-ingesta-rollback.yml up -d control_scraping
```

El escritor histórico requiere credenciales con permisos compartidos sobre trabajos
y catálogo (`DB_USER`/`DB_PASSWORD` en `.env`).

### Rollback de consultas y enrutamiento (Docker Compose)

Para revertir el enrutamiento de Gateway y consultar productos a través de una imagen
anterior desacoplando Catálogo, se aplica
[docker-compose.catalogo-rollback.yml](../../docker-compose.catalogo-rollback.yml):

El override deja `CATALOGO_URL=''` en Gateway y devuelve las rutas públicas a una
imagen histórica de la API (`SUP268_LEGACY_API_IMAGE`). El orden importa: primero
debe estar sana la imagen histórica y solo después se cambia Gateway; si no, las
consultas de productos responden 404. El procedimiento paso a paso, con los
comandos exactos, está en el
[README del Gateway](../gateway/README.md) (sección «Rollback de Catálogo (SUP-268) en Compose»).

### Rollback en Kubernetes

Revertir la versión del despliegue con:

```bash
kubectl rollout undo deployment/catalogo
```

No se requieren migraciones SQL de reversión; el esquema y los permisos del rol
`app_catalogo` son compatibles de forma retroactiva.

## Pruebas y validación

Desde este directorio:

```bash
go test -race ./...
go vet ./...
go build -o /tmp/catalogo ./cmd/catalogo
```

Pruebas de integración desde la raíz del repositorio:

```bash
docker build -t catalogo:local backend/catalogo
bash scripts/smoke_catalogo_compose.sh
bash scripts/smoke_catalogo_ingesta_compose.sh
```
