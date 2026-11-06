# Métricas — Calidad y CI

[← Índice de métricas](README.md)

## Archivos de test por servicio 📏

Se cuentan archivos `*_test.go`, `test_*.py` y `*.test.mjs`, no tests individuales.

| Servicio | Archivos | En CI |
|---|---|---|
| Identidad | 20 | ❌ |
| Scraper Go | 18 | ✅ |
| Control de scraping | 16 | ✅ (en `gateway-checks`) |
| Catálogo | 9 | ✅ |
| Gateway | 6 | ✅ |
| Motor de rutas | 4 | ❌ |
| IA conversacional | 2 | ❌ |
| Bot Discord | 1 | ❌ |
| Frontend | 1 (`tests/listas-api.test.mjs`) | ❌ |

Resultados locales de la limpieza KISS (#123–#125):

- Identidad: `go test` y `go vet` en verde.
- Motor de rutas: 19 tests OK y 3 saltados, porque requieren PostgreSQL.
- Frontend: `tsc` bajó de 8 errores a 0, y `oxlint` quedó limpio.

## Workflows (`.github/workflows/`)

| Workflow | Qué valida |
|---|---|
| `catalogo-checks.yml` | `go test -race`, `go vet`, build y un smoke test con Compose (`scripts/smoke_catalogo_compose.sh`) |
| `gateway-checks.yml` | `go test -race` y `go vet` del Gateway y de Control, más el build de la imagen de Control |
| `scraper-go-checks.yml` | `go test -race`, `go vet` y el **control de memoria** |
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

`npm run build` tolera los errores de `tsc` (`tsc -b || true`). Por eso un build exitoso no garantiza que los tipos sean correctos, y hay que correr `npx tsc -b --noEmit` por separado.

## Lo que falta

- ❌ Medición de cobertura.
- ❌ CI para Identidad, Motor de rutas, IA, Bot y Frontend.
