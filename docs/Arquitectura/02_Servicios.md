# Arquitectura — Servicios

[← Índice de arquitectura](README.md)

| Servicio | Carpeta | Tecnología | Puerto | Rol BD |
|---|---|---|---|---|
| Gateway | `backend/gateway` | Go `net/http` | 8080 (host `127.0.0.1:8082`) | — |
| Identidad | `backend/identidad` | Go, Gin, GORM | 8080 | `app_identity` |
| Catálogo | `backend/catalogo` | Go, Gin, GORM | 8080 | `app_catalogo` |
| Control de scraping | `backend/control_scraping` | Go, Gin, GORM | 8080 | `app_scraping` |
| Scraper Go | `backend/scraper_go` | Go | — (worker) | ninguno |
| Bot Discord | `backend/bot_discord` | Go, discordgo | 8081 | ninguno |
| Motor de rutas 🟡 | `backend/motor_rutas` | Python, FastAPI | 8001 | `app_rutas` |
| IA conversacional 🟡 | `backend/ia_conversacional` | Python, FastAPI | 8002 | ninguno |
| Frontend | `frontend/web` | React 19, TS, Vite, Tailwind 4; Nginx | 3000 | — |

## Gateway

Es un proxy inverso que enruta según el prefijo de la ruta:

- Genera `X-Request-ID`.
- Elimina las cabeceras `X-User-ID` y `X-Role` que vengan del cliente, para evitar suplantación.
- Usa transportes separados para lecturas y para escrituras.
- Expone `/_gateway/live` y `/_gateway/dependencies`.

## Identidad

Gestiona las cuentas y la sesión:

- Registro y login local.
- Login con Google, con validación criptográfica del ID token.
- Perfil de usuario.
- Recuperación de contraseña mediante una cola asíncrona.
- `POST /auth/logout`, agregado en #124.
- Administración de usuarios.

Emite la cookie `jwt`. Los roles son `registrado`, `colaborador` y `admin`. El plan (`basico` o `premium`) es independiente del rol.

## Catálogo

- Lectura pública de productos y precios.
- Administración de productos.
- Endpoint interno de ingesta **idempotente**, basado en recibos de lote. Solo Control puede llamarlo, con `CATALOGO_INTERNAL_TOKEN`, y no se publica a través del Gateway.

## Control de scraping

- Crea los trabajos de scraping y sigue su estado.
- Los publica en Redis Streams mediante un **outbox**, para que una publicación fallida se pueda recuperar.
- Reenvía los resultados a Catálogo con entregas durables.

## Scraper Go

Reemplaza al scraper Python desde #126:

- Consume trabajos desde Redis Streams.
- Extrae datos de Jumbo, Líder, Santa Isabel, Acuenta y Cugat (`internal/spiders/*.go`).
- Guarda los lotes y el progreso en un **spool SQLite**.
- Envía los resultados por HTTP. No tiene acceso a PostgreSQL.

## Bot Discord

Es la interfaz operativa del equipo: lanza y consulta trabajos a través del Gateway con `SCRAPING_SERVICE_TOKEN`.

## Motor de rutas 🟡

- CRUD de listas del usuario, con un máximo de 100 listas.
- El usuario se obtiene siempre del JWT, nunca de cabeceras ni del cuerpo.
- Se activa con el perfil `rutas` y la variable `RUTAS_URL`.
- Pendiente: artículos y optimización de rutas.

## IA conversacional 🟡

- Pipeline en tres pasos: extracción con LLM (Gemini/Groq), luego filtrado determinista, luego una explicación que solo usa datos validados.
- Consulta Catálogo por HTTP, nunca PostgreSQL.
- No pasa por el Gateway. El cliente web aún no está alineado con su ruta (SUP-253).

## Frontend

- El router está en `src/app/router.tsx`.
- La sesión vive en `AuthContext` y las peticiones usan `credentials: 'include'`.
- Distingue datos de API, mocks y estado local.
- El build de producción (`npm run build`) tolera errores de `tsc`.
