# Frontend de RutaAhorro

Aplicación web en **React 19, TypeScript, Vite y Tailwind CSS 4**. Consulta el
catálogo mediante el Gateway, gestiona sesiones de Identidad y ofrece interfaces
de administración y listas. En producción, Nginx sirve los archivos estáticos;
Vite se utiliza para desarrollo y compilación.

## Antes de comenzar

- Leer el [README principal](../../README.md) para preparar `.env`, PostgreSQL y
  los servicios del backend.
- En el MVP el inicio de sesión de usuarios se realiza exclusivamente con Google
  (además de la navegación como invitado).
- En `/chat`, las respuestas con `productos_recomendados` muestran tarjetas con
  precio, cadena, acceso a `/producto/:id` y agregado directo al carrito.
- Usar Node.js 22 actualizado (el Dockerfile utiliza `node:22-alpine`) y npm si
  se desarrolla fuera de Docker. Instalar las versiones del lockfile con `npm ci`.
- No confundir una ruta disponible con un flujo integrado: en el MVP las secciones
  Misiones, Historial, Planes, Onboarding y Métricas de administración están
  ocultas o redirigidas a las vistas disponibles.

## Recomendación de compra por lista

En **Mis listas**, seleccionar una lista y pulsar **Calcular ruta** abre
`/route?lista=<UUID>`, protegida por sesión. La pantalla consulta
`POST /api/v1/rutas/optimizar` por Gateway, enviando `lista_id`, `medio_transporte`,
el origen opcional y la cookie HttpOnly. **Usar mi ubicación actual** solicita
permiso solo al pulsar; no guarda las coordenadas en el perfil. Se puede volver
al centro de Temuco. Si se rechaza el permiso o vence la solicitud, se informa
y se conserva el origen anterior. En móvil se requiere HTTPS; localhost sirve
para probar desde el mismo equipo.

Se pide una lectura sin caché (`maximumAge: 0`), pero el navegador puede usar
Wi-Fi o IP: `enableHighAccuracy` no garantiza GPS ni posición exacta. El marcador
O muestra el origen devuelto por Motor; el círculo representa la precisión
reportada. El encuadre incluye origen y paradas aunque el recorrido comience en
una calle cercana. **Centrar en origen** y **Ver recorrido completo** permiten
comprobarlo sin imprimir coordenadas personales en logs.

Si el navegador entrega una posición equivocada, **Elegir punto de partida en
el mapa** abre un selector independiente del resultado. Marcar, arrastrar o
seleccionar el centro solo prepara un borrador; **Confirmar punto de partida**
envía el origen y recalcula. Cancelar no cambia la ruta. No se consulta GPS ni
se persiste el punto manual. El resultado lo identifica como elegido en el mapa,
sin atribuirle precisión GPS. Se puede abrir incluso ante un error del cálculo.

La recomendación muestra total de los artículos incluidos, ahorro frente a una
sola tienda (o ausencia de comparación), distancia estimada con retorno al
origen, paradas ordenadas y compra por sucursal. Con ORS configurado, muestra
el mapa Leaflet/OpenStreetMap, geometría por calles y tiempo estimado del perfil,
con máximo dos tiendas. **No incluye combustible ni costo del tiempo.**
**Ver recorrido en el mapa** y los enlaces de cada parada llevan al mapa integrado.
También se conserva la ubicación externa en Google Maps sin API key.
Los artículos `no_cubiertos` se advierten como
excluidos del total; sus nombres se consultan como complemento en el detalle de
la lista y, si no se pueden recuperar, se conserva su identificador.

Cada parada presenta una compra compacta: cantidad, precio por envase y subtotal.
Las líneas de una misma **oferta y precio unitario** se muestran juntas, sumando
las cantidades y los subtotales recibidos, sin modificar los totales ni el orden
calculados por Motor. No se agrupan productos solo porque sus nombres se parezcan
ni compras de tiendas diferentes. Una parada con más de cuatro líneas agrupadas
permite desplegar las restantes; el total siempre incluye toda su compra.

Una sola parada puede tener ahorro cero; ahorro `null` significa que no hay
base de una tienda, no que el ahorro sea cero. La pantalla distingue sesión
vencida (401), lista ausente (404), restricciones o falta de cobertura (422),
precios no disponibles (503) y errores de conexión. Recalcular no modifica la
lista. El resultado es una instantánea: se debe recalcular si cambia la compra.

### Cómo interpretar los rechazos del cálculo (SUP-341)

La selección exige un UUID válido: un enlace inválido no consulta el motor ni
conserva ese parámetro al volver a las listas. Un 403 se presenta como falta de
permisos, no como desconexión, y no ofrece un reintento de red.

Cada restricción 422 tiene una orientación concreta: agregar o revisar artículos
pendientes, dividir una lista demasiado grande, revisar la ausencia de sucursales
físicas cercanas o la imposibilidad de cubrir la compra con el máximo de paradas.
Cuando el Motor identifica productos en el detalle público «productos sin
cobertura», se muestran sus nombres si el detalle de lista está disponible, o su
identificador si no. Esa lectura complementaria no retrasa ni oculta el rechazo.
No se imprime el detalle arbitrario del error.

**Sin ofertas vigentes no equivale necesariamente a precios vencidos.** Puede
deberse también a stock o cobertura; la pantalla presenta estas causas como
posibilidades, no como un diagnóstico confirmado. Si se obtiene una ruta parcial
con `no_cubiertos`, sus artículos siguen excluidos del total y se advierte antes
del resumen. No se calcula una alternativa ficticia para sustituir un rechazo.

El pulido visual se limita a la pantalla de rutas: resumen con importes destacados,
paradas numeradas, carga y errores diferenciados. Se conserva el formato actual
de `router.tsx`, sin otra reformateada global, y no se modifica el catálogo ni el
layout compartido del PR #209.

El alcance acordado conserva **vehículo particular, bicicleta y a pie**; sin
transporte público ni scooter. Cambiar el perfil o el origen recalcula y cancela
la petición anterior; no se reutilizan sus resultados. Motor usa una matriz
dirigida ORS antes de elegir el orden y devuelve las geometrías del mismo perfil.
Las duraciones son estimaciones sin tráfico ni tiempo de compra. El fondo OSM
es externo y requiere conexión: se muestra atribución y no se precargan tiles.
Se respeta la [política del servidor de tiles](https://operations.osmfoundation.org/policies/tiles/).
Un fallo de ORS no produce una polilínea ficticia; un fallo del fondo no borra
la compra confirmada. No se necesitan mapas ni grafos locales. Origen y sucursales
se envían a ORS por HTTPS desde Motor, con clave privada; el mapa atribuye ORS/HeiGIT
y OpenStreetMap. No se guardan coordenadas personales en logs.

### Probar las conexiones localmente

Se necesitan **Motor y frontend reconstruidos**, no toda la plataforma.
Guardar URL y clave ORS en `.env` raíz. Con PostgreSQL, Catálogo y Gateway activos:

```sh
docker compose up -d --build --no-deps motor_rutas frontend
```

Abrir `http://localhost:3000`, seleccionar una lista, calcular, probar los tres
perfiles, pedir ubicación y pulsar **Ver recorrido en el mapa**. Comprobar en
Network el cuerpo con modo/origen y la respuesta con `fuente_recorrido: "ors_osm"`,
tramos con geometría y `max_paradas: 2`. Probar también permiso denegado.
No copiar cookies ni coordenadas personales a capturas de diagnóstico.

El archivo Compose base inyecta `RUTAS_ORS_URL` y `RUTAS_ORS_API_KEY` solo en Motor.
No usar el complemento Compose anterior, que fue retirado.
No se requieren migraciones ni reconstrucción de Gateway, Catálogo o BD.
Ver configuración y cuotas en el [README de Motor](../../backend/motor_rutas/README.md#recorridos-viales-con-go--openrouteservice).

### Desarrollo y pruebas sin backend

`tests/fixtures/ruta-dos-paradas.json` reproduce el ejemplo de la sección 0 del
[contrato vigente](../../docs/Contratos/CONTRATO_MOTOR_RUTAS.md). Las pruebas
`tests/rutas.test.mjs` ejecutan el cliente, el hook y la presentación reales con
HTTP controlado, incluyendo una parada, ahorro nulo, cobertura parcial y errores.

```bash
node --test tests/rutas.test.mjs
npm run lint
npx tsc -b --noEmit
npm test
```

Los datos de ejemplo viven solo en pruebas: **no existe modo mock ni fallback
ficticio en la aplicación**. Para pasar a datos reales, abrir la pantalla normal
con una sesión y una lista guardada en el stack local. No se debe simular una
sesión en el router para mostrar ejemplos.

### Comprobación manual del recorrido

1. Entrar con Google, agregar productos al carrito, confirmar el guardado y
   abrir la lista de destino.
2. Pulsar **Calcular ruta** y comprobar en Red un POST con `{ "lista_id": "…" }`
   a `/api/v1/rutas/optimizar`; comparar importes, paradas y cobertura con su JSON.
3. Volver a la lista, cambiar cantidades y recalcular: no debe reutilizar el
   resultado anterior. Probar también recarga directa de la URL.
4. Comprobar móvil, teclado y enlaces Google Maps. Probar una sola parada,
   ausencia de base y artículos no cubiertos con respuestas controladas.
5. Probar lista sin pendientes, lista inexistente, cookie expirada y servicio
   caído: no deben aparecer paradas o ahorros de ejemplo.
6. Con red lenta, cambiar de lista, salir o cambiar de cuenta: la respuesta
   abandonada no debe reaparecer en la nueva vista.

`scripts/demo/probar_ruta.sh` es una validación complementaria del endpoint local:
crea una lista de prueba y muestra el JSON real; no contiene un fixture fijo.
No ejecutarlo contra entornos compartidos. Estas instrucciones y las pruebas
sin backend no acreditan una validación manual ni un despliegue.

## Verificación de correo: preparación visual (SUP-249, draft)

La ruta pública `/verificar-email` no requiere sesión y todavía no llama al
backend. Sin `token` (o con uno vacío) muestra un enlace incompleto; con token
muestra que la verificación aún no está disponible. No lo imprime, guarda ni
envía, y ningún parámetro de URL puede activar una confirmación exitosa.

`EstadoVerificacion`, en `src/features/autenticacion/VerificarEmail.tsx`, prepara
la presentación de carga, confirmación, enlace rechazado, error de conexión y
aviso de correo enviado. Los estados de confirmación y envío no están
conectados a la navegación ni al registro: sólo deben mostrarse cuando Identidad
confirme esos resultados. El registro actual no cambia.

Pendiente de SUP-231: acordar cuerpo y respuestas de la API, conectar la
verificación con cancelación y descarte de respuestas obsoletas, y mostrar el
aviso post-registro únicamente cuando el servidor confirme el envío. El PR
permanece en draft; esta pantalla no acredita verificación ni entrega de correo.

Comprobación visual local (sin backend, usando un token ficticio):

- Abrir `/verificar-email`: aviso de enlace incompleto y acceso al login.
- Abrir `/verificar-email?token=ejemplo-local`: aviso de función no disponible,
  sin peticiones de verificación ni mensaje de éxito.
- Añadir `&estado=confirmado`: no debe cambiar a un resultado exitoso.
- Probar móvil, escritorio y navegación con teclado. El token no debe aparecer
  en el contenido de la página ni copiarse a los enlaces de navegación.

## Categorías desde Catálogo

El lateral consulta `GET ${VITE_API_URL}/productos/categorias` al montarse y
al reintentar un error. Muestra una lista plana de valores de la BD, sin
departamentos fijos ni respaldo ficticio. Puede incluir valores genéricos o
semillas existentes y categorías sin stock: no normaliza ni corrige esos datos.
Destacados conserva sus tres acciones estáticas; no son categorías.

Seleccionar una categoría envía `categoria_exacta`; pulsarla de nuevo o limpiar
filtros la desactiva. El prefijo interno `categoria:` evita colisiones con
Destacados y no viaja a la API. La lista no cambia al paginar o filtrar.
Los supermercados se sincronizan por separado con las facetas del listado (SUP-375).

## Supermercados sincronizados con el catálogo (SUP-375)

`MainLayout` comparte una sola instancia de `useCatalog` con el lateral y el
listado. Cada consulta recibe productos, totales y `supermercados` del mismo
`GET /productos`: no se deducen las cadenas de la primera página ni se descarga
todo el catálogo.

- Búsqueda, categoría, ofertas, Marcas blancas y cadena seleccionada reinician la
  página. Catálogo aplica los filtros antes de paginar.
- La selección de cadena filtra productos, pero las facetas conservan las otras
  cadenas válidas. Sólo una respuesta exitosa puede limpiar una selección que
  ya no está disponible; una carga o un fallo no la desactivan.
- Cargar más productos no cambia las alternativas del lateral. Un fallo de
  paginación conserva lo descargado y permite reintentar esa página.
- Los cambios de filtros cancelan peticiones anteriores y descartan sus
  respuestas. No se aplica un segundo filtro local de búsqueda.
- Una búsqueda no vacía de menos de 3 bytes UTF-8 muestra un aviso y no consulta
  productos ni facetas, conforme al mínimo actual de Catálogo. Por ejemplo,
  `te` espera más texto; `té` ya tiene 3 bytes. El servidor conserva la validación
  y el saneamiento definitivos.
- Carga, error con reintento y facetas vacías tienen mensajes explícitos. No hay
  cadenas de ejemplo como respaldo.

### Comprobación manual de supermercados

1. En escritorio y móvil, buscar un producto y comparar el lateral con
   `supermercados` de la respuesta de `/api/v1/productos`. Paginar no debe cambiar
   esas alternativas.
2. Seleccionar una cadena y comprobar que viaja en `supermercado`, sin ocultar
   las otras alternativas válidas. Probar categoría, ofertas y Marcas blancas.
3. Con red lenta, cambiar rápidamente la búsqueda: sólo debe quedar el último
   resultado. Repetir mientras carga una página adicional.
4. Probar una búsqueda sin coincidencias, limpiar filtros y una selección que
   deje de ser válida: no deben quedar cadenas antiguas ni peticiones en bucle.
5. Bloquear `/productos` sin bloquear `/productos/categorias`: ambos estados de
   error deben ser explícitos. Desbloquear y reintentar con los filtros actuales.
6. Escribir `te`, luego `té`: el primero muestra el aviso, el segundo consulta
   la API. Estos pasos son una guía; no acreditan una prueba manual realizada.

### Comprobación manual

Desde la raíz, con el entorno local configurado:

```bash
docker compose up -d --build catalogo frontend
curl -i http://localhost:8082/api/v1/productos/categorias
```

1. En `/dashboard`, comparar las categorías del lateral con la respuesta HTTP,
   tanto en escritorio como en móvil. La jerarquía desplegable anterior ya no existe.
2. Seleccionar categoría y verificar `categoria_exacta` en Red; los resultados
   deben tener esa categoría, no sólo su nombre en el título. Probar paginación,
   cambios rápidos, supermercado, deselección, limpieza y Destacados.
3. Bloquear sólo `*/productos/categorias`, recargar y comprobar error/reintento.
   Para vacío, usar una respuesta controlada `{"categorias":[]}`, sin borrar datos.

Si aparece la jerarquía antigua, reconstruir frontend y recargar sin caché.
Si un nombre coincide con el anterior pero viene en la respuesta, es un dato
de la BD, no una categoría fija del frontend. No requiere migraciones ni cambios
al `.env`. Estas instrucciones no acreditan pruebas manuales ni despliegue.

## Desarrollo con Docker y recarga automática

El Compose principal construye el frontend con el target `production` por
defecto. Para usar Vite sin editarlo, crear un override local, por ejemplo
`/tmp/rutaahorro-frontend-dev.yml`, con:

```yaml
services:
  frontend:
    build:
      target: development
```

Desde la raíz del repositorio, con el backend y los secretos ya configurados:

```bash
docker compose -f docker-compose.yml -f /tmp/rutaahorro-frontend-dev.yml up -d --build frontend
```

Abrir <http://localhost:3000>. Vite resuelve `/api` hacia `http://api:8080`, el
alias interno del Gateway. Para volver a Nginx, reconstruir el frontend con solo
`docker-compose.yml`. Los cambios de código no requieren recompilar en modo Vite;
los estáticos de producción sí necesitan una nueva compilación.

## Desarrollo sin Docker para el frontend

El backend puede seguir ejecutándose en Compose. Desde `frontend/web`:

```bash
npm ci
```

Crear `.env.local` (no versionarlo) con la URL completa del Gateway:

```dotenv
VITE_API_URL=http://localhost:8082/api/v1
VITE_GOOGLE_CLIENT_ID=
```

```bash
npm run dev
```

Fuera de Docker, el nombre `api` no resuelve normalmente. La URL absoluta anterior
evita usar el proxy interno de Vite sin modificar su configuración. Si `3000`
está ocupado, detener el frontend de Compose; si se utiliza otro puerto, añadir
su origen exacto a la configuración CORS de los servicios correspondientes.
No mezclar `localhost` y `127.0.0.1` al probar cookies entre orígenes.

## Variables de entorno

| Variable | Uso |
| --- | --- |
| `VITE_API_URL` | Base de la API **incluyendo `/api/v1`**; `/api/v1` dentro de Compose/Ingress o URL absoluta del Gateway para Vite local |
| `VITE_GOOGLE_CLIENT_ID` | ID público del cliente OAuth; no es una clave privada |

`VITE_IA_URL` se retiró (SUP-391): el chat usa `VITE_API_URL` y el Gateway; no configurar acceso directo a IA.

En Vite se leen al arrancar/compilar. La imagen de producción genera placeholders
y `entrypoint.sh` los sustituye por las variables del contenedor antes de arrancar
Nginx. No se debe asumir que cambiar `.env` modifica un `dist` ya compilado fuera
de ese mecanismo. **Toda variable `VITE_*` es visible en el navegador**: nunca
incluir claves Gemini/Groq, contraseñas ni tokens de servicio.

## Rutas y estado funcional

La fuente de verdad es `src/app/router.tsx`, con constantes en
`src/app/routes.ts`.

| Ruta | Acceso y estado |
| --- | --- |
| `/` | Página de inicio pública |
| `/dashboard`, `/producto/:id` | Catálogo y detalle; consumen la API |
| `/login` | Login local y Google según configuración |
| `/onboarding`, `/profile`, `/planes` | Requieren sesión; no implican preferencias persistidas o pagos integrados |
| `/listas` | Requiere sesión y Motor disponible; CRUD de listas, búsqueda de productos, agregado, cantidades, marcado como comprado y eliminación de artículos |
| `/admin`, `/admin/metricas` | Solo rol admin; usuarios, fuentes y métricas según endpoints disponibles |
| `/chat` | Chat básico mediante `POST /api/v1/chat` del Gateway y cookie de sesión. Requiere IA habilitada en el entorno; receta → lista sigue pendiente |
| `/colaborador` | Colaborador/admin; aviso de misiones no disponibles |
| `/route`, `/history` | Requieren sesión; avisos de cálculo e historial aún no integrados |
| `/terminos` | Página pública de términos |

Las vistas pendientes usan `FeatureUnavailable`. El chat muestra únicamente la
respuesta del servicio o un error explícito, sin simular etapas del procesamiento
ni confirmaciones de guardado.
La portada es informativa: sus textos, productos y cifras ilustrativas provienen
de `src/features/landing/landing.json`, sin consultas a la API ni estadísticas en tiempo real.
El catálogo funcional no usa esos ejemplos. Sus cadenas se deducen únicamente de los productos recibidos: hasta SUP-371 no
representan la cobertura completa ni las facetas de una búsqueda. El filtro
«Marcas blancas» está deshabilitado hasta disponer de clasificación en el backend.
El carrito temporal del catálogo **no se sincroniza con las listas**.

El chat envía `{"mensaje":"..."}` a `${VITE_API_URL}/chat` con
`credentials: 'include'` y lee el campo `respuesta` del contrato actual de IA.
No envía tokens de servicio ni claves del proveedor desde el navegador. Los
campos de productos recibidos no se convierten automáticamente en artículos:
los IDs y las cantidades de envases requieren el
[contrato de artículos](../../docs/Contratos/CONTRATO_ARTICULOS_LISTA.md).
El backend de recetas (SUP-325) y de artículos/lotes (SUP-241/SUP-318) existe;
la confirmación editable de recetas (SUP-329) y su conexión desde el chat siguen
pendientes. La conexión básica no acredita que receta → lista esté terminada.

Antes de modificar o retirar `landing.json`, consultar el
[contrato de la landing informativa](../../docs/Contratos/CONTRATO_LANDING_INFORMATIVA.md):
documenta el motivo de su conservación y los límites de la excepción propuesta
al barrido.

## Organización del código

Carpetas por tema: cada feature agrupa sus pantallas, componentes, hooks, cliente
HTTP y tipos. Los nombres de archivo existentes se conservan; su renombrado al
español se trata aparte (SUP-393).

```text
src/app/                 router.tsx (rutas y proveedores) y routes.ts (constantes de navegación)
src/features/
  autenticacion/         AuthContext, useAuth, useRole, roles.ts, tipos.ts (Usuario, plan),
                         Login, Profile, AuthPanel y ProtectedRoute
  catalogo/              Dashboard, ProductDetail, useCatalog, products-api.ts, carrito
                         (CartContext, CartSidebar), tarjetas, formatters y tipos.ts
  chat/                  Chatbot, useChatbot y TypingIndicator
  landing/               Home, componentes de portada y landing.json (contenido editorial)
  listas/                ShoppingLists, ArticulosLista, clientes y hooks de detalle/búsqueda
  admin/                 Panel, clientes API y hooks administrativos
src/layouts/             MainLayout, TopNav, Sidebar (filtros) y BottomNav
src/components/          ErrorBoundary, FeatureUnavailable, Footer, ToastContext y ui/ (primitivas)
src/lib/                 utils.ts (cn) y logger.ts
src/pages/               Páginas sin feature propia: avisos de integraciones pendientes y términos
tests/                   Pruebas críticas de sesión, listas, administración y errores del chat
```

### Datos: API, contenido estático y estado local

| Tipo | Dónde vive | Notas |
| --- | --- | --- |
| Datos de API | `features/*/*-api.ts`, `features/autenticacion/AuthContext.tsx`, `features/catalogo/useCatalog.ts` | Siempre con `credentials: 'include'`; los errores se muestran, nunca se sustituyen por ejemplos |
| Contenido estático | `features/landing/landing.json` | Textos y cifras ilustrativas de la portada; no son datos de API (ver contrato de la landing) |
| Estado local | `CartContext` (carrito temporal), `ToastContext`, `auth_user` en `localStorage` | No se sincroniza con listas ni acredita sesión |

No hay mocks de datos: no existe ninguna carpeta de mocks ni se usan ejemplos
como respaldo de una respuesta fallida.

El alias `@/` apunta a `src/`. Se usan React Context y hooks; no Redux/Zustand.
Comprobar imports del router y sus componentes antes de declarar un archivo
obsoleto: el nombre PascalCase o kebab-case no determina por sí solo si se usa.

## Sesión y permisos

- La sesión del backend utiliza la cookie HttpOnly `jwt`; los clientes protegidos
  envían `credentials: 'include'`. No leer el JWT desde JavaScript ni introducir
  tokens internos de scraping en el navegador.
- `AuthContext` conserva una representación del usuario en `localStorage` para
  la UI. Esto no acredita por sí solo una sesión válida en el servidor.
- Cerrar sesión espera `POST /auth/logout` con la cookie, mediante `VITE_API_URL`
  (que incluye `/api/v1`). El servidor responde `204` sin JSON y expira la cookie.
  Siempre se limpia `auth_user`; si no se confirma el cierre remoto, se muestra
  una advertencia porque la cookie podría seguir activa. No se revocan JWT copiados.
- `ProtectedRoute` controla navegación; la autorización efectiva corresponde
  al backend. Roles: `registrado`, `colaborador`, `admin`; el plan (`basico`/`premium`,
  `PlanUsuario` en `features/autenticacion/tipos.ts`) es un concepto distinto del
  rol. No se aceptan alias heredados (`administrador`, `usuario`, `plus`): la base
  restringe ambos campos con CHECK.
- Sin base API, administración muestra un error de configuración y no utiliza
  datos de ejemplo. Los fallos parciales de métricas se muestran como «sin datos».
- No existe una cuenta admin precreada documentada. Solicitar su asignación a
  un operador autorizado, sin cambiar validaciones ni falsificar el usuario local.

## Comprobaciones

### Listas y artículos (SUP-252)

La UI utiliza el contrato vigente de `producto_id` y no accede a PostgreSQL.
La búsqueda se envía a Catálogo por Gateway, con al menos 3 bytes UTF-8
(`té` ya cumple); los resultados se paginan en el servidor y se descartan al
editar la búsqueda o cambiar de lista.

El agregado usa el endpoint de lote (SUP-318), también para un único producto.
Conserva la misma `Idempotency-Key` al reintentar un envío no confirmado y no
permite escrituras simultáneas. Las cantidades son enteros de 1 a 99 envases.
Los productos repetidos suman cantidades según el contrato; una lista admite
hasta 100 productos distintos. El servidor sigue siendo la autoridad de estos
límites y del dueño de la lista.

El detalle muestra los datos y la disponibilidad que devuelve Motor. Un precio
ausente no se sustituye por cero. `sin_stock`, `desconocido` y `no_encontrado`
se muestran con advertencias, sin borrar artículos automáticamente. PUT de
cantidad/comprado no vuelve a consultar Catálogo; «Actualizar detalle y
disponibilidad» permite consultar de nuevo. Si falla una escritura, se
conservan el artículo y el borrador, sin anunciar éxito. El enlace de inicio
de sesión aparece solo ante 401, también en errores del detalle.

#### Preparación y validación real

Motor debe estar habilitado con sus credenciales, permisos y consulta a
Catálogo: seguir el [README de Motor](../../backend/motor_rutas/README.md),
incluida su revisión de cuota antes del despliegue. Tener la UI compilada
no prueba que el servicio esté disponible en el clúster.

En un entorno de prueba autorizado, con dos cuentas propias y datos reales:

1. Iniciar sesión, entrar en `/listas` y crear una lista.
2. Seleccionarla, buscar un producto, elegirlo y agregar 2 envases. Comprobar
   nombre, supermercado y la advertencia de disponibilidad que corresponda.
3. Cambiar la cantidad a 3 y marcar como comprado. Recargar el detalle y la
   página: ambas modificaciones deben conservarse.
4. Cerrar sesión, volver a entrar y comprobar la misma lista y sus artículos.
5. Agregar otro producto y quitarlo; recargar y comprobar que no reaparece.
6. Con la segunda cuenta, comprobar que la lista de la primera no aparece.
   Consultar su UUID por la API debe responder 404, sin revelar su existencia.
7. Con DevTools, ralentizar las escrituras: los controles se bloquean y no
   duplican peticiones. Simular una pérdida de respuesta de agregado y reintentar
   el mismo producto/cantidad: comprobar la misma clave de operación y que no
   vuelve a sumar. Un fallo conserva la selección; no se sustituye por éxito.
8. Probar búsqueda vacía y fallida, cantidades fuera de 1–99, un producto sin
   stock y un error 401. Solo el 401 debe ofrecer iniciar sesión.

Las pruebas automatizadas de frontend usan HTTP controlado. Este procedimiento
es una checklist pendiente, no evidencia de ejecución contra la BD del clúster.
Carrito → lista, receta → lista y optimización quedan fuera de SUP-252.

### Chat básico por Gateway (SUP-253)

#### Preparar el entorno local

Usar el entorno integrado de SUP-278. En `.env` de la raíz, conservar los
secretos existentes y configurar:

```dotenv
IA_URL=http://ia-conversacional:8002
```

Identidad e IA deben recibir el mismo `JWT_SECRET`. IA necesita las claves de
proveedor y Catálogo disponible para una respuesta real. No generar otra clave
JWT solo para IA ni copiar secretos a variables `VITE_*`. Desde la raíz:

```bash
docker compose up -d --build ia_conversacional gateway frontend
docker compose ps ia_conversacional gateway frontend
curl -i -X POST http://localhost:8082/api/v1/chat \
  -H 'Content-Type: application/json' \
  -d '{"mensaje":"Busco leche"}'
```

Sin cookie, esa petición debe responder **401**: comprueba ruta y autenticación,
no una recomendación del proveedor. Un **404** apunta a la ruta/configuración;
un **502/504**, a disponibilidad o tiempo de espera del upstream. Consultar los
logs locales de Gateway e IA sin publicar secretos:

```bash
docker compose logs --tail=50 gateway ia_conversacional
```

El frontend de Compose usa `VITE_API_URL=/api/v1`. Si se usa Vite fuera de
Docker, mantener `VITE_API_URL=http://localhost:8082/api/v1` en `.env.local`,
reiniciar Vite y usar un origen permitido por CORS. No usar `VITE_IA_URL`.

#### Recorrido en navegador

Abrir `http://localhost:3000/chat` y DevTools → Red:

1. **Invitado:** intentar enviar una consulta. Aparece el aviso de inicio de
   sesión y no se hace un POST de chat.
2. **Con sesión:** entrar con una cuenta registrada, volver a `/chat` y enviar
   «Busco leche por menos de $2.000». Debe salir un único POST al Gateway,
   `/api/v1/chat`, con `mensaje` y cookie. Nunca a `/chat/gemini` ni al puerto
   8002 desde el navegador.
3. **Respuesta:** si hay un 200 con `respuesta`, comparar ese texto con el
   mensaje mostrado. Sin coincidencias o sin catálogo, mostrar el texto que
   realmente devuelve IA; no inventar productos, precios ni un éxito genérico.
4. **Espera:** con red lenta, no se pueden enviar consultas simultáneas por
   botón, Enter o sugerencias. El indicador solo dice que se está consultando;
   no afirma haber completado pasos internos del servidor.
5. **Error de red:** bloquear solo `*/api/v1/chat` en DevTools y enviar.
   Debe aparecer un error, desaparecer la espera y conservarse el texto para
   reintentar. Retirar el bloqueo y enviar otra vez.
6. **Sesión vencida:** con `auth_user` todavía en Local Storage, eliminar solo
   la cookie `jwt` y enviar. El 401 debe mostrar el aviso de sesión y permitir
   ir al login; no debe presentarse como respuesta válida del asistente.
7. **Vaciar durante la espera:** pulsar «Vaciar chat». Se conserva solo la
   bienvenida y una respuesta tardía no repuebla la conversación. Se puede
   iniciar otra consulta. Cancelar la espera del navegador no garantiza cancelar
   trabajo que el proveedor ya haya recibido.
8. **Límites de alcance:** no aparece una confirmación ficticia de ingredientes
   agregados al carrito/lista ni se genera una ruta.

Las pruebas locales automatizadas usan respuestas HTTP controladas y no consumen
cuota del proveedor. Registrar navegador, entorno, resultado y cualquier bloqueo
de Catálogo/IA al hacer la prueba real. Esta guía no acredita por sí sola que
el chat esté desplegado o validado en el clúster.

### Inicio de sesión

Tras iniciar sesión por contraseña o Google, `/login` redirige a `/admin` si el
usuario devuelto por Identidad tiene rol `admin`; las demás cuentas van al
catálogo. No se asignan permisos por correo. Para probar una promoción de rol,
cerrar sesión e iniciar nuevamente, y comprobar `rol` en la respuesta de login.
La redirección no sustituye la autorización del backend.

Para comprobar que una sesión local antigua no bloquea el formulario:

1. Iniciar sesión con una cuenta registrada y permanecer en `/dashboard`.
2. En DevTools, eliminar solo la cookie `jwt`, conservando `auth_user` en Local
   Storage. No pasar por una página protegida que pueda limpiar la sesión.
3. Entrar directamente a `/login`: debe aparecer el formulario, sin redirección.
4. Repetir el acceso con contraseña y, si está configurado, con Google. Solo
   tras una autenticación exitosa debe navegar según el rol recién recibido.
   Un rechazo debe mantener el formulario y mostrar el error.

Las pruebas automatizadas cubren el usuario local antiguo, la identidad devuelta
por ambos métodos y el destino por rol; no sustituyen esta comprobación de
cookies en el navegador.

### Panel de usuarios y fuentes (SUP-255)

Probar con el frontend actualizado y una cuenta administradora en la misma BD
que utiliza la API. Usar otra cuenta de prueba para los cambios; no desactivar
ni degradar al administrador con el que se realiza la comprobación.

- En `/admin?section=users`, buscar la cuenta de prueba, cambiar su rol y su
  estado activo, y recargar para verificar persistencia. Restaurar los valores
  originales. El rol propio y la desactivación propia deben estar bloqueados.
- En `/admin?section=sources`, deshabilitar una fuente local de prueba, recargar,
  verificar el estado y habilitarla nuevamente. Durante el guardado se bloquean
  los cambios simultáneos y se aplica la respuesta de Catálogo.
- Con DevTools, ralentizar la red para ver la carga; bloquear la solicitud de
  usuarios o fuentes para verificar el error y «Reintentar» al retirar el bloqueo.
  Un fallo no debe presentarse como un listado vacío exitoso.
- Con una cuenta registrada, abrir `/admin`: no debe mostrar el panel. Los
  endpoints administrativos deben rechazar peticiones sin permisos (`401/403`).

La redirección de admin ya fue confirmada manualmente por la autora. Las pruebas
automatizadas cubren los clientes, los rechazos y las carreras de carga/guardado
de fuentes; no reemplazan el recorrido anterior contra la BD local.

Desde `frontend/web`:

```bash
npm run lint
npx tsc -b --noEmit
npm test
npm run build
```

El script actual de build es `tsc -b || true && vite build`: **puede generar
estáticos aunque existan errores de TypeScript**. Ejecutar la comprobación de
tipos por separado y reportar sus fallos; no desactivar más controles para
obtener una salida exitosa. La prueba de listas no constituye una suite E2E.

`npm run preview` sirve el build de Vite para inspección local; no reemplaza a
Nginx ni añade automáticamente el proxy `/api` de desarrollo. La base API debe
estar configurada para ese entorno y su origen permitido en CORS.

Para revisar una entrega, comprobar búsqueda con/sin resultados, detalle,
scroll y legibilidad en escritorio/móvil. Registrar versión, esperado y observado;
no inventar resultados si no hay un entorno disponible.

### Comprobar el cierre de sesión en local (SUP-361)

Con frontend, Gateway e Identidad ejecutándose y la base API configurada:

1. Iniciar sesión y abrir las herramientas del navegador, pestaña Red.
2. Cerrar sesión desde el menú. Comprobar `POST /api/v1/auth/logout` con respuesta
   `204`, sin cuerpo; la cookie `jwt` debe desaparecer y también `auth_user` en
   el almacenamiento local. Recargar y abrir `/listas`: debe solicitar acceso.
3. Con una cuenta admin de prueba, repetir desde el panel administrativo.
4. Volver a iniciar sesión y bloquear solo `*/auth/logout` desde las herramientas
   de red. Al salir, debe limpiarse la cuenta local, mostrarse la advertencia y
   permitir la navegación. No esperar que desaparezca la cookie: el servidor
   no pudo recibir esa petición.
5. Retirar el bloqueo y volver a pulsar Cerrar Sesión, disponible en el menú del
   catálogo incluso sin usuario local, para expirar la cookie que pudo quedar.
6. Comprobar también «Continuar como invitado»: espera el cierre y abre el catálogo.

### Comprobar filtros móviles (SUP-372)

En Chrome o Edge, abrir DevTools (`F12`) y activar la barra de dispositivos
(`Ctrl+Shift+M`). Elegir «Responsive» y probar anchos de 360 y 390 px:

1. Entrar a `/dashboard`: el botón «Abrir filtros» debe estar visible.
2. Abrir el panel, elegir una categoría y volver a abrirlo: la selección se conserva.
3. Elegir un supermercado disponible y comprobar que se conserva al reabrir.
4. Pulsar «Limpiar filtros»: se restablecen búsqueda, categoría y supermercado.
5. Cerrar con la X, pulsando fuera y con Escape. La búsqueda y navegación siguen accesibles.
6. Con teclado, abrir el panel y recorrerlo con Tab/Shift+Tab: el foco permanece
   dentro. Al cerrar, vuelve al botón de apertura; el panel cerrado no recibe foco.
7. Repetir en escritorio: el panel empieza cerrado y el foco no salta a «Cerrar
   filtros» al cargar. Abrirlo con el botón y comprobar el mismo recorrido de
   teclado. Verificar que no aparece desplazamiento horizontal.

La emulación permite comprobar tamaño e interacción, pero no sustituye una
prueba en un teléfono real. Registrar el navegador, ancho y resultado observado.

## Referencias

- [Arquitectura y ejecución del proyecto](../../README.md).
- [Índice documental y advertencias de vigencia](../../docs/README.md).
- [Gateway](../../backend/gateway/README.md).
- [Identidad](../../backend/identidad/README.md).
- [Contrato de listas](../../backend/motor_rutas/README.md).
