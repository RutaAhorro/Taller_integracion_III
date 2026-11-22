# Bot Discord — contexto IA

- Go/discordgo; entrada `cmd/bot_discord/main.go`; webhook de GitHub en `PORT`
  (8081 en Compose, sin endpoint de salud). Estructura y comandos: `README.md`.
- Capas: `internal/config`, `internal/scraping` (cliente HTTP de Control) e
  `internal/discord` (comandos, interacciones, GitHub, webhook). Pruebas en
  `tests/unit/`, de caja negra y sin Discord ni Control reales.
- Conexión WSS a Discord; sin estado global: todo cuelga de `discord.Manejador`.
- HTTP hacia Gateway mediante `SCRAPER_API_BASE_URL`, con token
  `SCRAPING_SERVICE_TOKEN`. No SQL, JWT de usuario ni token privado de Catálogo.
- Credenciales Discord solo en entorno/Secrets; no imprimirlas en logs.
- Respetar contratos de trabajos, polling y respuestas pendientes; no asumir
  éxito de extracción por aceptar una solicitud de ejecución.
- Verificar con `gofmt -l .`, `go vet ./...` y `go test -race ./...` desde el servicio.
- No conectar un bot a Discord real ni disparar trabajos sin autorización.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

14 archivos fuente, incluidos los de pruebas. Distribución:
- `cmd`: 1 archivos.
- `internal`: 8 archivos.
- `tests`: 5 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
