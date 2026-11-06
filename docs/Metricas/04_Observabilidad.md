# Métricas — Observabilidad ⚙️

[← Índice de métricas](README.md)

## Log por petición (`http_request`)

| Servicio | Campos | Archivo |
|---|---|---|
| Gateway | `request_id`, `upstream`, `status`, `duration_ms`, `response_bytes` | `backend/gateway/internal/proxy/gateway.go` |
| Identidad | `request_id`, `method`, `path`, `status`, `duration_ms` | `backend/identidad/internal/server/server.go` |
| Catálogo | `request_id`, `path`, `status`, `duration_ms` | `backend/catalogo/internal/server/server.go` |
| Control de scraping | `request_id`, `method`, `path`, `status`, `duration_ms` | `backend/control_scraping/internal/middleware/runtime.go` |
| Motor de rutas | `duration_ms` y datos de la petición | `backend/motor_rutas/app/main.py` |

Los servicios Go usan `slog`, que escribe logs estructurados de clave y valor.

## Correlación

El Gateway genera un `X-Request-ID` y lo propaga a los servicios internos. Así, una misma petición se puede seguir por los logs del Gateway y del servicio que la atendió.

## Errores

| Evento | Qué registra |
|---|---|
| `upstream_error` (Gateway) | El servicio de destino y el estado, distinguiendo si estaba **no disponible** o si hubo **timeout**. Cada servicio tiene su propio mensaje |
| `identidad_http_error` | `request_id`, estado y error |
| `scraping_http_error` | `request_id`, estado y error |

Los logs no incluyen cookies, JWT ni tokens. Así lo exigen el contexto del proyecto y la regla de no imprimir secretos.

## Lo que falta

❌ Nadie agrega esos datos. La duración y el estado de cada petición quedan en los logs, pero no se calculan percentiles, tasa de error ni QPS. Tampoco hay paneles ni alertas.
