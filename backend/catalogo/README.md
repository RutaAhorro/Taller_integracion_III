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
| `CATALOGO_CONSULTA_TOKEN` | Opcional | Clave Bearer de sólo lectura para `/internal/v1/productos/disponibilidad` y `/internal/v1/productos/equivalencias`. Vacía deshabilita ambas consultas |
| `PORT` | `8080` | Puerto de escucha HTTP |
| `CATALOGO_CORS_ALLOWED_ORIGINS` | Vacía | Orígenes HTTP(S) exactos separados por comas; tiene prioridad sobre `CORS_ALLOWED_ORIGINS`. Ambas vacías, se rechazan las peticiones con `Origin` |
| `CORS_ALLOWED_ORIGINS` | Vacía | Respaldo genérico. Compose la define a partir de `CATALOGO_CORS_ALLOWED_ORIGINS` del `.env` raíz |
| `DB_MAX_OPEN_CONNS` | `10` | Límite de conexiones abiertas al pool de PostgreSQL |
| `DB_MAX_IDLE_CONNS` | `5` | Conexiones inactivas máximas (no puede superar `DB_MAX_OPEN_CONNS`) |
| `DB_CONN_MAX_LIFETIME` | `30m` | Tiempo máximo de vida por conexión |
| `CATALOGO_MAX_PRODUCTOS_CONSULTA` | `1000` | IDs aceptados por lote en las consultas internas de disponibilidad y equivalencia |
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
| GET | `/api/v1/productos/categorias` | Público | Categorías existentes completas, sin paginación |
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

### API interna de ingesta, disponibilidad y equivalencia

Rutas privadas bajo el prefijo `/internal/v1` destinadas a la coordinación de
scraping y a las consultas de Listas, IA y Rutas. No se exponen a través del
Gateway ni en Ingress público:

| Método | Ruta | Payload JSON | Descripción |
| --- | --- | --- | --- |
| POST | `/internal/v1/cadenas/resolver` | `{"cadena_id": int, "nombre": string}` | Resuelve o registra una cadena con lock transaccional |
| POST | `/internal/v1/sucursales/resolver` | `{"cadena_id": int, "sucursal_id": *int, "codigo_sucursal": *string}` | Resuelve una sucursal existente; nunca la crea (404 `branch_not_found` si no hay) |
| POST | `/internal/v1/ingestas` | `{"sucursal_id": int, "lote_id": string, "productos": [...]}` | Persistencia atómica de 1 a 1000 productos y capturas de precio |
| POST | `/internal/v1/productos/disponibilidad` | `{"producto_ids": ["uuid", ...]}` | Consulta en lote de existencia, stock, precio, sucursal y captura más reciente |
| POST | `/internal/v1/productos/equivalencias` | `{"producto_ids": ["uuid", ...]}` | Agrupa por EAN exacto los productos equivalentes y su oferta actual por sucursal |

## Ingesta interna y token privado

Las operaciones de `/internal/v1` aplican las siguientes reglas:

1. **Autenticación en tiempo constante:** la ingesta exige
   `Authorization: Bearer <CATALOGO_INTERNAL_TOKEN>` y las consultas de
   disponibilidad y equivalencia exigen `Authorization: Bearer
   <CATALOGO_CONSULTA_TOKEN>`. La comparación es segura frente a ataques de
   temporización (`subtle.ConstantTimeCompare`).
2. **Rechazo estricto de CORS:** cualquier petición a rutas internas que incluya
   cabecera `Origin` se rechaza con HTTP 403 (`internal_origin_forbidden`),
   incluso si presenta un token válido.
3. **Límites del cuerpo HTTP:** tamaño máximo de 8 MiB (HTTP 413 si se supera), rechazo de campos JSON desconocidos
   (`DisallowUnknownFields`), rechazo de contenido extra tras el cierre del JSON
   y lote acotado de 1 a 1000 productos.
4. **Modo sólo lectura:** si `CATALOGO_INTERNAL_TOKEN` está vacío no se registra
   la ingesta; si `CATALOGO_CONSULTA_TOKEN` está vacío no se registran la
   disponibilidad ni la equivalencia. Cada ruta depende sólo de su propio token.
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

## Consulta interna de equivalencia por EAN (SUP-244)

Rutas usa esta consulta para saber si dos productos de sucursales distintas son
el mismo artículo y comparar su precio. La coincidencia es **exacta por EAN**: no
hay nombres, IA, homologación manual ni coincidencia difusa. Comparte el token
`CATALOGO_CONSULTA_TOKEN`, el límite de lote y el cuerpo JSON estricto de la
consulta de disponibilidad. No crea tablas, columnas ni variables de entorno.

```http
POST /internal/v1/productos/equivalencias
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
- Deduplica los UUID y conserva el orden de la primera aparición. Un UUID
  inválido rechaza el lote completo.
- Devuelve un elemento por producto solicitado, incluso si no existe en la base.

### Normalización del EAN

1. Se conservan sólo los dígitos del `ean_gtin` almacenado; los separadores se
   descartan. El código se compara como texto, nunca como número, para no perder
   ceros iniciales.
2. Sólo son válidos los largos GTIN de 8, 12, 13 o 14 dígitos con dígito
   verificador módulo 10 correcto.
3. El código válido se canoniza a 14 dígitos rellenando con ceros a la izquierda.
   Así un UPC-A de 12 dígitos (`036000291452`) y su EAN-13 con cero inicial
   (`0036000291452`) son la misma clave.
4. Un código inválido nunca se usa para unir productos: no se corrige ni se
   adivina un dígito.

### Estados

| Valor | Significado | Ofertas devueltas |
| --- | --- | --- |
| `comparable` | Tiene EAN válido y el grupo no presenta conflicto | Todas las sucursales del grupo |
| `sin_ean` | No tiene `ean_gtin` o está vacío | Sólo la oferta del propio producto |
| `ean_invalido` | Tiene código, pero el largo o el dígito verificador no son válidos | Sólo la oferta del propio producto |
| `conflicto_presentacion` | El grupo comparte EAN, pero tiene formatos explícitos distintos | Sólo la oferta del propio producto |
| `no_encontrado` | El UUID no existe en `productos_crudos` | `[]` |

**Conflicto de presentación.** Se comparan sólo formatos explícitos
(`formato_crudo`) normalizados a minúsculas y espacios simples. Dos formatos no
vacíos distintos, por ejemplo `1 L` y `1.5 L`, marcan el grupo como conflictivo.
Un formato vacío no conflictúa porque no prueba una presentación distinta.

### Ofertas y vigencia

Cada oferta corresponde al producto real que se compra en una sucursal y usa su
captura más reciente (`scraper.capturas_precios`):

- `producto_id_oferta` conserva el `id` real de Catálogo de esa sucursal, igual
  que el contrato del motor de rutas. Puede diferir del producto solicitado.
- `precio`, `precio_normal`, `precio_oferta` y `en_stock` sólo se publican si la
  captura tiene a lo más `CATALOGO_DISPONIBILIDAD_ANTIGUEDAD_MAXIMA` (48 h por
  defecto). Si falta, está vencida o tiene fecha futura, llegan en `null` y
  `estado_precio` es `sin_captura` o `vencido`. `capturado_el` se informa igual
  para que el consumidor conozca la fecha del dato.
- Las ofertas se ordenan por `sucursal_id` y luego por `producto_id_oferta`.
- **Sin geografía.** No incluye latitud, longitud ni filtro por radio: SUP-317
  agrega esos datos sobre este contrato.

```json
{
  "productos": [
    {
      "producto_id": "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e",
      "estado": "comparable",
      "ean": "00036000291452",
      "presentacion": "1 L",
      "ofertas": [
        {
          "producto_id_oferta": "1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f",
          "sucursal_id": 1,
          "cadena": "Jumbo",
          "sucursal": "Jumbo Centro",
          "nombre": "Leche entera 1 L",
          "unidad": "1 L",
          "precio": 1190,
          "precio_normal": 1190,
          "precio_oferta": null,
          "en_stock": true,
          "capturado_el": "2026-11-16T10:00:00Z",
          "estado_precio": "vigente"
        }
      ]
    }
  ],
  "resumen": {
    "solicitados": 1,
    "comparables": 1,
    "sin_ean": 0,
    "ean_invalidos": 0,
    "conflictos_presentacion": 0,
    "no_encontrados": 0
  }
}
```

`resumen` mide la cobertura de esa misma consulta. Es la forma honesta de
informar cuántos productos no se pueden comparar: EAN ausente, código inválido,
conflicto o UUID inexistente. Si la cobertura real de EAN en la base es baja,
este contador lo muestra; no se rellena con datos ficticios. Las fechas del
ejemplo son ilustrativas: la vigencia se calcula con la hora de la consulta.

**Límites conocidos.** La calidad depende de que el scraper haya extraído el EAN
y de que esté almacenado sin errores. La normalización en SQL
(`regexp_replace` + `lpad` hasta 14 dígitos, igual que en el servicio) no usa un
índice funcional, por lo que la consulta recorre los productos candidatos. En
una medición con 100.000 productos y 100 IDs, `EXPLAIN ANALYZE` mostró un
`Seq Scan on productos_crudos` de ~360 ms. Añadir un índice funcional (o filtrar
antes por `ean_gtin` crudo) requiere migración y va fuera de SUP-244.
No se usa la tabla legada `api.equivalencias_productos`.

Para medir la cobertura real en una base concreta, sin inventar datos:

```sql
SELECT count(*) AS productos,
       count(*) FILTER (
         WHERE regexp_replace(COALESCE(ean_gtin, ''), '[^0-9]', '', 'g')
               ~ '^([0-9]{8}|[0-9]{12}|[0-9]{13}|[0-9]{14})$'
       ) AS con_largo_gtin
FROM scraper.productos_crudos;
```

Ese conteo no valida el dígito verificador; la respuesta de la consulta sí lo
hace producto por producto y lo refleja en `resumen`.

### Medición en una base desechable (2026-11-23)

Se hicieron dos mediciones distintas. No se debe confundir la cobertura de los
datos que vienen en el repositorio con la de una extracción real cargada a mano
en una base aislada.

#### Datos sembrados por `infrastructure/db`

Con `infrastructure/db` cargado en PostgreSQL y `CATALOGO_TEST_DB_URL`
apuntando al rol `app_catalogo`:

- **45 productos sembrados**, todos de Jumbo en una sola categoría
  (`Scraped Category`), **0 con EAN** (0 %). Las otras cinco cadenas no tienen
  productos cargados, por lo que **no hay una categoría en común con datos**
  entre las seis.
- **`TestPostgresEquivalenciaCategoriaComunPorCadena`** crea un producto por
  cada una de las seis cadenas con el mismo EAN y verifica seis ofertas. Son
  datos sintéticos de la prueba: validan el contrato, **no la cobertura**.

```bash
export CATALOGO_TEST_DB_URL='postgres://app_catalogo:...@127.0.0.1:55432/sup244_ean?sslmode=disable'
cd backend/catalogo
go test -count=1 -v -run Postgres ./tests/integration/
```

#### Extracción real de quesos y fiambres

Se extrajo HTML real con `backend/scraper_go/cmd/extract` (sin Redis, Control
ni spool), una página por categoría y con pausa entre peticiones. Los JSONL
resultantes se cargaron en la misma base desechable con el esquema vigente; no
se crearon columnas ni tablas.

| Cadena | URL de categoría | Productos | EAN válidos | Observación |
| --- | --- | ---: | ---: | --- |
| Jumbo | `https://www.jumbo.cl/quesos-y-fiambres` | 40 | 40 | El listado no trae EAN; se completó con `-enrich-ean` en cada ficha. |
| Santa Isabel | `https://www.santaisabel.cl/quesos-y-fiambres` | 40 | 40 | El listado no trae EAN; se leyó cada ficha con `-product`. |
| A Cuenta | `https://www.acuenta.cl/ca/frescos-y-lacteos/quesos-y-quesillos/07/0707` | 50 | 49 | Un producto del listado no publica EAN. |
| Cugat | `https://cugat.cl/categoria-producto/fiambreria-embutidos-y-quesos/` | 0 | 0 | El sitio respondió `challenge-platform`; el crawler bloqueó la petición. |
| Lider | `/browse/.../quesos/45669105_72564080` y `/browse/.../fiambres-y-embutidos/45669105_22196003` | 0 | 0 | Los selectores actuales del spider no reconocieron productos en esas páginas. |
| Unimarc | — | — | — | No hay spider configurado. |

Total cargado: **130 productos reales, 129 con EAN válido**. Al llamar a
`POST /internal/v1/productos/equivalencias` con los 130 UUID:

| `resumen` | Valor |
| --- | ---: |
| `solicitados` | 130 |
| `comparables` | 124 |
| `sin_ean` | 1 |
| `ean_invalidos` | 0 |
| `conflictos_presentacion` | 5 |
| `no_encontrados` | 0 |

Además, agrupando las respuestas por EAN canónico:

- **115 grupos distintos**, de los cuales **13 tienen ofertas de más de una
  cadena**: 7 Jumbo–Santa Isabel, 4 Jumbo–A Cuenta, 1 A Cuenta–Santa Isabel y
  1 con las tres cadenas.
- **11 de esos 13 grupos son `comparables`**. Los otros 2 terminaron en
  `conflicto_presentacion` porque el `formato_crudo` de Santa Isabel es una
  descripción de marketing y difiere del formato de la otra cadena.
- **22 de los 130 productos (16,9 %)** recibieron al menos una oferta
  comparable de otra cadena. Las 130 capturas estaban vigentes.

Esta medición usa datos reales de tres cadenas; no usa el test sintético de seis
cadenas como cobertura. Cugat, Lider y Unimarc quedan fuera por las razones de
la tabla. El contrato está validado con datos reales, pero la cobertura sigue
dependiendo de que la extracción persista `ean_gtin` y de que los selectores de
cada spider sigan vigentes.

Errores: `401 internal_unauthorized`, `403 internal_origin_forbidden`,
`400 invalid_batch`, `400 invalid_product_id`, `400 invalid_payload`,
`413` para cuerpos mayores a 8 MiB y `500 catalogo_internal_error` sin detalle
SQL.

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

### Sucursales: Catálogo ya no las crea (SUP-376)

`POST /internal/v1/sucursales/resolver` solo busca (por `sucursal_id`, por
`codigo_sucursal` o, con solo `cadena_id`, la primera sucursal activa por `id`).
Una cadena nueva necesita una sucursal real registrada (con dirección y
coordenadas verdaderas) antes de ingestar. Rutas, campos, tablas y variables no
cambian; Control ya trata el 404 como `ErrSucursalNoEncontrada` permanente.

| Antes | Ahora |
| --- | --- |
| Solo `cadena_id` y sin sucursal activa: creaba "<Cadena> Central" (dirección "Casa Matriz / Online", Temuco, -38.7359, -72.5904) y respondía 200 | No crea nada: responde 404 `{"error":"branch_not_found"}` |
| La resolución era una transacción con `pg_advisory_xact_lock` (clave 2692) | Es una lectura simple; la clave 2692 se retiró |
| La ingesta de una cadena nueva "funcionaba" con coordenadas inventadas que el motor de rutas habría usado | El lote se rechaza hasta que exista una sucursal real |

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
- `internal/services`: normalización de filtros, fuentes, vigencia y equivalencia
  exacta por EAN.
- `internal/repositories`: SQL de productos, fuentes, disponibilidad,
  equivalencia e ingesta, conexión y verificación de permisos.
- `internal/domain`: DTO e interfaces de persistencia.

## Categorías del lateral

`GET /api/v1/productos/categorias` devuelve `{"categorias":["…"]}` con los
valores distintos de `scraper.productos_crudos.categoria_cruda`, ordenados por
categoría. Excluye `NULL` y vacíos, recorta espacios exteriores y devuelve `[]`
si no hay categorías. Consulta directamente esa columna: no descarga productos,
no une capturas ni depende de páginas, stock, búsqueda o supermercado seleccionado.

Es una lista plana de valores existentes: no agrupa por departamentos ni unifica
mayúsculas, tildes, nombres entre cadenas o categorías genéricas del scraper.
Puede incluir categorías cuyos productos no tienen stock.

`GET /api/v1/productos?categoria_exacta=<valor>` compara ese valor por igualdad
con la categoría guardada, recortando espacios exteriores. Conserva signos,
tildes y mayúsculas; `%` y `_` son literales, no comodines. Se combina con los
demás filtros y afecta tanto a productos/total como a `supermercados`.
Si se envían `categoria` y `categoria_exacta`, se aplican ambos.
El parámetro anterior `categoria` conserva su búsqueda aproximada en el título:
no se cambia el contrato de sus consumidores. `/buscar` sigue admitiendo sólo
`q`, `page` y `limit`; usar el listado para filtrar por categoría.

La ruta aprovecha el prefijo de productos del Gateway. No requiere cambios de
BD, permisos, secretos ni configuración de infraestructura.

## Supermercados disponibles en el listado (SUP-371)

`GET /api/v1/productos` (y `/buscar`) añade el campo `supermercados` para el
filtro lateral. Contiene las cadenas con al menos un producto que cumple TODOS
los filtros activos (`q`, `categoria`, `categoria_exacta`, `en_stock`, `marca`, `precio_min/max`,
`en_oferta`, `solo_marcas_blancas`) EXCEPTO `supermercado`, calculadas sobre todos
los resultados y no sobre la página descargada.

```json
{
  "data": [{"id": "…", "supermercado": "Lider", "…": "…"}],
  "supermercados": [{"id": 1, "nombre": "Jumbo"}, {"id": 2, "nombre": "Lider"}],
  "paginacion": {"total_registros": 57, "pagina_actual": 2, "total_paginas": 3, "limite": 20},
  "total_registros": 57, "pagina_actual": 2, "total_paginas": 3, "limite": 20
}
```

Reglas del contrato:

- Orden estable por `nombre` y luego `id`; sin duplicados. Sin coincidencias es
  `[]`, nunca `null`.
- `page`, `limit`, `sort_by` y `order` no cambian el campo.
- `supermercado=X` filtra `data` y `total_registros`, pero `supermercados` sigue
  mostrando las alternativas válidas para los demás filtros.
- "Disponible" usa la semántica del listado: con `en_stock=true` sólo cuentan
  productos con stock; sin el parámetro cuentan también los agotados, igual que
  `total_registros`.
- Implementación: una sola función (`consultaBase`, `internal/repositories/productos.go`)
  traduce los filtros a SQL para el listado, el total y esta agregación (opción
  `omitirSupermercado`); un filtro nuevo se agrega ahí y lo heredan los tres. Es
  una consulta extra `SELECT DISTINCT cs.id, cs.nombre` sin `LIMIT/OFFSET` ni
  `LATERAL` de capturas salvo que un filtro de precio u oferta lo requiera. No
  requiere migraciones ni permisos nuevos (`SELECT` ya concedido).

| Antes | Ahora |
| --- | --- |
| La respuesta no traía cadenas; el cliente las deducía de la página descargada | Campo aditivo `supermercados: [{id, nombre}]` calculado sobre todos los resultados |
| Cambiar de página alteraba las cadenas visibles | Las cadenas no dependen de `page` ni `limit` |
| Elegir una cadena dejaba sólo esa cadena en el filtro | Se conservan las alternativas válidas para los demás filtros |

Clientes que ignoran el campo no se rompen. Los demás campos no cambian.

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
- La sucursal central autocreada se retiró (SUP-376). Sobra el permiso `INSERT`
  de `app_catalogo` sobre `scraper.sucursales_supermercado` (y su secuencia
  `USAGE`); el arranque aún lo exige para esa tabla en `VerificarAlmacenamiento`.
  Retirarlo es cambio de `infrastructure/db` y del código, a coordinar.
- Las sucursales sembradas 5 y 6 (A Cuenta y Cugat, `ACUENTA-CENTRAL` y
  `CUGAT-CENTRAL`) en `infrastructure/db/init.sql`, `04_migracion_nuevas_cadenas.sql`
  y `infrastructure/k8s/db/configmap.yaml` tienen los mismos datos inventados
  ("Casa Matriz / Online", coordenadas del centro de Temuco). Sin corregir.
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
compartida), la prueba de ingesta, precios, disponibilidad, equivalencia y
fuentes corre con el rol de runtime:

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
