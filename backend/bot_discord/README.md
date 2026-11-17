# Bot de Discord

El Bot de Discord permite iniciar y consultar trabajos de scraping mediante
Control. También publica notificaciones de push, pull requests, revisiones y
comentarios de GitHub. No se conecta a PostgreSQL ni contiene credenciales de
usuario.

## Configuración

El proceso carga y valida las variables de entorno una vez al arrancar. Para
desarrollo local, copiar [.env.example](./.env.example) a `.env` y completar los
valores privados fuera del control de versiones.

| Variable | Requisito | Uso |
| --- | --- | --- |
| `DISCORD_BOT_TOKEN` | Obligatoria | Token privado del bot de Discord. |
| `DISCORD_CHANNEL_ID` | Obligatoria | Canal de notificaciones. |
| `DISCORD_PUSH_CHANNEL_ID` | Opcional | Canal de push; por defecto usa `DISCORD_CHANNEL_ID`. |
| `SCRAPING_SERVICE_TOKEN` | Obligatoria, mínimo 32 caracteres y sin espacios | Token M2M compartido con Control; no es un JWT ni se expone al navegador. |
| `SCRAPER_API_BASE_URL` | Obligatoria | URL de Control terminada en `/api/v1/scraper/trabajos`. En Compose se configura hacia Gateway. |
| `PORT` | Opcional, por defecto `8080` | Puerto del webhook de GitHub. |
| `SCRAPER_HTTP_TIMEOUT` | Opcional, por defecto `15s`, máximo `2m` | Timeout de peticiones del bot a Control. |
| `BOT_HTTP_READ_HEADER_TIMEOUT` | Opcional, por defecto `5s`, máximo `30s` | Presupuesto para cabeceras del webhook HTTP. |
| `SCRAPER_POLL_INTERVAL` | Opcional, por defecto `5s`, máximo `1m` | Intervalo de consulta del estado de trabajos. |
| `SCRAPER_MONITOR_MAX_FAILURES` | Opcional, por defecto `12`, rango `1–100` | Fallos consecutivos antes de detener el seguimiento. |
| `DISCORD_REVIEW_DEBOUNCE` | Opcional, por defecto `2500ms`, máximo `30s` | Ventana para agrupar solicitudes de revisión repetidas. |

Los nombres compartidos `SCRAPING_SERVICE_TOKEN` y `SCRAPER_API_BASE_URL`
respetan la [guía de configuración](../../docs/Guias/GUIA_CONFIGURACION.md).
La URL no admite credenciales, query ni fragmento. Si falta una variable
obligatoria o su formato es inválido, el proceso falla al arrancar.

## Uso y pruebas

Comandos de Discord: `!ping`, `!help` y `!scrapear`. El webhook de GitHub usa
`POST /webhooks/github`. El proceso se apaga de forma ordenada ante SIGINT o
SIGTERM.

Desde esta carpeta:

```bash
go test ./...
go vet ./...
go build ./...
```

Las pruebas unitarias están en `tests/unit/`; no necesitan conectarse a Discord
real. No iniciar el bot con tokens productivos ni ejecutar scraping real como
parte de una prueba.
