# Métricas — Disponibilidad ⚙️

[← Índice de métricas](README.md)

## Endpoints de salud

| Servicio | Vivo (*live*) | Listo (*ready*) | Otros |
|---|---|---|---|
| Gateway | `/_gateway/live` | `/_gateway/ready` | `/_gateway/dependencies`: estado de cada servicio interno |
| Catálogo | `/_catalogo/live` | `/_catalogo/ready` | `/health` |
| Motor de rutas | `/_rutas/live` | `/_rutas/ready` | `/health` |
| Control de scraping | — | — | `/health`, `/api/v1/health` (el Gateway los enruta) |
| IA conversacional | — | — | `/health` |

*Live* significa que el proceso responde. *Ready* significa que, además, sus dependencias (por ejemplo, la base de datos) están disponibles.

## Sondas de Kubernetes

| Servicio | Startup | Liveness | Readiness |
|---|---|---|---|
| Gateway | ✅ | ✅ | ✅ |
| Identidad | ✅ | ✅ | ✅ |
| Catálogo | ✅ | ✅ | ✅ |
| Control de scraping | ✅ | ✅ | ✅ |
| Motor de rutas | ✅ | ✅ | ✅ |
| Scraper Go, Bot, Frontend, PostgreSQL, Redis | ❌ | ❌ | ❌ |

Sin sondas, Kubernetes no reinicia un contenedor que se colgó y no saca de la rotación una réplica que no está lista.

## Disponibilidad medida

📏 En el benchmark se obtuvo un 100 % de éxito en 45.000 peticiones, sin reinicios ([Rendimiento medido](01_Rendimiento_medido.md)). ❌ No hay un registro histórico de *uptime*.
