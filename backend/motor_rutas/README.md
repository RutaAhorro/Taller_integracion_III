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

### Carrito persistente

Motor/Listas implementa además un borrador por cuenta en `rutas.carritos`,
separado de las listas confirmadas. Requiere **14_carritos_persistentes.sql**
antes de publicar la nueva imagen. No se ejecuta SQL de migración al arrancar.
La edición es versionada y la confirmación aplica artículos y vacía el carrito
en una única transacción, conservando clave y último recibo para recuperar
respuestas perdidas. No almacena precios ni consulta tablas de otros servicios.

Rutas: GET/PUT `/api/v1/listas/carrito`, POST `.../preparar` y POST
`.../confirmar`; cookie JWT y cabecera de coherencia `X-Carrito-Cuenta`.
Contrato, errores y orden de habilitación:
[carrito persistente](../../docs/Contratos/CONTRATO_CARRITO.md).

Las pruebas de integración cubren persistencia, aislamiento, versiones,
confirmaciones concurrentes y conservación del borrador ante fallos, sobre una
BD desechable. CI inicializa un PostgreSQL efímero, nunca un clúster compartido.
Sin las variables de pruebas, la suite SQL se omite; un test unitario no acredita
una migración ni un despliegue.

### Listas y artículos

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
| POST | `/api/v1/rutas/optimizar` | Cuerpo `{"lista_id", "origen"?}`. Calcula cadena(s) más barata(s) y orden Haversine sin combustible. Si los productos no caben en `RUTAS_MAX_PARADAS` tiendas, responde 200 con ruta parcial y `no_cubiertos` (`fuera_del_maximo_de_paradas`); 422 solo si ningún artículo tiene oferta ([contrato](../../docs/Contratos/CONTRATO_MOTOR_RUTAS.md)) |

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
- Agregado en lote (SUP-318): `POST /api/v1/listas/{id}/articulos/lote` con
  `{"articulos": [{"producto_id", "cantidad"}, ...]}` y `Idempotency-Key`
  obligatoria (400 sin ella). Todo o nada: si un producto no existe o supera
  la cantidad máxima al sumar, responde 409 con `detalle` como arreglo
  `[{producto_id, motivo}]` (`no_encontrado` o `cantidad_maxima`) y no guarda
  nada. Los repetidos se suman antes de guardar. Éxito: 200 con la lista
  completa, igual que el detalle.
- El reintento con la misma clave y el mismo lote (sin importar el orden)
  devuelve el detalle sin volver a sumar; la misma clave con otro lote, 409
  con `"codigo": "idempotency_conflict"`. Las claves viven en
  `rutas.lotes_articulos_aplicados` (migración 12).
- **Pendiente:** las claves de idempotencia sólo se borran con su lista (FK en
  cascada). No hay vencimiento ni limpieza periódica: una lista muy usada
  acumula una fila por lote aplicado.

#### Antes / Ahora

| Antes | Ahora |
| --- | --- |
| Artículo con `producto_normalizado_id` (FK a `api.productos_normalizados`, tabla vacía) | Artículo con `producto_id` de Catálogo, sin FK; `null` en filas previas |
| Detalle con `id`, `producto_normalizado_id`, `cantidad`, `esta_comprado` | Detalle con `id`, `producto_id`, `cantidad`, `esta_comprado`, `estado_producto` y `producto` |
| Sin endpoints de artículos | `POST`, `PUT` y `DELETE` bajo `/api/v1/listas/{id}/articulos`, y `POST .../articulos/lote` (SUP-318) |
| Arrancaba sólo con datos de la base | Exige `CATALOGO_INTERNAL_URL` y `CATALOGO_CONSULTA_TOKEN`, y las migraciones 11 y 12 aplicadas |

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
| `RUTAS_CATALOGO_TIMEOUT` | Plazo por consulta a Catálogo, `5s` por defecto (hasta `30s`) |
| `RUTAS_RADIO_KM` | Radio de búsqueda de sucursales en km, `10` por defecto (`(0, 50]`) |
| `RUTAS_MAX_SUCURSALES` | Sucursales candidatas pedidas a Catálogo, `10` por defecto (`1` a `10`) |
| `RUTAS_MAX_PARADAS` | Máximo de paradas por ruta, `3` por defecto (`1` a `3`) |
| `RUTAS_MAX_ARTICULOS` | Máximo de artículos pendientes por cálculo, `30` por defecto (`1` a `100`) |
| `RUTAS_FACTOR_DISTANCIA` | Factor de corrección sobre Haversine, `1.3` por defecto (`(0, 5]`) |
| `RUTAS_UMBRAL_AHORRO` | Ahorro mínimo en CLP para recomendar varias tiendas (RN-08), `500` por defecto (`>= 0`) |
| `PORT` | Puerto HTTP, 8001 por defecto |

Ejemplo en [`.env.example`](.env.example).

Al arrancar valida una vez los grants de `06_propiedad_permisos.sql` y que las
migraciones 11, 12 y 14 hayan corrido (columna `producto_id`, índice único, tabla
de claves del lote y carrito); sin ellos no
abre HTTP. No ejecuta migraciones.

## Datos y despliegue

1. Aplicar como operador `infrastructure/db/06_propiedad_permisos.sql` y luego
   `11_articulos_lista_producto.sql`: crean `app_rutas` (NOLOGIN), le dan CRUD
   sobre `rutas.listas_compras` y `rutas.articulos_lista_compras`, retiran la FK
   de listas hacia `api.usuarios` y cambian los artículos a `producto_id`. Ambos
   son idempotentes. Antes de la 11, contar los artículos existentes en una copia:
   se conservan con `producto_id` NULL. Después, `12_lotes_articulos_idempotencia.sql`
   crea la tabla de claves del lote con `SELECT` e `INSERT` para `app_rutas`.
2. Activar LOGIN: en volúmenes nuevos, `07_runtime_credentials.sh` lo hace si
   existe `RUTAS_DB_PASSWORD`. En una base existente, asignar contraseña no basta:
   también se requiere LOGIN. Usar el aprovisionamiento revisado de abajo, sin
   ejecutar el script 07 completo ni rotar credenciales de los otros servicios.
3. Kubernetes: Secret `rutas-db-secret` con `DB_URL` y `DB_PASSWORD`, y la clave
   `consulta-token` en `catalogo-internal-secret` (la misma que usa Catálogo).
   Antes de aplicar, revisar la cuota, imagen y HPA según la preparación de abajo.
   Gateway debe tener `RUTAS_URL=http://motor-rutas:8001`.
4. Compose: definir `RUTAS_DB_PASSWORD`, `CATALOGO_CONSULTA_TOKEN` y
   `RUTAS_URL=http://motor-rutas:8001` en `.env`, y levantar
   `docker compose --profile rutas up -d motor_rutas gateway`.
   Sin `RUTAS_URL`, el Gateway no enruta `/api/v1/listas` (404).

### Preparación de habilitación del entorno (SUP-252 / SUP-357)

**Consulta de solo lectura del 24-11-2026, namespace `student-vmatus`. No es
un despliegue ni una garantía del estado futuro: repetir las comprobaciones
antes de actuar y coordinar con quien mantiene el clúster.**

- No había Deployment ni Service `motor-rutas`, aunque Gateway ya apuntaba a él.
- El rol `app_rutas` existía con `NOLOGIN`. Las tablas de listas/artículos tenían
  CRUD; faltaban `SELECT` e `INSERT` sobre `rutas.lotes_articulos_aplicados`.
  La columna `producto_id` y la tabla de lotes existían. La verificación completa
  de esquema, secuencia e índice sigue siendo obligatoria al arrancar.
- Faltaba `rutas-db-secret`; `catalogo-internal-secret` tenía la clave de ingesta
  `token`, pero no `consulta-token`. Catálogo no recibía
  `CATALOGO_CONSULTA_TOKEN`.
- La cuota permitía 2000m CPU de límites y estaban asignados 1800m. El Deployment
  versionado de Motor pide otros 500m: **no cabe tal cual**, aunque el uso real
  de CPU sea bajo. Había 9/12 pods, 8/10 services y 1382Mi/2Gi de límites de
  memoria. El HPA de Motor admite dos réplicas y tampoco debe activarse sin
  presupuesto para ambas.
- La imagen del manifiesto usa el propietario `danirc2024`, mientras los
  servicios activos usaban `RutaAhorro`. Verificar la imagen real publicada y
  fijar su versión/digest antes de aplicar; no asumir que ambas son equivalentes.

Orden propuesto para el operador, **pendiente de autorización de ejecución**:

1. Acordar responsable y ventana, registrar imágenes actuales y revisar otra vez
   `kubectl get resourcequota -n student-vmatus`. Elegir un presupuesto para una
   réplica. Un límite de 200m cabe aritméticamente en la captura, pero consume
   todo el margen y no acredita rendimiento; ampliar/reasignar cuota o adoptar
   otro presupuesto requiere acuerdo. No aplicar la carpeta completa ni el HPA
   automáticamente.
2. Respaldar la BD y revisar la matriz de permisos. Con autorización, reconciliar
   los grants exactos de la migración 12, idempotente. No reconstruir el volumen
   ni ejecutar migraciones por tener la tabla presente. Confirmar todos los
   requisitos de `internal/db/db.go`, no solo la existencia de tablas.
3. Provisionar LOGIN y una contraseña exclusiva de `app_rutas` de forma segura,
   sin ampliarle permisos ni usar `postgres` en el servicio. Crear
   `rutas-db-secret` sin imprimir su DSN ni versionar credenciales.
4. Provisionar `consulta-token` conservando la clave de ingesta `token`. Debe ser
   distinto de JWT e ingesta. Configurarlo en Catálogo y Motor, y coordinar el
   reinicio de Catálogo para habilitar su endpoint de consulta. No enviarlo al
   navegador ni publicarlo a través de Gateway.
5. Preparar los manifiestos con la imagen verificada, cuota acordada y una
   réplica; comprobar con `kubectl diff` solo los recursos sin secretos y luego
   con validación de servidor antes de aplicar. Revisar conectividad y políticas
   de red sin abrir PostgreSQL al exterior. Aplicar únicamente los recursos
   acordados: ConfigMap, Service y Deployment de Motor; no cambiar el Gateway
   si ya tiene el destino correcto.
6. Verificar rollout, `/_rutas/live`, `/_rutas/ready` y la respuesta 401 de listas
   sin sesión a través de Gateway. Esa comprobación no prueba persistencia.
7. Con una cuenta de prueba autorizada, ejecutar la checklist de
   [listas del frontend](../../frontend/web/README.md#listas-y-artículos-sup-252).
   Registrar versión, entorno y resultados. Ante un fallo, retirar el servicio
   nuevo o volver a su imagen anterior; no borrar listas ni revertir migraciones
   automáticamente.

### Asistente de preparación y aprovisionamiento (SUP-357)

`scripts/habilitar_motor_listas.py` no ejecuta `kubectl apply`, no reinicia
servicios y no crea tablas. Requiere Python 3 y kubectl con Kustomize. Mantiene
el contexto y namespace explícitos en cada llamada. La preparación sirve para
la **primera habilitación**: si ya existen Motor/Service/HPA, se detiene para
no modificar despliegues ajenos.

**Revisar, sin modificar el clúster:**

```bash
python3 scripts/habilitar_motor_listas.py revisar \
  --contexto estudiante-vmatus --namespace student-vmatus
```

Comprueba todos los permisos base que exige el arranque, el estado de LOGIN,
la columna e índice de productos y la tabla de lotes. Solo muestra nombres de
claves de Secrets, no sus valores. El informe separa bloqueos y pendientes
que podrá resolver el aprovisionamiento. La cuota se calcula con sus unidades
reales: `2G` no es `2Gi`.

La propuesta en `infrastructure/k8s/overlays/motor-listas-ajustado` contiene
únicamente ConfigMap, Service y Deployment: una réplica, requests de 50m/64Mi,
limits de 100m/128Mi, sin HPA. No modifica el presupuesto de otros servicios.
En la revisión del 25-11-2026 cabía aritméticamente y dejaba 100m de límite CPU
libre; no es una medición de rendimiento ni una reserva de cuota.

Además de ResourceQuota, el asistente comprueba los LimitRanges visibles. El
namespace exige un mínimo de 64Mi por contenedor: la propuesta anterior de
32Mi fue rechazada al crear el pod y se corrigió a 64Mi. El dry-run de un
Deployment valida ese objeto, no crea ni valida todos sus pods futuros.

**Cuotas del clúster anfitrión:** el namespace puede tener margen y aun así
fallar al crear un Service. En la habilitación se observó el rechazo
`students-host-envelope`, con 120/120 Services en el anfitrión. Esa cuota puede
no ser visible desde el contexto del estudiante. Se necesita capacidad del
administrador o retirar, con autorización y comprobaciones, un Service propio
realmente sobrante. No borrar ni reutilizar Services de otros estudiantes.
No usar IPs efímeras de pods como sustituto permanente del Service.

En la recuperación autorizada del 25-11-2026 se respaldó y retiró únicamente
el Service duplicado `api-gateway`, conservando `api` y el Deployment del
Gateway. Se corrigió la memoria solicitada de Motor a 64Mi y se creó su Service.
Motor quedó 1/1 Running, su ready por Service devolvió 200 y listas por Gateway,
sin sesión, devolvió 401. Gateway, frontend y Catálogo público siguieron
respondiendo 200. La cuota visible quedó en 10/12 pods, 8/10 Services y
1900m/2000m de límites CPU.

Estas comprobaciones no prueban guardado ni aislamiento de listas con sesión.
La checklist de navegador sigue pendiente. No reaplicar manifiestos locales
anteriores que pidan 32Mi ni repetir el aprovisionamiento tras esta recuperación.

Si `apply` creó ConfigMap y Deployment pero rechazó el Service, el estado es
parcial: no volver a aprovisionar contraseñas, no borrar Secrets y no aplicar
la carpeta completa. Revisar por separado la admisión del pod y la creación
del Service. El asistente de primera habilitación se detiene ante recursos
existentes; la recuperación requiere operaciones acotadas y coordinadas.

**Preparar archivos locales, sin desplegar:**

Verificar primero la imagen publicada y su digest con el operador o el registro.
El asistente exige la forma `ghcr.io/.../...-motor-rutas@sha256:...` pero **no
comprueba por sí mismo que ese digest exista**. En la revisión autenticada del
25-11-2026 existía la imagen bajo `rutaahorro`; bajo `danirc2024` devolvía 404.
No copiar a ciegas el propietario del manifiesto base.

```bash
python3 scripts/habilitar_motor_listas.py preparar \
  --contexto estudiante-vmatus --namespace student-vmatus \
  --imagen 'ghcr.io/rutaahorro/taller_integracion_iii-motor-rutas@sha256:DIGEST_CONFIRMADO' \
  --salida /tmp/opencode/motor-listas.yaml
```

Sustituir `DIGEST_CONFIRMADO` por los 64 caracteres reales. Genera
`motor-listas.yaml` y `motor-listas.catalogo-patch.json`, sin secretos. No
sobrescribe planes anteriores. El parche de Catálogo añade solo la referencia
`CATALOGO_CONSULTA_TOKEN` y conserva imagen/configuración restante. Incluye
`resourceVersion`: si otro integrante actualiza Catálogo, falla por conflicto
y se debe revisar y preparar un plan nuevo, no forzar el parche viejo.

**Aprovisionar, solo tras respaldo, revisión y autorización:**

Este modo **sí escribe** en Kubernetes y PostgreSQL. Antes de ejecutarlo:

- Acordar responsable/ventana y confirmar un respaldo recuperable de la BD.
  `--respaldo-confirmado` es una confirmación del operador; el script no hace
  ni certifica el respaldo.
- Revisar los planes y la política de registro/auditoría de PostgreSQL. El
  asistente deshabilita el log de sentencias solo en su sesión sensible y se
  detiene si detecta pgaudit activo; otras soluciones de auditoría deben revisarse
  manualmente. No ejecutarlo con depuradores, trazas o captura de stdin.
- Confirmar otra vez que no hay un despliegue de Motor en curso.

```bash
python3 scripts/habilitar_motor_listas.py aprovisionar \
  --contexto estudiante-vmatus --namespace student-vmatus \
  --confirmar-cambios --respaldo-confirmado
```

Genera credenciales independientes en memoria y crea `rutas-db-secret`. Añade
`consulta-token` con control de versión al Secret existente, preservando su
token de ingesta. Reutiliza credenciales previas y no rota la contraseña de
un rol ya activo. Habilita LOGIN de `app_rutas` cuando estaba deshabilitado y
reconcilia **solo SELECT/INSERT** de la tabla de lotes existente. No modifica
usuarios, listas ni artículos. Comprueba la contraseña de Motor por TCP local
al pod PostgreSQL, no mediante una conexión Unix que podría usar `trust`.

Kubernetes y PostgreSQL no comparten una transacción: si hay un fallo parcial,
conservar los Secrets y revisar el estado antes de reintentar. El script crea
primero el Secret para reutilizar la misma contraseña si se interrumpe la
primera habilitación. No borrar Secrets ni regenerar claves para “arreglar”
un fallo. Si el rol ya admite LOGIN sin Secret conocido, se bloquea; una rotación
requiere un procedimiento distinto y acuerdo explícito.

**Validar y aplicar los recursos, en una operación autorizada aparte:**

1. Repetir la revisión y consultar el diff de los manifiestos **sin Secrets**.
2. Validar el YAML con el servidor; esto no reserva recursos ni prueba descarga:

   ```bash
   kubectl --context estudiante-vmatus -n student-vmatus apply \
     --dry-run=server -f /tmp/opencode/motor-listas.yaml
   ```

3. Con `consulta-token` ya preparado, revisar y aplicar el parche de Catálogo:

   ```bash
   kubectl --context estudiante-vmatus -n student-vmatus patch deployment catalogo \
     --type=strategic --patch-file=/tmp/opencode/motor-listas.catalogo-patch.json
   kubectl --context estudiante-vmatus -n student-vmatus rollout status deployment/catalogo --timeout=180s
   ```

   Con estrategia Recreate hay una interrupción breve de Catálogo. No aplicar
   el Deployment base completo ni cambiar su imagen por este procedimiento.

4. Aplicar únicamente el manifiesto preparado de Motor y revisar su rollout:

   ```bash
   kubectl --context estudiante-vmatus -n student-vmatus apply -f /tmp/opencode/motor-listas.yaml
   kubectl --context estudiante-vmatus -n student-vmatus rollout status deployment/motor-rutas --timeout=180s
   ```

5. Verificar probes y 401 sin sesión por Gateway. Después, con una cuenta
   de prueba autorizada, comprobar persistencia e aislamiento según la checklist
   del frontend. Un rollout verde o un 401 no acreditan ese recorrido.

Pruebas aisladas del asistente (no usan credenciales ni ejecutan SQL real):

```bash
python3 -m unittest discover -s scripts/tests -p 'test_habilitar_motor_listas.py' -v
```

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
