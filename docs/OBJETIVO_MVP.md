# Objetivo del MVP (fecha límite: último commit sábado 29-nov, 12:00)

## Historia que debe funcionar de punta a punta, con un solo `docker compose up`
1. Entro con mi cuenta de Google.
2. Busco un producto y veo precios por cadena, sin valores imposibles.
3. Lo agrego a una lista y veo los artículos y el total.
4. Pregunto al chat y veo productos recomendados.
5. Pido la ruta de mi lista: veo la cadena más barata y su total (orden de visita si hay datos).
6. Un administrador (correo en `ADMIN_EMAILS`) ve el panel de usuarios.

## Recortes declarados (y por qué)
- Login solo con Google: la identidad se delega a un proveedor externo; los microservicios propios siguen siendo el núcleo.
- Misiones, historial, planes y onboarding: ocultos, sin backend completo.
- Métricas de Catálogo y Control: fuera del MVP.

## Terminado significa (los 5 puntos)
Backend con tests, enrutado en Gateway y compose por defecto, pantalla que lo usa, paso en
`scripts/recorrido_mvp.sh` que pasa, y una línea en la documentación.
Lo que no llegue a pantalla antes del viernes 28 se reduce o se oculta; nunca queda visible a medias.

- Validación multi-tienda con datos reales: `bash scripts/demo/probar_ruta.sh` (consulta de solo lectura en `bd_supermercados`, crea lista de prueba y llama a `POST /api/v1/rutas/optimizar`; con dumps históricos, levantar `catalogo` con `CATALOGO_DISPONIBILIDAD_ANTIGUEDAD_MAXIMA=2160h`).

## Prioridad (si falta tiempo, se cae lo de abajo)
P0 Entorno reproducible y datos · P1 Catálogo sano y login con Google · P2 Listas con artículos y chat con tarjetas ·
P3 Ruta mínima (primero "cadena más barata", luego orden de visita) · P4 Ocultar secciones, docs y limpieza.

## Cómo se toma una tarea
- Anota tu nombre junto a la tarea en la lista del equipo y crea la rama `mvp/<área>/<tarea>-<nombre>`.
- PR chicos hacia `develop`. Nadie toca `docker-compose.yml`, `.env.example` ni `CLAUDE.md` sin avisar.
- Si una tarea lleva más de 2 horas sin avanzar o su dueño no responde, cualquiera puede tomarla.
- Siempre: sin secretos en el repo, sin commit directo a `develop`, sin migraciones ni despliegue sin autorización.
