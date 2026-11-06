# D05 — Scraper en Go en vez de Scrapy

[← Índice de decisiones](README.md)

**Contexto:** había dos scrapers en paralelo: uno en Python con Scrapy (`backend/scraper`) y otro en Go (`backend/scraper_go`). Mantener los dos duplicaba los spiders y la configuración.

**Decisión:** quedarse solo con el Scraper Go y retirar el de Python en #126. El worker consume trabajos desde Redis Streams, guarda su progreso en un spool SQLite y envía los resultados por HTTP. No tiene acceso a PostgreSQL.

**Motivo:**
- Consume menos memoria: el límite bajó de 400M a 200M.
- Compila a un binario estático.
- Usa el mismo lenguaje que el resto del backend.
- Al no acceder a la base de datos, cumple el principio de que ningún dominio escribe tablas ajenas.

**Alternativa descartada:** quedarse con Scrapy. Tiene un ecosistema maduro, pero consume más memoria, obliga a mantener dependencias de Python y la versión original escribía directamente en la base de datos.

**Consecuencias:**
- Los spiders de Scrapy se reescribieron en `internal/spiders/*.go`.
- Pendiente: Compose apunta a `SCRAPER_CATEGORY_DIR=/app/categories`, pero el Dockerfile copia las categorías a `/app/research`.
