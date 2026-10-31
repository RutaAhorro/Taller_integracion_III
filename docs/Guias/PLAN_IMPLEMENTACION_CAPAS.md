# Plan de Implementación por Capas: Arquitectura de Microservicios

Este documento establece la hoja de ruta técnica para migrar de forma progresiva, segura y ordenada desde la arquitectura centralizada hacia la malla de **Microservicios** contenerizada en **Kubernetes (UCT)** y **Docker Compose**.

El plan está estructurado bajo un enfoque **Bottom-Up por Capas** (desde la base de datos hasta los clientes), garantizando que ningún componente se desacople sin tener su infraestructura subyacente y contratos estables, respetando los límites de hardware (**2 vCPUs y 512 MB de RAM por pod**).

---

## Índice de Capas de Implementación

1. [Capa 0: Persistencia y Aislamiento de Datos](#capa-0-persistencia-y-aislamiento-de-datos)
2. [Capa 1: Infraestructura, Red y Service Discovery](#capa-1-infraestructura-red-y-service-discovery)
3. [Capa 2: Microservicios de Dominio](#capa-2-microservicios-de-dominio)
4. [Capa 3: API Gateway y Seguridad Perimetral](#capa-3-api-gateway-y-seguridad-perimetral)
5. [Capa 4: Clientes y Consumidores](#capa-4-clientes-y-consumidores)
6. [Capa 5: Resiliencia, Pruebas y Observabilidad](#capa-5-resiliencia-pruebas-y-observabilidad)
7. [Matriz de Asignación Scrum (Equipo de 6)](#matriz-de-asignación-scrum-equipo-de-6)

---

## Capa 0: Persistencia y Aislamiento de Datos

### Objetivo:
Hacer cumplir el principio **Schema-per-Service** en el pod único de PostgreSQL 15 + PostGIS, garantizando independencia lógica sin sobrecargar la memoria con múltiples instancias de base de datos.

### Tareas Técnicas:
1. **Separación de Roles y Permisos en PostgreSQL:**
   * Crear usuarios dedicados en `infrastructure/db/init.sql`:
     * `user_api` ➔ Acceso exclusivo con permisos CRUD a `api.*`.
     * `user_scraper` ➔ Acceso exclusivo a `scraper.*`.
     * `user_rutas` ➔ Acceso exclusivo a `rutas.*`.
2. **Eliminación de Llaves Foráneas Inter-Esquema:**
   * Auditar los scripts DDL (`01_schema_api.sql`, `02_schema_scraper.sql`, `03_schema_rutas.sql`).
   * Reemplazar restricciones `FOREIGN KEY` entre esquemas diferentes por identificadores lógicos (UUID o enteros). La integridad referencial entre servicios se validará a nivel de aplicación vía HTTP/Eventos.
3. **Limpieza de Modelos GORM en Go:**
   * En [infrastructure/models.go](file:///c:/Users/itand/OneDrive/Documents/GitHub/Taller_integracion_III/backend/api/infrastructure/models.go), eliminar los structs correspondientes a `scraper.*` y `rutas.*`. Mantener únicamente los 13 modelos del esquema `api.*`.

### Criterio de Aceptación (DoD):
* Ningún servicio puede ejecutar consultas `SELECT/INSERT/UPDATE` en esquemas que no le pertenecen.
* Las migraciones de un esquema pueden ejecutarse de manera independiente sin romper los demás.

---

## Capa 1: Infraestructura, Red y Service Discovery

### Objetivo:
Definir los canales de comunicación interna y límites de recursos para cada servicio, tanto en local como en Kubernetes.

### Tareas Técnicas:
1. **Configuración de Red Local (`docker-compose.yml`):**
   * Configurar nombres de host fijos y alias dentro de `microservices_net`:
     * `api-gateway` (puerto expuesto `8080`)
     * `ms-catalogo-auth` (interno)
     * `ms-motor-rutas` (interno)
     * `ms-ia-conversacional` (interno)
     * `scraper-worker` (interno)
     * `bd-supermercados` (Postgres, puerto `5432`)
     * `redis-broker` (Redis, puerto `6379`)
2. **Manifiestos de Kubernetes (`infrastructure/k8s/`):**
   * Crear un `Deployment` y un `Service` (tipo `ClusterIP`) por cada microservicio.
   * Fijar cuotas de cómputo estrictas en los manifiestos:
     ```yaml
     resources:
       requests:
         cpu: "200m"
         memory: "150Mi"
       limits:
         cpu: "2"
         memory: "512Mi"
     ```
3. **Service Discovery por CoreDNS:**
   * La comunicación interna entre pods se realizará mediante URLs nativas de Kubernetes:
     * `http://ms-catalogo-service:8080`
     * `http://ms-rutas-service:8000`
     * `http://ms-ia-service:8000`

### Criterio de Aceptación (DoD):
* Todos los servicios pueden comunicarse entre sí por nombre de host en Docker Compose y por Service DNS en Kubernetes.
* Ningún contenedor compite desordenadamente por recursos en el clúster.

---

## Capa 2: Microservicios de Dominio

### Objetivo:
Independizar cada unidad de negocio en su propio contenedor, con su propio framework y ciclo de vida.

### Tareas Técnicas:

#### 2.1. Microservicio Catálogo & Identidad (`backend/api` ➔ `ms-catalogo-auth`):
* **Tecnología:** Go 1.26 + Gin + GORM.
* **Dominio:** Autenticación (JWT, Google OAuth), usuarios, categorías, marcas, productos normalizados y precios actuales.
* **Ajustes:** Aislar sus repositorios y handlers para responder únicamente a su contexto.

#### 2.2. Microservicio de Rutas y Logística (`backend/motor_rutas`):
* **Tecnología:** Python 3.11 + FastAPI + OR-Tools.
* **Dominio:** Optimización espacial (TSP), cálculo de distancias sobre mapas y gestión de paradas (`rutas.*`).
* **Ajustes:** Exponer endpoints REST (`/api/v1/rutas/optimizar`) y limitar Uvicorn a 1 o 2 workers para evitar fugas de memoria en los 512 MB.

#### 2.3. Microservicio de IA Conversacional (`backend/ia_conversacional`):
* **Tecnología:** Python 3.11 + FastAPI.
* **Dominio:** NLP y consultas semánticas bajo el patrón anti-alucinación de 3 etapas.
* **Ajustes:** Consumo stateless de APIs externas (Groq / Gemini) mediante `httpx.AsyncClient`.

#### 2.4. Worker de Scraping (`backend/scraper`):
* **Tecnología:** Python + Scrapy + Redis.
* **Dominio:** Extracción periódica de precios de retail.
* **Ajustes:** Ingesta directa en lotes pequeños (chunks de 100 productos) sobre `scraper.*` para mantener el uso de RAM bajo 450 MB.

### Criterio de Aceptación (DoD):
* Cada microservicio puede compilarse, testearse y ejecutarse de forma 100% aislada.

---

## Capa 3: API Gateway y Seguridad Perimetral

### Objetivo:
Establecer un punto único de entrada perimetral que libere a los microservicios internos de la seguridad y el enrutamiento.

### Tareas Técnicas:
1. **Desacoplar la Persistencia del Gateway:**
   * El API Gateway (en Go) no mantendrá pool de conexiones a PostgreSQL ni ejecutará consultas de base de datos.
2. **Reverse Proxy Dinámico (`httputil.ReverseProxy` en Go o Ingress K8s):**
   * Configurar el despacho de peticiones entrantes:
     * `/api/v1/auth/*` ➔ `http://ms-catalogo-auth:8080/auth/*`
     * `/api/v1/productos/*` ➔ `http://ms-catalogo-auth:8080/productos/*`
     * `/api/v1/rutas/*` ➔ `http://ms-motor-rutas:8000/rutas/*`
     * `/api/v1/chat/*` ➔ `http://ms-ia-conversacional:8000/chat/*`
     * `/api/v1/scraper/*` ➔ `http://ms-scraper-service:8080/scraper/*`
3. **Descarga de Autenticación (Auth Offloading):**
   * El Gateway intercepta las peticiones que requieren sesión, valida la firma del token JWT y propaga los claims del usuario a los servicios internos mediante cabeceras HTTP seguras:
     * `X-User-ID: <uuid>`
     * `X-User-Role: <rol>`
4. **Rate Limiting y CORS:**
   * Centralizar en el Gateway el rate limiting respaldado por Redis para proteger los microservicios internos de saturación o ataques de fuerza bruta.

### Criterio de Aceptación (DoD):
* El Frontend solo realiza peticiones al puerto público `:8080`.
* Si un token JWT es inválido o expiró, el Gateway responde `401 Unauthorized` de inmediato sin cargar a los microservicios de dominio.

---

## Capa 4: Clientes y Consumidores

### Objetivo:
Conectar las interfaces finales al API Gateway sin fricciones y adaptando los contratos de consumo.

### Tareas Técnicas:
1. **Frontend (React 19 + TypeScript):**
   * Verificar que todas las llamadas de la API en `frontend/web/src/` (incluyendo `useCatalog.ts`, `products-api.ts`, etc.) apunten al prefijo relativo `/api/v1/*` del Gateway.
   * Asegurar que las variables de entorno (`VITE_API_URL=/api/v1`) sean respetadas tanto en desarrollo local como en producción detrás del Ingress.
2. **Bot de Discord (`backend/bot_discord`):**
   * Actualizar la variable `SCRAPER_API_BASE_URL` para consultar los estados de scraping y salud del sistema a través del endpoint perimetral del Gateway.

### Criterio de Aceptación (DoD):
* Los usuarios pueden navegar el catálogo, autenticarse, consultar a la IA y solicitar rutas sin percibir la distribución de servicios subyacente.

---

## Capa 5: Resiliencia, Pruebas y Observabilidad

### Objetivo:
Garantizar la estabilidad operativa en el clúster de Kubernetes, aislar fallos y validar la arquitectura.

### Tareas Técnicas:
1. **Sondas de Salud (Liveness & Readiness Probes):**
   * Configurar endpoints `/health` ligeros en cada microservicio:
     * *Liveness:* Verifica que el proceso no esté congelado.
     * *Readiness:* Verifica que las dependencias mínimas (ej. Redis o su esquema de BD) respondan antes de recibir tráfico.
2. **Timeouts y Circuit Breaking en el Gateway:**
   * Establecer un timeout máximo de 5 segundos en el proxy inverso para llamadas a microservicios externos (especialmente IA y Rutas). Si un servicio tarda o se reinicia por `OOMKilled`, el Gateway responderá con un error controlado (`504 Gateway Timeout` o fallback elegante) sin caerse.
3. **Pruebas de Integración End-to-End:**
   * Implementar tests que validen el flujo completo: Registro ➔ Búsqueda de Productos ➔ Cálculo de Ruta con paradas ➔ Consulta a la IA.

### Criterio de Aceptación (DoD):
* La caída simulada de `ms-ia-conversacional` o `scraper-worker` no afecta la navegación de productos ni el login en la plataforma web.

---

## Matriz de Asignación Scrum (Equipo de 6)

Para organizar los Sprints respetando las convenciones de GitFlow (`[área]/[tipo]/[tarea]-[nombre_dev]`):

| Integrante / Área | Responsabilidad Principal | Rama Sugerida de Ejemplo |
| :--- | :--- | :--- |
| **Dev 1 (DevOps / Docker)** | Capa 0 y 1: Configurar K8s manifests, isolation de esquemas SQL y cgroups de 512 MB. | `docker/feat/k8s-manifests-devops` |
| **Dev 2 (Backend Go - Gateway)** | Capa 3: Desacoplar persistencia del Gateway, implementar Reverse Proxy y Auth Offload. | `backend/refactor/gateway-proxy-dev` |
| **Dev 3 (Backend Go - Catálogo)** | Capa 2.1: Limpiar `models.go`, dejar `ms-catalogo-auth` enfocado en esquema `api.*`. | `backend/feat/ms-catalogo-dev` |
| **Dev 4 (Rutas / Python)** | Capa 2.2: Ajustar `ms-motor-rutas` con OR-Tools, esquema `rutas.*` y control de memoria. | `backend/feat/ms-rutas-dev` |
| **Dev 5 (IA / Python)** | Capa 2.3: Consolidar `ms-ia-conversacional` con endpoints REST y pipeline anti-alucinación. | `ia/feat/ms-ia-rest-dev` |
| **Dev 6 (Frontend / QA)** | Capa 4 y 5: Integración de hooks React al Gateway, pruebas E2E y healthchecks. | `frontend/feat/api-gateway-hooks-dev` |

---

## Resumen de Riesgos y Mitigaciones

| Riesgo Identificado | Impacto | Estrategia de Mitigación |
| :--- | :---: | :--- |
| **`OOMKilled` (Memoria > 512 MB)** | Alto | Limitar workers de Uvicorn a 1, procesar scraping en batches de 100 y fijar connection pool de SQL a max 10 conexiones. |
| **Divergencia de Contratos JSON** | Medio | Diseñar y validar los archivos OpenAPI / Swagger antes de comenzar a escribir código. |
| **Latencia entre Microservicios** | Bajo | Utilizar la red interna de Kubernetes (`ClusterIP`) y CoreDNS, evitando saltos hacia Internet exterior. |
