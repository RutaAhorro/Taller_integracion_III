# Identidad

Servicio Go/Gin/GORM de cuentas, sesión y administración de usuarios. Es dueño
de `api.usuarios`, de la auditoría administrativa y de las direcciones guardadas
(`api.direcciones_usuario`).
Usa PostgreSQL con `app_identity` y Redis para limitar intentos de autenticación.
Gateway enruta las solicitudes; la autorización efectiva pertenece a Identidad.
No importa otros dominios ni consulta tablas ajenas; tampoco ejecuta migraciones.

Este README reúne la documentación vigente de configuración, seguridad,
contratos y operación consolidada en SUP-347, que absorbió y retiró la antigua
`docs/Guias/GUIA_IDENTIDAD_DESACOPLADA.md`. Los pasos de extracción del
monolito de SUP-272 y su procedimiento de corte quedan en el historial de Git.

## Contratos HTTP implementados

| Método | Ruta pública | Comportamiento |
| --- | --- | --- |
| POST | `/api/v1/auth/register` | Registro local con auto-login y cookie de sesión |
| POST | `/api/v1/auth/login` | Login local; errores de credenciales no revelan si existe la cuenta |
| POST | `/api/v1/auth/google` | Valida criptográficamente el token de Google y emite sesión |
| POST | `/api/v1/auth/restablecer` | Cambia la contraseña con un token de un solo uso; sin sesión previa |
| POST | `/api/v1/auth/logout` | Expira la cookie `jwt`; responde 204 sin cuerpo |
| GET | `/api/v1/auth/me` | Identidad de la sesión: `user_id`, rol vigente, provider y mensaje; cuenta revalidada en PostgreSQL |
| PUT | `/api/v1/auth/me` | Actualiza el perfil de la cuenta de la sesión |
| GET | `/api/v1/usuarios/me/direcciones` | Direcciones del usuario de la sesión: `{"data":[...]}`, principal primero y luego de la más antigua a la más reciente (desempate por id) |
| POST | `/api/v1/usuarios/me/direcciones` | Crea una dirección (201). La primera del usuario queda principal; con `es_principal:true` desmarca la anterior en la misma transacción |
| PUT | `/api/v1/usuarios/me/direcciones/{id}` | Reemplaza los campos editables (200). Ver el contrato de direcciones |
| DELETE | `/api/v1/usuarios/me/direcciones/{id}` | Elimina la dirección (204) |
| GET | `/api/v1/admin/usuarios` | Listado paginado; exige rol `admin` vigente |
| GET | `/api/v1/admin/usuarios/metricas` | Totales de cuentas y cuentas activas; exige `admin` |
| PUT | `/api/v1/admin/usuarios/{id}` | Modifica `esta_activo`, `rol` o `plan`; exige `admin` |

El listado acepta `q`, `rol`, `esta_activo`, `page` y `limit`. Devuelve `data`
y `paginacion` con `total_registros`, `pagina_actual`, `total_paginas` y
`limite`: 20 filas por defecto y máximo 200. Roles: `registrado`,
`colaborador`, `admin`. Los planes `basico` y `premium` son campos separados y
no conceden permisos administrativos.

> [!WARNING]
> **Cambios de contrato en SUP-347 (barrido de Identidad)**
>
> | Antes | Ahora |
> | --- | --- |
> | `GET /admin/usuarios` aceptaba `offset` (excluyente con `page`) | Sólo `page` y `limit`. Un `offset` recibido responde 400 con el mensaje "paginar con page y limit", para que ningún cliente reciba otra página sin enterarse |
> | `rol=administrador` se aceptaba como alias de `admin` en `PUT /admin/usuarios/{id}` y en el filtro `?rol=` | Responde 400: los únicos roles son `registrado`, `colaborador` y `admin` |
> | Una fila de `api.usuarios` con rol `administrador` se trataba como `admin` | La migración `10_restringir_rol_usuarios.sql` normaliza esas filas y agrega `usuarios_rol_check`: la base ya no acepta otro rol. Identidad no arranca sin esa restricción, así que no puede quedar una cuenta admin sin acceso |
> | El registro generaba un UUID en `api.usuarios.token_verificacion` | Las cuentas nuevas lo dejan en `NULL`: no hay flujo que lo consuma. La columna se conserva y ninguna respuesta HTTP lo exponía |
> | `identidad correo-prueba <destinatario>` enviaba un correo de prueba (SUP-189) | El comando se eliminó; el binario sólo arranca el servidor |
> | El arranque armaba un servicio interno de recuperación de contraseña (SUP-184) y un transporte SMTP (SUP-189), sin ningún endpoint que los usara | Se eliminaron junto con `internal/mailer`. Identidad ya no lee `IDENTIDAD_CORREO_MODO`, `SMTP_*` ni `IDENTIDAD_URL_RESTABLECER`. La tabla y sus grants se conservan; SUP-185 volvió a usarla para consumir tokens, sin reintroducir el envío de correos. La solicitud pública de token sigue pendiente (SUP-233) |
>
> Ningún consumidor del repositorio enviaba `offset` ni escribía `administrador`
> al momento del cambio (el frontend usa `page` y `limit`).

`PUT /auth/me` acepta nombre, correo, teléfono, dirección y contraseña.
Cambiar `correo` o `password` exige `current_password` válida; de lo contrario
responde 403. Las cuentas sin contraseña local no pueden establecerla por
esta vía. Los handlers y DTO de [internal/api](internal/api) y
[internal/domain](internal/domain) son la referencia de campos y respuestas.
El envoltorio de errores usa `estado`, `mensaje` y `detalle` cuando corresponde;
los errores internos no incluyen SQL ni trazas en el JSON.

`POST /auth/restablecer` recibe `{"token":"...","password":"..."}` y aplica la
misma política de contraseñas que `PUT /auth/me`: complejidad y hash bcrypt se
validan en un único helper del servicio, sin una segunda implementación. El
token se recibe como SHA-256 de 32 bytes, se comprueba contra
`api.tokens_recuperacion` usando el reloj de PostgreSQL y se consume con un
`UPDATE ... WHERE usado_el IS NULL AND expira_el > now()` en la misma
transacción que escribe `password_hash` e inutiliza cualquier otro token
pendiente del mismo usuario. Token inexistente, vencido o ya usado responden 400
con el mismo mensaje; una clave inválida también es 400 y deja el token vigente.
La tabla no se limpia desde la aplicación: el operador conserva las filas como
historial.

**Límite conocido:** el restablecimiento no revoca las sesiones ya abiertas. Un
JWT emitido antes del cambio sigue siendo válido hasta que expira, igual que tras
`PUT /auth/me`. Revocar sesiones requeriría un registro de tokens o una versión
de contraseña en el JWT y queda fuera de este cambio; para el caso "alguien entró
a mi cuenta", el usuario debe cerrar sesión en sus otros dispositivos.

Gateway conserva ruta, query, cuerpo, cookie y `Set-Cookie`, y elimina headers
de identidad forjados. Una caída de Identidad responde 502
`identidad_unavailable`; un timeout, 504 `identidad_timeout`, sin fallback a
otro servicio. Toda ruta bajo `/api/v1/auth/` pertenece a Identidad aunque
no exista (404 propio); prefijos parecidos como `/api/v1/authentication` no
se enrutan allí. Sus probes no se publican por Gateway. `/health`,
`/api/v1/health` y `/swagger/*` pertenecen a Control de scraping; no son rutas
de salud ni una especificación agregada de Identidad.

## Direcciones del usuario (SUP-239)

CRUD de las direcciones guardadas, cuyas coordenadas usará el motor de rutas como
origen (por HTTP, nunca por SQL cruzado). Exige la cookie `jwt` y revalida la
cuenta como el resto de rutas protegidas. **El dueño sale siempre de la sesión**:
el cuerpo rechaza campos desconocidos (`usuario_id`, `id`...) con 400 y los headers
de identidad se ignoran.

Cuerpo de POST y PUT (snake_case, JSON estricto de un solo objeto):

| Campo | Regla |
| --- | --- |
| `texto_direccion` | Obligatorio; se recorta; no vacío; hasta 255 caracteres |
| `comuna` | Obligatoria; se recorta; no vacía; hasta 100 caracteres |
| `etiqueta` | Opcional; hasta 100 caracteres; vacía equivale a `null` |
| `lat`, `lon` | Opcionales pero **ambas o ninguna**; numéricas; lat en [-90, 90], lon en [-180, 180]; se redondean a 8 decimales (precisión de la columna) |
| `es_principal` | Opcional. `true` la marca principal; omitido conserva el estado; `false` sobre la principal vigente responde 409 |

Los textos no admiten caracteres de control. Respuesta (`DireccionDTO`): `id`,
`etiqueta` (null si falta), `texto_direccion`, `comuna`, `lat`, `lon` (null si no
hay), `es_principal`, `creado_el`.

- **PUT reemplaza** los campos editables: omitir `etiqueta`, `lat` o `lon` los borra.
  Se eligió PUT (y no PATCH) por coherencia con `PUT /auth/me` y para no tener que
  distinguir "omitido" de "null" al limpiar coordenadas. No hay `/principal`: `PUT` con
  `es_principal:true` lo cubre.
- **Una sola principal por usuario**: la garantiza el índice único parcial
  `uq_direcciones_usuario_principal` y, además, un advisory lock transaccional por
  usuario que serializa alta, edición y borrado. Si el índice igualmente detectara
  una carrera responde 409 ("otra operación cambió la dirección principal"), nunca 500.
- **Borrar la principal** promueve a la más antigua de las restantes (por `creado_el` e
  id) en la misma transacción, de modo que un usuario con direcciones siempre tiene una
  principal. En filas anteriores a la migración 11 `creado_el` es la fecha de la
  migración y manda el id.
- **Límite**: 10 direcciones por usuario (`domain.MaximoDireccionesPorUsuario`); la
  undécima responde 409 con mensaje claro. El conteo está bajo el mismo lock.
- **Aislamiento**: toda consulta filtra por `usuario_id`. Una dirección ajena y una
  inexistente responden el mismo 404 (nunca 403) para no revelar su existencia.
  Un `{id}` que no es UUID responde 400.
- El `lat`/`lon` ausente es válido (dirección sin geocodificar); el motor de rutas
  debe tratar `null` como "sin origen utilizable".
- `comuna` es NULL en la base para filas previas a la migración 11 (no se podía
  inventar); la API siempre la exige, así que esas filas se devuelven con `comuna` vacía
  hasta que se editen.

| Estado | Caso |
| --- | --- |
| 400 | Campo inválido o desconocido, JSON mal formado, `{id}` no UUID |
| 401 | Sin cookie, token inválido o cuenta inactiva |
| 404 | Dirección inexistente o de otra cuenta |
| 409 | Límite alcanzado, desmarcar la principal, carrera por la principal |

## Sesión y seguridad

- JWT HS256 con `user_id`, `rol`, `provider` y expiración obligatoria de 24 h.
  La cookie `jwt` dura lo mismo: HttpOnly, Path `/`, SameSite=Lax y Secure
  activado por defecto. No hay clave de respaldo.
- Las rutas protegidas consultan `api.usuarios` en cada petición para comprobar
  actividad y rol actuales. No confían en `X-User-ID`, `X-Role` ni
  `X-User-Role`. Catálogo y Motor validan sus sesiones según sus propios
  contratos; esta revalidación no revoca globalmente un JWT ya emitido.
- Logout expira la cookie del navegador; no revoca JWT copiados. No están
  implementados aquí refresh tokens ni los endpoints de verificación de correo
  o de restablecimiento de contraseña. El auto-login del registro sigue activo.
- Google usa el validador oficial: firma, audience, issuer de Google, email
  verificado y subject. Se rechazan cuentas inactivas o vinculadas a otra
  identidad. Ante creación concurrente se vuelve a consultar y comprobar la
  cuenta. Un `PasswordHash` vacío o nulo no permite login local.
- Mientras el registro local no verifique el correo, el primer acceso con
  Google a una cuenta sin `google_id` vincula el subject e invalida la
  contraseña local. Es la medida vigente frente al registro anticipado del
  correo ajeno; no debe retirarse como parte de una limpieza.
- Los cambios administrativos usan una transacción con advisory lock: no
  pueden dejar al sistema sin administradores activos ni permitir que un admin
  se desactive o degrade a sí mismo. Los cambios efectivos de rol/estado quedan
  auditados; las escrituras idempotentes no inventan cambios.
- CORS permite credenciales sólo para orígenes exactos autorizados. Se limita
  el cuerpo y la duración de cada petición, y se genera o propaga `X-Request-ID`.
  Se registran ruta de plantilla, método, estado y duración, sin cuerpos ni cookies.
- Se conserva la política de contraseña de 8 a 72 bytes, mayúscula, número y
  símbolo. Los nombres se sanitizan hasta 100 caracteres Unicode; un nombre
  vacío tras sanitizar se rechaza antes de persistir. Un avatar opcional inválido
  o de más de 500 caracteres se omite sin bloquear el acceso con Google.

Deudas conocidas, fuera de este servicio por ahora: HS256 obliga a compartir
la clave con los validadores (pasar a firma asimétrica exige despliegue
coordinado), y cambiar la contraseña o desactivar una cuenta no revoca los
JWT ya emitidos fuera de las rutas que revalidan contra `api.usuarios`.

## Configuración

La lista completa, sus defaults y rangos están en [`.env.example`](.env.example).
Go no carga ese archivo ni el `.env` privado de la raíz automáticamente:
las variables deben llegar al proceso desde el entorno o el despliegue.
`config.Load` valida la configuración antes de conectar dependencias y abrir
HTTP; el arranque inyecta esa instantánea en los componentes. Cambiar valores
requiere reiniciar el proceso.

| Variable compartida o de conexión | Requisito |
| --- | --- |
| `IDENTITY_DB_URL` | DSN con `app_identity`, base y contraseña explícitos; sin fallback a administrador ni `DB_URL` |
| `JWT_SECRET` | Clave compartida con los validadores de sesión; al menos 32 bytes y sin espacios externos |
| `REDIS_URL` | Obligatoria y válida; se conserva el namespace de contadores |
| `GOOGLE_CLIENT_ID` | Audience de Google; vacía deshabilita SSO y conserva login local |
| `ADMIN_EMAILS` | Lista opcional de correos separados por coma que reciben rol `admin` tras validar `email_verified=true` en `POST /api/v1/auth/google` (promueve `registrado` a `admin`, nunca degrada ni altera `colaborador`); `POST /api/v1/auth/register` rechaza esos correos con 409 |
| `CORS_ALLOWED_ORIGINS` | Orígenes HTTP(S) exactos separados por comas, sin path ni comodines |
| `IDENTIDAD_COOKIE_SECURE` | `true` por defecto; `false` sólo para HTTP local |
| `AUTH_LOCAL_HABILITADO` | `false` por defecto; `true` habilita registro y login con contraseña local en desarrollo |
| `IDENTIDAD_TRUSTED_PROXIES` | IP/CIDR de los proxies reales; obligatorio, sin redes universales |
| `PORT` | 8080 por defecto |

No renombrar estas variables ni rotar `JWT_SECRET` sólo en un servicio. Una
rotación invalida las sesiones anteriores y se coordina con todos sus
validadores. Ver la [guía de configuración compartida](../../docs/Guias/GUIA_CONFIGURACION.md).

Plazos, límites por IP y tamaño del pool no son
variables de entorno: son constantes con nombre en
[internal/config/config.go](internal/config/config.go), porque ningún ambiente
los cambia. Sólo van al entorno los valores que difieren entre ambientes (los de
la tabla anterior). JWT y cookie conservan las 24 h del contrato.

| Grupo | Valores |
| --- | --- |
| Arranque y cierre | arranque 5 s, apagado 15 s |
| HTTP | cabeceras 5 s, lectura 15 s, escritura 20 s, inactividad 60 s; petición 10 s; cabeceras y cuerpo hasta 64 KiB |
| Dependencias y sonda | Redis 1 s, readiness 2 s, validación de Google 5 s |
| Pool PostgreSQL | 5 conexiones abiertas, 2 inactivas, vida máxima 30 min |
| Contadores por IP | ventana 15 min; login 5 fallos, registro 30, Google 30 |

> [!NOTE]
> SUP-347 llegó a exponer estos valores como variables `IDENTIDAD_*`. Se
> retiraron antes del merge: ningún manifiesto las definía. Si un ambiente
> necesita un valor distinto, se cambia la constante y se documenta aquí.

## Datos, permisos y arranque

Aplicar las migraciones como operador siguiendo el
[orden de scripts SQL](../../infrastructure/db/README.md), no desde Identidad.
El servicio comprueba tablas y permisos antes de abrir HTTP:

- `06_propiedad_permisos.sql`: rol `app_identity` y grants de usuarios.
- `08_normalizar_rol_admin_y_auditoria.sql`: normalización y auditoría;
  requiere SELECT/INSERT en `api.auditoria_admin_usuarios` y USAGE en su secuencia.
- `09_plan_y_ultimo_acceso.sql`: columnas de plan comercial y último acceso.
- `10_restringir_rol_usuarios.sql`: restricción `usuarios_rol_check` (sólo
  `admin`, `colaborador` y `registrado`). Aplicar antes de desplegar esta versión.
- `11_direcciones_usuario.sql` (SUP-239): columnas `comuna` y `creado_el`, `lat`/`lon`
  opcionales con CHECK, índice único de la principal y permisos de
  `api.direcciones_usuario`. El arranque verifica permisos, columnas e índice.

El rol runtime tiene SELECT/INSERT/UPDATE sobre usuarios, sin DELETE, DDL ni
acceso a tablas de otros servicios. Sobre `api.direcciones_usuario` tiene SELECT,
INSERT, UPDATE y DELETE (nada más): la dirección es un dato del propio usuario. La ausencia de almacenamiento base o
permisos impide iniciar. `api.tokens_recuperacion` (`05_tokens_recuperacion.sql`)
se verifica al arrancar: el consumo necesita SELECT y UPDATE, y la aplicación
nunca hace DELETE sobre esa tabla. No se ejecuta AutoMigrate ni se amplían
grants desde la aplicación.

Cada réplica tiene su propio pool. Al cambiar su tamaño máximo o el
número de réplicas, recalcular el presupuesto total de conexiones junto con
los otros servicios. Como referencia, con 4 réplicas por dominio y pools de
5 (Identidad y Control) y 10 (Catálogo) el runtime usa hasta 80 conexiones,
más las sesiones administrativas. Los valores no acreditan capacidad medida.
Ver [propiedad de datos y permisos](../../docs/Guias/GUIA_DATOS_Y_PERMISOS_BACKEND.md).

## IP real y rate limiting

Gateway e Identidad deben confiar únicamente en sus proxies reales. Gateway
normaliza `X-Forwarded-For` según `GATEWAY_TRUSTED_PROXIES`; Identidad lo
interpreta sólo desde `IDENTIDAD_TRUSTED_PROXIES` e ignora `X-Real-IP`.
Una configuración incorrecta puede agregar a todos los usuarios bajo la IP
del proxy o permitir falsificar el origen.

Compose usa `identidad_edge`, el alias `identidad-upstream` y una IP reservada
para Gateway (`IDENTIDAD_GATEWAY_IP`). Al ajustar `IDENTIDAD_EDGE_SUBNET` o
`IDENTIDAD_EDGE_RANGE`, mantener la IP reservada fuera del rango dinámico y
evitar solapamientos con otras redes.

El [Deployment](../../infrastructure/k8s/identidad/deployment.yaml) obtiene los
proxies del [ConfigMap](../../infrastructure/k8s/identidad/configmap.yaml).
La [NetworkPolicy](../../infrastructure/k8s/identidad/networkpolicy.yaml) actual
permite entrada a 8080 desde pods Gateway y Catálogo del mismo namespace;
su aplicación efectiva depende del CNI. No confiar en `0.0.0.0/0` ni `::/0`.

Login y restablecimiento cuentan intentos fallidos; registro y Google, todas
las solicitudes. Los defaults son 5 fallos de login, 10 de restablecimiento y
30 solicitudes de registro/Google por IP en 15 minutos. Las réplicas comparten
claves `rate_limit:login:<IP>`, `rate_limit:restablecer:<IP>`,
`rate_limit:register:<IP>` y `rate_limit:google:<IP>`. Lua incrementa y fija
expiración atómicamente, repara TTL ausentes y devuelve el intento de una
respuesta 2xx. Redis debe permitir EVAL/EVALSHA, INCR, PTTL, PEXPIRE, EXISTS,
GET y DECR. Sin Redis, estas rutas fallan cerradas con 500 genérico; no se
desactiva el límite ni se reintentan sus escrituras.

## Salud y operación

`/_identidad/live` responde sin consultar SQL ni Redis. `/_identidad/ready`
comparte su presupuesto entre Ping SQL y Redis; no repite la auditoría de
permisos. Con los defaults, el arranque tiene 5 s, readiness 2 s y el apagado
15 s.
Una caída posterior de dependencias no provoca un fallo de liveness; readiness
puede recuperarse al volver esas dependencias. Cambiar permisos después del
arranque requiere reiniciar para volver a verificarlos.

Los probes Kubernetes usan BusyBox `wget` por loopback dentro del contenedor:
startup/liveness consultan `live`, readiness consulta `ready`. Si se modifican
puerto o timeouts del servicio, coordinar también probes, Service y Gateway.
Gateway mantiene readiness propia; `/_gateway/dependencies` diagnostica los
upstreams sin retirar rutas sanas por la caída de otro dominio.

El rollback de esta refactorización consiste en restaurar una imagen aprobada
de Identidad compatible con el esquema y la configuración existentes, y verificar
sesión, salud y administración. No borrar datos, cambiar grants ni rotar claves
para realizarlo. Los overrides de retorno al monolito retirados en SUP-355
no forman parte del procedimiento actual.

## Pruebas y estructura

Desde `backend/identidad`, con Go 1.26 y compilador C/CGO para el detector de carreras:

```bash
go test -race ./...
go vet ./...
go build ./cmd/identidad
```

| Paquete | Archivos | Rol |
| --- | --- | --- |
| `cmd/identidad` | `main.go` | Arranque, conexiones y apagado |
| `internal/config` | `config.go` | Entorno validado al arrancar, plazos y límites fijos |
| `internal/api` | `rutas.go`, `auth.go`, `admin.go`, `direcciones.go`, `sesion.go`, `limites.go`, `errores.go` | Router y middlewares globales, endpoints, JWT y RBAC, contadores por IP, formato de error |
| `internal/services` | `auth.go`, `admin.go`, `direcciones.go` | Casos de uso y bcrypt |
| `internal/repositories` | `conexion.go`, `usuarios.go`, `direcciones.go` | Conexión, verificación de permisos y SQL |
| `internal/domain` | `usuario.go`, `admin.go`, `direcciones.go`, `repositorios.go` | Entidades, roles, DTO y contratos de repositorio |

Las pruebas siguen los mismos temas: `tests/unit` cubre configuración, sesión,
límites, servicios y handlers con dependencias aisladas; `tests/integration`
cubre el router completo y PostgreSQL y Redis reales opcionales.
`arquitectura_test.go` falla si Identidad importa otro servicio.

`direcciones_postgres_test.go` cubre el repositorio (CRUD, una principal, promoción al
borrar, límite, carreras con goroutines), las restricciones de la migración, los
permisos exactos de `app_identity` y el aislamiento entre dos cuentas a través del router
completo; usa las mismas variables que el panel y se omite sin ellas. En
`tests/unit/direcciones_test.go` el repositorio es un doble en memoria: valida
autenticación, campos, 404 por aislamiento y límite, pero no la concurrencia real.

Las pruebas Google usan el validador real con firmas RSA y una respuesta JWKS
controlada, sin credenciales ni llamadas al proveedor. Ninguna reemplaza una
prueba de Google en navegador.

`admin_postgres_test.go` ejercita el panel contra PostgreSQL real (listado,
cambios auditados, guarda del último admin, `usuarios_rol_check` y permisos de
`app_identity`) y requiere `IDENTIDAD_TEST_DB_URL` (rol `app_identity`) e
`IDENTIDAD_TEST_ADMIN_URL` (operador, sólo para preparar y limpiar);
la prueba de contador Redis requiere `IDENTIDAD_TEST_REDIS_URL`. Sin ellas esos
casos se omiten. Deben apuntar a infraestructura desechable y roles apropiados,
nunca a una base compartida. Un resultado verde con casos omitidos no acredita
esa integración.

Desde la raíz, el harness de integración se inicia con:

```bash
bash scripts/smoke_identity_compose.sh
```

Usa un proyecto Compose, credenciales y recursos temporales, y comprueba sesión,
CORS, rate limiting, errores, probes y aislamiento entre servicios. Ejecutarlo
sin otro harness que compita por sus puertos. Su alcance exacto está en
[el script](../../scripts/smoke_identity_compose.sh) y en
[smoke_identity.py](tests/smoke_identity.py); los nombres históricos SUP-272 y
la etiqueta `rollback` de su resumen no acreditan un rollback al monolito vigente.
Este README describe las comprobaciones disponibles, no afirma una nueva
ejecución del harness ni un despliegue del clúster.
