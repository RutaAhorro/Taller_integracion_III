# Métricas — Escalado y recursos ⚙️

[← Índice de métricas](README.md)

## HPA (`infrastructure/k8s/*/hpa.yaml`)

| Servicio | Réplicas mín.–máx. | Escala cuando la CPU supera |
|---|---|---|
| Identidad | 1–4 | 70 % del request |
| Control de scraping | 1–4 | 70 % del request |
| Catálogo | 1–2 | 50 % del request (≈ 25m) |
| Motor de rutas | 1–2 | 50 % del request |

El Gateway, el frontend, el bot, el scraper, la base de datos, Redis y el CronJob programador **no tienen HPA**: corren con una réplica fija (el CronJob, como un pod efímero por ejecución).

## Requests y limits en Kubernetes

| Servicio | Request CPU / memoria | Limit CPU / memoria |
|---|---|---|
| Gateway | 30m / 64Mi | 150m / 96Mi |
| Identidad | 50m / 64Mi | 200m / 160Mi |
| Catálogo | 50m / 64Mi | 200m / 160Mi |
| Control de scraping | 50m / 64Mi | 200m / 160Mi |
| Motor de rutas | 50m / 96Mi | 500m / 384Mi |
| Scraper Go | 50m / 64Mi | 500m / 128Mi |
| Bot Discord | 50m / 64Mi | 100m / 128Mi |
| Frontend | 30m / 64Mi | 100m / 150Mi |
| PostgreSQL | 50m / 64Mi | 250m / 300Mi |
| Redis | 25m / 64Mi | 100m / 100Mi |
| Programador de scraping (CronJob) | 50m / 64Mi | 200m / 128Mi |

**Cuota del namespace:** 2 GiB de memoria (1.907 MiB útiles), 2.000m de CPU y 20 pods. La asigna el clúster del curso (no hay un `ResourceQuota` versionado); el dato viene del [benchmark](Benchmark_HPA_Catalogo.md). El CronJob de SUP-366 sólo crea un pod a la vez (`concurrencyPolicy: Forbid`), con `backoffLimit: 0` y 900 s de deadline; sus Jobs terminados se limitan a 2 correctos y 1 fallido para no acumular pods.

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
