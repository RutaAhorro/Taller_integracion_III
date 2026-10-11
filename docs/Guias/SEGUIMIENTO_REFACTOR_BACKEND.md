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

La tarea en curso es SUP-271, en
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
