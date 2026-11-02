# Catálogo — contexto IA

- Leer `README.md` del servicio y `docs/Guias/GUIA_DATOS_Y_PERMISOS_BACKEND.md`
  desde la raíz. Puerto interno 8080; entrada `cmd/catalogo/main.go`.
- Go/Gin/GORM. HTTP en `internal/server`, `handlers`; negocio en `services`;
  consultas en `repositories`; ingesta privada en `internal/ingesta`.
- `DB_URL` usa `app_catalogo`: productos, capturas, cadenas, sucursales y lotes.
  No leer usuarios ni trabajos. No Redis ni AutoMigrate.
- JWT HS256 local; no confiar en headers de identidad ni consultar SQL de Identidad.
- API `/api/v1/productos*` pública; `/api/v1/admin/productos*` exige admin.
- `/internal/v1/*`: token `CATALOGO_INTERNAL_TOKEN`, distinto de JWT; no publicar
  mediante Gateway. Lote y recibo se confirman juntos, preservando idempotencia.
- Fuentes: actualizar solo `esta_activa`; no ampliar permisos de tabla completa.
- Startup valida permisos una vez; `/_catalogo/live` sin SQL y `ready` con Ping.
- Pruebas desde este servicio: `go test -race ./...`, `go vet ./...`.
- No usar cifras/contratos históricos de SUP-267 como estado completo actual.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

34 archivos fuente, incluidos los de pruebas. Distribución:
- `cmd`: 3 archivos.
- `internal`: 31 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
