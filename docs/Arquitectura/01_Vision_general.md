# Arquitectura — Visión general

[← Índice de arquitectura](README.md)

## Estilo

La plataforma usa **microservicios por dominio** detrás de un **API Gateway**
único. Este esquema reemplaza a la API monolítica `backend/api`, que se retiró en
SUP-273.

Cada dominio es dueño de sus tablas y se comunica con los demás **solo por HTTP**.
La excepción es el scraping, que además usa Redis Streams. Nunca hay SQL cruzado
entre dominios.

## Diagrama

```text
Navegador ──► Ingress / Nginx ──► Gateway (Go) ──┬─► Identidad ──► PostgreSQL + Redis
                                                  ├─► Catálogo  ──► PostgreSQL
                                                  ├─► Control   ──► PostgreSQL + Redis Streams
                                                  └─► Motor rutas (solo si RUTAS_URL) 🟡
Bot Discord ─────────────────────► Gateway                 │
                                                           ▼ (Redis Streams)
                                                     Scraper Go ──► Supermercados
                                                           │ (HTTP, spool SQLite)
                                                           ▼
                                          Gateway ──► Control ──► Catálogo (HTTP interno)

Frontend ─ ─ ─► IA conversacional :8002 (directo, sin Gateway) 🟡 ──► Catálogo (HTTP)
```

## Estado de los componentes

| Componente | Estado | Nota |
|---|---|---|
| Frontend | ✅ | Catálogo, auth y admin reales; chat, misiones, rutas e historial son ⚪ |
| Gateway | ✅ | Solo enruta |
| Identidad | ✅ | Cuentas, sesión y administración de usuarios |
| Catálogo | ✅ | Productos, precios, fuentes e ingesta idempotente |
| Control de scraping | ✅ | Trabajos, outboxes y entrega a Catálogo |
| Scraper Go | ✅ | Reemplaza al scraper Python (Scrapy) desde #126 |
| PostgreSQL/PostGIS | ✅ | Una instancia, permisos por tabla |
| Redis | ✅ | Cola de scraping y contadores de autenticación |
| Bot Discord | ✅ | Opera trabajos vía API con un token de servicio |
| Motor de rutas | 🟡 | CRUD de listas en el perfil `rutas`; faltan artículos y optimización |
| IA conversacional | 🟡 | Existe en Compose, pero el frontend llama a `/chat/gemini`, que no existe (SUP-253) |

## Principios

1. **El Gateway no decide**: no valida JWT, no usa SQL ni Redis y no aplica reglas de negocio.
2. **Cada servicio autoriza** sus propias rutas.
3. **Un dominio, un rol de BD**: el aislamiento viene de los permisos, no de tener instancias separadas.
4. **Integración por HTTP**, con idempotencia en la ingesta.
5. **Secretos fuera del navegador**: los tokens de servicio solo circulan entre backends.
