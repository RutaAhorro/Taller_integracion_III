# Motor de rutas

Servicio Go (`net/http`) que implementa el CRUD de listas de compra y de sus
artículos sobre tablas de `rutas.*` (SUP-240, SUP-241). Se reescribió desde Python en SUP-348
([D11](../../docs/Decisiones/11_Motor_de_rutas_en_Go.md)) sin cambiar el
contrato HTTP ni la base. El cálculo de rutas todavía no existe (SUP-313 y
SUP-328); las ejecuciones y paradas aún no tienen permisos runtime.

El servicio es opcional en Compose y no forma parte del inventario de nueve pods
informado por el equipo para el cierre del Sprint 2. Tener código y manifiestos
no acredita un despliegue activo.

## API (tras el Gateway)

| Método | Ruta | Respuesta |
| --- | --- | --- |
| GET | `/api/v1/listas` | `{"listas": [...]}` del usuario, más recientes primero |
| POST | `/api/v1/listas` | 201 con la lista creada. Cuerpo `{"nombre": "..."}` |
| GET | `/api/v1/listas/{id}` | Lista con `articulos`, cada uno con su estado en Catálogo |
| PUT | `/api/v1/listas/{id}` | Cuerpo con `nombre` y/o `estado` (`activa`, `completada`, `archivada`) |
| DELETE | `/api/v1/listas/{id}` | 204; sus artículos se borran en cascada |
| POST | `/api/v1/listas/{id}/articulos` | Cuerpo `{"producto_id", "cantidad"}`. 201 si es nuevo; 200 si ya estaba y se sumó la cantidad |
| PUT | `/api/v1/listas/{id}/articulos/{articulo_id}` | Cuerpo con `cantidad` y/o `esta_comprado`. No consulta a Catálogo |
| DELETE | `/api/v1/listas/{id}/articulos/{articulo_id}` | 204 |

- La sesión sale sólo de la cookie HttpOnly `jwt` emitida por Identidad (HS256,
  `exp` obligatorio, `user_id` UUID). Se ignoran `Authorization`, `X-User-ID` y
  cualquier dueño enviado en el cuerpo.
- Una lista ajena o un id mal formado responden 404, sin revelar si existe.
- Nombre: 1 a 150 caracteres tras quitar caracteres de control y espacios extra.
  Máximo de listas por usuario configurable, 100 por defecto (409 al superarlo).
- Errores con el envoltorio común `{"estado", "mensaje", "detalle"?}`; las
  validaciones nunca repiten el valor recibido.

### Artículos

La forma de cada operación está en el
[contrato de artículos](../../docs/Contratos/CONTRATO_ARTICULOS_LISTA.md).

- Un artículo guarda el `producto_id` de Catálogo (UUID de
  `scraper.productos_crudos`) y una cantidad entera de envases. Motor no lee
  tablas de Catálogo: consulta `POST /internal/v1/productos/disponibilidad` con
  `CATALOGO_CONSULTA_TOKEN` y un plazo máximo.
- Al agregar: producto inexistente, 422; Catálogo caído o lento, 503 y no se
  guarda nada. `sin_stock` o `desconocido` se guardan igual, con
  `estado_producto` en la respuesta para que el frontend advierta.
- El detalle consulta Catálogo en una sola llamada. Si Catálogo no responde,
  la lista se muestra igual con `estado_producto: "desconocido"`. Un artículo
  anterior a la migración 11 (sin `producto_id`) aparece como `no_encontrado`.
- Topes fijos del contrato: 100 artículos distintos por lista y 99 envases por
  artículo (409 al superarlos, también al sumar). Cantidad menor que 1, 400.
  Son constantes de `internal/listas`, no variables de entorno.
- Un artículo de otra lista o de otro usuario responde 404.
- El agregado en lote (`/articulos/lote`) es SUP-318.

#### Antes / Ahora

| Antes | Ahora |
| --- | --- |
| Artículo con `producto_normalizado_id` (FK a `api.productos_normalizados`, tabla vacía) | Artículo con `producto_id` de Catálogo, sin FK; `null` en filas previas |
| Detalle con `id`, `producto_normalizado_id`, `cantidad`, `esta_comprado` | Detalle con `id`, `producto_id`, `cantidad`, `esta_comprado`, `estado_producto` y `producto` |
| Sin endpoints de artículos | `POST`, `PUT` y `DELETE` bajo `/api/v1/listas/{id}/articulos` |
| Arrancaba sólo con datos de la base | Exige `CATALOGO_INTERNAL_URL` y `CATALOGO_CONSULTA_TOKEN`, y la migración 11 aplicada |

Probes internos: `/_rutas/live` (sin SQL) y `/_rutas/ready` (ping SQL, 2 s).

## Configuración

| Variable | Requisito |
| --- | --- |
| `RUTAS_DB_URL` | DSN con usuario `app_rutas`, base y contraseña explícitos |
| `JWT_SECRET` | La misma clave de Identidad y Catálogo, al menos 32 bytes |
| `CORS_ALLOWED_ORIGINS` | Orígenes HTTP(S) exactos del frontend, separados por comas |
| `RUTAS_DB_POOL_MAX` | Conexiones máximas por proceso, 5 por defecto (1 a 20) |
| `RUTAS_MAX_LISTAS` | Máximo de listas por usuario, 100 por defecto (1 a 1000) |
| `CATALOGO_INTERNAL_URL` | Obligatoria. Origen HTTP(S) interno de Catálogo, sin ruta (`http://catalogo:8080`) |
| `CATALOGO_CONSULTA_TOKEN` | Obligatoria. El mismo token de consulta que define Catálogo, al menos 32 bytes y distinto de `JWT_SECRET` |
| `RUTAS_CATALOGO_TIMEOUT` | Plazo por consulta a Catálogo, `2s` por defecto (hasta `30s`) |
| `PORT` | Puerto HTTP, 8001 por defecto |

Ejemplo en [`.env.example`](.env.example).

Al arrancar valida una vez los grants de `06_propiedad_permisos.sql` y que la
migración 11 haya corrido (columna `producto_id` e índice único); sin ellos no
abre HTTP. No ejecuta migraciones.

## Datos y despliegue

1. Aplicar como operador `infrastructure/db/06_propiedad_permisos.sql` y luego
   `11_articulos_lista_producto.sql`: crean `app_rutas` (NOLOGIN), le dan CRUD
   sobre `rutas.listas_compras` y `rutas.articulos_lista_compras`, retiran la FK
   de listas hacia `api.usuarios` y cambian los artículos a `producto_id`. Ambos
   son idempotentes. Antes de la 11, contar los artículos existentes en una copia:
   se conservan con `producto_id` NULL.
2. Activar LOGIN: en volúmenes nuevos, `07_runtime_credentials.sh` lo hace si
   existe `RUTAS_DB_PASSWORD`; en la base existente, `\password app_rutas` en psql.
3. Kubernetes: Secret `rutas-db-secret` con `DB_URL` y `DB_PASSWORD`, y la clave
   `consulta-token` en `catalogo-internal-secret` (la misma que usa Catálogo).
   Luego `kubectl apply -f infrastructure/k8s/motor-rutas/` y el Deployment del
   Gateway con `RUTAS_URL=http://motor-rutas:8001`.
4. Compose: definir `RUTAS_DB_PASSWORD`, `CATALOGO_CONSULTA_TOKEN` y
   `RUTAS_URL=http://motor-rutas:8001` en `.env`, y levantar
   `docker compose --profile rutas up -d motor_rutas gateway`.
   Sin `RUTAS_URL`, el Gateway no enruta `/api/v1/listas` (404).

## Estructura

| Ruta | Contenido |
| --- | --- |
| `cmd/motor_rutas` | Arranque, conexión y apagado ordenado |
| `internal/config` | Variables de entorno validadas; los errores nunca incluyen secretos |
| `internal/auth` | Validación del JWT de Identidad |
| `internal/db` | Pool `pgx` y verificación de grants al arrancar |
| `internal/listas` | Modelo, normalización del nombre, SQL y cliente de disponibilidad de Catálogo |
| `internal/httpapi` | Rutas, CORS, `X-Request-ID`, logs y errores |
| `internal/optimizacion` | Cálculo de la ruta según el [contrato del motor](../../docs/Contratos/CONTRATO_MOTOR_RUTAS.md): conjuntos de tiendas y asignación (SUP-313), orden de visita, traslado, base de una tienda y umbral de ahorro (SUP-328). Sin HTTP ni SQL; todavía no tiene endpoint (SUP-316) |
| `tests/unit`, `tests/integration` | Pruebas fuera del código de producción |

## Pruebas

```bash
go test ./... && go vet ./...
docker build --target test backend/motor_rutas   # lo mismo en la CI, con -race
```

`tests/integration` corre contra una base desechable sólo si existen
`RUTAS_TEST_DB_URL` (app_rutas) y `RUTAS_TEST_ADMIN_URL` (operador), con las
migraciones de `infrastructure/db` aplicadas, incluida la 11. Prueba los grants
reales, el aislamiento por usuario, alta, suma, topes, cambio y baja de
artículos, altas simultáneas del mismo producto, el CHECK de cantidad, la edición
de artículos previos sin `producto_id` y el
borrado en cascada. Nunca apuntarlas a una base compartida.
