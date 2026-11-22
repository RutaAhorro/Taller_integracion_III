# Scraper Go — contexto IA

- Worker actual de Compose (`scraper_worker`), Go, sin servidor HTTP público ni
  SQL PostgreSQL. Entradas `cmd/worker` y `cmd/extract`.
- `README.md` es la fuente única (configuración, contratos, operación y
  pruebas). Python ya fue retirado; no recrear sus carpetas/servicios.
- `internal/worker`: orquestación; `config`: variables de entorno y validación;
  `dominio`: contratos (`Product`, `Item`, `Job`) y normalización/recorte;
  `extraccion`: catálogo (`spiders.json`), crawler HTTP/robots/reintentos y
  spiders de las cinco cadenas; `cola`: Redis Streams/leases, spool SQLite
  durable, cliente HTTP a Control y retención de spools.
- Streams `scraper:jobs:stream`, grupo `scraper-workers`; no LPUSH/BRPOP.
- Enviar resultados a Control vía Gateway con `SCRAPING_SERVICE_TOKEN`;
  nunca recibir JWT_SECRET o token privado de Catálogo como sustituto.
- Persistir ítems y clave estable antes de enviar. Un 202 no confirma entrega;
  conservar spool/lease y no ACK antes de confirmar el estado terminal.
- No modificar hashes/JSON canónico sin revisar idempotencia y compatibilidad
  del spool. Truncar campos según límites reales antes de persistir/enviar.
- Bloqueo y trabajo sin productos no son éxitos; registrar conteo y diagnóstico.
- No escalar consumidores compartiendo el mismo SQLite sin diseñar almacenamiento.
- Pruebas: `go test ./...`, `go vet ./...`; el gate de RAM es una prueba aparte,
  no una garantía del consumo en Kubernetes. No ejecutar scraping real sin permiso.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

40 archivos fuente, incluidos los de pruebas. Distribución:
- `cmd`: 2 archivos.
- `internal`: 17 archivos.
- `tests`: 21 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
