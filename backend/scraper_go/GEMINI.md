# Scraper Go — contexto IA

- Worker actual de Compose (`scraper_worker`), Go, sin servidor HTTP público ni
  SQL PostgreSQL. Entradas `cmd/worker` y `cmd/extract`.
- Leer `README.md` distinguiendo contratos útiles de instrucciones históricas de
  cutover: Python ya fue retirado del código; no recrear sus carpetas/servicios.
- `internal/worker`: orquestación; `queue`: Redis Streams y leases; `spool`:
  SQLite durable; `crawler`: HTTP/robots/reintentos; `spiders`: cinco cadenas;
  `normalize`: normalización; `controlscraping`: cliente HTTP; `storage`: retención.
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

38 archivos fuente, incluidos los de pruebas. Distribución:
- `cmd`: 3 archivos.
- `internal`: 35 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
