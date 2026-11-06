# Métricas — Métricas de negocio

[← Índice de métricas](README.md)

## Trabajos de scraping 📏

Control de scraping guarda estos campos por cada trabajo (`backend/control_scraping/internal/domain/scraper.go`):

| Campo | Significado |
|---|---|
| `estado` | En curso, `completado` o `fallido` |
| `iniciado_el` / `finalizado_el` | Inicio y fin del trabajo |
| `duracion_segundos` | Cuánto tardó la extracción |
| `elementos_extraidos` | Productos obtenidos |
| `respuestas_bloqueadas` | Respuestas bloqueadas por el supermercado (anti-bot) |
| `registro_errores` | Detalle de las fallas |

Hay datos de cinco cadenas: Jumbo, Líder, Santa Isabel, Acuenta y Cugat.

## Panel de administración

`GET /api/v1/admin/usuarios/metricas` (Identidad) entrega las métricas de usuarios del panel. Las demás tarjetas del panel dependen de los endpoints que tenga cada servicio. Las que no tienen respaldo en la API son simuladas (⚪) y no deben presentarse como datos reales.

## Límites funcionales ⚙️

| Regla | Valor | Servicio |
|---|---|---|
| Logins fallidos | 5 por IP cada 15 min | Identidad |
| Registros | 30 por IP cada 15 min | Identidad |
| Logins con Google | 30 por IP cada 15 min | Identidad |
| Duración de la sesión | 24 h (86.400 s) | Identidad |
| Largo máximo de contraseña | 72 bytes (límite de bcrypt) | Identidad |
| Listas por usuario | 100 | Motor de rutas |

## Lo que falta

- ❌ Usuarios activos.
- ❌ Búsquedas realizadas.
- ❌ Ahorro real calculado: los costos y ahorros de las rutas son simulados.
- ❌ Uso del chat de IA.
