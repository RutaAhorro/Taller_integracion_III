# Gateway — contexto IA

- Leer `README.md`: fuente única de rutas, variables, contratos y pruebas.
  Go estándar `net/http`, entrada `cmd/gateway/main.go`. Puerto 8080 en
  Compose/K8s; predeterminado del binario 8082.
- `internal/config`: carga y validación del entorno. `internal/api`: enrutador
  (`pasarela.go`), proxy inverso (`proxy.go`), `X-Request-ID`/`X-Forwarded-*`
  (`reenvio.go`) y sondas `/_gateway/*` (`salud.go`).
- Sin SQL, Redis, JWT_SECRET ni lógica de negocio. No agregar autenticación
  centralizada ni agregación de métricas; cada servicio aplica sus permisos.
- Enrutar auth, admin usuarios y direcciones del usuario (`/api/v1/usuarios/me/direcciones`) a Identidad, productos/admin productos a Catálogo,
  listas a Motor de rutas (`RUTAS_URL`), chat a IA (`IA_URL`) y scraper,
  swagger y `/health` a Control (`SCRAPING_URL`, obligatoria). Misiones no está
  enrutado. Prefijos delimitados; ruta sin dueño responde 404, sin respaldo
  entre dominios (`BACKEND_URL` se retiró en SUP-386).
- Mantener cookies, path/query codificados, Authorization, Idempotency-Key y
  X-Request-ID. Eliminar headers de identidad falsificables; forwarding solo
  por proxies fiables.
- No reintentar escrituras automáticamente. Live/ready locales; diagnóstico
  de dependencias separado para no retirar rutas sanas por otro dominio caído.
- Pruebas: `go test -race ./...` y `go vet ./...`, o en Windows
  `docker build --target test .` (la etapa copia `tests/`).

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

12 archivos fuente, incluidos los de pruebas. Distribución:
- `cmd`: 1 archivos.
- `internal`: 5 archivos.
- `tests`: 6 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
