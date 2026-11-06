# Métricas — Rendimiento medido 📏

[← Índice de métricas](README.md)

Fuente: [`docs/Metricas/Benchmark_HPA_Catalogo.md`](Benchmark_HPA_Catalogo.md). La prueba se hizo con Fortio 1.76.0 en el clúster del curso.

## Prueba de carga y escalado (Catálogo)

Comando: `-c 20 -qps 500 -t 90s -timeout 2s`, ejecutado contra `/_catalogo/live`.

| Métrica | Valor |
|---|---|
| Carga sostenida | 500 QPS durante 90 s, 20 conexiones simultáneas |
| Peticiones totales | 45.000 |
| Tasa de éxito | 100 % (HTTP 200) |
| Reinicios de contenedores | 0 |
| Réplicas | 1 → 2 → 1 (subió y luego bajó de forma ordenada) |
| CPU de Catálogo bajo carga | 40m–70m (el umbral del HPA es 25m) |

## Tiempo de escalado

| Tramo | Duración | Proporción |
|---|---|---|
| Clúster detecta la carga y ordena una réplica | 30 s | 75 % |
| La réplica arranca y queda lista | 10 s | 25 % |
| **Total** | **40 s** | 100 % |

Los 30 s del clúster no son configurables desde la cuenta de estudiante. El proyecto solo controla los 10 s de arranque.

## Consumo del namespace

| Escenario | Memoria | CPU | Pods |
|---|---|---|---|
| 1 réplica de Catálogo | 1.454 / 1.907 MiB | 1.550m / 2.000m | 9 / 20 |
| 2 réplicas de Catálogo | 1.614 / 1.907 MiB | 1.750m / 2.000m | 10 / 20 |

En el escenario de 2 réplicas, al namespace le quedan **~293 MiB y 250m de CPU**. Eso equivale aproximadamente a una réplica extra de un servicio Go.

## Cuello de botella: la base de datos

| Prueba | Endpoint | Resultado |
|---|---|---|
| A (limitada por E/S) | `/api/v1/productos`, 20 conexiones | Latencia media de **3,86 s**; CPU de solo 1m–2m |
| B (limitada por CPU) | `/_catalogo/live`, 500 QPS | Escala en 30 s |

La causa de la prueba A es el pool de GORM (`MaxOpenConns: 10`): las peticiones esperan una conexión libre. Mientras esperan, la CPU casi no se usa y **el HPA no reacciona**, porque escala solo por CPU.

## Lo que no se midió

- Identidad, Control, Gateway y Motor de rutas bajo carga.
- Percentiles de latencia (p50/p90/p99) publicados.
- La latencia de punta a punta pasando por el Gateway.
