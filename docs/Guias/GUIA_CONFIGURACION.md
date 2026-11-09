# Guía de configuración y variables de entorno compartidas (Sprint 3)

> **Estado:** acordado · SUP-345  
> **Aplica a:** Identidad, Catálogo, Control de scraping, Scraper Go, Motor de rutas, IA conversacional, Gateway, Frontend y scripts de despliegue.

## Propósito

Esta guía fija de forma definitiva los nombres, dueños, consumidores, formatos y valores de las variables de entorno compartidas del sistema antes de que cada integrante realice el barrido de su servicio (SUP-347 a SUP-354).

Al trabajar con microservicios independientes, las variables compartidas constituyen un **contrato de infraestructura**. Si un integrante cambia una variable por su cuenta, rompe la integración en Docker Compose, los despliegues en Kubernetes y los flujos de CI/CD.

---

## Regla innegociable

> **Durante el barrido, ningún servicio renombra una variable compartida por su cuenta.**

- El barrido de cada servicio es una refactorización interna. Renombrar una variable compartida (por ejemplo, cambiar `JWT_SECRET` por `SECRET_KEY`, o `IDENTITY_DB_URL` por `DATABASE_URL`) invalida el contrato con los demás servicios y con los archivos de infraestructura compartida (`docker-compose.yml`, manifiestos K8s, GitHub Actions).
- Si durante la limpieza encuentras una variable mal nombrada o una inconsistencia, **no la cambies unilateralmente**: regístrala en la descripción de tu PR. Los cambios de nombres compartidos se planifican y ejecutan de forma coordinada entre productor y consumidores.

---

## Tabla exhaustiva de variables compartidas

### 1. Autenticación y credenciales de usuario

| Nombre | Servicio Dueño | Servicios Consumidores | Propósito y formato requerido | Valor por defecto en Compose / local |
| --- | --- | --- | --- | --- |
| `JWT_SECRET` | Identidad | Catálogo, Motor de rutas, IA conversacional | Secreto para firma y validación de tokens JWT de usuario (HMAC-SHA256). Cadena alfanumérica de al menos 32 bytes (64 caracteres hexadecimales recomendados), sin espacios. | Obligatorio sin valor por defecto (`${JWT_SECRET:?}`). Generar con `openssl rand -hex 32`. |
| `GOOGLE_CLIENT_ID` | Identidad / Frontend | Identidad, Frontend (`VITE_GOOGLE_CLIENT_ID`) | Identificador de cliente OAuth 2.0 de Google. Cadena con terminación `.apps.googleusercontent.com`. | Opcional, vacío por defecto (`${GOOGLE_CLIENT_ID:-}`). Sin él, el login con Google queda deshabilitado. |

> [!NOTE]
> `control_scraping` y `gateway` **no** consumen ni deben recibir `JWT_SECRET`: Control usa tokens de servicio dedicados y Gateway opera como proxy inverso ciego.

### 2. Tokens de servicio (M2M / autenticación interna)

| Nombre | Servicio Dueño | Servicios Consumidores | Propósito y formato requerido | Valor por defecto en Compose / local |
| --- | --- | --- | --- | --- |
| `CATALOGO_INTERNAL_TOKEN` | Catálogo (`/internal/v1/*`) | Control de scraping | Autoriza la ingesta durable de lotes de productos y precios directamente a Catálogo. Al menos 32 caracteres sin espacios. **Debe ser estrictamente distinto de `JWT_SECRET` y de `SCRAPING_SERVICE_TOKEN`**. Se envía como `Authorization: Bearer <token>`. | Obligatorio sin valor por defecto (`${CATALOGO_INTERNAL_TOKEN:?}`). Generar con `openssl rand -hex 32`. |
| `SCRAPING_SERVICE_TOKEN` | Control de scraping (`/api/v1/scraper/*`) | Scraper Go (`scraper_worker`), Bot de Discord (`bot_discord`) | Autoriza la ejecución de trabajos de scraping y el envío de resultados hacia Control. Al menos 32 caracteres sin espacios. **Debe ser estrictamente distinto de `JWT_SECRET` y de `CATALOGO_INTERNAL_TOKEN`**. Se envía como `Authorization: Bearer <token>`. | Obligatorio sin valor por defecto (`${SCRAPING_SERVICE_TOKEN:?}`). Generar con `openssl rand -hex 32`. |

### 3. Broker y colas

| Nombre | Servicio Dueño | Servicios Consumidores | Propósito y formato requerido | Valor por defecto en Compose / local |
| --- | --- | --- | --- | --- |
| `REDIS_URL` | Infraestructura (Redis) | Identidad, Control de scraping, Scraper Go | Cadena de conexión al broker Redis para limitación de tasa (rate limiting de autenticación en Identidad), coordinación de locks distribuidos y colas de scraping. Formato: `redis://[user:pass@]host:port/db`. | En Compose: `redis://redis:6379/0`. En ejecución local directa: `redis://localhost:6379/0`. |

### 4. URLs internas de servicios

Todas las URLs de servicio deben ser **orígenes HTTP(S) limpios**: deben incluir esquema (`http` o `https`) y host/puerto, pero **no** deben contener credenciales, rutas/path, parámetros de query ni fragmentos `#`.

| Nombre | Servicio Dueño | Servicios Consumidores | Propósito y formato requerido | Valor por defecto en Compose / local |
| --- | --- | --- | --- | --- |
| `CATALOGO_URL` | Catálogo | Gateway | Origen HTTP de Catálogo para enrutar `/api/v1/productos*`. Las rutas `/api/v1/cadenas*` y `/api/v1/sucursales*` son rutas objetivo pendientes de implementación (Gateway hoy enruta principalmente `/api/v1/productos*`; un 404 en ellas no es fallo de configuración). Formato origen sin path. | `http://catalogo:8080` (en Compose) / `http://127.0.0.1:8083` (local). |
| `CATALOGO_INTERNAL_URL` | Catálogo | Control de scraping, IA conversacional | Origen HTTP interno de Catálogo para ingesta de lotes y consulta de productos/precios sin pasar por Gateway. Formato origen sin path. | `http://catalogo:8080` (en Compose) / `http://127.0.0.1:8080` o `:8083` (local). |
| `IDENTIDAD_URL` | Identidad | Gateway | Origen HTTP de Identidad para enrutar `/api/v1/auth/*` a través de la red aislada `identidad_edge`. Formato origen sin path. | `http://identidad-upstream:8080` (en Compose) / `http://127.0.0.1:8084` (local). |
| `SCRAPING_URL` | Control de scraping | Gateway | Origen HTTP de Control de scraping para enrutar `/api/v1/scraper/*`. Formato origen sin path. | `http://control-scraping:8080` (en Compose) / `http://127.0.0.1:8080` (local). |
| `RUTAS_URL` | Motor de rutas | Gateway | Origen HTTP del motor de rutas para enrutar `/api/v1/listas*`. Si está vacío, Gateway devuelve 404 (servicio opcional bajo perfil `rutas`). Formato origen sin path. | Vacío por defecto (`${RUTAS_URL:-}`). Con perfil `rutas`: `http://motor-rutas:8001`. |
| `IA_URL` | IA conversacional | Gateway / Frontend (`VITE_IA_URL`) | [Convención objetivo / Pendiente SUP-278] Origen HTTP del microservicio de IA conversacional (`/api/v1/chat*`, `/api/v1/explicacion*`). Gateway actualmente NO la consume ni la enruta (el chat hoy se conecta directo o mediante frontend). Formato origen sin path. | `http://ia-conversacional:8002` (en Compose) / `http://localhost:8002` (local). |

#### Excepciones a la regla de URLs sin path

- `SCRAPER_API_URL`: consumida por Scraper Go (`scraper_worker`), requiere el path completo (en Compose: `http://gateway:8080/api/v1/scraper/productos`).
- `SCRAPER_API_BASE_URL`: consumida por Bot de Discord (`bot_discord`), requiere el path base (en Compose: `http://api:8080/api/v1/scraper/trabajos`).

### 5. Base de datos PostgreSQL

El sistema utiliza una base de datos PostgreSQL compartida dividida lógicamente mediante esquemas y roles dedicados por microservicio (principio de mínimo privilegio).

#### A. Variables maestras de administración (infraestructura / inicialización)

Utilizadas únicamente por el contenedor `db` al inicializarse y por los scripts de migración ejecutados por el administrador:

| Nombre | Propósito | Valor por defecto en Compose / local |
| --- | --- | --- |
| `DB_USER` | Usuario superadministrador de PostgreSQL para inicialización y migraciones estructurales. | `postgres` |
| `DB_PASSWORD` | Contraseña del usuario superadministrador. | `secret_password` (en local) |
| `DB_NAME` | Nombre de la base de datos única donde residen los esquemas de todos los servicios. | `supermercados` |

#### B. Contraseñas por rol de servicio

Cada servicio cuenta con un rol exclusivo en PostgreSQL. Ningún servicio accede con el usuario `postgres`:

| Variable | Rol asignado en PostgreSQL | Servicio correspondiente | Regla de generación |
| --- | --- | --- | --- |
| `IDENTITY_DB_PASSWORD` | `app_identity` | Identidad | Obligatorio, generar con `openssl rand -hex 32` |
| `CATALOGO_DB_PASSWORD` | `app_catalogo` | Catálogo | Obligatorio, generar con `openssl rand -hex 32` |
| `SCRAPING_DB_PASSWORD` | `app_scraping` | Control de scraping | Obligatorio, generar con `openssl rand -hex 32` |
| `RUTAS_DB_PASSWORD` | `app_rutas` | Motor de rutas | Opcional, requerido con perfil Compose `rutas` |

#### C. DSNs de conexión por servicio

Los microservicios reciben su conexión completa en una variable DSN con la siguiente estructura estándar:

$$\text{postgres://}\langle\text{usuario}\rangle:\langle\text{password}\rangle@\langle\text{host}\rangle:\langle\text{puerto}\rangle/\langle\text{database}\rangle\text{?sslmode=}\langle\text{modo}\rangle$$

| Nombre | Consumidor | Rol obligatorio | Valor típico en Compose |
| --- | --- | --- | --- |
| `IDENTITY_DB_URL` | Identidad | `app_identity` | `postgres://app_identity:${IDENTITY_DB_PASSWORD}@db:5432/${DB_NAME}?sslmode=disable` |
| `DB_URL` | Catálogo | `app_catalogo` | `postgres://app_catalogo:${CATALOGO_DB_PASSWORD}@db:5432/${DB_NAME}?sslmode=disable` |
| `SCRAPING_DB_URL` | Control de scraping | `app_scraping` | `postgres://app_scraping:${SCRAPING_DB_PASSWORD}@db:5432/${DB_NAME}?sslmode=disable` |
| `RUTAS_DB_URL` | Motor de rutas | `app_rutas` | `postgres://app_rutas:${RUTAS_DB_PASSWORD}@db:5432/${DB_NAME}?sslmode=disable` |

> [!IMPORTANT]
> - Cada servicio valida al arrancar que el usuario de su DSN corresponda estrictamente a su rol asignado (por ejemplo, Identidad rechaza arrancar si el DSN no usa `app_identity`).
> - **Nombre de `DB_URL` en Catálogo:** Catálogo utiliza históricamente `DB_URL`. **No** se debe renombrar a `CATALOGO_DB_URL` durante el barrido para no romper la compatibilidad con el entorno actual.

### 6. CORS unificado: `<SERVICIO>_CORS_ALLOWED_ORIGINS`

Para evitar colisiones entre servicios en desarrollo local y permitir orígenes diferenciados por servicio, se adopta el patrón prefijado:

$$\langle\text{SERVICIO}\rangle\text{\_CORS\_ALLOWED\_ORIGINS}$$

> [!NOTE]
> El patrón `<SERVICIO>_CORS_ALLOWED_ORIGINS` con fallback a `CORS_ALLOWED_ORIGINS` es la convención objetivo para los barridos de código del Sprint 3. Hoy en día Compose inyecta variables específicas y algunos servicios aún leen directamente `CORS_ALLOWED_ORIGINS`.

| Variable | Servicio que la utiliza | Valor por defecto Compose / local |
| --- | --- | --- |
| `IDENTIDAD_CORS_ALLOWED_ORIGINS` | Identidad | `http://localhost:3000,http://localhost:5173` |
| `CATALOGO_CORS_ALLOWED_ORIGINS` | Catálogo | `http://localhost:3000,http://localhost:5173` |
| `SCRAPING_CORS_ALLOWED_ORIGINS` | Control de scraping | `http://localhost:3000,http://localhost:5173` |
| `RUTAS_CORS_ALLOWED_ORIGINS` | Motor de rutas | `http://localhost:3000,http://localhost:5173` |
| `IA_CORS_ALLOWED_ORIGINS` | IA conversacional | `http://localhost:3000,http://localhost:5173` |

#### Reglas de formato para CORS:
1. Lista separada por comas de orígenes HTTP(S) exactos (esquema + host + puerto).
2. **Sin slash final** (por ejemplo, `http://localhost:3000`, nunca `http://localhost:3000/`).
3. **Sin comodines** `*` ni rutas/paths.
4. **Regla de lectura en el código:** Al leer la configuración, el servicio debe buscar primero `<SERVICIO>_CORS_ALLOWED_ORIGINS` y, si no está definida, usar `CORS_ALLOWED_ORIGINS` como fallback antes del valor predeterminado.

---

## Pautas de seguridad transversal

### 1. Generación de secretos con `openssl`

Todos los secretos (`JWT_SECRET`, contraseñas de PostgreSQL, `CATALOGO_INTERNAL_TOKEN`, `SCRAPING_SERVICE_TOKEN`) deben generarse mediante:

```bash
openssl rand -hex 32
```

**Por qué hexadecimal (`-hex 32`):**
Genera 64 caracteres seguros con 256 bits de entropía criptográfica real. Al contener exclusivamente caracteres alfanuméricos (`0-9`, `a-f`), evita caracteres especiales de escape de URLs (como `@`, `:`, `/`, `?`, `#` o `%`), impidiendo errores de parseo en los DSNs de base de datos o Redis.

### 2. Archivos `.env` y control de versiones

- **Nunca subir `.env` a git.** El archivo `.env` local contiene secretos reales y debe figurar siempre en `.gitignore`.
- **Mantener `.env.example` al día:** Cada servicio y la raíz del proyecto mantienen un `.env.example` con comentarios claros y valores vacíos o placeholders ilustrativos.
- **Falla rápida al arrancar:** Si una variable obligatoria falta en tiempo de ejecución, el servicio debe terminar de inmediato con un mensaje descriptivo en lugar de continuar con valores por defecto inseguros o cadenas vacías.

### 3. Variables de Frontend: prefijo `VITE_*`

En aplicaciones creadas con Vite (`frontend/web`), cualquier variable con prefijo `VITE_` se **empaqueta estáticamente dentro del código Javascript del cliente**:

- **Públicas por diseño:** Cualquier usuario puede abrir las herramientas de desarrollador del navegador e inspeccionar el contenido de cualquier variable `VITE_*`.
- **Cero secretos en `VITE_*`:** Nunca asignar credenciales privadas, tokens de servicio, contraseñas de base de datos ni claves secretas a variables `VITE_*`.
- **Uso válido:** Únicamente configuraciones públicas como la URL base de la API (`VITE_API_URL=/api/v1`), la URL pública de IA (`VITE_IA_URL`) o el Client ID público de Google (`VITE_GOOGLE_CLIENT_ID`).

---

## Documentos relacionados

- [Guía del barrido de código](GUIA_BARRIDO_CODIGO.md): reglas de limpieza de tests, mocks y hardcodeo.
- [Datos y permisos del backend](GUIA_DATOS_Y_PERMISOS_BACKEND.md): roles de PostgreSQL, propiedad de esquemas y permisos.
- [Docker para desarrollo](GUIA_DOCKER.md): ejecución local con Compose.
- [Índice de documentación](../README.md): mapa completo de la documentación del proyecto.
