# Seguimiento del refactor del backend

Registro de la revisión del 2026-10-10 (America/Santiago). Consultarlo al
retomar las tareas y actualizarlo cuando cambie la base de integración.

## Base de integración revisada

- Rama: `develop-refactor`.
- HEAD remoto al comenzar SUP-268: `1ae1f8662e1d0fa972b04700ec7a84c9e037565c`.
- SUP-267 se integró por squash en `1ae1f86` ([PR #94](https://github.com/danirc2024/Taller_integracion_III/pull/94)),
  con los fixes de CORS y errores internos. La base anterior de SUP-267 era `b2d1b5f`.
- SUP-266 se integró por squash en `b2d1b5f` ([PR #93](https://github.com/danirc2024/Taller_integracion_III/pull/93));
  el seguimiento de SUP-264/265 se integró en `e263379` (PR #92).
- La revisión histórica de SUP-264/265 descrita debajo corresponde a `bd94276`.
- SUP-265 se integró mediante squash en `27f15cd` ([PR #91](https://github.com/danirc2024/Taller_integracion_III/pull/91)).
- SUP-264 se integró después mediante squash en `bd94276` ([PR #90](https://github.com/danirc2024/Taller_integracion_III/pull/90)).
- Al revisar, la rama publicada de SUP-265 estaba en `5841afc`. No contiene por sí sola los
  cambios posteriores del script de humo. Las próximas tareas deben partir de
  la base actualizada de `develop-refactor`.

## Cambios posteriores a nuestro commit de SUP-265

Renato incorporó `develop-refactor` a la rama de SUP-264 (`301264d`). Ese merge
dejó dos condiciones `if` superpuestas en `TestLoadDefaults`, en
`backend/gateway/internal/config/config_test.go`, y las pruebas Go no compilaban.

1. `6d5d99b`: eliminó la condición antigua duplicada, manteniendo la comprobación
   de `ReadinessTimeout == 2s`. También copió el bundle público de CA del sistema
   a un archivo temporal para la integración Docker.
2. `072103d`: movió esa copia desde `/tmp` a `.test-ca.crt` dentro del repositorio,
   exportó `SUP265_CA_BUNDLE=./.test-ca.crt` y añadió su eliminación al cleanup.
   La ruta relativa resolvió el fallo de acceso de archivos de BuildKit en CI.

La diferencia final entre `5841afc` y `bd94276` consiste únicamente en ocho líneas
añadidas a `scripts/smoke_gateway_compose.sh`. La corrección de sintaxis restaura
el test que ya teníamos; el código y las pruebas Go finales coinciden con nuestra
versión. No se modificaron rutas públicas, autenticación, datos ni lógica de negocio.

## Evidencia de validación

Historial de `Gateway checks` en GitHub Actions:

- `301264d`: fallaron `gateway` e `integration`.
- `6d5d99b`: quedó fallando únicamente `integration`.
- `072103d`: ejecución del PR aprobada.
- `bd94276`: [ejecución tras la integración aprobada](https://github.com/danirc2024/Taller_integracion_III/actions/runs/38012109437);
  jobs `gateway`, `integration` y `manifests` completados con éxito.

Validación local de la revisión:

- Sintaxis Bash del script integrado aprobada.
- Humo ejecutado desde una exportación temporal de `bd94276`, con
  `SUP265_SKIP_BUILD=1`. Se reutilizaron las imágenes locales cuyo código de
  aplicación coincide con esa revisión.
- 23 comprobaciones de contratos, HTML/ruteo del frontend y 10 comprobaciones de
  rollback aprobadas; Gateway estaba detenido durante la comprobación de retorno.
- Proyecto, red, volumen y `.test-ca.crt` de prueba eliminados al terminar.
- No se reconstruyeron imágenes localmente en esta revisión. El build y las
  comprobaciones de BuildKit están respaldados por la ejecución de CI citada.
- No se desplegó ni se comprobó el clúster universitario.

## Decisiones que deben conservarse

- La Gateway sigue siendo un proxy independiente; Identidad, Catálogo y Control
  de scraping todavía pertenecen a la API actual.
- `api:8080` es la entrada Gateway; su upstream es `go_service:8080` en Compose
  y `api-backend:8080` en Kubernetes. No crear un bucle apuntando a `api:8080`.
- X-Forwarded-For se recorre desde la derecha, únicamente a través de proxies
  confiables. Se detiene en la primera IP no confiable o el primer elemento
  inválido. Conservar sus pruebas de regresión y configurar proxies reales.
- Readiness comprueba API, PostgreSQL y Redis; liveness es independiente.
- El humo usa datos y proyectos aislados, sin bot ni spiders reales.
- Nginx debe recrearse después de mover el alias `api` durante el rollback.
- Los certificados de construcción se montan mediante secretos BuildKit. Mantener
  la verificación TLS y las firmas de paquetes; no incorporar CA del entorno a
  la imagen publicada para resolver errores de instalación.

## Detalles pendientes del script de certificados

El cambio corrigió CI, pero dejó tres aspectos que conviene ajustar cuando se
trabaje nuevamente en esta integración:

1. Sobrescribe cualquier `SUP265_CA_BUNDLE` proporcionado por quien ejecuta el
   script. Esto impide usar el bundle personalizado que permite el override de
   Compose y describe la guía. La futura corrección debería respetar ese origen
   y copiarlo a una ruta permitida para BuildKit cuando haga falta.
2. `.test-ca.crt` tiene un nombre fijo. Puede sobrescribir un archivo preexistente
   y dos ejecuciones simultáneas pueden interferir. El cleanup lo elimina incluso
   si el fallo ocurre antes de crear la copia. Usar un nombre propio del proyecto
   y eliminar únicamente archivos creados por esa ejecución.
3. Si falta el bundle del sistema, crea un archivo vacío. Un archivo vacío no
   constituye un almacén TLS válido; se comprobó que el cargador SSL de Python lo
   rechaza. Esto no demuestra que todos los clientes de las imágenes fallen,
   pero tampoco garantiza que la descarga funcione. Validar el origen o fallar
   con un mensaje claro, conservando la verificación TLS.

La guía de Gateway todavía describe la selección de `SUP265_CA_BUNDLE` anterior
al cambio; actualizarla junto con la futura corrección. Estos aspectos quedaron
registrados y no se modificaron durante la revisión.

## Continuidad del trabajo

SUP-266 está integrada: Identidad y Scraping tienen módulos privados, conservan
sus contratos y siguen en el binario de API. Se verificó el review de login SSO:
la comprobación de `PasswordHash` nulo/vacío estaba conservada en `390a2bd`, con
prueba de regresión aprobada. No fue necesario modificar la lógica de login.

SUP-267 está integrada: Catálogo tiene módulo, proceso, imagen y pruebas propios.
SUP-268 se publicó en [PR #95](https://github.com/danirc2024/Taller_integracion_III/pull/95),
commit `e6c73ac`, pendiente de integración al comenzar SUP-269, en
`backend/refactor/sup-268-enrutar-desplegar-catalogo-vmatus`, desde `1ae1f86`.
Activa las consultas en Gateway, incorpora Compose/Kubernetes/HPA y elimina
el lector duplicado de API. Consultar `GUIA_CATALOGO_ENRUTAMIENTO.md` para
activación, rollback y límites de la validación local.
El review de seguridad de PR #94 detectó dos deudas heredadas: CORS con reflexión
de cualquier origen y exposición de errores internos. Se corrigen en Catálogo
con `CORS_ALLOWED_ORIGINS`, mensajes públicos genéricos y logs correlacionados.
Al activar SUP-268, configurar los orígenes exactos del frontend. La política
CORS y los errores de las otras rutas de la API heredada requieren seguimiento
propio; esta corrección no cambia el proceso de Identidad ni sus cookies.
Gateway usa readiness local y diagnóstico separado de dependencias para que
una caída de API/Redis no retire el acceso a Catálogo. El rollback de Catálogo
exige una imagen anterior de API con lector; quitar sólo `CATALOGO_URL` no basta.
El humo de SUP-268 reemplaza el antiguo script de integración y resuelve los
pendientes de certificados de ese script mediante una copia única, respetando
el bundle recibido y eliminando sólo archivos propios. Las observaciones de
SUP-264/265 anteriores son históricas.
Los squashes ya incorporaron el contenido de las ramas anteriores; evitar
reintroducir sus commits históricos al preparar la siguiente tarea.

SUP-269 se implementó en
`backend/refactor/sup-269-transferir-ingesta-catalogo-vmatus`, creada desde
`e6c73ac` por autorización del usuario para continuar mientras se revisa PR #95.
Scraping entrega los lotes mediante HTTP autenticado a Catálogo; no escribe
productos ni precios. El contador se actualiza en coordinación después del
commit de Catálogo. Conservar el manejo explícito de `contador_pendiente` y
no habilitar reintentos de lotes antes de SUP-270. Ver `GUIA_CATALOGO_INGESTA.md`.
SUP-268 ya se integró mediante el squash `b465d0a`, con el mismo contenido que
la base original `e6c73ac`. Se incorporó `develop-refactor` en SUP-269 y se
actualizó la referencia de rollback al commit integrado. El usuario autorizó
abrir el PR de SUP-269 hacia `develop-refactor`.
SUP-269 se integró mediante [PR #96](https://github.com/danirc2024/Taller_integracion_III/pull/96),
con el squash `fc2b1c2`, cuyo contenido coincide con la base de trabajo `d45650a`.
SUP-270 se publicó en [PR #97](https://github.com/danirc2024/Taller_integracion_III/pull/97), en
`backend/refactor/sup-270-recuperacion-trabajos-ingesta-vmatus`, preparada
sobre `fc2b1c2` de `develop-refactor` para incluir sólo los cambios de SUP-270.
El usuario autorizó commit, push y PR hacia `develop-refactor`, con revisión
de Renato (`RCarrascoO`) y Dani (`danirc2024`).
La API recupera entregas y ejecuciones desde outboxes SQL; Catálogo confirma
recibos idempotentes junto con capturas. El worker confirma Redis Streams sólo
tras cierre en API y conserva su spool en un volumen persistente. No tiene
acceso a PostgreSQL. Ver `GUIA_SCRAPING_RECUPERACION.md` para migración,
permisos, límites de reintentos, operación y rollback. Aplicar la migración y
drenar la lista BRPOP anterior antes del corte; no desplegar sin autorización.
Correcciones de review en [PR #97](https://github.com/danirc2024/Taller_integracion_III/pull/97):
- Se distinguen entregas pendientes/fallidas y se permite cerrar como fallido
  sin detener la conciliación de entregas ya registradas.
- El publisher no retiene locks SQL durante Redis; ACK coloca una marca de
  cierre con TTL para impedir republicación tardía. Se purga el outbox de
  ejecuciones terminales; coordinación requiere DELETE sólo en esa tabla.
- Spools completados se borran tras ACK. Fallidos se retienen 24 h y pueden
  retirarse antes bajo presión, sólo con estado terminal y sin mensaje.
  Reserva de 64 MiB y GC conservan trabajos activos ante disco lleno.
- Se corrigen clasificación SQL 400/500, prioridad tras reinicio y LREM
  desde la cola. Se documenta y prueba el orden SQL/permisos antes del rollout.

Validación SUP-270 tras el review: 194 resultados Go (193 unitarios más
1 integración con PostgreSQL real), 25 pruebas Python y 338 comprobaciones
HTTP sin fallos; 16 recursos Kubernetes válidos. API y Catálogo compilan y
pasan race detector/vet. Los builders completos quedan en CI.

SUP-270 fue aprobada e integrada mediante PR #97 (`42b6ca4`). SUP-271 se retomó
sobre ese merge, conservando cierre fallido, conciliación posterior, clasificación
de errores, retención del spool, recuperación de leases y publisher sin locks SQL.
El rol `app_scraping` tiene DELETE exclusivamente sobre `scraper.entregas_trabajos`,
comprobado al arrancar y también en el rollback a la imagen aprobada de SUP-270.

SUP-271 se implementó en
`backend/refactor/sup-271-separar-permisos-datos-vmatus`, basada en `42b6ca4`
de `develop-refactor`.
El usuario autorizó commit, push y apertura del PR de SUP-271 hacia
`develop-refactor`, con revisión de Renato (`RCarrascoO`), Dani (`danirc2024`)
y Esban (`EsbanV`).
Se preparan roles `app_identity`, `app_scraping`, `app_catalogo`, dos conexiones
privadas en API y una exclusiva de Catálogo. La migración 06 conserva los datos,
guarda el nombre de cadena y elimina las FK externas desde trabajos. El script 07
provisiona contraseñas privadas distintas; no corre al iniciar los servicios.
Consultar `GUIA_DATOS_Y_PERMISOS_BACKEND.md` antes de desplegar. El trabajo local
no aplica migraciones, Secrets ni cambios en el clúster universitario.
Validación inicial de SUP-271 (`4d019dd`): 200 resultados Go (199 unitarios y
una integración con PostgreSQL real), 25 pruebas Python, 352 comprobaciones HTTP
y 118 accesos SQL por rol aprobados. Veinte recursos Kubernetes válidos; bootstrap
del script 07 sourceado con modo 0644 y rollback a SUP-270 y vuelta comprobados.
Se usaron imágenes runtime verificadas con binarios nuevos; builders completos
de producción y despliegue universitario quedan fuera de la comprobación local.


Correcciones solicitadas en review de SUP-271 (H-01/H-02): API devuelve 503
cuando falla cualquiera de sus dos pools, con liveness independiente en
`/_api/live` y presupuesto compartido de 2 s. Catálogo valida permisos una vez,
antes de abrir HTTP (5 s), y readiness queda limitado a Ping. Cada pool de API
baja a cinco conexiones: ambos HPA al máximo suman 80 conexiones de runtime.
No se amplían max_connections/recursos DB ni se modifica Keep-Alive en este fix.
La prueba real de probes registra la caída/recuperación de DB sin reiniciar API
ni Catálogo, cuenta consultas de permisos en PostgreSQL y verifica el rechazo
al arrancar sin grants. Validación del fix: 212 resultados Go (211 unitarios y una integración PostgreSQL),
25 pruebas Python, 413 comprobaciones HTTP y 118 controles SQL por rol.
Veinte recursos Kubernetes válidos, workflows/sintaxis/Swagger/diff comprobados.
Los recursos temporales se eliminaron. El autor autorizó publicar estas
correcciones en la misma rama de SUP-271 y actualizar el PR #98 hacia
`develop-refactor`, solicitando nuevamente revisión a Renato, Dani y Esban.

Autor de commits: Vicente Sebastian Matus Mora, `vmatus2024@alu.uct.cl`.
No hacer nuevos commits ni push sin autorización para la tarea correspondiente.
El autor abre los PR manualmente salvo autorización expresa para crearlos.


## SUP-272: extracción de Identidad

SUP-271 fue integrada en PR #98 por squash `49770b335aa6e6998cbbfd23df8b8868ff1c26f4`.
La rama local `backend/refactor/sup-272-extraer-identidad-vmatus` se creó desde
ese HEAD actualizado de `develop-refactor`. Tras revisar el resumen, el usuario
autorizó commit, push y apertura del PR hacia `develop-refactor`, con revisión
de Renato (`RCarrascoO`), Dani (`danirc2024`) y Esban (`EsbanV`).

Identidad se mueve a `backend/identidad` con módulo/binario/imagen propios.
La API sólo compone coordinación y sólo recibe su DSN; Gateway conserva auth
mediante IDENTIDAD_URL. Hay Deployment, Service, HPA, configuración y política
de entrada propios. No se migran usuarios ni se amplían sus grants.
Se conservan las cookies/JWT existentes, con validación HS256/expiración estricta,
clave obligatoria, CORS por allowlist, cookies Secure configurables, contador
Redis atómico y manejo seguro de claims Google/SSO y conflictos concurrentes.
Google distingue autenticación rechazada, configuración ausente y fallo interno;
`detalle` queda sólo para validación 400, sin SQL en errores de autenticación.
Ver `GUIA_IDENTIDAD_DESACOPLADA.md` para contratos, configuración y rollback.

La revisión comprueba sesiones antiguas en ambos sentidos, ruta real del frontend,
IP de cliente ante spoofing de forwarding, TTL reparado, registros/login
concurrentes, caída independiente de cada dominio, Redis/SQL, arranque sin grants,
cookies Secure y tres réplicas de Identidad sin recrear API/Gateway.
Los rollbacks históricos actualizan también la configuración de Gateway para
no consultar una Identidad detenida. La suite de ingesta reconoce 202 pendientes
con clave estable y exige confirmación final, contador y capturas exactos.

Configuración obligatoria previa al despliegue: proxies Gateway reales en
Identidad, proxies Nginx/Ingress reales en Gateway, allowlist de frontend y Secrets
existentes. No dejar IPs/CIDRs genéricos ni confiar en cualquier cliente.
La imagen corre sin privilegios; el clúster exige CNI con NetworkPolicy.
La firma asimétrica, revocación/refresh, verificación de correo y nuevas cuotas
son tareas distintas; las limitaciones heredadas están registradas en la guía.

Validación inicial de SUP-272 (`185e5e3`): 479 resultados Go (64 API, 124 Identidad, 182 Gateway,
108 Catálogo y 1 integración del publisher con PostgreSQL real), con race
detector y vet. La integración omitida por el suite unitario se ejecutó
explícitamente en el harness. Veinticinco pruebas Python, 478 comprobaciones
HTTP (110 específicas de Identidad y 368 de regresiones), 118 controles SQL
por rol y 25 recursos Kubernetes válidos, sin fallos pendientes.
Fuzzing del parser JWT: 45.909 entradas durante 15 segundos, sin fallos.
La referencia aprobada SUP-271 y sus cookies se probaron en ambos sentidos.

Se construyó la imagen de Identidad desde su Dockerfile real; también se
comprobaron las pruebas y vet en el target test y los binarios con build estático.
Los últimos cambios de recuperación de errores/logs se comprobaron con el
suite Go y la imagen final, seguida de las 110 pruebas de Identidad. Las otras
regresiones se ejecutaron en el harness completo; no se cuentan dos veces.
API y Gateway usan binarios nuevos sobre runtimes verificados en la integración
local; sus builders completos quedan para CI. El acceso real a Google desde
el navegador, HPA en el clúster y aplicación de NetworkPolicy no se probaron.
Se conservaron TLS y sumas verificadas en las descargas: el DNS de Docker cloud
necesitó network host y resolución explícita del proxy, sin modificar la imagen
publicada ni sus CA. Los recursos de prueba y certificados temporales se
eliminaron; sólo quedaron los servicios originales de la sesión.
Los intentos de prueba fallidos se diagnosticaron y corrigieron: IP reservada
que entraba en el rango dinámico, puerto fijo al escalar el harness, cleanup de
profile de referencia, aceptación concurrente 202 con confirmación pendiente,
comprobación de outage duplicada y actualización de Gateway en rollback histórico.
No se aplicaron migraciones/Secrets en el clúster ni se publicaron imágenes.
El usuario autorizó publicar los cambios en la rama de SUP-272 y abrir el PR;
esa autorización no incluye integrar el PR ni desplegar el clúster.


### Correcciones del review de PR #99 (SUP-272)

H-01 se resuelve usando probes exec por loopback dentro de la imagen Alpine,
con wget y timeouts acotados. NetworkPolicy conserva exclusivamente los pods
Gateway del mismo namespace en 8080. No se agregan CIDRs de nodos ni se abre el
namespace completo. La especificación Kubernetes permite el tráfico del nodo
local; la caída indicada no es una consecuencia inevitable de NetworkPolicy.
Se documenta ese matiz y la independencia de ingress de los probes locales.
La aplicación real de políticas de host/CNI requiere comprobarse en el clúster.

H-02 incorpora ErrNombreInvalido y respuesta 400 en perfil y registro, sin
persistir nombres vacíos tras sanitizar. H-03 omite el avatar opcional de Google
cuando excede 500 caracteres o tiene UTF-8 inválido; mantiene el login y no
trunca URLs. H-11 aplica límite de 100 runas y regexp compilada una vez, con
pruebas de límites de tildes/ñ y persistencia real en PostgreSQL.

H-10 regenera docs.go, swagger.json y swagger.yaml con swag init --parseInternal.
Swagger publica sólo operaciones existentes en el router de coordinación y
salud; retira Identidad/Catálogo de esta especificación. Las rutas funcionales
públicas continúan por Gateway. El alcance de /swagger/* queda explícito y se
verifica automáticamente que cada operación publicada existe en el router.
Los humos históricos conservan su contrato integrado cuando usan imágenes
anteriores; SUP-272 comprueba explícitamente la especificación por servicio.

El usuario autorizó commit y push de estas correcciones en la misma rama,
actualizando PR #99 hacia develop-refactor. Esa autorización no incluye merge
ni despliegue del clúster.

Validación de las correcciones: 208 resultados Go de los módulos afectados
(65 API y 143 Identidad) con race detector, y 1 integración adicional del
publisher con PostgreSQL real; vet aprobado. El harness completo pasó 499
comprobaciones: 484 HTTP y 15 ejecuciones de los comandos exactos de probes,
además de 118 controles SQL por rol. Incluye 131 comprobaciones de Identidad
(116 HTTP y 15 probes), recuperación de scraping, carga con tres réplicas de
Catálogo y rollbacks. Veinticinco manifiestos Kubernetes válidos; imagen de
Identidad construida desde el Dockerfile real, Swagger/workflows/sintaxis/diff
comprobados. Los recursos temporales se eliminaron. No se desplegó en el clúster
ni se verificó allí el CNI o el HPA; Google real desde navegador sigue fuera de
esta comprobación. Las cifras anteriores corresponden a la validación inicial.


## SUP-273: extracción final de Control de scraping

SUP-272 se integró por squash `45aed821b76cd3179092fe9b686671da7700f05b` en
PR #99. La rama `backend/refactor/sup-273-extraer-coordinacion-scraping-vmatus`
parte de ese HEAD de develop-refactor. El usuario autorizó completar la tarea,
hacer commit/push y abrir PR con revisión de Renato, Dani y Esban.

El último dominio de backend/api pasa a backend/control_scraping. Tiene módulo,
cmd/control_scraping, imagen, despliegue, HPA y probes propios. Se retira el
servidor API heredado y sus modelos globales/utilidades sin uso. El seeder de
productos se conserva como comando de desarrollo de Catálogo, sin modificar
permisos runtime ni ejecutarlo contra datos de la sesión.

Gateway usa SCRAPING_URL sin BACKEND_URL en modo final; enruta las rutas scraper,
health, raíz y Swagger. Control caído devuelve 502/504 sin fallback. Se conservan
los alias de diagnóstico previos y se bloquean los probes privados desde la
entrada pública. BACKEND_URL y las imágenes históricas se usan sólo para rollback.
Los overrides restauran expresamente probes, DSN y JWT de las imágenes antiguas.
Service/api permanece como alias de Gateway; se retiran Deployment/HPA/api y
Service/api-backend de los manifiestos nuevos. No borrar Service/api en el clúster.

Control conserva todas las correcciones de recuperación/idempotencia de SUP-270/271
sin migración de tablas, nombres de cola, claves de lote ni spools. Sólo recibe
app_scraping (pool 5), Redis y token de Catálogo; no recibe JWT_SECRET, DSN de
Identidad ni Google. Catálogo conserva la comprobación token interno distinto de
JWT. El worker continúa sin acceso SQL. HPA Control 1–4, CPU 70%, requests
100m/64Mi y límites 500m/256Mi; el presupuesto total de pools sigue en 80 al máximo.

CORS allowlist, cuerpo 8MiB, contextos/timeouts acotados, logs correlacionados y
500 sin trazas para fallos internos. Redis requiere URL válida sin fallback;
su caída de red puede degradarse manteniendo readiness SQL y outbox durable.
SIGTERM drena HTTP y termina los dos loops antes de cerrar sus conexiones.
Runtime no root/read-only; los tres probes Kubernetes son exec por loopback.
No se amplía la NetworkPolicy a todo el namespace o a CIDRs de nodos.

Validación Go: 533 resultados unitarios (81 Control, 201 Gateway, 108 Catálogo,
143 Identidad); race/vet aprobados en los tres módulos modificados. Identidad
se comprueba también en modo readonly durante la validación de las nuevas
instrucciones de instalación. Una prueba del publisher omitida en el suite
unitario se ejecutó explícitamente con PostgreSQL real: 534 resultados Go
incluyendo esa integración, sin paquetes fallidos pendientes.

El harness completo pasó 563 comprobaciones funcionales (530 HTTP y 33 exec de
probes), 118 controles SQL por rol y los rollbacks de las etapas anteriores.
Tras los últimos ajustes de Swagger/headers se construyó la imagen de Control
desde su Dockerfile real y pasó nuevamente su tramo: 47 comprobaciones HTTP y
18 probes, incluido un chequeo adicional de Swagger. Estas corridas se solapan;
no se suman como si fueran pruebas diferentes. Tres procesos de Control recibieron
18 entregas simultáneas del mismo lote: una captura, un contador y confirmación
final por Gateway; Identidad/Catálogo/Gateway no se recrearon al escalar Control.

Veinticinco pruebas Python del worker aprobadas. Veintisiete recursos Kubernetes
válidos; workflows, Compose, sintaxis, Swagger
y diff comprobados. Gateway se probó con binario nuevo sobre runtime verificado;
su builder completo queda para CI. Se conservaron TLS y sumas en descargas. Los
intentos iniciales detectaron permiso del binario del runtime local y un campo
renombrado accidentalmente en el seeder; ambos se corrigieron y se revalidaron.
Los recursos efímeros se eliminaron, conservando DB/Redis originales y stashes.
No se desplegó en el clúster ni se probó allí CNI/HPA; tampoco Google real desde
navegador, scraping externo o Discord. No se publicaron imágenes durante el test.

Guía operativa final: GUIA_CONTROL_SCRAPING_DESACOPLADO.md. El borrador de entorno
actualiza install_script/start_skill para los módulos nuevos, conservando el
flujo de ramas anteriores, configuraciones y credenciales existentes. Se verifican
los comandos Go del nuevo bloque y el arranque funcional mediante el harness;
el guardado del borrador no publica una nueva versión del entorno.

Esta tarea completa la extracción de los tres dominios planificados. Autorización
propia del worker, quotas/reglas nuevas, refresh/revocación, firma asimétrica e
IA/rutas siguen siendo backlog de producto/hardening, sin ampliar este PR.
La autorización de publicación no incluye merge ni despliegue universitario.
