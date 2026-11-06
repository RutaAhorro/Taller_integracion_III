# 04 — Métricas

Recoge lo que el proyecto **mide** o **configura** en la rama `develop` (`584506f`, 02-11-2026). Cada dato indica de dónde sale:

- 📏 **Medido:** valor obtenido en una prueba real.
- ⚙️ **Configurado:** valor declarado en código o en un manifiesto. Indica lo que el sistema debería hacer, no lo que se observó.
- ❌ **No existe:** brecha conocida.

El proyecto **no tiene un sistema de métricas** (Prometheus, OpenTelemetry o un endpoint `/metrics`). Las cifras salen de logs, manifiestos, CI y un benchmark puntual.

| # | Documento | Contenido |
|---|---|---|
| 01 | [Rendimiento medido](01_Rendimiento_medido.md) | Benchmark Fortio sobre Catálogo |
| 02 | [Escalado y recursos](02_Escalado_y_recursos.md) | HPA, requests/limits, cuota del namespace, Compose |
| 03 | [Disponibilidad](03_Disponibilidad.md) | Endpoints de salud y sondas de Kubernetes |
| 04 | [Observabilidad](04_Observabilidad.md) | Logs estructurados, `X-Request-ID`, errores de upstream |
| 05 | [Calidad y CI](05_Calidad_y_CI.md) | Tests por servicio, workflows y el control de memoria |
| 06 | [Métricas de negocio](06_Metricas_de_negocio.md) | Trabajos de scraping, panel de administración, límites funcionales |
| 07 | [Brechas](07_Brechas.md) | Lo que no se mide y las inconsistencias detectadas |

[← Decisiones](../Decisiones/README.md)
