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
| `CATALOGO_INTERNAL_TOKEN` | Opcional | Clave Bearer de escritura para la ingesta interna. Vacía deshabilita `/internal/v1/ingestas` y las rutas de resolución |
| `CATALOGO_CONSULTA_TOKEN` | Opcional | Clave Bearer de sólo lectura para `/internal/v1/productos/disponibilidad`. Vacía deshabilita la consulta |
| `PORT` | `8080` | Puerto de escucha HTTP |
| `CATALOGO_CORS_ALLOWED_ORIGINS` | Vacía | Orígenes HTTP(S) exactos separados por comas; tiene prioridad sobre `CORS_ALLOWED_ORIGINS`. Ambas vacías, se rechazan las peticiones con `Origin` |
| `CORS_ALLOWED_ORIGINS` | Vacía | Respaldo genérico. Compose la define a partir de `CATALOGO_CORS_ALLOWED_ORIGINS` del `.env` raíz |
| `DB_MAX_OPEN_CONNS` | `10` | Límite de conexiones abiertas al pool de PostgreSQL |
| `DB_MAX_IDLE_CONNS` | `5` | Conexiones inactivas máximas (no puede superar `DB_MAX_OPEN_CONNS`) |
| `DB_CONN_MAX_LIFETIME` | `30m` | Tiempo máximo de vida por conexión |
| `CATALOGO_MAX_PRODUCTOS_CONSULTA` | `1000` | IDs aceptados por lote en `/internal/v1/productos/disponibilidad` |
| `CATALOGO_DISPONIBILIDAD_ANTIGUEDAD_MAXIMA` | `48h` | Antigüedad máxima de la captura más reciente para considerarla vigente |
| `CATALOGO_TEST_DB_URL` | Sólo pruebas | Base PostgreSQL aislada con rol `app_catalogo` para `tests/integration/postgres_test.go`; vacía omite la prueba |

Las duraciones usan el formato de Go (`500ms`, `5s`, `30m`) y deben ser positivas;
los límites, enteros positivos. Un valor inválido impide arrancar con un mensaje
que nombra la variable. `JWT_SECRET` requiere al menos 32 bytes.

### Plazos y límites

Opcionales, con valor por defecto en `internal/config` (sección 3 de la guía de
barrido). Ningún manifiesto de Compose, K8s, `.github` ni `scripts/` las define
hoy, así que rige el valor por defecto.

| Variable | Por defecto | Uso |
| --- | --- | --- |
| `CATALOGO_READ_HEADER_TIMEOUT` / `CATALOGO_READ_TIMEOUT` | `5s` / `30s` | Lectura de cabeceras / petición completa |
| `CATALOGO_WRITE_TIMEOUT` / `CATALOGO_IDLE_TIMEOUT` | `30s` / `60s` | Escritura de respuesta / keep-alive inactivo |
| `CATALOGO_SHUTDOWN_TIMEOUT` / `CATALOGO_STARTUP_TIMEOUT` / `CATALOGO_READINESS_TIMEOUT` | `15s` / `5s` / `2s` | Apagado ordenado / verificación al arrancar / ping de `ready` |
| `CATALOGO_DEFAULT_PAGE_SIZE` / `CATALOGO_MAX_PAGE_SIZE` | `20` / `100` | Página sin `limit` / tope de `limit` (la primera no puede superar la segunda) |
| `CATALOGO_MAX_SEARCH_LENGTH` | `100` | Bytes conservados de cada texto de búsqueda |
| `CATALOGO_MAX_BATCH_PRODUCTOS` / `CATALOGO_MAX_BODY_BYTES` | `1000` / `8388608` (8 MiB) | Productos por lote / bytes de cuerpo interno (413 si se supera) |

### CORS y seguridad de errores

`CATALOGO_CORS_ALLOWED_ORIGINS` (o, si está vacía, `CORS_ALLOWED_ORIGINS`) contiene orígenes HTTP(S) exactos sin rutas ni comodines.
Si está vacío o el origen no coincide, las peticiones con cabecera `Origin`
reciben 403; las peticiones sin `Origin` continúan funcionando. Las cabeceras
permitidas se limitan a `Content-Type`, `Authorization` y `X-Request-ID`, y los
métodos a `GET, PUT, OPTIONS`. CORS no reemplaza autenticación ni protección CSRF.

Los errores internos 500 y recuperaciones de pánico devuelven respuestas genéricas
sin volcar trazas ni detalles SQL. El diagnóstico se registra internamente con
`slog`, ruta normalizada y el identificador `X-Request-ID` propagado o generado.

### Autenticación administrativa

`JWT_SECRET` valida de forma local la firma HS256, expiración y claim de rol
`admin` exacto sin consultar a Identidad. El alias histórico `administrador` y
las variantes con mayúsculas ya no se aceptan: Identidad sólo emite roles de
`usuarios_rol_check` (migración 10). Esto mantiene la disponibilidad del catálogo
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

### API interna de ingesta y disponibilidad

Rutas privadas bajo el prefijo `/internal/v1` destinadas a la coordinación de
scraping y a la consulta de disponibilidad de Listas e IA. No se exponen a
través del Gateway ni en Ingress público:

| Método | Ruta | Payload JSON | Descripción |
| --- | --- | --- | --- |
| POST | `/internal/v1/cadenas/resolver` | `{"cadena_id": int, "nombre": string}` | Resuelve o registra una cadena con lock transaccional |
| POST | `/internal/v1/sucursales/resolver` | `{"cadena_id": int, "sucursal_id": *int, "codigo_sucursal": *string}` | Resuelve o registra una sucursal para la cadena |
| POST | `/internal/v1/ingestas` | `{"sucursal_id": int, "lote_id": string, "productos": [...]}` | Persistencia atómica de 1 a 1000 productos y capturas de precio |
| POST | `/internal/v1/productos/disponibilidad` | `{"producto_ids": ["uuid", ...]}` | Consulta en lote de existencia, stock, precio, sucursal y captura más reciente |

## Ingesta interna y token privado

Las operaciones de `/internal/v1` aplican las siguientes reglas:

1. **Autenticación en tiempo constante:** la ingesta exige
   `Authorization: Bearer <CATALOGO_INTERNAL_TOKEN>` y la consulta de
   disponibilidad exige `Authorization: Bearer <CATALOGO_CONSULTA_TOKEN>`. La
   comparación es segura frente a ataques de temporización
   (`subtle.ConstantTimeCompare`).
2. **Rechazo estricto de CORS:** cualquier petición a rutas internas que incluya
   cabecera `Origin` se rechaza con HTTP 403 (`internal_origin_forbidden`),
   incluso si presenta un token válido.
3. **Límites del cuerpo HTTP:** tamaño máximo de 8 MiB (HTTP 413 si se supera), rechazo de campos JSON desconocidos
   (`DisallowUnknownFields`), rechazo de contenido extra tras el cierre del JSON
   y lote acotado de 1 a 1000 productos.
4. **Modo sólo lectura:** si `CATALOGO_INTERNAL_TOKEN` está vacío no se registra
   la ingesta; si `CATALOGO_CONSULTA_TOKEN` está vacío no se registra la
   consulta. Cada ruta depende sólo de su propio token.
5. **Seguridad de los tokens:** ambos deben ser cadenas aleatorias de al menos
   32 caracteres (generables con `openssl rand -hex 32`), privadas,
   estrictamente distintas de `JWT_SECRET` y distintas entre sí. En Kubernetes
   se inyectan desde el Secret `catalogo-internal-secret` (claves `token` y
   `consulta-token`); `consulta-token` es opcional y, si falta, la consulta
   queda deshabilitada sin impedir el arranque.

## Consulta interna de disponibilidad (SUP-312)

Listas e IA consultan esta ruta para verificar un producto antes de guardarlo o
mostrarlo. No se publica en el Gateway: sólo responde en la red interna de los
servicios y exige `CATALOGO_CONSULTA_TOKEN`, un token de sólo lectura distinto
del token de ingesta.

```http
POST /internal/v1/productos/disponibilidad
Authorization: Bearer <CATALOGO_CONSULTA_TOKEN>
Content-Type: application/json

{
  "producto_ids": [
    "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e",
    "1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f"
  ]
}
```

- Acepta entre 1 y `CATALOGO_MAX_PRODUCTOS_CONSULTA` IDs (1000 por defecto).
- Deduplica los IDs y conserva el orden de la primera aparición.
- Lee siempre la captura más reciente de `scraper.capturas_precios`. El stock y
  la fecha se toman de esa misma captura (`esta_disponible` y `capturado_el`),
  para que ambos correspondan a la vigencia evaluada.
- Una captura con antigüedad mayor a
  `CATALOGO_DISPONIBILIDAD_ANTIGUEDAD_MAXIMA` (48 h por defecto) no se presenta
  como vigente.
- Una captura con fecha futura se trata como no vigente, para no confiar en un
  reloj desfasado o en un dato erróneo. Para no ampliar el contrato se informa
  `estado_precio: "vencido"`; el consumidor debe tratarla como no vigente.
- Si `esta_disponible` es `NULL`, el scraper no confirmó el stock: se informa
  `estado_producto: "desconocido"` y `en_stock: null`, sin asumir disponibilidad.

**Límite conocido:** la ingesta actual convierte un `en_stock` ausente del lote en
`true` (`internal/repositories/ingesta.go`) y `scraper.capturas_precios.esta_disponible`
tiene `DEFAULT true`. Por eso, hoy `desconocido` por stock sólo aparece si la
base contiene un `NULL` explícito. Cambiar ese valor por defecto exige modificar
la ingesta y queda fuera del alcance de SUP-312.

```json
{
  "productos": [
    {
      "producto_id": "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e",
      "estado_producto": "disponible",
      "en_stock": true,
      "capturado_el": "2026-11-16T10:00:00Z",
      "estado_precio": "vigente",
      "producto": {
        "nombre": "Leche entera 1 L",
        "marca": "Colún",
        "categoria": "Lácteos",
        "supermercado": "Lider",
        "sucursal_id": 1,
        "sucursal": "Lider Ñuñoa",
        "unidad": "1 L",
        "precio": 990,
        "precio_normal": 1290,
        "precio_oferta": 990,
        "url_imagen": "https://..."
      }
    }
  ]
}
```

`estado_producto` puede ser:

| Valor | Significado | `en_stock` | Precio |
| --- | --- | --- | --- |
| `disponible` | Existe y la captura vigente indica stock | `true` | Sí |
| `sin_stock` | Existe y la captura vigente indica sin stock | `false` | Sí, si la captura es vigente |
| `no_encontrado` | El UUID no existe en `productos_crudos` | `null` | `null` |
| `desconocido` | Existe, pero no hay captura vigente o el scraper no confirmó el stock | `null` | `null` sin captura vigente; vigente si sólo falta el stock |

`estado_precio` puede ser `vigente`, `vencido` o `sin_captura`. Para
`desconocido` se conservan los datos descriptivos del producto y `capturado_el`.
Si la captura falta, está vencida o tiene fecha futura, no se publican precios
(`sin_captura` o `vencido`). Si sólo falta el stock, el precio vigente se
conserva y `en_stock` queda `null`. Para `no_encontrado`, `producto` es `null`.

Errores: `401 internal_unauthorized`, `403 internal_origin_forbidden`,
`400 invalid_batch`, `400 invalid_product_id`, `400 invalid_payload`,
`413` para cuerpos mayores a 8 MiB y
`500 catalogo_internal_error` sin detalle SQL.

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

En el arranque, `repositories.VerificarAlmacenamiento` valida estas tablas y permisos en un plazo
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
  `scraper.capturas_precios` con `esta_disponible` según el stock informado. Si
  el lote omite `en_stock`, la ingesta asume `true` (ver límite conocido).

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
  y `CATALOGO_INTERNAL_TOKEN`/`CATALOGO_CONSULTA_TOKEN` de
  `catalogo-internal-secret` (claves `token` y `consulta-token`; la segunda es
  opcional).

## Procedimientos de rollback

### Rollback en Kubernetes

Revertir la versión del despliegue con:

```bash
kubectl rollout undo deployment/catalogo
```

No se requieren migraciones SQL de reversión; el esquema y los permisos del rol
`app_catalogo` son compatibles de forma retroactiva.

## Estructura

- `cmd/catalogo`: arranque, verificación de permisos y apagado ordenado.
- `internal/config`: lectura única del entorno y plazos/límites fijos.
- `internal/api`: router, sondas, CORS, correlación, sesión JWT, rutas internas,
  handlers de productos y fuentes y formato de errores.
- `internal/services`: normalización de filtros, fuentes y regla de vigencia.
- `internal/repositories`: SQL de productos, fuentes, disponibilidad e ingesta,
  conexión y verificación de permisos.
- `internal/domain`: DTO e interfaces de persistencia.

## Cambios de contrato (SUP-385)

Rutas, campos JSON, tablas, columnas y variables de entorno no cambian. Sí cambia:

| Antes | Ahora |
| --- | --- |
| JWT con `rol` `administrador`, `ADMIN` o con espacios se normalizaba a `admin` | Sólo `rol: "admin"` exacto; lo demás responde 403 |
| `JWT_SECRET` aceptaba cualquier largo | Mínimo 32 bytes, como en Identidad |

## Pendientes coordinados (SUP-394)

- Parámetros inválidos (`page=abc`, `en_stock=quizas`, `sort_by` desconocido) y
  `solo_marcas_blancas=false` se ignoran en silencio; responder 400 cambia el
  contrato con Frontend e IA.
- La búsqueda se sanea por bytes (una tilde ocupa 2) y descarta caracteres sin
  avisar; el mínimo de 3 se mide en bytes.
- Alias heredados de ingesta `precio` e `imagen`; Control debería enviar sólo
  `precio_oferta` y `url_imagen`.
- La sucursal central autocreada usa datos fijos de Temuco
  (`internal/repositories/ingesta.go`).
- `cadenas_supermercado.nombre` no tiene unicidad sin distinguir mayúsculas: la
  resolución usa `ILIKE` bajo lock. Candidato a índice único `LOWER(BTRIM(nombre))`
  (sección 8.4, coordinar con `infrastructure/db`).
- `capturado_el` y `ultima_extraccion_el` son `timestamp without time zone` y se
  escriben con la hora local del proceso; fuera de un contenedor UTC la
  disponibilidad recién ingestada sale `vencida`.
- La búsqueda pública filtra categoría por `titulo_crudo` y la de administración
  por `categoria_cruda`.
- Compose traduce `CATALOGO_CORS_ALLOWED_ORIGINS` del `.env` raíz a
  `CORS_ALLOWED_ORIGINS` del contenedor, y Catálogo lee ambas: conviene unificar
  en una sola variable (`docker-compose.yml`, K8s y `GUIA_CONFIGURACION.md`).
- `GET /api/v1/productos/buscar` no tiene consumidores en Frontend ni IA, pero lo
  usan los humos de Gateway y Control.

## Pruebas y validación

Desde este directorio:

```bash
go test -race ./...
go vet ./...
go build -o /tmp/catalogo ./cmd/catalogo
```

Con un PostgreSQL desechable inicializado con `infrastructure/db` (nunca una base
compartida), la prueba de ingesta, precios, disponibilidad y fuentes corre con el
rol de runtime:

```bash
CATALOGO_TEST_DB_URL='postgres://app_catalogo:<clave>@127.0.0.1:55432/<base>?sslmode=disable' \
  go test -count=1 -run Postgres ./tests/integration/
```

Pruebas de integración desde la raíz del repositorio:

```bash
docker build -t catalogo:local backend/catalogo
bash scripts/smoke_catalogo_compose.sh
bash scripts/smoke_catalogo_ingesta_compose.sh
```
