# RutaAhorro

Plataforma académica de comparación de precios de supermercados, desarrollada
para Taller de Integración III de la Universidad Católica de Temuco. Permite
consultar productos y precios obtenidos mediante scraping y administrar usuarios
y fuentes. El asistente de recetas y la optimización de compras forman parte del
alcance del producto, pero no todas sus integraciones están terminadas.

## Estado del proyecto

Después del refactor, el backend se divide por dominio; ya no hay una API
monolítica que reúna autenticación, catálogo y coordinación de scraping.

| Componente | Tecnología | Responsabilidad |
| --- | --- | --- |
| Frontend | React 19, TypeScript, Vite, Tailwind CSS; Nginx en producción | Catálogo, autenticación e interfaces de administración y listas |
| Gateway | Go, `net/http` | Proxy HTTP por dominio, sin SQL, Redis ni lógica de negocio |
| Identidad | Go, Gin, GORM | Registro, login local/Google, perfil y administración de usuarios |
| Catálogo | Go, Gin, GORM | Productos, precios, fuentes e ingesta privada idempotente |
| Control de scraping | Go, Gin, GORM | Trabajos, outboxes, publicación recuperable y entrega a Catálogo |
| Scraper | Go | Extracción de Jumbo, Líder, Santa Isabel, Acuenta y Cugat; Redis Streams y spool SQLite |
| PostgreSQL/PostGIS | PostgreSQL 15 | Persistencia con permisos separados por servicio |
| Redis | Redis 7 | Cola de scraping y controles de autenticación |
| Bot Discord | Go, discordgo | Operación de trabajos mediante la API |

El inventario de clúster informado por el equipo para el cierre del Sprint 2
incluye estos nueve componentes. **No es una consulta en vivo del clúster.**
El código incluye además:

- **Motor de rutas/Listas (FastAPI):** CRUD de listas implementado y opcional en
  Compose; gestión de artículos y optimización de rutas pendientes.
- **IA conversacional (FastAPI):** consulta Catálogo por HTTP y usa Gemini/Groq.
  Existe en Compose, pero el cliente web y el enrutamiento del chat todavía no
  están alineados. No está acreditado como flujo integrado en producción.
- **Vistas simuladas:** misiones, rutas e historial no deben presentarse como
  capacidades reales solo porque exista una pantalla. Las métricas del panel
  dependen de los endpoints disponibles en cada servicio.

## Arquitectura

```text
Navegador → Ingress/Nginx → Gateway → Identidad → PostgreSQL y Redis
                                  → Catálogo → PostgreSQL
                                  → Control → PostgreSQL y Redis Streams
Bot Discord ─────────────────────→ Gateway        ↓
                                             Scraper Go → Supermercados
                                                 ↓
                                        Gateway → Control → Catálogo
```

El worker persiste lotes y progreso en un spool SQLite y envía los resultados por
HTTP; no escribe directamente en PostgreSQL. Control registra entregas durables
y Catálogo confirma productos, precios y recibos de lote de forma transaccional.

Se utiliza **una instancia PostgreSQL/PostGIS**, no una base física por servicio.
Los esquemas `api`, `scraper` y `rutas` conservan nombres históricos; el aislamiento
se aplica mediante roles y permisos por tabla. Catálogo y Control tienen tablas
distintas dentro de `scraper`. Ningún dominio consulta tablas ajenas para resolver
sus casos de uso: se comunica por HTTP interno.

### Rutas del Gateway

| Prefijo | Destino |
| --- | --- |
| `/api/v1/auth/*`, `/api/v1/admin/usuarios/*` | Identidad |
| `/api/v1/productos*`, `/api/v1/admin/productos*` | Catálogo |
| `/api/v1/scraper/*`, `/health`, `/api/v1/health` | Control de scraping |
| `/api/v1/listas*` | Motor de rutas, solo si se configura `RUTAS_URL` |

El Gateway no valida JWT: los servicios aplican autenticación y autorización.
Scraping exige un token de servicio que **nunca debe enviarse al navegador**.
Los endpoints internos de Catálogo no se publican mediante el Gateway.

## Ejecución local

### Requisitos

- Docker Engine o Docker Desktop y Docker Compose v2.
- Git. No se requiere instalar Go o Python para levantar los contenedores.
- Puertos locales disponibles: `3000`, `8082`, `5432` y `6379` para el conjunto base.

### 1. Configurar el entorno

Desde la raíz, copiar la plantilla **solo si todavía no existe `.env`**:

```bash
cp .env.example .env
```

Completar los siguientes valores antes de arrancar:

| Variables | Uso |
| --- | --- |
| `DB_USER`, `DB_PASSWORD`, `DB_NAME` | Inicialización de PostgreSQL; cambiar la contraseña de ejemplo |
| `IDENTITY_DB_PASSWORD`, `CATALOGO_DB_PASSWORD`, `SCRAPING_DB_PASSWORD` | Credenciales distintas para cada dominio |
| `JWT_SECRET` | Clave compartida de sesión, al menos 32 bytes |
| `CATALOGO_INTERNAL_TOKEN` | Autenticación Control → Catálogo |
| `SCRAPING_SERVICE_TOKEN` | Autenticación worker/bot → Control |

Generar cada contraseña y token por separado, por ejemplo con
`openssl rand -hex 32`. No reutilizar secretos ni publicarlos en Git, capturas o
issues. No cambiar las contraseñas de un volumen existente solo editando `.env`:
la rotación debe coordinarse con PostgreSQL.

Google SSO, correo e IA requieren configuración adicional; sus variables están
documentadas en `.env.example`. Las claves `VITE_*` son públicas para el navegador:
no colocar allí contraseñas, tokens internos ni claves de proveedores IA.

### 2. Levantar el conjunto base

```bash
docker compose config --quiet
docker compose up -d --build db redis identidad catalogo control_scraping gateway frontend
docker compose ps
```

Este comando permite probar web/API sin arrancar el bot sin credenciales, la IA
sin configuración ni el worker que podría consumir trabajos existentes. No es
necesario preparar mapas ni OTP.

| Acceso local | URL |
| --- | --- |
| Aplicación web | <http://localhost:3000> |
| API mediante Gateway | <http://127.0.0.1:8082/api/v1/productos> |
| Vida del Gateway | <http://127.0.0.1:8082/_gateway/live> |
| Diagnóstico de dependencias | <http://127.0.0.1:8082/_gateway/dependencies> |

No hay una cuenta administradora predefinida documentada. La asignación inicial
del rol `admin` corresponde a un operador autorizado; no se elige al registrarse.

### 3. Componentes opcionales

- **Scraper Go:** Compose lo define como `scraper_worker`. Antes de arrancarlo,
  comprobar categorías y cola. Hay una discrepancia en la configuración del
  repositorio: Compose define `SCRAPER_CATEGORY_DIR=/app/categories`, pero el
  Dockerfile copia las categorías a `/app/research`. Alinear ese valor mediante
  configuración local antes de ejecutar `docker compose up -d --build scraper_worker`.
  Esta limpieza documental no cambia el runtime ni dispara trabajos.
- **Bot:** configurar `DISCORD_BOT_TOKEN` y `DISCORD_CHANNEL_ID` en el entorno de
  Compose y ejecutar `docker compose up -d --build bot_discord`.
- **IA:** configurar Gemini/Groq y ejecutar
  `docker compose up -d --build ia_conversacional`. No implica que el chat web
  quede integrado automáticamente.
- **Listas:** seguir el [README del motor](backend/motor_rutas/README.md), definir
  `RUTAS_DB_PASSWORD` y `RUTAS_URL=http://motor-rutas:8001`, provisionar el rol y
  ejecutar `docker compose --profile rutas up -d --build motor_rutas gateway`.

### Datos y operación diaria

Los scripts de `infrastructure/db/` se aplican automáticamente **solo al crear
un volumen vacío**. Para una base existente, seguir el [orden de migraciones](infrastructure/db/README.md)
con un operador autorizado. No borrar el volumen para actualizar el esquema.

```bash
docker compose logs -f gateway identidad catalogo control_scraping scraper_worker
docker compose down
```

`down` detiene el entorno sin solicitar borrar los volúmenes. No añadir `-v` si
se deben conservar PostgreSQL, Redis y el spool.

## Estructura del repositorio

```text
frontend/web/             Aplicación React y su README de desarrollo
backend/gateway/          Entrada HTTP pública
backend/identidad/        Cuentas y autenticación
backend/catalogo/         Productos, precios, fuentes e ingesta
backend/control_scraping/ Coordinación de trabajos
backend/scraper_go/       Worker y spiders Go
backend/bot_discord/      Integración operativa con Discord
backend/ia_conversacional/ Asistente IA; integración web pendiente
backend/motor_rutas/      Listas y prototipos de optimización
infrastructure/          SQL y manifiestos Kubernetes
scripts/                 Pruebas de integración y herramientas
docs/                    Guías, diagramas, requisitos y bibliografía
```

## Pruebas

Cada servicio tiene sus propias pruebas. Ejemplos con las herramientas instaladas
localmente; los README de servicio también describen opciones con Docker:

```bash
# Desde la raíz; repetir por módulo Go según el cambio.
(cd backend/gateway && go test -race ./... && go vet ./...)
(cd backend/scraper_go && go test ./... && go vet ./...)

# Cliente de listas del frontend (requiere npm ci en frontend/web).
node --test frontend/web/tests/listas-api.test.mjs
```

Los scripts `scripts/smoke_*` incluyen pruebas y rollbacks históricos; comprobar
sus dependencias antes de usarlos. Algunos todavía referencian el worker Python
retirado. No usar el clúster compartido para fixtures o pruebas destructivas.

En frontend, `npm run build` no garantiza ausencia de errores TypeScript: el
script actual permite continuar tras un fallo de `tsc`. Ver las comprobaciones
separadas en el [README del frontend](frontend/web/README.md).

## Kubernetes y documentación

Los manifiestos están en `infrastructure/k8s/`; no son prueba de qué imagen se
ejecuta realmente. Verificar namespace, imágenes/digests, Secrets, migraciones,
cuotas y probes antes de desplegar. **No aplicar toda la carpeta recursivamente**:
contiene recursos opcionales e históricos, incluido el despliegue del scraper Python.
Fortio es una herramienta de pruebas, no un servicio necesario para la aplicación.

- [Índice de documentación y vigencia](docs/README.md).
- [Frontend: configuración, rutas y límites](frontend/web/README.md).
- [Gateway](backend/gateway/README.md) e [Identidad](backend/identidad/README.md).
- [Catálogo](backend/catalogo/README.md) y [scraper Go](backend/scraper_go/README.md).
- [Propiedad de datos y permisos](docs/Guias/GUIA_DATOS_Y_PERMISOS_BACKEND.md).
- [Control de scraping desacoplado](docs/Guias/GUIA_CONTROL_SCRAPING_DESACOPLADO.md).
- [Flujo de ramas y pull requests](docs/Guias/GUIA_GITFLOW_Y_PR.md).

Las investigaciones y los documentos académicos formales se consultan en el
Drive del equipo. Los requisitos del repositorio describen también objetivos
pendientes; no son una lista de funcionalidades ya desplegadas.
