# Guía de Arquitectura y Uso del Backend en Go (API Gateway y Servicios de Dominio)

> Estado vigente SUP-273: Gateway, Identidad, Catálogo y Control de scraping
> son servicios Go independientes. `backend/api` y su despliegue fueron retirados.
> Consulta [la arquitectura final y operación](GUIA_CONTROL_SCRAPING_DESACOPLADO.md).
> Las secciones sobre API, Air y modelos globales de esta guía describen el diseño
> anterior; no usar sus comandos de `backend/api` en el checkout actual.


Esta guía documenta la estructura, el rol arquitectónico y las convenciones del backend en **Go (Golang)** dentro de la plataforma de comparación de precios y optimización de rutas, adaptado a la arquitectura de **Microservicios** contenerizada en Kubernetes y Docker Compose.

---

## 1. Visión General: El Rol de Go en los Microservicios

El backend de la plataforma se compone de servicios especializados desacoplados según sus requerimientos de cómputo y dominio. **Go 1.26+** es la tecnología central para la capa de enrutamiento perimetral y los servicios transaccionales de alto rendimiento gracias a:

1. **Eficiencia Extrema de Recursos:** Un servicio en Go con Gin consume entre **15 MB y 40 MB de memoria RAM en reposo**, encajando holgadamente en el límite de cuota de **512 MB de RAM por pod** del clúster Kubernetes.
2. **Concurrencia Nativa:** Modelo de *goroutines* ligero capaz de atender miles de peticiones simultáneas con latencia mínima.
3. **Seguridad y Tipado Estricto:** Validación de tipos en tiempo de compilación y sanitización centralizada de entradas.

---

## 2. Patrón API Gateway (Go + Gin)

El componente principal en `backend/api/` asume la responsabilidad del **API Gateway** y la gestión transaccional de catálogo/usuarios:

### Responsabilidades Exclusivas del API Gateway:
* **Punto Único de Entrada (Single Entry Point):** Expone un puerto público único (`8080`) accesible por el Frontend (React 19) y el Bot de Discord.
* **Descarga de Autenticación (Auth Offloading):** Valida las cabeceras de autorización (`Bearer <token>`) mediante el middleware `RequireAuth()` antes de reenviar el tráfico a servicios internos.
* **Control de Tráfico y Seguridad:** Aplica **Rate Limiting** respaldado por Redis (`middleware/rate_limit.go`), políticas de CORS dinámicas y middleware global de recuperación de errores (`middleware/error_handler.go`).
* **Reverse Proxy / Enrutamiento Interno:** Redirige peticiones a microservicios satélite mediante el DNS interno de Kubernetes o Docker.

> **Regla Arquitectónica:** El API Gateway desacopla la seguridad de la lógica pesada. Los microservicios de cómputo intensivo (optimización de rutas y procesamiento de lenguaje natural) no gestionan sesiones de usuario ni exponen puertos públicos directos.

---

## 3. Tabla de Enrutamiento y Microservicios Satélite

Todas las rutas públicas se exponen bajo el prefijo unificado `/api/v1/`:

| Prefijo de Ruta | Servicio Destino | Tecnología | Responsabilidad de Dominio |
| :--- | :--- | :--- | :--- |
| `/api/v1/auth/*` | `ms-auth` / Core | Go + Gin | Registro, Login, Google OAuth, JWT, perfiles |
| `/api/v1/productos/*` | `ms-catalogo` / Core | Go + Gin | Búsqueda, filtros de catálogo, precios normalizados |
| `/api/v1/rutas/*` | `ms-motor-rutas` | Python + FastAPI + OR-Tools | Optimización geoespacial, problema del viajante (TSP/OTP) |
| `/api/v1/chat/*` | `ms-ia-conversacional` | Python + FastAPI | Pipeline anti-alucinación, integración con LLMs (Groq/Gemini) |
| `/api/v1/scraper/*` | `scraper-worker` | Python + Scrapy | Ingesta de capturas crudas y auditoría de arañas |

---

## 4. Persistencia: Patrón *Schema-per-Service*

Para maximizar los recursos del hardware sin violar el principio de microservicios, el clúster utiliza un único pod de **PostgreSQL 15 + PostGIS** con aislamiento estricto por esquemas (*Schema-per-Service*):

* **Esquema `api.*`:** Propiedad de Catálogo e Identidad (`usuarios`, `productos_normalizados`, `marcas`, `categorias`).
* **Esquema `scraper.*`:** Propiedad del subsistema de extracción (`cadenas_supermercado`, `sucursales`, `trabajos_scraper`, `capturas_precios`).
* **Esquema `rutas.*`:** Propiedad del motor de rutas (`listas_compras`, `ejecuciones_optimizacion`, `paradas_optimizacion`).

> **Regla de Oro:** Ningún microservicio realiza consultas `JOIN` directas contra esquemas ajenos. La comunicación entre dominios se realiza exclusivamente vía contratos HTTP REST o eventos en Redis.

---

## 5. Estructura de Directorios (`backend/api/`)

El código Go está organizado bajo una arquitectura limpia y modular:

```text
backend/api/
├── cmd/                 # Puntos de entrada auxiliares (ej. scripts de seed de productos)
├── domain/              # DTOs, contratos de entrada/salida y structs de transferencia
├── handlers/            # Controladores HTTP (Gin Handlers con anotaciones Swag)
├── infrastructure/      # Modelos de base de datos GORM correspondientes al esquema api.*
├── middleware/          # Middlewares transversales (JWT, RBAC, Rate Limiting, ErrorHandler, CORS)
├── repositories/        # Interfaces y lógica de persistencia SQL con GORM
├── routes/              # Registro y agrupación modular de endpoints (/auth, /productos, /scraper, /admin)
├── services/            # Reglas de negocio y orquestación de casos de uso
├── utils/               # Utilidades de seguridad (SanitizarInputBusqueda, JWT, bcrypt)
├── tests/               # Pruebas unitarias y de integración de handlers, servicios y middlewares
├── docs/                # Archivos auto-generados de Swagger UI (swaggo)
├── Dockerfile           # Imagen multi-stage optimizada (Alpine / builder Air)
└── main.go              # Inicialización de dependencias, conexión a DB/Redis y servidor Gin
```

---

## 6. Entorno de Desarrollo Local (Hot-Reload)

El entorno local se levanta mediante `docker-compose.yml` en la raíz del repositorio:

```bash
docker-compose up --build -d
```

### Recarga Automática con Air:
El contenedor `go_service_api` utiliza [Air](https://github.com/air-verse/air). Cualquier modificación guardada en archivos `.go` dentro de `backend/api/` será detectada automáticamente, recompilando el binario y reiniciando el servidor en milisegundos sin necesidad de reiniciar Docker.

---

## 7. Despliegue en Kubernetes (Clúster UCT)

En el entorno de producción/staging en Kubernetes:
* **Límites de Cómputo por Pod:** 2 vCPUs y **512 MB de memoria RAM**.
* **Service Discovery:** La comunicación interna entre pods se resuelve automáticamente por CoreDNS (`http://<service-name>:<port>`).
* **Healthcheck:** El endpoint `GET /health` responde el estado del servicio y la conectividad con la base de datos y Redis para las sondas *liveness* y *readiness* de Kubernetes.

---

## 8. Documentación con Swagger UI

La API expone documentación viva auto-generada compatible con OpenAPI 2.0 / 3.0:

1. **Ruta en navegador:**
   👉 `http://localhost:8082/swagger/index.html` (Gateway; Swagger lo sirve Control de scraping)

2. **Actualización de Documentación:**
   Al agregar o modificar anotaciones `@Summary`, `@Tags`, `@Param` o `@Router` en `handlers/`, regenerar los archivos estáticos ejecutando en `backend/api/`:
   ```bash
   swag init --parseInternal
   ```
   Air detectará los cambios en `docs/docs.go` y actualizará Swagger UI automáticamente.

---

## 9. Pruebas Automatizadas

El backend incluye pruebas unitarias para autenticación, productos, middlewares y seguridad. Para ejecutarlas localmente:

```bash
cd backend/api
go test -v ./tests/...
```
