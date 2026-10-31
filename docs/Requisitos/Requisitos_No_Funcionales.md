# 🛠️ Requisitos No Funcionales y Restricciones de Hardware

Este documento explica las limitaciones técnicas de nuestro entorno (Servidor Local Pentium / Clúster UCT) y cómo el código debe adaptarse a ellas.

---

## 1. Restricciones de Infraestructura (Producción vs Desarrollo)
Por exigencia del proyecto, la arquitectura **NO** es híbrida en producción: todo el sistema debe vivir en el clúster.
* **Producción (Clúster K8s UCT):** TODOS los servicios (Frontend, Backend, Scraper, Bot, Base de Datos) se despliegan en Kubernetes. Cada pod tiene un límite estricto de **512MB de RAM y 2 Núcleos**. **Impacto:** Cualquier _memory leak_ o falta de paginación en Go/Python matará el pod por OOM (Out Of Memory). El frontend en Nginx debe estar ultra-optimizado.
* **Desarrollo (Local con Docker):** El desarrollo se realiza localmente utilizando `docker-compose`. Las máquinas locales (ej. Intel Pentium) deben emular estas restricciones para garantizar que lo que funciona en local, funcione en el clúster.
* **IA en la Nube:** Ningún entorno puede correr Modelos de Lenguaje Locales (LLaMA). **Impacto:** Es obligatorio delegar la IA a APIs externas (Groq/Gemini).
* **Portabilidad Total:** El sistema debe levantar en la máquina de cualquier profesor con un solo comando. **Impacto:** Todo debe estar 100% contenerizado (Docker/Docker Compose).

## 2. Actualización de Datos (Scraping)
* **Ejecución Asíncrona (Cron):** El web scraping no puede bloquear la API web. Debe correr en segundo plano (Workers) idealmente en la madrugada.
* **Tolerancia a Fallos (Transacciones):** Si la página del supermercado se cae a la mitad del scraping, **NO se deben borrar** los precios antiguos de nuestra base de datos. Se exige usar transacciones SQL.

## 3. Mapas y Privacidad
* **Desplazamiento Realista:** Las distancias se calculan por calles (usando OpenTripPlanner/OSRM), no por línea recta.
* **Privacidad Estricta (RAM):** Las coordenadas GPS del usuario SOLO deben existir en la memoria volátil (RAM) mientras se calcula la ruta. **Impacto:** Queda prohibido guardar permanentemente la ubicación de un usuario en PostgreSQL (a menos que guarde una "Dirección Frecuente" explícitamente).

## 4. Seguridad y Resiliencia
* **Límite de Peticiones (Rate Limiting):** Obligatorio en Go para evitar que un ataque de fuerza bruta bote el servidor.
* **Reintentos en IA (Backoff):** Si la API de Groq o Gemini falla, el backend debe reintentar un par de veces antes de mostrarle un error feo al usuario.
* **Cifrado Total:** Todas las peticiones al servidor (especialmente porque enviamos ubicación) deben ir por HTTPS. Las contraseñas en DB deben estar hasheadas (Bcrypt/Argon2).
