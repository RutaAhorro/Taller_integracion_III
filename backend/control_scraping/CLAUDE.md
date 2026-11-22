# Control de scraping — contexto IA

- Leer `README.md` del servicio y `.env.example`: configuración, estructura,
  contratos y pruebas. Go/Gin/GORM, puerto 8080, entrada `cmd/control_scraping/main.go`.
- HTTP en `internal/api` (router, CORS, token de servicio, sondas, handlers y
  errores); negocio en `services`; SQL, Redis Streams y cliente HTTP de Catálogo
  en `repositories`; modelos, contratos y errores en `domain`; entorno sólo en
  `internal/config`. Un archivo por tema; `repositories` no importa `services`.
- Solo `SCRAPING_DB_URL`/`app_scraping`: trabajos, entregas y outboxes propios.
  No SQL de usuarios/productos, JWT_SECRET ni AutoMigrate.
- `/api/v1/scraper/*` exige `SCRAPING_SERVICE_TOKEN` (bot/worker), no JWT admin.
- Catálogo por HTTP interno con `CATALOGO_INTERNAL_TOKEN`; mantener timeouts,
  clave estable de lote (`lote_id`) y confirmación idempotente del contador.
- SQL antes de publicar en Redis Streams; respetar leases, backoff y cierre
  terminal. No sustituir por LPUSH/BRPOP ni prometer exactly-once general.
- Contratos compartidos que no se cambian en una tarea de servicio: claves
  `scraper:jobs:*` y campo `payload`, JSON de bot y Scraper Go, rutas
  `/internal/v1/*`, columnas del esquema `scraper` y `/_api/live` (smoke del CI).
- Live independiente; readiness depende de SQL y sólo diagnostica Redis.
- Pruebas en `tests/unit` y `tests/integration` con dobles de `tests/dobles`:
  `go test -race ./...`, `go vet ./...`. Las de SQL/Redis reales requieren
  `SCRAPING_TEST_DB_URL` y `SCRAPING_TEST_REDIS_URL` hacia recursos aislados.
- Nombres descriptivos en español, comentarios por sección y constantes con
  nombre; formatear con `gofmt`. Swagger se regenera con el comando del README.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

27 archivos fuente, incluidos los de pruebas. Distribución:
- `cmd`: 1 archivos.
- `docs`: 1 archivos.
- `internal`: 16 archivos.
- `tests`: 9 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
