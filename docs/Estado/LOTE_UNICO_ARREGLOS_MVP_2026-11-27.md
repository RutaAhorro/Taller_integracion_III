# Arreglos pequeños del MVP RutaAhorro (lote único)

Nov 27, 2026 · @Esban Vejar

## Cómo usar este documento

Los arreglos pequeños se resuelven todos de una vez, con un PR por responsable, antes del congelamiento de funcionalidades, el domingo 29 de noviembre temprano. La presentación es el miércoles 2 de diciembre a las 10:00.

- Un PR por persona: Renato, Daniela y Matus. Esban lleva la infraestructura y los datos.
- Cada punto es un objetivo, no un paso: quien lo toma (o su agente) decide cómo.
- Se marca al terminar. Si algo no cabe en el plazo, pasa al final de su lista y se avisa; no se deja a medias.
- Reglas de siempre: KISS, nombres y comentarios en español descriptivos, sin secretos, PR contra `develop` y una aprobación humana que no sea la del autor.
- No tocar archivos ajenos a la tarea (por ejemplo, no reformatear `router.tsx`).

## Frontend: Renato

Todo es pulido de la interfaz que ya existe, sin pantallas nuevas (rama `mvp/frontend/ajustes-layout-renato`).

**Categorías**

- [ ] Mostrar las categorías formateadas en todas las vistas (dashboard, filtros y ficha): quitar guiones y guiones bajos y poner mayúscula inicial ("frescos-y-lacteos" → "Frescos y lácteos", "Dulces\_mermeladas\_y\_manjar").
- [ ] La ficha de producto no debe mostrar el identificador crudo de la categoría.

**Ocultar lo que está fuera del MVP**

- [ ] Ficha de producto: ocultar el botón "Comparar precio".
- [ ] Perfil: ocultar "Cambiar Contraseña", la insignia "BÁSICO", "Cuota informada: consultas IA" y la pestaña "Preferencias de Ruta" (es un placeholder y en móvil se corta).
- [ ] Admin, Usuarios: ocultar la columna y el selector "Plan" (Básico/Premium).

**Textos y detalles**

- [ ] Listas: cambiar los textos técnicos ("Escribe al menos 3 bytes UTF-8 para buscar", "requiere que el servicio de listas esté habilitado en este entorno") por mensajes para el usuario.
- [ ] Corregir "1 envases" y el nombre de marca que falta en "· Cugat ·".
- [ ] El ícono de "Listas" es un reloj y sugiere historial: usar un ícono de lista. Revisar que ningún enlace apunte a `/history` (`MainLayout.tsx:46` debe ir a `/listas`).
- [ ] Móvil: el botón flotante del carrito tapa contenido, y el menú dice "Tu Ciudad" mientras la cabecera dice "Temuco" (unificar).
- [ ] Detalle de lista: mostrar el "Total estimado" más arriba.
- [ ] Dashboard: el banner "comparados en 0 supermercados" aparece mientras carga; ocultarlo hasta tener datos.
- [ ] Admin, Fuentes: mostrar "Sin datos" y no "Operativa" cuando una fuente tiene 0 productos (hoy Unimarc).
- [ ] Opcional: ignorar productos con precio 0 en las listas, con su test.

## Frontend: Daniela

Ajustes de la ruta y la recomendación de compra (#203), más el reintento del chat.

- [ ] Ruta: explicar la diferencia entre el total de la ruta y el total de la lista. Ejemplo real: ruta $1.830 y lista $1.730. El motor solo agrega una parada si ahorra al menos $500, y el producto que muestra es el de la tienda elegida. Texto sugerido: "Total de esta ruta: $X. Tu lista, con el precio más bajo de cada producto, suma $Y."
- [ ] Sin cobertura: usar `no_cubiertos` de la respuesta para decir qué productos no se encontraron en las tiendas cercanas, en vez del mensaje genérico.
- [ ] Validar el UUID de la lista antes de calcular la ruta y manejar el 403 con un mensaje claro.
- [ ] Corregir "1 envases" (singular y plural) en la recomendación.
- [ ] No reformatear `router.tsx` (deja un diff ruidoso).
- [ ] Chat (o quien lo tenga): tras un 503, la interfaz devuelve el texto al campo y el reintento lo duplica ("…$2.000Busco leche…"). Vaciar el campo o no repetir el texto, y mostrar un mensaje con botón "Reintentar".

## Backend Catálogo y datos: Matus

Lo más visible es la carga inicial de 4 s y las coordenadas de las sucursales; el resto es de baja prioridad.

- [ ] Coordenadas reales de las sucursales 2 y 4 (migración nueva en `infrastructure/db/`), para que la ruta no use puntos aproximados.
- [ ] Carga inicial sin filtro: la primera página tarda cerca de 4 s por el conteo sobre el cruce de precios. Evitar el `COUNT` en la primera página sin filtros o cachearlo (objetivo: menos de 2 s).
- [ ] Confirmar con Renato si el dashboard ofrece orden por precio con una búsqueda activa: con `q`, la relevancia (PR #208) va primero y el orden elegido solo desempata.
- [ ] Fuente sin datos: Unimarc figura "Operativa" con 0 productos. Decidir si se oculta o se marca "Sin datos" (en coordinación con Renato).
- [ ] Baja prioridad: productos repetidos en la misma cadena con precios distintos (pan Bauducco en Santa Isabel a $1.390 y a $1.490); quedarse con el más reciente.
- [ ] Baja prioridad: "azucar" sin tilde no encuentra "Azúcar" en el título porque el ILIKE distingue tildes; evaluar `unaccent`.

## IA conversacional

Estos cuatro puntos no bloquean la demo. Lo que sí la bloquea (los plazos del chat) está en la sección «QA en el cluster».

- [ ] Medir de nuevo la latencia del chat después del despliegue: antes del arreglo de la clave, un intento exitoso tardó 10 a 18 s. Si sigue lento, subir la CPU de la IA de 100m a 200m (la cuota deja +350m de margen).
- [ ] Groq falla a veces con "Tool choice is required, but model did not call a tool": reintentar una vez o pasar a un modo sin herramienta. Gemini es el proveedor principal, así que no es urgente.
- [ ] La precarga del catálogo al arrancar agota su plazo y deja la IA en modo degradado (inofensivo, pero llena los logs). Quitarla o acortar su plazo, ya que la IA busca por término en Catálogo.
- [ ] Confirmar si el paquete de la imagen de la IA es privado: si el pod no puede descargarla, agregar `imagePullSecrets` (`ghcr-secret`) al deployment, como en Motor.

## Infraestructura, secretos y datos: Esban

Lo urgente es mergear #204 y desplegar; la rotación de secretos se decide con el encargado después del ensayo.

- [x] Mergear #204 (retira `secret.yaml`; ya está aprobado).
- [x] Después del merge de #206, #207 y #208: aplicar solo el `deployment.yaml` de la IA (con `kubectl diff` antes) y reiniciar `catalogo` e `ia-conversacional`. No ejecutar `03_indices_optimizacion.sql` en el cluster: duplicaría los índices.
- [ ] Decidir con el encargado la rotación de secretos: `secret.yaml` con credenciales reales sigue en el historial (commits 9cbcbe3 y 9d3f281). Si alguna vez se aplicó en el cluster, rotar `JWT_SECRET` y `POSTGRES_PASSWORD`.
- [ ] Avisar al encargado de los 5 trabajos de scraping que quedaron en `en_progreso`.
- [ ] Confirmar que la clave de Gemini filtrada en el chat y en los logs quedó revocada.
- [ ] Compartir en privado `catalogo_nov26.sql.gz` (16 MB, solo tablas del scraper) con el equipo y cargarlo con `scripts/demo/cargar_catalogo.sh` (puede requerir filtrar las líneas `SELECT pg_catalog.setval`). El respaldo completo del cluster (`respaldo_cluster.sql`) queda solo en tu computador porque contiene usuarios.
- [ ] Quitar las menciones que queden de `VITE_IA_URL` (no se usa) fuera de `DESPLIEGUE.md`: `.env.example` y README del frontend.
- [ ] `scripts/recorrido_mvp.sh` firma el JWT localmente y no sirve contra el cluster (otro `JWT_SECRET`): documentarlo o dejarlo solo para local.
- [ ] Runbook corto para encender `scraper-go` bajo demanda (el CronJob `scraping-scheduler` aún no está aplicado y la cuota de 2000m no alcanza con todo encendido). Probarlo en el ensayo.

## Demo y ensayo

La defensa se prepara solo cuando la app esté terminada y desplegada.

- [ ] Lista de demostración con productos que estén en dos cadenas con EAN (A Cuenta, Santa Isabel, Cugat), para que la ruta muestre ahorro. Jumbo no tiene EAN, y Lider y Jumbo pueden no tener sucursales en el radio.
- [ ] Segunda pasada con Claude in Chrome (cuenta de usuario y cuenta de administrador) después del despliegue, para comprobar la relevancia ya corregida.
- [ ] Ensayo del despliegue con el encargado del cluster (sábado 28 por la noche o domingo 29 temprano, junto con el despliegue final).
- [ ] Congelamiento de funcionalidades: domingo 29 de noviembre, temprano. Después solo se corrigen errores críticos. Presentación: miércoles 2 de diciembre, 10:00.
- [ ] Con la app terminada: guiones y presentaciones de la defensa.
- [ ] Opcional, como "plus" con tiempo acotado: rediseño visual a nivel de tema, en un solo PR y solo después de mergear la página de ruta.

## Ya resuelto (no repetir)

Estos puntos aparecieron en las pruebas y ya tienen arreglo, hecho o en un PR abierto.

- Clave de Gemini con un espacio al final (la IA daba 503 y respuestas irrelevantes): corregida en el secret; el código ahora limpia las claves (PR #207).
- Relevancia de la búsqueda del chat y de Catálogo ("leche" mostraba yogures): PR #207 y #208, mergeados y desplegados el 27 de noviembre (la búsqueda q=leche pasó de \~1,4 s a \~0,4 s).
- Admin, Fuentes: carga bien en el cluster, sin el 404 que se temía.
- Lentitud de las búsquedas en Catálogo: índices `pg_trgm`, JIT apagado y base de datos en 500m; paridad con el repo en el PR #206.
- Lista, total estimado y ruta: funcionan en el cluster.
- `secret.yaml` con credenciales reales: retirado en el PR #204 (aprobado).

## Hallazgos del escaneo de develop (27 nov)

El escaneo (commit `e398cb4`, incluye #207 y #208) dejó cinco cosas por arreglar antes del congelamiento y varios riesgos que se documentan para la defensa.

- [ ] Identidad (prioridad alta): cerrar el registro y el login locales con una bandera `AUTH_LOCAL_HABILITADO` apagada en el cluster, y borrar `password_hash` cuando una cuenta se vincula con Google. Ajustar `scripts/recorrido_mvp.sh --registro-local`.
- [ ] Cluster: comprobar que las cuentas admin no tengan clave local (consulta de solo lectura, sin correos) y decidir la rotación de `JWT_SECRET`.
- [ ] Frontend: cabeceras básicas en `frontend/web/nginx.conf` (`X-Content-Type-Options`, `Referrer-Policy`, protección contra iframes), sin romper el login con Google.
- [ ] Infra: `06_propiedad_permisos.sql` pierde un permiso del Motor si se vuelve a ejecutar; moverlo al propio script.
- [ ] Documentación: actualizar `CLAUDE.md` y el `README.md` del repo; agregar a `scripts/recorrido_mvp.sh` los pasos 1 (Google) y 6 (panel admin).

**Riesgos conocidos que no se arreglan antes de la entrega (se explican en la defensa):**

- Las sesiones no se revocan: cerrar sesión, desactivar o degradar a un usuario no invalida su JWT (24 h) en Catálogo, Motor e IA.
- El webhook de GitHub no tiene firma y los botones de Discord no tienen autorización. El bot está en 0 réplicas en el cluster.
- Sin cabeceras de seguridad ni TLS en el Ingress del repo (el TLS del cluster lo gestiona el encargado).
- `ADMIN_EMAILS` vuelve a promover a admin en cada login: un admin degradado desde el panel recupera el rol al entrar de nuevo.
- El registro permite enumerar correos.
- El mensaje del usuario entra al prompt de explicación del chat (se renderiza como texto y la IA no tiene herramientas).
- El JWT llega al pod de PostgreSQL, y el scraper y el frontend corren como root.

## QA en el cluster (27 nov, tarde)

Segunda prueba con cuenta de usuario después de desplegar #204 a #209. La pasada como administrador sigue pendiente. Primero lo bloqueante (PR de los bloqueantes abiertos entre hoy y el sábado 28 al mediodía; el resto, el sábado 28 en la tarde; así queda el sábado por la noche para el despliegue final y la prueba, antes del congelamiento del domingo 29 temprano); lo demás es cosmético.

### Bloqueantes

- [ ] Matus, Motor de rutas: la ruta no debe caerse completa. Con 7 artículos (algunos solo en Jumbo o Lider) responde "No se encontró una forma de comprar la lista en las tiendas cercanas", sin ruta ni detalle. Objetivo: devolver la ruta con lo que sí se puede comprar y el resto en `no_cubiertos`, con su nombre. Revisar si las sucursales de esas cadenas no tienen coordenadas o quedan fuera del radio.
- [ ] Matus, Catálogo: sin filtro cada página tarda \~4 s (medido: 4,2 / 4,0 / 3,9 s con limit=100); con `q=leche` tarda \~0,4 s. Explica las ofertas de la portada (4,9 s) y las marcas blancas (2,8 s). Objetivo: primera página sin filtro bajo \~1 a 2 s.
- [ ] IA (rama `mvp/ia/chat-plazos`): el chat falla o tarda porque la explicación se queda sin plazo ("plazo agotado tras 2.85s") y se cae a un mensaje de error. Objetivo: responder siempre con productos; buscar con el término (`q`) y solo la primera página; si la explicación falla, una frase fija corta; precios con formato ($10.600, no "10600.0" ni "$2 000").

### Frontend: Renato (QA)

- [ ] Textos técnicos: "Consultas vía Gateway" (chat), "requiere que el servicio de listas esté habilitado en este entorno", "Escribe al menos 3 bytes UTF-8 para buscar".
- [ ] Plurales y unidades: "1 envases", "1 artículos"; no mostrar la unidad "Format" (zanahoria granel de Jumbo, portada y buscador de la lista).
- [ ] Listas: separador suelto ("Iansa · Santa Isabel · "), nombre del producto repetido dos veces y la nota técnica que ocupa media pantalla en móvil.
- [ ] Móvil: el botón flotante "Revisar carrito" tapa categorías, tarjetas y "Ver 23 productos más".
- [ ] El campo de búsqueda conserva el término anterior al abrir otras pantallas; el menú de usuario no se cierra con Escape.

### Frontend: Daniela (QA)

- [ ] Chat: el texto lista los productos y las tarjetas los repiten; que el texto no los duplique. Placeholder del input cortado en móvil; "distancia 0 km" y el slug "frescos-y-lacteos" visibles en las tarjetas.
- [ ] Ruta: reescribir "Sin comparación disponible: ninguna tienda sola cubre los mismos artículos" en palabras simples, y mostrar `no_cubiertos` cuando Matus devuelva la ruta parcial.
- [ ] Chat: tras un 503, que el reintento no duplique el texto y haya botón "Reintentar" (ya estaba en su lista).

### Después del congelamiento (no se toca hoy)

- Búsqueda con y sin tilde da conjuntos distintos ("azúcar" / "azucar"), y "azúcar 1 kg" no encuentra nada en el buscador de la lista.
- Relevancia fina: "arroz" mezcla arroz con leche; "té" trae té helado en polvo; "leche" parte con cajitas de 200 ml por el orden de precio dentro del nivel de relevancia.
- Duplicados: el mismo producto como tarjetas separadas por cadena (azúcar Ruf ×5) y SKU repetidos en la misma cadena (Santa Isabel).
- Categorías: más de 90, repetidas con variantes, sin tildes y mezcladas con nombres de tienda; íconos genéricos.
- Marcas blancas: el orden por precio pone bicarbonato, yogures y jugo en polvo primero.
- Unidades inconsistentes en los datos (azúcar de 1 kg figura como "Unidad") y "Mostrando coincidencias para…" aun sin coincidencias.
- Pendiente de aclarar: una búsqueda de "leche" registró 3,8 s en Network, probablemente caché fría.
