# Control de scraping

Servicio `backend/control_scraping`: crea los trabajos de scraping, sigue su
estado, los publica en Redis Streams mediante un outbox y entrega los
resultados a Catálogo. Se extrajo del monolito en SUP-273 (PR #100).

```mermaid
flowchart LR
  Web[Frontend y clientes] --> Gateway
  Bot --> Gateway
  Worker[Scraper Go] -->|API existente: lotes y leases| Gateway
  Gateway -->|auth| Identidad
  Gateway -->|productos y administración| Catalogo[Catálogo]
  Gateway -->|scraper y health heredado| Control[Control de scraping]
  Identidad -->|app_identity| Usuarios[(Usuarios)]
  Control -->|app_scraping| Outboxes[(Trabajos y outboxes)]
  Control -->|token interno y lote_id estable| Catalogo
  Catalogo -->|app_catalogo| Precios[(Productos y precios)]
  Control -->|publicación recuperable| Redis[(Redis Streams)]
  Redis --> Worker
```

Gateway no accede a SQL ni contiene reglas de negocio. No se crea una biblioteca
compartida de dominios. Control no importa Identidad/Catálogo y no persiste
productos o precios. El worker no recibe credenciales PostgreSQL. IA y rutas
quedan fuera de esta refactorización.

## Contratos y enrutamiento

| Ruta | Método | Propietario |
|---|---|---|
| `/api/v1/scraper/trabajos` | POST | Control, crea trabajo |
| `/api/v1/scraper/trabajos/{id}` | GET | Control, consulta estado |
| `/api/v1/scraper/trabajos/{id}/ejecutar` | POST | Control, outbox de ejecución y Redis |
| `/api/v1/scraper/trabajos/{id}/finalizar` | PUT | Control, conserva validaciones y contador |
| `/api/v1/scraper/trabajos/{id}/productos` | POST | Control, entrega durable a Catálogo |
| `/api/v1/scraper/productos` | POST | Control, ingesta directa idempotente |
| `/health`, `/api/v1/health` | GET | Control, JSON heredado compatible |
| `/swagger/*`, `/` | GET | Control, documentación de coordinación y raíz compatible |
| `/api/v1/auth/*` | existentes | Identidad |
| `/api/v1/productos/*`, `/api/v1/admin/productos/*` | existentes | Catálogo |

No cambian los estados, códigos de negocio, headers de lease, Idempotency-Key,
cookies ni formatos de lote. La clave de lote y los namespaces Redis existentes
se conservan. No hay migración de datos, cambio de cola ni vaciado de spools.

Gateway usa `SCRAPING_URL=http://control-scraping:8080`. En el despliegue normal
no necesita `BACKEND_URL` ni un servidor API residual. Sólo enruta prefijos
delimitados: `scrapertools` no pertenece a scraping. Los probes privados
`/_scraping/live` y `/_scraping/ready` devuelven 404 desde Gateway.

Un fallo de Control devuelve `scraping_unavailable` (502) o `scraping_timeout`
(504), sin fallback ni reintento automático de una escritura. Identidad,
Catálogo y readiness local de Gateway siguen atendiendo. `/_gateway/dependencies`
añade `scraping`, `scraping_database`, `scraping_redis`; mantiene los alias de
diagnóstico `backend`, `database`, `redis` por compatibilidad.

`BACKEND_URL` queda como modo legado explícito para imágenes aprobadas anteriores.
Los overrides históricos desactivan `SCRAPING_URL` y restauran los probes y
credenciales que necesita cada imagen. No mezclar la imagen nueva con probes
antiguos durante un rollback sin comprobar el destino.

## Datos, seguridad y ciclo de vida

Control recibe sólo `SCRAPING_DB_URL` con `app_scraping`, contraseña y base
explícitas. No hay fallback a DB_URL/administrador. Conserva un pool de 5
conexiones, 2 idle y vida de 30 minutos. Usa únicamente las tablas de trabajos
y outboxes autorizadas por SUP-271. El seeder pertenece a Catálogo y requiere
DB_URL explícito; sus permisos operativos no se añaden a Control.

Control no recibe JWT_SECRET, GOOGLE_CLIENT_ID ni DSN de Identidad. Su token
interno de Catálogo debe tener al menos 32 caracteres. Catálogo sigue validando
que ese token sea distinto de su clave JWT; un token equivocado se rechaza en
la ingesta privada. No se emiten ni validan sesiones en Control.

Se conservan las correcciones de SUP-270: SQL antes de publicación recuperable,
leases/reintentos, deduplicación de Redis, cierre terminal y recuperación de
entregas con clave estable. Varios procesos usan el mismo outbox durable y sus
claims SQL; confirmar un lote incrementa el contador una sola vez. El worker
mantiene su volumen/spool y sus límites actuales; escalar Control no escala al
worker ni permite compartir su SQLite entre workers.

`REDIS_URL` es obligatorio y debe ser una URL válida. No hay fallback silencioso
a localhost. Su caída de red no impide arrancar ni retira readiness si SQL sigue
sano: el trabajo puede permanecer durable hasta que se recupere Redis.

CORS usa `SCRAPING_CORS_ALLOWED_ORIGINS`, con orígenes HTTP(S) exactos. Rechaza un
Origin ajeno antes de ejecutar handlers y conserva Idempotency-Key en preflight.
Vacío admite clientes sin Origin y bloquea navegadores de otros sitios.

Todas las rutas `/api/v1/scraper/*` exigen `Authorization: Bearer
<SCRAPING_SERVICE_TOKEN>`. Gateway es público (por `api-ingress` y por
`frontend-ingress` en `/api/v1`), así que sin este token cualquiera podía crear
trabajos, lanzar spiders, cerrar trabajos ajenos o inyectar precios que Control
entrega a Catálogo con su token interno. Sus únicos clientes son el worker y el
bot; el frontend no consume estas rutas. Control compara hashes SHA-256 en
tiempo constante, responde 401 con `WWW-Authenticate` y no arranca si el token
está vacío, tiene menos de 32 caracteres, contiene espacios o coincide con
`CATALOGO_INTERNAL_TOKEN`. Los probes no lo exigen. Swagger publica el contrato
completo, así que sólo se sirve con `SCRAPING_SWAGGER_ENABLED=true` (Compose local
y CI); en Kubernetes el ConfigMap lo deja en `false` y `/swagger` responde 404.
Cualquier otro valor impide el arranque. El Scraper Go y el bot
leen el mismo `SCRAPING_SERVICE_TOKEN`.
Permisos por usuario y cuotas siguen fuera de alcance.

Errores internos, panics y errores del contexto devuelven 500 genérico, sin
trazas SQL. Se registran localmente con slog y X-Request-ID; los logs HTTP usan
ruta de plantilla y omiten cuerpos, cookies y claves. El cuerpo máximo sigue
en 8 MiB, compatible con Catálogo. Contexto de petición 65 s, read-header 5 s,
read 30 s, write 70 s, idle 60 s y cabeceras 64 KiB. Gateway conserva su propio
presupuesto menor; una cancelación no elimina el outbox durable.

Startup verifica tablas/grants una vez, con 5 s de presupuesto. Liveness no
consulta SQL/Redis; readiness comprueba SQL y diagnostica Redis, con 2 s de
presupuesto total. Los tres probes Kubernetes son exec por loopback usando
wget de la imagen Alpine. NetworkPolicy mantiene ingreso exclusivo desde pods
Gateway del mismo namespace; requiere un CNI que aplique esas políticas.

El binario corre como 65532:65532, con filesystem de sólo lectura, sin
capabilities ni privilegios. SIGTERM primero drena HTTP, cancela los dos loops,
espera su terminación y cierra el publisher/pools. El presupuesto es de 15 s;
Compose/Kubernetes dan 20 s para terminar. Un fallo de arranque sale con código
no cero, en vez de aparentar una ejecución correcta.

## Despliegue y escalado

Compose usa el servicio `control_scraping`, alias DNS `control-scraping`, sin
puerto público propio. Bot/worker conservan `http://api:8080`, alias de Gateway.
En Kubernetes, `Service/api` también conserva ese nombre y selecciona Gateway;
su manifiesto pasa a `infrastructure/k8s/gateway/api-service.yaml`.

Control: request 100m CPU/64Mi, límite 500m/256Mi, HPA 1–4 réplicas y objetivo
CPU 70%. Con Identidad y Control a cinco conexiones y Catálogo a diez,
los HPA al máximo suman `4*5 + 4*5 + 4*10 = 80` conexiones runtime. No es una
medición de capacidad de PostgreSQL; hay que contar clientes administrativos.
Gateway no tiene pool SQL. Las réplicas del worker conservan su política propia.

**Secret del token de servicio.** Control, el worker y el bot leen
`scraping-service-secret` (clave `token`) y no arrancan sin él. Igual que
`catalogo-internal-secret`, no se versiona: crearlo desde un archivo privado
(`kubectl create secret generic scraping-service-secret --from-file=token=<archivo>`)
con un valor de `openssl rand -hex 32` distinto del token interno de Catálogo.
Desplegar Control junto con worker y bot: una versión sin token recibe 401.

**No retirar `Service/api`:** es el alias de Gateway que usan el bot y el worker.

## Validación

```bash
cd backend/control_scraping
go test -race ./...
go vet ./...
go build ./cmd/control_scraping
swag init --parseInternal -g cmd/control_scraping/main.go
```

Desde la raíz, `bash scripts/smoke_control_scraping_compose.sh` crea un proyecto
Compose efímero con DB, Redis y credenciales privadas. Ejecuta los humos de
Identidad, Catálogo, permisos, ingesta y recuperación existentes, más:

- Probes exactos del Deployment en runtime, caída SQL/Redis y recuperación.
- Shutdown limpio, aislamiento de fallos y rutas privadas inaccesibles por Gateway.
- Tres réplicas de Control, dieciocho entregas del mismo lote distribuidas entre
  las tres por puertos locales efímeros y confirmación final por Gateway: una
  captura, un incremento y ninguna recreación de Identidad/Catálogo/Gateway.
- CORS por allowlist, imagen no root/read-only y ausencia de secretos de Identidad.

Los puertos fijos heredados de Gateway/frontend/fixtures requieren ejecutar
un solo harness a la vez. Los recursos de prueba se retiran al terminar; no
se ejecuta scraping externo ni se toca el clúster universitario.
Los resultados del refactor quedan en el historial de git; las decisiones vigentes, en `docs/Decisiones/`.
