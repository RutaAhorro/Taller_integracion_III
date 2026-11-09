# Métricas — Calidad y CI

[← Índice de métricas](README.md)

## Archivos de test por servicio 📏

Se cuentan archivos `*_test.go`, `test_*.py` y `*.test.mjs`, no tests individuales.

| Servicio | Archivos | En CI |
|---|---|---|
| Identidad | 20 | ✅ (en `gateway-checks`) |
| Scraper Go | 18 | ✅ |
| Control de scraping | 16 | ✅ (en `gateway-checks`) |
| Catálogo | 9 | ✅ |
| Gateway | 6 | ✅ |
| Motor de rutas | 4 | ✅ (en `motor-rutas-checks`) |
| IA conversacional | 2 | ✅ (en `ia-conversacional-checks`) |
| Bot Discord | 1 | ✅ (en `bot-discord-checks`) |
| Frontend | 1 (`tests/listas-api.test.mjs`) | ✅ (en `frontend-checks`) |

## Workflows (`.github/workflows/`)

| Workflow | Qué valida |
|---|---|
| `catalogo-checks.yml` | `go test -race`, `go vet`, build y un smoke test con Compose (`scripts/smoke_catalogo_compose.sh`) |
| `gateway-checks.yml` | `go test -race`, `go vet` y build de Gateway, Control e Identidad, más los smokes con Compose de Gateway, Identidad, Control, ingesta, enrutamiento de Catálogo, propiedad de datos y recuperación del scraping |
| `scraper-go-checks.yml` | `go test -race`, `go vet` y el **control de memoria** |
| `motor-rutas-checks.yml` | `go test -race`, `go vet` y build (Go) con fallback condicional (SUP-346) |
| `ia-conversacional-checks.yml` | `pytest tests/` (Python 3.11) y build Docker (SUP-346) |
| `bot-discord-checks.yml` | `go test -race`, `go vet`, build Go y build Docker (SUP-346) |
| `frontend-checks.yml` | `npm ci`, typecheck (`tsc --noEmit`), lint (`oxlint`), tests (`node --test`) y build (SUP-346) |
| `docker-publish.yml` | Publicación de imágenes |

## Control de memoria del Scraper Go 📏⚙️

Esta prueba corre en CI y **falla si se supera** el límite. Es el único umbral de rendimiento que se aplica automáticamente en el proyecto.

| Parámetro | Valor |
|---|---|
| Ítems procesados | 200.000 |
| Límite de memoria del proceso | 150 MB |
| Límite de memoria del contenedor (cgroup) | 192 MB |
| Resultado | `ram-result.json` (artefacto) |

## Política conocida

`npm run build` tolera los errores de `tsc` (`tsc -b || true`). Por eso el workflow `frontend-checks.yml` ejecuta `npx tsc --noEmit` de forma independiente para asegurar la corrección de tipos antes de empaquetar.

## Lo que falta

- ❌ Medición de cobertura.
