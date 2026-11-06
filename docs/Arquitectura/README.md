# 02 — Arquitectura

Describe la arquitectura vigente en `develop` (commit `584506f`, 02-11-2026).
Fuente de verdad: código, SQL y `docker-compose.yml`. Si un plan o un diagrama
antiguo contradice al código, prevalece el código.

| # | Documento | Contenido |
|---|---|---|
| 01 | [Visión general](01_Vision_general.md) | Diagrama, estilo arquitectónico y estado de cada componente |
| 02 | [Servicios](02_Servicios.md) | Ficha por servicio: tecnología, puerto, responsabilidad y límites |
| 03 | [Flujo de una petición](03_Flujo_de_una_peticion.md) | Recorrido navegador → Gateway → servicio, y flujo de scraping |
| 04 | [Datos y permisos](04_Datos_y_permisos.md) | PostgreSQL único, esquemas, roles por dominio, Redis |
| 05 | [Redes y despliegue](05_Redes_y_despliegue.md) | Compose, redes, puertos, límites de memoria, Kubernetes |
| 06 | [Seguridad transversal](06_Seguridad_transversal.md) | Sesión, tokens de servicio, límites de tasa, cabeceras |

**Convención de estado:**

- ✅ desplegado según el inventario del Sprint 2.
- 🟡 existe en el código o en Compose, pero es opcional o no está integrado.
- ⚪ vista simulada (mock).

[← Glosario](../Glosario.md)
