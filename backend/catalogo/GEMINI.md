# Catálogo — contexto IA

- Leer `README.md` del servicio (configuración, contratos, operación y
  pruebas) y `docs/Guias/GUIA_DATOS_Y_PERMISOS_BACKEND.md` desde la raíz.
  Go/Gin/GORM, puerto 8080, entrada `cmd/catalogo/main.go`.
- HTTP en `internal/api` (router, sondas, CORS, sesión JWT, rutas internas,
  handlers y errores); negocio en `services`; SQL y verificación de permisos en
  `repositories`; DTO e interfaces en `domain`; entorno en `config`. Un archivo
  por tema, sin paquetes de un solo archivo.
- `DB_URL` usa `app_catalogo`: productos, capturas, cadenas, sucursales y lotes.
  No leer usuarios ni trabajos. No Redis ni AutoMigrate.
- JWT HS256 local con rol `admin` exacto; no confiar en headers de identidad ni
  consultar SQL de Identidad.
- API `/api/v1/productos*` pública; `/api/v1/admin/productos*` exige admin.
- `/internal/v1/*`: ingesta con `CATALOGO_INTERNAL_TOKEN` (escritura) y
  disponibilidad con `CATALOGO_CONSULTA_TOKEN` (sólo lectura), distintos de JWT
  y entre sí; no publicar mediante Gateway. Lote y recibo se confirman juntos,
  preservando idempotencia y las claves de advisory lock.
- Fuentes: actualizar solo `esta_activa`; no ampliar permisos de tabla completa.
- Startup valida permisos una vez; `/_catalogo/live` sin SQL y `ready` con Ping.
- Plazos y límites salen de variables `CATALOGO_*` con valor por defecto en
  `config`; CORS lee `CATALOGO_CORS_ALLOWED_ORIGINS` y luego `CORS_ALLOWED_ORIGINS`.
- No cambiar contratos HTTP ni columnas del esquema `scraper` (compartido con
  Control): anotarlo en los pendientes SUP-394 del README.
- Pruebas en `tests/unit` y `tests/integration`, contra la API pública de los
  paquetes: `go test -race ./...`, `go vet ./...`. La prueba con PostgreSQL usa
  `CATALOGO_TEST_DB_URL` y sólo una base desechable.
- No usar cifras/contratos históricos de SUP-267 como estado completo actual.
- Nombres descriptivos en español para variables, parámetros y receptores
  (`configuracion`, `listaID`, `respuesta`), nunca `c`, `p`, `u` o `l`. Sin
  excepciones por convención del lenguaje: `peticion`/`respuesta`, `contexto`,
  `errorObtenido`, `prueba` e `indice`.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

45 archivos fuente, incluidos los de pruebas. Distribución:
- `cmd`: 1 archivos.
- `internal`: 22 archivos.
- `tests`: 22 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
