# 03 — Decisiones de arquitectura

Cada archivo registra una decisión con el mismo formato:

- **Contexto**: el problema de partida.
- **Decisión**: lo que se eligió.
- **Motivo**: por qué se eligió.
- **Alternativa descartada**: qué otra opción se consideró y por qué no se adoptó.
- **Consecuencias**: el costo que se acepta.

El estado de D01 a D10 corresponde a `develop` (`584506f`, 02-11-2026). D11 y D12 son decisiones del Sprint 3.

Una decisión nueva va en un archivo nuevo con el número siguiente. Una decisión reemplazada no se borra: se marca como *Reemplazada por Dxx*.

| # | Decisión | Estado |
|---|---|---|
| 01 | [Microservicios por dominio en vez de monolito](01_Microservicios_por_dominio.md) | Vigente (SUP-273) |
| 02 | [Una instancia PostgreSQL con roles por dominio](02_PostgreSQL_unico_con_roles.md) | Vigente |
| 03 | [Gateway que solo enruta](03_Gateway_solo_enruta.md) | Vigente |
| 04 | [JWT en cookie HttpOnly](04_JWT_en_cookie_HttpOnly.md) | Vigente |
| 05 | [Scraper en Go en vez de Scrapy](05_Scraper_Go.md) | Vigente (#126) |
| 06 | [Redis Streams con outbox](06_Redis_Streams_con_outbox.md) | Vigente |
| 07 | [Ingesta idempotente por lotes](07_Ingesta_idempotente.md) | Vigente |
| 08 | [Rate limit en Redis que falla cerrado](08_Rate_limit_falla_cerrado.md) | Vigente |
| 09 | [Logout del lado del servidor](09_Logout_en_servidor.md) | Vigente (#124) |
| 10 | [Componentes opcionales por perfil u opt-in](10_Componentes_opcionales.md) | Vigente |
| 11 | [Motor de rutas en Go en vez de Python](11_Motor_de_rutas_en_Go.md) | Aceptada, en curso (SUP-348) |
| 12 | [El artículo de lista guarda el id de producto de Catálogo](12_Identidad_del_producto_en_listas.md) | Propuesta (SUP-321) |

[← Arquitectura](../Arquitectura/README.md)
