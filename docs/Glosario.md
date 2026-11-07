# Glosario técnico

> Siglas y términos usados en RutaAhorro. Cada entrada indica qué significa y, cuando aplica, dónde se usa en el proyecto. Si una entrada contradice al código, vale el código.

## Categorías

- [Seguridad y autenticación](#seguridad-y-autenticación)
- [Redes y protocolos](#redes-y-protocolos)
- [Códigos de estado HTTP usados](#códigos-de-estado-http-usados)
- [Arquitectura y diseño](#arquitectura-y-diseño)
- [Datos y bases de datos](#datos-y-bases-de-datos)
- [Infraestructura y despliegue](#infraestructura-y-despliegue)
- [Desarrollo y herramientas](#desarrollo-y-herramientas)
- [IA y algoritmos](#ia-y-algoritmos)
- [Gestión del proyecto](#gestión-del-proyecto)

## Seguridad y autenticación

| Término | Significado | En el proyecto |
|---|---|---|
| **JWT** | *JSON Web Token*. Token firmado que transporta datos (claims) verificables sin consultar la base de datos. | Identidad lo emite al iniciar sesión; Identidad, Catálogo, Motor de rutas e IA lo validan. |
| **HS256** | *HMAC con SHA-256*. Algoritmo de firma simétrica: la misma clave firma y verifica. | Único algoritmo aceptado para el JWT (`JWT_SECRET` compartido). |
| **HMAC** | *Hash-based Message Authentication Code*. Código que prueba integridad y autenticidad usando una clave secreta. | Base de HS256. |
| **SHA-256** | *Secure Hash Algorithm, 256 bits*. Función hash criptográfica. | Hash de tokens de recuperación y comparación del token de servicio. |
| **Bcrypt** | Función de hash de contraseñas, lenta a propósito para frenar ataques de fuerza bruta. | Contraseñas en `api.usuarios`. Límite de 72 bytes. |
| **Claim** | Dato incluido dentro de un JWT (ej. `user_id`, `rol`, `exp`). | `user_id`, `rol`, `provider`, `exp`. |
| **OAuth 2.0** | Protocolo de autorización delegada (iniciar sesión con una cuenta de terceros). | Login con Google. |
| **OIDC** | *OpenID Connect*. Capa de identidad sobre OAuth 2.0 que entrega un `id_token`. | El `id_token` de Google que valida Identidad. |
| **GSI** | *Google Identity Services*. Biblioteca de Google para el botón "Iniciar sesión con Google". | `@react-oauth/google` en el frontend. |
| **Client ID** | Identificador público de una aplicación registrada en un proveedor OAuth. | `GOOGLE_CLIENT_ID`. Debe tener `http://localhost:3000` como origen autorizado. |
| **HttpOnly** | Atributo de cookie que impide leerla desde JavaScript. | La cookie `jwt`. Por eso el logout debe hacerlo el servidor. |
| **SameSite** | Atributo de cookie que limita su envío entre sitios (`Lax`, `Strict`, `None`). | `SameSite=Lax` en la cookie `jwt`. |
| **Secure (cookie)** | La cookie sólo viaja por HTTPS. | Configurable con `CookieSecure`. |
| **RBAC** | *Role-Based Access Control*. Permisos según el rol del usuario. | Roles `registrado`, `colaborador`, `admin` (`RequireRole`). |
| **IDOR** | *Insecure Direct Object Reference*. Acceder a recursos ajenos cambiando un ID. | Se evita tomando el usuario siempre del JWT, nunca del cuerpo ni de la URL. |
| **XSS** | *Cross-Site Scripting*. Inyección de JavaScript malicioso en una página. | Mitigado con cookies HttpOnly y el escape de React. |
| **CSRF** | *Cross-Site Request Forgery*. Hacer que el navegador de la víctima envíe peticiones no deseadas. | Mitigado con `SameSite=Lax` y la lista blanca CORS. |
| **CORS** | *Cross-Origin Resource Sharing*. Reglas del navegador para peticiones entre orígenes distintos. | Lista blanca `CORS_ALLOWED_ORIGINS` en cada servicio. |
| **Preflight** | Petición `OPTIONS` que el navegador envía antes de una petición CORS "no simple". | Respondida con 204 por el middleware CORS. |
| **Rate limiting** | Límite de peticiones por cliente en una ventana de tiempo. | 5 logins fallidos / 30 registros / 30 Google por IP cada 15 min (Redis). |
| **Fuerza bruta** | Probar muchas contraseñas o tokens hasta acertar. | Frenada con bcrypt y rate limiting. |
| **Enumeración de cuentas** | Descubrir qué correos están registrados según las respuestas del sistema. | La cola asíncrona de recuperación responde igual exista o no la cuenta. |
| **Timing attack** | Deducir secretos midiendo cuánto tarda una respuesta. | Comparación en tiempo constante (`subtle.ConstantTimeCompare`). |
| **Pre-account hijacking** | Registrar el correo de otra persona antes que ella para tomar su cuenta. | Al vincular con Google se invalida la contraseña local no verificada. |
| **Fail closed** | Ante una falla, rechazar en vez de permitir. | El rate limiter rechaza el login si Redis no responde. |
| **Token de servicio** | Secreto compartido entre servicios internos (no usuarios). | `SCRAPING_SERVICE_TOKEN` (worker/bot → Control) y `CATALOGO_INTERNAL_TOKEN`. |
| **Bearer** | Esquema del header `Authorization: Bearer <token>`. | Token de servicio de scraping. |
| **TLS** | *Transport Layer Security*. Cifrado de la conexión (sucesor de SSL). | HTTPS y envío de correo (TLS ≥ 1.2). |
| **STARTTLS** | Comando que convierte una conexión SMTP en texto plano a TLS. | Puerto 587 del mailer; si no hay STARTTLS no se envían credenciales. |
| **App Password** | Contraseña específica de aplicación (Gmail) en lugar de la contraseña real. | `SMTP_PASS` del mailer. |
| **Sanitización** | Limpiar la entrada del usuario antes de usarla. | `SanitizarInputBusqueda()` en Go; `normalizar_nombre()` en Python. |

## Redes y protocolos

| Término | Significado | En el proyecto |
|---|---|---|
| **DNS** | *Domain Name System*. Traduce nombres de dominio a direcciones IP. | Resolución interna de servicios en Docker y K8s (`api`, `identidad`, …). |
| **HTTP / HTTPS** | *HyperText Transfer Protocol (Secure)*. Protocolo de la web; HTTPS va cifrado con TLS. | Toda la comunicación frontend ↔ Gateway ↔ servicios. |
| **REST** | *Representational State Transfer*. Estilo de API basado en recursos y verbos HTTP. | `/api/v1/productos`, `/api/v1/listas`, … |
| **CRUD** | *Create, Read, Update, Delete*. Las cuatro operaciones básicas sobre datos. | CRUD de listas en el Motor de rutas (SUP-240). |
| **API** | *Application Programming Interface*. Contrato para que un programa use a otro. | API REST de cada microservicio. |
| **API Gateway** | Punto de entrada único que enruta peticiones a los servicios internos. | Servicio `gateway` (Go). |
| **Reverse proxy** | Servidor que recibe peticiones y las reenvía a otros servidores. | nginx del frontend y el Gateway (`httputil.ReverseProxy`). |
| **SMTP** | *Simple Mail Transfer Protocol*. Protocolo de envío de correo. | Mailer de Identidad (SUP-189). |
| **SPA** | *Single Page Application*. App web que carga una vez y navega sin recargar. | Frontend React. |
| **HMR** | *Hot Module Replacement*. Reemplazo de módulos en caliente sin recargar la página. | Vite en modo desarrollo. |
| **NAT** | *Network Address Translation*. Varios equipos comparten una IP pública. | Por eso el rate limit de login sólo cuenta intentos fallidos. |
| **Ingress** | Recurso de K8s que expone servicios HTTP hacia fuera del clúster. | `api-ingress`, `frontend-ingress`. |
| **Port-forward** | Redirigir un puerto local hacia un servicio del clúster. | Acceso a la API en el clúster UCT. |
| **X-Request-ID** | Header que identifica una petición de punta a punta. | Lo genera el Gateway y lo propagan todos los servicios. |
| **X-Forwarded-For** | Header con la IP original del cliente detrás de un proxy. | Usado por Identidad para el rate limit (proxies de confianza). |

## Códigos de estado HTTP usados

| Código | Nombre | Cuándo aparece en el proyecto |
|---|---|---|
| **200** | OK | Respuesta exitosa. |
| **201** | Created | Registro de usuario, creación de lista. |
| **204** | No Content | Logout, eliminar lista, preflight CORS. |
| **400** | Bad Request | Datos inválidos (validación). |
| **401** | Unauthorized | Sin sesión, JWT inválido o token de servicio incorrecto. |
| **403** | Forbidden | Rol insuficiente, cuenta inactiva u origen CORS no permitido. |
| **404** | Not Found | Recurso inexistente o ajeno (no se distingue a propósito). |
| **405** | Method Not Allowed | Verbo HTTP no soportado. |
| **409** | Conflict | Correo duplicado o máximo de listas alcanzado. |
| **429** | Too Many Requests | Rate limit excedido. |
| **500** | Internal Server Error | Error interno (sin exponer detalles). |
| **502** | Bad Gateway | El Gateway no pudo contactar un servicio. |
| **503** | Service Unavailable | Servicio no listo (readiness) o Google Login sin configurar. |
| **504** | Gateway Timeout | Un servicio no respondió a tiempo. |

## Arquitectura y diseño

| Término | Significado | En el proyecto |
|---|---|---|
| **Microservicios** | Arquitectura de servicios pequeños e independientes que se comunican por red. | Gateway, Identidad, Catálogo, Control de scraping, Motor de rutas, IA. |
| **Monolito** | Aplicación desplegada como una sola unidad. | La API Go original (`backend/api`), retirada en SUP-273 al separar Identidad, Catálogo y Control. |
| **DTO** | *Data Transfer Object*. Estructura para transportar datos entre capas sin exponer el modelo interno. | `RegistroDTO`, `PerfilUsuarioDTO`, `ProductoDTO`, … |
| **ORM** | *Object-Relational Mapping*. Traduce tablas SQL a objetos del lenguaje. | GORM (Go). |
| **Repository (patrón)** | Capa que encapsula el acceso a datos. | `UsuarioRepository`, `RepositorioListas`. |
| **Service (capa)** | Capa con la lógica de negocio (casos de uso). | `AuthService`, `RecuperacionService`. |
| **Handler** | Función que atiende una petición HTTP. | `AuthHandler`, `ProductoHandler`. |
| **Middleware** | Función que se ejecuta antes o después del handler (auth, CORS, logs). | `RequireAuth`, `RequireRole`, `RateLimitLogin`, CORS. |
| **Inyección de dependencias** | Pasar las dependencias desde fuera en vez de crearlas dentro. | Repositorios inyectados en servicios (permite mocks en tests). |
| **Strategy (patrón)** | Algoritmos intercambiables tras una misma interfaz. | `RoutingStrategy` del Motor de rutas; `ProveedorIA` de la IA. |
| **Fallback** | Alternativa automática cuando falla la opción principal. | `FallbackProveedor`: Groq → Gemini. |
| **Worker** | Proceso en segundo plano que consume trabajos de una cola. | Scraper Go (`scraper_worker`); cola de recuperación de contraseña de Identidad. |
| **Cola (queue)** | Estructura FIFO donde se encolan trabajos pendientes. | Redis Streams (trabajos de scraping) y canal Go (recuperación). |
| **Broker** | Intermediario de mensajes entre productores y consumidores. | Redis: Control publica trabajos y el Scraper Go los consume. |
| **Redis Streams** | Estructura de Redis para colas persistentes con confirmación de lectura. | Cola de trabajos entre Control y el Scraper Go. |
| **Spool** | Almacén local donde un proceso guarda trabajo pendiente para no perderlo si se reinicia. | SQLite del Scraper Go (lotes y progreso). |
| **Outbox** | Patrón que guarda eventos en la BD antes de publicarlos, para no perderlos. | Recuperación de scraping (SUP-270). |
| **Idempotencia** | Repetir una operación da el mismo resultado que hacerla una vez. | Migraciones SQL, logout, `Idempotency-Key` en el Gateway. |
| **Liveness / Readiness** | Sondas de salud: ¿el proceso vive? / ¿está listo para recibir tráfico? | `/_gateway/live`, `/_identidad/ready`, `/_rutas/ready`, … |
| **Healthcheck** | Comprobación periódica del estado de un servicio. | `healthcheck` de Docker Compose. |
| **Graceful shutdown** | Apagado ordenado que termina el trabajo pendiente. | `Cerrar()` del servicio de recuperación. |
| **KISS** | *Keep It Simple, Stupid*. Principio: preferir la solución más simple que funcione. | Criterio de la limpieza de los PRs #123–#125. |
| **Anti-alucinación (3 etapas)** | Patrón: LLM extrae filtros → Python filtra el catálogo → LLM explica sólo con datos validados. | IA conversacional (`filtros.py`). |

## Datos y bases de datos

| Término | Significado | En el proyecto |
|---|---|---|
| **SQL** | *Structured Query Language*. Lenguaje de bases de datos relacionales. | PostgreSQL. |
| **DDL** | *Data Definition Language*. SQL que define estructuras (`CREATE`, `ALTER`). | `infrastructure/db/*.sql`. |
| **DSN** | *Data Source Name*. Cadena de conexión a una base de datos. | `RUTAS_DB_URL`, `IDENTITY_DB_URL`. Nunca se registra en logs. |
| **Esquema (schema)** | Espacio de nombres dentro de una base de datos. | `api.*`, `scraper.*`, `rutas.*`. |
| **Grant** | Permiso SQL otorgado a un rol. | `06_propiedad_permisos.sql` (matriz de permisos por servicio). |
| **Rol de BD** | Usuario de PostgreSQL con permisos acotados. | `app_identity`, `app_catalogo`, `app_rutas`, … (mínimo privilegio). |
| **Migración** | Script que cambia el esquema de una BD existente de forma controlada. | `05_*.sql` a `09_*.sql`. |
| **FK** | *Foreign Key*. Columna que referencia otra tabla. | `rutas.listas_compras.usuario_id` → `api.usuarios`. |
| **UUID** | *Universally Unique Identifier*. Identificador de 128 bits prácticamente irrepetible. | IDs de usuarios, listas y productos. |
| **Pool de conexiones** | Conjunto de conexiones reutilizables a la BD. | `psycopg_pool` (máx. 5) y GORM. |
| **PostGIS** | Extensión geoespacial de PostgreSQL. | Ubicación de sucursales. |
| **Lua (script Redis)** | Script que Redis ejecuta de forma atómica. | `INCR` + `PEXPIRE` del rate limiter. |
| **TTL** | *Time To Live*. Tiempo de vida antes de expirar. | Claves de rate limit, tokens de recuperación (30 min). |
| **EAN / GTIN** | *European Article Number / Global Trade Item Number*. Código de barras de un producto. | Normalización de productos scrapeados. |
| **CLP** | *Peso chileno*. | Precios normalizados por `normalizar_precio_clp()`. |
| **JSON** | *JavaScript Object Notation*. Formato de intercambio de datos. | Todas las APIs. |

## Infraestructura y despliegue

| Término | Significado | En el proyecto |
|---|---|---|
| **Docker** | Plataforma de contenedores. | Todos los servicios. Sólo imágenes `-alpine` / `-slim`. |
| **Contenedor** | Proceso aislado con sus dependencias empaquetadas. | `frontend_web`, `bd_supermercados`, `redis_broker`, … |
| **Docker Compose** | Herramienta para definir y levantar varios contenedores. | `docker-compose.yml` (+ `override` local). |
| **Multi-stage build** | Dockerfile con etapas (ej. `development` / `production`). | Frontend: Vite (dev) o nginx (prod). |
| **cgroups** | *Control groups*. Límites de CPU y memoria de un contenedor en Linux. | `deploy.resources.limits` (frontend 250M en prod). |
| **OOM** | *Out Of Memory*. Proceso terminado por falta de memoria (exit 137). | OSMnx eliminado por OOM; `tsc` dentro del contenedor del frontend. |
| **K8s** | *Kubernetes*. Orquestador de contenedores. | Clúster UCT. |
| **Pod** | Unidad mínima de ejecución en K8s (uno o más contenedores). | Un pod por servicio. |
| **Deployment** | Recurso de K8s que mantiene N réplicas de un pod. | `infrastructure/k8s/*/deployment.yaml`. |
| **Secret / ConfigMap** | Recursos de K8s para configuración sensible / no sensible. | `scraping-service-secret`, `identidad-smtp`. |
| **NetworkPolicy** | Reglas de K8s sobre qué pods pueden comunicarse. | `infrastructure/k8s/identidad/networkpolicy.yaml`. |
| **CI/CD** | *Continuous Integration / Continuous Delivery*. Pruebas y despliegue automáticos. | GitHub Actions (`.github/workflows`). |
| **Smoke test** | Prueba rápida de punta a punta para verificar que lo esencial funciona. | `smoke_*_compose.sh`, `smoke_identity.py`. |
| **Zapto** | Servicio de DNS dinámico (No-IP). | Acceso público al frontend en el servidor local. |
| **Variable de entorno** | Configuración que el proceso lee del sistema. | `.env`, `VITE_API_URL`, `JWT_SECRET`, … |

## Desarrollo y herramientas

| Término | Significado | En el proyecto |
|---|---|---|
| **React** | Biblioteca de UI basada en componentes. | Frontend (v19). |
| **Vite** | Herramienta de build y servidor de desarrollo con HMR. | Frontend. |
| **TypeScript (TS)** | JavaScript con tipos estáticos. | Frontend. `tsc` verifica los tipos. |
| **Tailwind CSS** | Framework CSS de clases utilitarias. | Estilos del frontend (v4). |
| **Breakpoint** | Ancho de pantalla en que cambia el diseño responsive. | `sm:`, `md:`, `lg:` de Tailwind. |
| **Hook** | Función de React que agrega estado o efectos a un componente. | `useCatalog`, `useChatbot`, `useAuth`, … |
| **Context (React)** | Mecanismo para compartir estado sin pasar props. | `AuthContext`, `CartContext`, `ToastContext`. |
| **Linter** | Herramienta que detecta errores y malas prácticas. | `oxlint` (frontend), `go vet` (Go). |
| **Gin** | Framework HTTP de Go. | Identidad, Catálogo, Control. |
| **GORM** | ORM de Go. | Acceso a PostgreSQL en los servicios Go. |
| **Goroutine** | Hilo liviano de Go. | Worker de la cola de recuperación. |
| **Channel (Go)** | Mecanismo de comunicación entre goroutines. | `cola`, `detener`, `termino` del servicio de recuperación. |
| **FastAPI** | Framework web asíncrono de Python. | Motor de rutas e IA conversacional. |
| **Pydantic** | Validación de datos con tipos en Python. | Modelos `ListaCrear`, `ListaActualizar`. |
| **Scrapy** | Framework de web scraping en Python. | Scraper original; **retirado en #126**, reemplazado por el Scraper Go. |
| **Spider** | Componente que recorre un sitio y extrae sus datos. | `backend/scraper_go/internal/spiders/*.go` (Jumbo, Líder, Santa Isabel, Acuenta, Cugat). |
| **SQLite** | Base de datos embebida en un archivo, sin servidor. | Spool persistente del Scraper Go. |
| **Swagger / OpenAPI** | Especificación y documentación interactiva de una API. | `swag` en los servicios Go. |
| **Mock** | Objeto falso que simula una dependencia en los tests. | `mockUsuarioRepository`, `mockScraperRepository`. |
| **Fuzzing** | Pruebas con entradas aleatorias para encontrar fallos. | `jwt_fuzz_test.go`. |

## IA y algoritmos

| Término | Significado | En el proyecto |
|---|---|---|
| **LLM** | *Large Language Model*. Modelo de lenguaje de gran escala. | Groq y Gemini en la IA conversacional. |
| **Function calling** | El LLM devuelve una llamada estructurada (JSON) en vez de texto libre. | Etapa 1 del patrón anti-alucinación. |
| **Alucinación** | Respuesta del LLM inventada o no sustentada en datos. | Mitigada con el patrón de 3 etapas. |
| **TSP** | *Travelling Salesman Problem*. Encontrar la ruta más corta que visita todos los puntos. | Orden de visita a supermercados. Con 3 paradas como máximo se prueban todas las combinaciones ([D11](Decisiones/11_Motor_de_rutas_en_Go.md)). |
| **OR-Tools** | Biblioteca de Google para optimización (rutas, asignación). | Descartada para el Motor de rutas ([D11](Decisiones/11_Motor_de_rutas_en_Go.md)). |
| **OTP** | *OpenTripPlanner*. Planificador de rutas multimodal (incluye transporte público). | **No hay un OTP operativo** ni código que lo use: el transporte público queda fuera del Sprint 3. |
| **Haversine** | Fórmula de distancia entre dos coordenadas sobre una esfera. | Prototipo de ruta ficticia. |
| **Greedy** | Algoritmo voraz: elige la mejor opción local en cada paso. | Cálculo TSP inicial del prototipo. |

## Gestión del proyecto

| Término | Significado | En el proyecto |
|---|---|---|
| **Scrum** | Marco ágil de trabajo en iteraciones (sprints). | Semestral, UCT. |
| **Sprint** | Iteración de duración fija con objetivos definidos. | Sprint 2: 8 oct – 5 nov 2026. |
| **Scrum Master (SM)** | Facilita el proceso Scrum y elimina impedimentos. | Daniela. |
| **Backlog** | Lista priorizada de trabajo pendiente. | Linear. |
| **RN** | *Regla de Negocio*. | RN-07 (máx. paradas), RN-08 (umbral de rentabilidad), RN-20 (monto mínimo). |
| **GitFlow** | Modelo de ramas: `main` (producción), `develop` (integración) y ramas de trabajo. | `[área]/[tipo]/[tarea]-[nombre]`. |
| **PR** | *Pull Request*. Solicitud para integrar una rama, con revisión de código. | Squash and merge hacia `develop`. |
| **Squash and merge** | Combinar todos los commits de un PR en uno solo al integrarlo. | Regla del equipo. |
| **Commit convencional** | Formato `tipo: descripción` (`feat`, `fix`, `docs`, `refactor`). | Todos los commits. |
| **Code review** | Revisión del código por otro integrante antes de integrarlo. | Mínimo 1 aprobación por PR. |
| **SUP-xxx** | Identificador de una tarea en Linear. | Ej. SUP-240 (CRUD de listas), SUP-184 (recuperación). |
| **SWEBOK** | *Software Engineering Body of Knowledge*. Guía del conocimiento de la ingeniería de software (IEEE). | Guía de aprendizaje del equipo (#89). |
