# Motor de rutas / Listas — contexto IA

- Leer `README.md`. Go 1.26 con `net/http` (sin Gin), puerto 8001, entrada
  `cmd/motor_rutas/main.go`; opcional en Compose (perfil `rutas`). Reescrito
  desde Python en SUP-348 (D11) con el mismo contrato HTTP.
- `internal/config` (entorno), `internal/auth` (JWT), `internal/db` (pool pgx y
  verificación de grants), `internal/listas` (modelo y SQL),
  `internal/httpapi` (rutas, CORS, `X-Request-ID`, errores y handlers).
- `RUTAS_DB_URL`/`app_rutas`: CRUD listas y solo lectura de artículos.
  No leer `api.usuarios` ni tablas de Catálogo. Sin migraciones ni AutoMigrate.
- Sesión solo cookie HttpOnly `jwt` de Identidad. Filtrar siempre por dueño;
  lista ajena/id inválido devuelve 404 sin revelar existencia.
- 404 y 405 los despacha `httpapi.rutas` a mano para conservar el envoltorio
  `{"estado","mensaje","detalle"?}`; no reemplazar por `ServeMux` con métodos.
- Agregar/quitar artículos y validar stock llegan con SUP-241: no conceder
  permisos preventivos. No hay algoritmo de rutas ni OTP/OSRM todavía.
- Pruebas: `go test ./...` y `go vet ./...` desde el servicio (`-race` en
  Docker/CI). `tests/integration` requiere una BD desechable (ver README).

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

13 archivos fuente, incluidos los de pruebas. Distribución:
- `cmd`: 1 archivos.
- `internal`: 8 archivos.
- `tests`: 4 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
