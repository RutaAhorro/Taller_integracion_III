# Bot Discord — contexto IA

- Go/discordgo, entrada `main.go`, servidor operativo en puerto 8081.
- Conexión WSS a Discord; comandos de scraping en `scraper_commands.go`.
- HTTP hacia Gateway mediante `SCRAPER_API_BASE_URL`, con token
  `SCRAPING_SERVICE_TOKEN`. No SQL, JWT de usuario ni token privado de Catálogo.
- Credenciales Discord solo en entorno/Secrets; no imprimirlas en logs.
- Respetar contratos de trabajos, polling y respuestas pendientes; no asumir
  éxito de extracción por aceptar una solicitud de ejecución.
- Verificar pruebas con `go test ./...` y `go vet ./...` desde el servicio.
- No conectar un bot a Discord real ni disparar trabajos sin autorización.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

3 archivos fuente, incluidos los de pruebas. Distribución:
- `(raíz)`: 3 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
