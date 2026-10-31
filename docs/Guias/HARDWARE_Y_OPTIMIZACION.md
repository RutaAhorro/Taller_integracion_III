# Guía de Hardware y Optimización

Nuestra arquitectura se divide estrictamente en dos entornos: **Producción** (Clúster Kubernetes UCT) y **Desarrollo** (Local vía Docker). Por exigencia del proyecto, **TODA la producción (incluido el Frontend) debe alojarse en el clúster**.

Ambos entornos tienen recursos estrictamente limitados, por lo que la plataforma entera debe diseñarse con reglas defensivas para no agotar la RAM ni asfixiar la CPU.

## 1. Producción: Clúster Kubernetes UCT
El clúster aloja **todos** los microservicios: Frontend (React/Vite), API Gateway (Go), Scraper (Python), Bot de Discord, Base de Datos (PostgreSQL) y Redis. La restricción principal aquí es por Pod:

* **Límites de Pod en K8s:** `512MB de RAM` y `2 Núcleos (Cores)` máximo por Pod.
* *Justificación:* Al tener solo 512MB de RAM, **ningún servicio puede derrochar memoria**. El motor de rutas, el scraper y el backend en Go deben ser extremadamente cuidadosos con las estructuras (evitando cargar datasets enteros a la RAM). Un _memory leak_ o falta de paginación matará el Pod inmediatamente por *OOMKilled*.

## 2. Desarrollo: Entorno Local (Docker Compose)
El desarrollo se realiza en computadoras locales (como procesadores Intel Pentium básicos) utilizando `docker-compose`. 

* **Límites en Docker Compose (Local):**
  * Para asegurar que el código no falle en producción, el entorno local debe emular las restricciones del clúster usando `deploy.resources.limits` en el archivo `docker-compose.yml`.
  * Se asignan límites bajos (ej. 100MB al Bot, 200MB al API) para no asfixiar la máquina física local mientras corren todos los contenedores al mismo tiempo.

## 3. Dieta de los Dockerfiles (Ahorro de Disco y Memoria)
Para cumplir con estos límites tan estrictos de K8s, todos los `Dockerfile` del proyecto están estandarizados bajo técnicas de "Slimming":

1. **Imágenes Base Enanas:** 
   - Backend (Python): `python:3.11-slim` (Solo las librerías base de Debian, omitiendo el peso de un Ubuntu completo).
   - Backend y Bot (Go): Las binarias de Go se sirven en contenedores `-alpine` o `distroless`.
   - Frontend: `node:20-alpine` o `nginx:alpine` para servir los estáticos ocupando el mínimo espacio.
2. **Prevención de Basura en Disco/Memoria:** 
   - `ENV PYTHONDONTWRITEBYTECODE=1`: Instruye a Python a no crear archivos intermedios `.pyc`.
   - `pip install --no-cache-dir`: Evita que pip guarde copias ocultas de las librerías tras instalarlas.

## Manteniendo la Optimización a Futuro
Para el equipo de desarrollo, la regla de oro al agregar o modificar servicios es:
1. Iniciar siempre con imágenes `-slim` o `-alpine`.
2. Vigilar el consumo de RAM en las iteraciones. Con un límite de 512MB en Kubernetes, nunca hagas peticiones `SELECT *` completas a la base de datos sin paginación.
