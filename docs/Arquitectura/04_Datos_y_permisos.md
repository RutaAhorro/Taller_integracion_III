# Arquitectura — Datos y permisos

[← Índice de arquitectura](README.md)

## PostgreSQL/PostGIS

Se usa **una sola instancia**: el contenedor `bd_supermercados`, con PostgreSQL 15. Los nombres de los esquemas son históricos:

| Esquema | Dueño lógico |
|---|---|
| `api` | Identidad: usuarios, tokens y auditoría |
| `scraper` | Catálogo **y** Control, cada uno con sus propias tablas |
| `rutas` | Motor de rutas: listas |

El aislamiento entre dominios se logra con **un rol por dominio** y permisos por tabla: `app_identity`, `app_catalogo`, `app_scraping` y `app_rutas`. Están prohibidos el AutoMigrate y el SQL entre dominios.

## Migraciones y permisos

El orden de los scripts y cuáles aplicar a mano en una base existente están en el
[README de `infrastructure/db`](../../infrastructure/db/README.md). La matriz de
permisos por rol está en [datos y permisos del backend](../Guias/GUIA_DATOS_Y_PERMISOS_BACKEND.md).

## Redis 7

| Uso | Servicio |
|---|---|
| Redis Streams: cola de trabajos | De Control al Scraper Go |
| Contadores de tasa: scripts Lua atómicos | Identidad |

## Almacenamiento local del worker

El Scraper Go guarda los lotes y el progreso en un spool **SQLite**. Así, si el worker se cae, puede retomar el trabajo sin perder resultados.
