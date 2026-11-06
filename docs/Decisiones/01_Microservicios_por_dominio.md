# D01 — Microservicios por dominio en vez de monolito

[← Índice de decisiones](README.md)

**Contexto:** `backend/api` concentraba en un solo servicio la autenticación, el catálogo y la coordinación del scraping. Un fallo o un despliegue en cualquiera de esas partes afectaba a todas, y varios integrantes modificaban el mismo código a la vez.

**Decisión:** dividir el backend por dominio: Identidad, Catálogo, Control de scraping y Scraper, detrás de un Gateway. El monolito se retiró en SUP-273.

**Motivo:**
- Cada servicio se despliega y escala por separado (HPA en Kubernetes).
- Cada dominio tiene un responsable claro dentro del equipo.
- Una falla del scraping no tumba el login ni el catálogo.
- Cumple el objetivo académico del curso: integrar varios servicios.

**Alternativa descartada:** un monolito modular, con paquetes separados dentro de un solo binario. Es más simple de operar, pero no aísla las fallas ni los despliegues, y no cumple con el objetivo de integración del curso.

**Consecuencias:**
- Hay más contenedores, más memoria total y más configuración (tokens, redes).
- La comunicación entre servicios pasa a ser HTTP, que puede fallar, así que hace falta idempotencia y reintentos (ver [D06](06_Redis_Streams_con_outbox.md) y [D07](07_Ingesta_idempotente.md)).
