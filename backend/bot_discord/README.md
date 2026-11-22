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
| `PORT` | Opcional, por defecto `8080` | Puerto del webhook de GitHub (Compose usa `8081`). |
| `SCRAPER_HTTP_TIMEOUT` | Opcional, por defecto `15s`, máximo `2m` | Timeout de peticiones del bot a Control. |
| `BOT_HTTP_READ_HEADER_TIMEOUT` | Opcional, por defecto `5s`, máximo `30s` | Presupuesto para cabeceras del webhook HTTP. |
| `SCRAPER_POLL_INTERVAL` | Opcional, por defecto `5s`, máximo `1m` | Intervalo de consulta del estado de trabajos. |
| `SCRAPER_MONITOR_MAX_FAILURES` | Opcional, por defecto `12`, rango `1–100` | Fallos consecutivos antes de detener el seguimiento. |
| `DISCORD_REVIEW_DEBOUNCE` | Opcional, por defecto `2500ms`, máximo `30s` | Ventana para agrupar solicitudes de revisión repetidas. |

Los nombres compartidos `SCRAPING_SERVICE_TOKEN` y `SCRAPER_API_BASE_URL`
respetan la [guía de configuración](../../docs/Guias/GUIA_CONFIGURACION.md).
La URL no admite credenciales, query ni fragmento. Si falta una variable
obligatoria o su formato es inválido, el proceso falla al arrancar.

## Comandos de Discord

| Comando | Efecto |
| --- | --- |
| `!ping` | Comprueba que el bot responde. |
| `!help` | Lista los comandos. |
| `!scrapear` | Publica el menú con un botón por cadena (Jumbo, Lider, Santa Isabel, A Cuenta, Cugat) y «Todos a la vez». |

Los botones piden a Control crear y ejecutar un trabajo, avisan al usuario
cuando termina (o falla) y ofrecen «Actualizar Estado». Que Control acepte la
ejecución no garantiza que la extracción termine bien: el resultado se conoce
solo consultando el estado. Los identificadores de botón son
`scrape_<cadenaID>_<rastreador>`, `scrape_0_todos` y `refresh_<trabajoID>`.

## Operación

- **Webhook de GitHub:** `POST /webhooks/github` en el puerto `PORT` (8081 en
  Compose; 8080 por defecto si no se define). Atiende `ping`, `push`,
  `pull_request`, `pull_request_review` e `issue_comment` y publica en
  `DISCORD_CHANNEL_ID` (push en `DISCORD_PUSH_CHANNEL_ID`). Otros eventos se
  aceptan y se ignoran. No hay endpoint de salud.
- **Control:** llamadas HTTP con `Authorization: Bearer <SCRAPING_SERVICE_TOKEN>`:
  `POST {SCRAPER_API_BASE_URL}` (crear, espera 201), `POST .../{id}/ejecutar`
  (espera 202) y `GET .../{id}` (estado). Esos contratos no cambian aquí.
- **Apagado:** ante SIGINT o SIGTERM cierra el servidor de webhooks, detiene los
  seguimientos de trabajos y cierra la sesión de Discord.
- **Seguridad:** el token de servicio solo vive en el cliente de Control; no se
  imprime ni se envía a Discord. El bot no usa PostgreSQL, JWT de usuario ni el
  token interno de Catálogo.

## Estructura

```text
cmd/bot_discord/main.go     arranque, señales y cierre ordenado
internal/config/            carga y validación de variables de entorno
internal/scraping/          cliente HTTP de Control: cliente.go, cadenas.go
internal/discord/           comandos.go, scraping.go, github.go, webhook.go,
                            manejador.go (estado compartido y registro)
tests/unit/                 pruebas por paquete (config, scraping, discord)
```

## Pruebas

Desde esta carpeta (el CI usa Go 1.26):

```bash
gofmt -l .
go vet ./...
go test -race ./...
go build -o /tmp/bot_discord ./cmd/bot_discord
```

Las pruebas son de caja negra en `tests/unit/`: simulan la API de Discord con
un transporte HTTP falso y Control con `httptest`, así que no necesitan
credenciales ni red. No hay pruebas de integración porque el bot no usa
PostgreSQL ni Redis. No iniciar el bot con tokens productivos ni ejecutar
scraping real como parte de una prueba.

## Pendientes conocidos

- El mapa de logins de GitHub a menciones de Discord está en
  `internal/discord/github.go`; moverlo a configuración exige acordar una
  variable nueva con el equipo.
- `IntentsGuildMessages` no incluye el intent privilegiado de contenido de
  mensajes; si los comandos de texto no responden en el servidor, revisar ese
  intent en el portal de Discord y en el código.
