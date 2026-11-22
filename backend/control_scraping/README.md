# Control de scraping

Control administra trabajos de scraping, guarda ejecuciones en un outbox SQL,
las publica de forma recuperable en Redis Streams y entrega lotes a Catálogo
por HTTP. Sólo usa su propia conexión `SCRAPING_DB_URL` y las tablas de
coordinación autorizadas; no consulta usuarios ni escribe directamente datos de
productos. Gateway enruta las peticiones públicas y no contiene lógica de este
dominio.

## Configuración

El punto de entrada es `cmd/control_scraping`. Las variables se leen al iniciar;
los nombres compartidos se mantienen según
[GUIA_CONFIGURACION.md](../../docs/Guias/GUIA_CONFIGURACION.md). Para ejecutar
localmente, usar [.env.example](./.env.example) como plantilla sin añadir
secretos al repositorio.

| Variable | Requisito | Uso |
| --- | --- | --- |
| `SCRAPING_DB_URL` | Obligatoria | DSN PostgreSQL con rol `app_scraping`. |
| `REDIS_URL` | Obligatoria | Redis Streams para publicar trabajos. Una caída de Redis degrada la cola, pero no la readiness si SQL está sano. |
| `CATALOGO_INTERNAL_URL` | Obligatoria | Origen HTTP interno de Catálogo, sin ruta ni query. |
| `CATALOGO_INTERNAL_TOKEN` | Obligatoria | Token M2M distinto de `SCRAPING_SERVICE_TOKEN`. |
| `SCRAPING_SERVICE_TOKEN` | Obligatoria, mínimo 32 caracteres y sin espacios | Autenticación de Bot y Scraper Go en `/api/v1/scraper/*`. |
| `CATALOGO_INGEST_TIMEOUT` | Opcional, por defecto `10s`, máximo `20s` | Presupuesto de llamadas internas a Catálogo. |
| `SCRAPING_CORS_ALLOWED_ORIGINS` | Opcional; si no está definida se usa `CORS_ALLOWED_ORIGINS` | Orígenes HTTP(S) exactos separados por coma. |
| `SCRAPING_SWAGGER_ENABLED` | Opcional, por defecto `false` | Publica Swagger sólo con `true`; rechaza otros valores. |
| `PORT` | Opcional, por defecto `8080` | Puerto HTTP del servicio. |

## Contratos HTTP

Las rutas de coordinación viven bajo `/api/v1/scraper/`: crear y consultar
trabajos, encolarlos, finalizar y recibir lotes. Se conservan las rutas,
campos JSON, estados, claves de idempotencia y headers existentes; ver
[GUIA_CONTROL_SCRAPING_DESACOPLADO.md](../../docs/Guias/GUIA_CONTROL_SCRAPING_DESACOPLADO.md)
para los contratos y límites operativos.

`/health` y `/api/v1/health` mantienen la respuesta compatible. Readiness exige
SQL; Redis es diagnóstico. `/_scraping/live` no consulta dependencias y
`/_scraping/ready` comprueba readiness.

## Estructura

| Paquete | Rol |
| --- | --- |
| `cmd/control_scraping` | Arranque, verificación de almacenamiento, ciclos de recuperación y apagado. |
| `internal/config` | Única lectura y validación del entorno. |
| `internal/api` | Router, middlewares (CORS, token de servicio, límites, errores), sondas y handlers (`trabajos.go`, `ingestas.go`). |
| `internal/services` | Reglas: `trabajos.go`, `entregas.go` (ledger durable) y `ejecuciones.go` (outbox y ciclos de recuperación). |
| `internal/repositories` | PostgreSQL (`conexion.go`, `trabajos.go`, `entregas.go`), outbox + Redis Streams (`ejecuciones.go`) y cliente HTTP de Catálogo (`catalogo.go`). |
| `internal/domain` | Modelos, contratos de persistencia y errores de negocio. |

Swagger se regenera desde esta carpeta con
`go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g internal/api/rutas.go -o docs --parseInternal`.

## Contratos: antes / ahora

La compactación (SUP-392) no cambia rutas, campos JSON, códigos de estado, el
formato de Redis Streams ni las llamadas a Catálogo.

| Antes | Ahora |
| --- | --- |
| `GET /` respondía "Bienvenido a la API Gateway de Supermercados." | Responde "Bienvenido a Control de scraping." (sólo el texto). |
| Swagger sin esquema de seguridad ni algunos códigos (`202`, `401`, `503`, `504`). | Declara `TokenServicio` (Bearer) y esos códigos. |
| Sin `SCRAPING_CORS_ALLOWED_ORIGINS` no había orígenes permitidos. | Se usa `CORS_ALLOWED_ORIGINS` como fallback, según GUIA_CONFIGURACION. |

## Pruebas y compilación

Desde esta carpeta (el CI usa las mismas órdenes):

```bash
go test -race ./...
go vet ./...
go build ./cmd/control_scraping
```

- `tests/unit/`: config, cliente de Catálogo, services y api con los dobles en
  memoria de `tests/dobles/`.
- `tests/integration/`: arquitectura, composición real, sondas y Swagger. Las
  pruebas con almacenamiento real se omiten salvo que se definan
  `SCRAPING_TEST_DB_URL` (rol `app_scraping` sobre una base desechable
  inicializada con `infrastructure/db`) y `SCRAPING_TEST_REDIS_URL` (Redis
  desechable). Nunca apuntarlas a recursos compartidos.
- Los smoke tests Python (`tests/smoke_*.py`) los ejecutan los scripts de
  integración del repositorio.
