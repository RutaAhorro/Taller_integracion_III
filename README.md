# Plataforma de Comparación de Precios y Optimización de Rutas (Taller de Integración III)

Este repositorio contiene la arquitectura de microservicios para la plataforma de comparación de precios de alimentos de supermercados, incluyendo capacidades de optimización espacial. El proyecto está diseñado íntegramente bajo un ecosistema de **Docker** para garantizar aislamiento, control de recursos (vía cgroups) y portabilidad total.

## Arquitectura de Directorios

El repositorio se divide en dos dominios macro (`frontend` y `backend`), aplicando patrones de **Clean Architecture** y **Feature-Sliced Design** para maximizar la mutabilidad y mantenibilidad del código fuente.

```text
/
├── frontend/
│   └── web/                   # Aplicación principal (React + Vite)
│       └── src/               # Organizado en Features, Shared, Services y Core
├── backend/
│   ├── gateway/               # Proxy público Go, sin SQL ni reglas de negocio
│   ├── identidad/             # Login/SSO/perfil y usuarios (Go)
│   ├── catalogo/              # Productos/precios e ingesta privada (Go)
│   ├── control_scraping/      # Trabajos, outboxes y coordinación (Go)
│   ├── ia_conversacional/     # Asistente LLM con patrón anti-alucinaciones (FastAPI)
│   ├── motor_rutas/           # Microservicio de algoritmia y trazado geoespacial (Python)
│   └── scraper/               # Extractor asíncrono de catálogos y ofertas (Python)
├── docs/                      # Documentación del proyecto (PDFs, LaTeX, Guías Markdown)
├── docker-compose.yml         # Orquestador principal de la red de microservicios
├── .env.example               # Plantilla de variables de entorno requeridas
└── .gitignore                 # Exclusiones de Git (node_modules, pycache, .env, etc.)
```

### Dominio Frontend (`frontend/web`)
Desarrollado sobre **React + Vite**. La estructura interna se orienta a funcionalidades (Feature-Sliced Design) en lugar de tipos de archivo (ej. `src/features/comparador`), lo cual permite escalar la UI de manera orgánica y predecible. La imagen Docker de producción utiliza un *multi-stage build* que compila los estáticos con Node.js y los sirve mediante Nginx ligero.

### Dominio Backend (`backend/`)
- **`gateway/`:** Proxy público en Go; conserva rutas, cookies y headers, y enruta por dominio.
- **`identidad/`:** Registro, login/Google y perfil; sólo permisos SQL sobre usuarios.
- **`catalogo/`:** Lectura de productos/precios e ingesta privada idempotente; dueño de esos datos.
- **`control_scraping/`:** Coordinación, trabajos y outboxes; entrega resultados a Catálogo por HTTP.
- **`scraper/`:** Nodo _worker_ asíncrono aislado. Se encarga exclusivamente de consumir e iterar catálogos web/APIs de supermercados sin bloquear el servidor web principal.
- **`motor_rutas/`:** Microservicio dedicado a la alta carga matemática de trazado espacial (Problema del Viajante, A*), impulsado por OTP y Google OR-Tools.
- **`ia_conversacional/`:** Microservicio que integra Modelos de Lenguaje (Gemini/Groq) para interpretar consultas naturales y explicar resultados, operando estrictamente sobre datos pre-filtrados para evitar alucinaciones.

## Infraestructura Docker y Servicios

El entorno se levanta unificado a través de `docker-compose.yml`, el cual despliega una subred local (`microservices_net`) y coordina los servicios declarados en ese archivo. La entrada HTTP es Gateway (`8082`), a través de Nginx del frontend (`3000`); los tres servicios Go de dominio tienen runtime y escalado independientes.

El inventario de componentes de la propuesta inicial se conserva abajo como contexto; para el despliegue vigente consultar [Control de scraping y arquitectura final](docs/Guias/GUIA_CONTROL_SCRAPING_DESACOPLADO.md).

1. **`db` (PostgreSQL):** Base de datos unificada persistente.
2. **`redis`:** Message Broker en memoria para encolar tareas asíncronas del web scraper.
3. **`api_gateway`:** Backend RESTful, expuesto en el host vía puerto `8000`.
4. **`frontend_web`:** Servidor Nginx que entrega la WebApp, expuesto en el puerto `3000`.
5. **`web_scraper_alimentos`:** Contenedor restringido intencionalmente a 3GB RAM para prevenir fugas de memoria.
6. **`spatial_optimizer`:** Contenedor Python para resolver logísticas algorítmicas (TSP).
7. **`otp_engine`:** Motor backend en C++ (OpenTripPlanner) que procesa mapas viales para el contenedor espacial.
8. **`ia_chatbot_service`:** Asistente inteligente aislado en FastAPI.

## Despliegue Rápido (Modo Desarrollo)

1. Clona el archivo de configuración base:
   ```bash
   cp .env.example .env
   ```
2. Asegúrate de tener el demonio de Docker corriendo (ej. `sudo systemctl start docker` en Linux).
3. Levanta la malla de microservicios en segundo plano:
   ```bash
   docker compose up -d --build
   ```

> **Nota:** Para un desglose exhaustivo de comandos, resolución de errores comunes (como permisos de `docker.sock`) y buenas prácticas del equipo, revisa la [Guía de Docker (GUIA_DOCKER.md)](docs/GUIA_DOCKER.md).