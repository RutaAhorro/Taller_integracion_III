# Control de scraping — contexto IA

- Leer `docs/Guias/GUIA_CONTROL_SCRAPING_DESACOPLADO.md` desde la raíz.
- Go/Gin/GORM, puerto 8080, entrada `cmd/control_scraping/main.go`.
- `internal/control` compone rutas y loops; negocio en `internal/services`;
  persistencia, publisher y cliente Catálogo en `internal/repositories`.
- Solo `SCRAPING_DB_URL`/`app_scraping`: trabajos, entregas y outboxes propios.
  No SQL de usuarios/productos, JWT_SECRET ni entidades compartidas.
- `/api/v1/scraper/*` exige `SCRAPING_SERVICE_TOKEN` (bot/worker), no JWT admin.
- Catálogo por HTTP interno con `CATALOGO_INTERNAL_TOKEN`; mantener timeouts,
  clave estable de lote y confirmación idempotente del contador.
- SQL antes de publicación en Redis Streams; respetar leases, backoff y cierre
  terminal. No sustituir por LPUSH/BRPOP ni prometer exactly-once general.
- Live independiente; readiness depende de SQL y diagnostica Redis.
- Cambios de DTO deben conservar compatibilidad con scraper Go y bot.
- Pruebas desde este servicio: `go test -race ./...`, `go vet ./...`.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

37 archivos fuente, incluidos los de pruebas. Distribución:
- `cmd`: 6 archivos.
- `docs`: 1 archivos.
- `internal`: 28 archivos.
- `tests`: 2 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
