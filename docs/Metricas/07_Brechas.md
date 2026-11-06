# Métricas — Brechas ❌

[← Índice de métricas](README.md)

## Medición

| Brecha | Impacto | Posible solución |
|---|---|---|
| No hay sistema de métricas | No hay QPS, tasa de error ni latencia en tiempo real | Exponer `/metrics` (Prometheus) o agregar los logs `http_request` |
| No hay percentiles de latencia | Solo hay una latencia media puntual (3,86 s en `/productos`) | Repetir el benchmark con Fortio guardando p50/p90/p99 |
| El benchmark solo cubrió Catálogo | No se sabe cómo responden Identidad, Control ni el Gateway bajo carga | Repetir la prueba contra cada servicio con HPA |
| El HPA escala solo por CPU | Cuando el cuello de botella es la base de datos, el HPA no reacciona | Ajustar el pool o usar métricas personalizadas (fuera de alcance) |
| No se mide la cobertura | No se puede comparar la calidad de los tests entre servicios | `go test -cover` y `coverage.py` en CI |
| Sin CI para Identidad, Motor, IA, Bot y Frontend | Las regresiones se detectan solo al probar a mano | Workflows análogos a `catalogo-checks.yml` |
| Sin sondas en el scraper, el bot, el frontend, la base de datos y Redis | Kubernetes no detecta si se cuelgan | Agregar `livenessProbe` |

## Inconsistencias detectadas

- `infrastructure/k8s/scraper/deployment.yaml` todavía describe el scraper Python, que se retiró en #126, y convive con `scraper-go/`.
- `infrastructure/k8s/fortio/` sigue en el repositorio con `image: fortio/fortio:latest` (sin versión fija), aunque según el informe del benchmark Fortio ya no está en el clúster.
- Compose define `SCRAPER_CATEGORY_DIR=/app/categories`, pero el Dockerfile del Scraper Go copia las categorías a `/app/research`.
- Sin Redis, el rate limiter de Identidad responde `500`; lo correcto sería `503`. Esto distorsiona la tasa de error si llega a medirse.
