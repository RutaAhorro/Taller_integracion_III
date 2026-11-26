# Métricas — Escalado y recursos ⚙️

[← Índice de métricas](README.md)

## HPA (`infrastructure/k8s/*/hpa.yaml`)

| Servicio | Réplicas mín.–máx. | Escala cuando la CPU supera |
|---|---|---|
| Identidad | 1–4 | 70 % del request |
| Control de scraping | 1–4 | 70 % del request |
| Catálogo | 1–2 | 50 % del request (≈ 25m) |
| Motor de rutas | 1–2 | 50 % del request |

El Gateway, el frontend, la IA conversacional, el bot, el scraper, la base de datos, Redis y el CronJob programador **no tienen HPA**: corren con una réplica fija (el CronJob, como un pod efímero por ejecución).

## Requests y limits en Kubernetes

| Servicio | Request CPU / memoria | Limit CPU / memoria |
|---|---|---|
| Gateway | 30m / 64Mi | 150m / 96Mi |
| Identidad | 50m / 64Mi | 200m / 160Mi |
| Catálogo | 50m / 64Mi | 200m / 160Mi |
| Control de scraping | 50m / 64Mi | 200m / 160Mi |
| Motor de rutas | 50m / 64Mi | 100m / 128Mi |
| IA conversacional | 25m / 96Mi | 100m / 192Mi |
| Scraper Go | 50m / 64Mi | 500m / 128Mi |
| Bot Discord | 50m / 64Mi | 100m / 128Mi |
| Frontend | 30m / 64Mi | 100m / 150Mi |
| PostgreSQL | 50m / 64Mi | 250m / 300Mi |
| Redis | 25m / 64Mi | 100m / 100Mi |
| **Total 11 Deployments (todo encendido)** | **460m / 736Mi** | **2000m / 1702Mi** |
| **Total 9 Deployments (sin `bot` ni `scraper-go`)** | **360m / 608Mi** | **1400m / 1446Mi** |
| Programador de scraping (CronJob, efímero) | 50m / 64Mi | 200m / 128Mi |

**Cuota del namespace (`student-vmatus`):** `2G` de memoria (≈ `1.907 MiB` útiles), `2.000m` de CPU y **12 pods**. `LimitRange` por contenedor: memoria mínima `64Mi`, máxima `512Mi`, límite por defecto `128Mi`; CPU mínima `25m`, máxima `500m`, límite por defecto `100m`. La asigna el clúster del curso (no hay un `ResourceQuota` versionado).

**Escenarios operativos tras desplegar Motor de rutas e IA conversacional:**
- **Todo encendido (`11` pods):** `2.000m` CPU (`0m` margen) y `1.702 MiB` memoria (`+205 MiB`). Sin margen de CPU, las actualizaciones o el CronJob fallarían por cuota (Motor e IA usan `strategy: type: Recreate` para no pedir un pod extra).
- **Recomendado — `bot` y `scraper-go` en 0 réplicas (`9` pods):** `1.400m` CPU (`+600m` libres) y `1.446 MiB` memoria (`+461 MiB` libres). Se recomienda apagarlos tras el scraping (`kubectl scale deployment/bot deployment/scraper-go --replicas=0`) y encenderlos sólo cuando se necesiten (ver [guía de despliegue](../DESPLIEGUE.md)).

El CronJob de SUP-366 sólo crea un pod a la vez (`concurrencyPolicy: Forbid`), con `backoffLimit: 0` y 900 s de deadline; sus Jobs terminados se limitan a 2 correctos y 1 fallido para no acumular pods.

## Límites de memoria en Docker Compose (local)

| Servicio | Límite |
|---|---|
| db | 300M |
| identidad, catálogo, control, motor de rutas | 256M cada uno |
| gateway | 128M |
| frontend | 250M (768M con el override de desarrollo) |
| redis | 100M |
| bot | 200M |
| scraper_worker | 200M (antes 400M con el scraper Python) |
| ia_conversacional | 300M |

## Límites de conexión y tiempo

| Parámetro | Valor | Dónde |
|---|---|---|
| Pool de conexiones a PostgreSQL | 10 por defecto (`DB_MAX_OPEN_CONNS`) | Catálogo (GORM) |
| Timeout de conexión y de cabeceras | `DialTimeout`, `ResponseHeaderTimeout` | Gateway, configurable |
| Transportes | Separados para lectura y escritura | Gateway |
