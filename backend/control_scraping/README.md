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
| `SCRAPING_CORS_ALLOWED_ORIGINS` | Opcional | Orígenes HTTP(S) exactos separados por coma. |
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

## Pruebas y compilación

Desde esta carpeta:

```bash
go test ./...
go vet ./...
go build ./cmd/control_scraping
```

Las pruebas unitarias están en `tests/unit/` y las pruebas de integración o
arquitectura en `tests/integration/`. El caso del publisher con PostgreSQL real
se omite salvo que se configure `SUP270_PUBLISHER_TEST_DB_URL` hacia una base
aislada. Los smoke tests Python se ejecutan mediante los scripts de integración
del repositorio y deben usar recursos desechables, nunca una base compartida.
