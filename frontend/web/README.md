# Frontend de RutaAhorro

Aplicación web en **React 19, TypeScript, Vite y Tailwind CSS 4**. Consulta el
catálogo mediante el Gateway, gestiona sesiones de Identidad y ofrece interfaces
de administración y listas. En producción, Nginx sirve los archivos estáticos;
Vite se utiliza para desarrollo y compilación.

## Antes de comenzar

- Leer el [README principal](../../README.md) para preparar `.env`, PostgreSQL y
  los servicios del backend.
- Usar Node.js 22 actualizado (el Dockerfile utiliza `node:22-alpine`) y npm si
  se desarrolla fuera de Docker. Instalar las versiones del lockfile con `npm ci`.
- No confundir una ruta disponible con un flujo integrado: misiones, rutas,
  historial, planes y configuración inicial muestran avisos de no disponibilidad.

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
| `VITE_IA_URL` | URL directa usada actualmente por el hook del chatbot; integración pendiente, no necesaria para el catálogo |

En Vite se leen al arrancar/compilar. La imagen de producción genera placeholders
y `entrypoint.sh` los sustituye por las variables del contenedor antes de arrancar
Nginx. No se debe asumir que cambiar `.env` modifica un `dist` ya compilado fuera
de ese mecanismo. **Toda variable `VITE_*` es visible en el navegador**: nunca
incluir claves Gemini/Groq, contraseñas ni tokens de servicio.

## Rutas y estado funcional

La fuente de verdad es `src/app/router.tsx`, con constantes en
`src/core/routes.ts`.

| Ruta | Acceso y estado |
| --- | --- |
| `/` | Página de inicio pública |
| `/dashboard`, `/producto/:id` | Catálogo y detalle; consumen la API |
| `/login` | Login local y Google según configuración |
| `/onboarding`, `/profile`, `/planes` | Requieren sesión; no implican preferencias persistidas o pagos integrados |
| `/listas` | Requiere sesión; CRUD de listas si el motor está disponible. Artículos en solo lectura |
| `/admin`, `/admin/metricas` | Solo rol admin; usuarios, fuentes y métricas según endpoints disponibles |
| `/chat` | Vista previa con aviso visible de no disponibilidad hasta SUP-253; el cliente conserva `/chat/gemini`, un endpoint retirado. No es un chat integrado |
| `/colaborador` | Colaborador/admin; aviso de misiones no disponibles |
| `/route`, `/history` | Requieren sesión; avisos de cálculo e historial aún no integrados |
| `/terminos` | Página pública de términos |

Las vistas pendientes usan `FeatureUnavailable`, excepto el chat, que conserva su interfaz con un aviso visible de integración pendiente; no generan resultados simulados.
La portada es informativa: sus textos, productos y cifras ilustrativas provienen
de `src/data/landing.json`, sin consultas a la API ni estadísticas en tiempo real.
El catálogo funcional no usa esos ejemplos. Sus cadenas se deducen únicamente de los productos recibidos: hasta SUP-371 no
representan la cobertura completa ni las facetas de una búsqueda. El filtro
«Marcas blancas» está deshabilitado hasta disponer de clasificación en el backend.
El carrito temporal del catálogo **no se sincroniza con las listas**.

Antes de modificar o retirar `landing.json`, consultar el
[contrato de la landing informativa](../../docs/Contratos/CONTRATO_LANDING_INFORMATIVA.md):
documenta el motivo de su conservación y los límites de la excepción propuesta
al barrido.

## Organización del código

```text
src/app/router.tsx       Router, proveedores y protección de rutas
src/core/routes.ts      Constantes de navegación
src/contexts/           Sesión, carrito temporal y notificaciones
src/features/admin/     Panel, clientes API y hooks administrativos
src/features/listas/    Cliente HTTP de listas
src/hooks/              Catálogo, autenticación y chat
src/lib/products-api.ts Cliente de catálogo y adaptación de respuestas
src/lib/roles.ts        Normalización de roles para la interfaz
src/pages/              Páginas funcionales y avisos de integraciones pendientes
src/layouts/            Estructura de navegación
src/components/         Componentes compartidos y primitivas de UI
tests/                  Pruebas críticas de listas, administración y errores del chat
```

El alias `@/` apunta a `src/`. Se usan React Context y hooks; no Redux/Zustand.
Comprobar imports del router y sus componentes antes de declarar un archivo
obsoleto: el nombre PascalCase o kebab-case no determina por sí solo si se usa.

## Sesión y permisos

- La sesión del backend utiliza la cookie HttpOnly `jwt`; los clientes protegidos
  envían `credentials: 'include'`. No leer el JWT desde JavaScript ni introducir
  tokens internos de scraping en el navegador.
- `AuthContext` conserva una representación del usuario en `localStorage` para
  la UI. Esto no acredita por sí solo una sesión válida en el servidor.
- `ProtectedRoute` controla navegación; la autorización efectiva corresponde
  al backend. Roles: `registrado`, `colaborador`, `admin`; el plan `premium` es
  un concepto distinto del rol.
- Sin base API, administración muestra un error de configuración y no utiliza
  datos de ejemplo. Los fallos parciales de métricas se muestran como «sin datos».
- No existe una cuenta admin precreada documentada. Solicitar su asignación a
  un operador autorizado, sin cambiar validaciones ni falsificar el usuario local.

## Comprobaciones

Tras iniciar sesión por contraseña o Google, `/login` redirige a `/admin` si el
usuario devuelto por Identidad tiene rol `admin`; las demás cuentas van al
catálogo. No se asignan permisos por correo. Para probar una promoción de rol,
cerrar sesión e iniciar nuevamente, y comprobar `rol` en la respuesta de login.
La redirección no sustituye la autorización del backend.

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

## Referencias

- [Arquitectura y ejecución del proyecto](../../README.md).
- [Índice documental y advertencias de vigencia](../../docs/README.md).
- [Gateway](../../backend/gateway/README.md).
- [Identidad](../../backend/identidad/README.md).
- [Contrato de listas](../../backend/motor_rutas/README.md).
