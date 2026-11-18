# Identidad

Servicio Go/Gin/GORM de cuentas, sesión y administración de usuarios. Es dueño
de `api.usuarios`, de sus tokens de recuperación y de la auditoría administrativa.
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
| POST | `/api/v1/auth/logout` | Expira la cookie `jwt`; responde 204 sin cuerpo |
| GET | `/api/v1/auth/me` | Identidad de la sesión: `user_id`, rol vigente, provider y mensaje; cuenta revalidada en PostgreSQL |
| PUT | `/api/v1/auth/me` | Actualiza el perfil de la cuenta de la sesión |
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
> | `GET /admin/usuarios` aceptaba `offset` (excluyente con `page`) | Sólo `page` y `limit`. Un `offset` recibido se ignora: ya no pagina ni devuelve 400 |
> | `rol=administrador` se aceptaba como alias de `admin` en `PUT /admin/usuarios/{id}` y en el filtro `?rol=` | Responde 400: los únicos roles son `registrado`, `colaborador` y `admin` |
> | Una fila de `api.usuarios` con rol `administrador` se trataba como `admin` | Ya no se reconoce como admin. La migración `08_normalizar_rol_admin_y_auditoria.sql` debe estar aplicada |
> | El registro generaba un UUID en `api.usuarios.token_verificacion` | Las cuentas nuevas lo dejan en `NULL`: no hay flujo que lo consuma. La columna se conserva y ninguna respuesta HTTP lo exponía |
> | `identidad correo-prueba <destinatario>` enviaba un correo de prueba (SUP-189) | El comando se eliminó; el binario sólo arranca el servidor. El envío SMTP se prueba en `tests/integration/mailer_test.go` |
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

Gateway conserva ruta, query, cuerpo, cookie y `Set-Cookie`, y elimina headers
de identidad forjados. Una caída de Identidad responde 502
`identidad_unavailable`; un timeout, 504 `identidad_timeout`, sin fallback a
otro servicio. Toda ruta bajo `/api/v1/auth/` pertenece a Identidad aunque
no exista (404 propio); prefijos parecidos como `/api/v1/authentication` no
se enrutan allí. Sus probes no se publican por Gateway. `/health`,
`/api/v1/health` y `/swagger/*` pertenecen a Control de scraping; no son rutas
de salud ni una especificación agregada de Identidad.

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
| `CORS_ALLOWED_ORIGINS` | Orígenes HTTP(S) exactos separados por comas, sin path ni comodines |
| `IDENTIDAD_COOKIE_SECURE` | `true` por defecto; `false` sólo para HTTP local |
| `IDENTIDAD_TRUSTED_PROXIES` | IP/CIDR de los proxies reales; obligatorio, sin redes universales |
| `PORT` | 8080 por defecto |

No renombrar estas variables ni rotar `JWT_SECRET` sólo en un servicio. Una
rotación invalida las sesiones anteriores y se coordina con todos sus
validadores. Ver la [guía de configuración compartida](../../docs/Guias/GUIA_CONFIGURACION.md).

Plazos, límites por IP, tamaño del pool y límites de recuperación no son
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
| Recuperación | vigencia 30 min, espera 1 min, 5 por hora, 45 s por envío |
| Correo | 30 s por envío SMTP |

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
- Para activar recuperación: `05_tokens_recuperacion.sql` antes de conceder
  sus permisos con `06_propiedad_permisos.sql`.

El rol runtime tiene SELECT/INSERT/UPDATE sobre usuarios y tokens, sin DELETE,
DDL ni acceso a tablas de otros servicios. La ausencia de almacenamiento base
o permisos impide iniciar; la ausencia de la tabla opcional de recuperación
deshabilita esa función con un diagnóstico y deja disponible el login.
No se ejecuta AutoMigrate ni se amplían grants desde la aplicación.

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

Login cuenta intentos fallidos; registro y Google, todas las solicitudes.
Los defaults son 5 fallos de login y 30 solicitudes de registro/Google por IP
en 15 minutos. Las réplicas comparten claves `rate_limit:login:<IP>`,
`rate_limit:register:<IP>` y `rate_limit:google:<IP>`. Lua incrementa y fija
expiración atómicamente, repara TTL ausentes y devuelve el intento de un login
2xx. Redis debe permitir EVAL/EVALSHA, INCR, PTTL, PEXPIRE, EXISTS, GET y DECR.
Sin Redis, estas rutas fallan cerradas con 500 genérico; no se desactiva el
límite ni se reintentan sus escrituras.

## Correo y recuperación

`IDENTIDAD_CORREO_MODO` acepta `smtp`, `log` o `deshabilitado`. Vacío elige SMTP
si hay credenciales; de lo contrario queda deshabilitado. En modo SMTP son
obligatorios `SMTP_HOST`, `SMTP_USER` y `SMTP_PASS`. `SMTP_PORT` vale 587 por
defecto; `SMTP_TLS` vacío elige STARTTLS salvo en 465, donde elige TLS implícito.
`SMTP_FROM` vacío usa `SMTP_USER`. No existe un modo de autenticación sin cifrado.
Los certificados se validan; si no hay STARTTLS no se envían credenciales.

El transporte es síncrono y aplica 30 s cuando el contexto no trae deadline;
si ya lo trae, prevalece ese presupuesto. La recuperación ejecuta cada envío
en segundo plano con un plazo de 45 s. Correo no forma
parte de readiness. El modo `log` imprime enlaces y tokens completos y sólo
se admite para desarrollo; `deshabilitado` devuelve `mailer.ErrDeshabilitado`.

**Cuenta emisora (SUP-188).** Activar la verificación en dos pasos y generar
una App Password dedicada a Identidad (Cuenta de Google → Seguridad →
Contraseñas de aplicaciones). Guardarla sin espacios en el gestor de secretos
del equipo y revocarla en la misma pantalla si se filtra. Gmail limita los
destinatarios diarios por cuenta (del orden de 500; 2000 en Workspace):
confirmar la cuota en la cuenta real.

**Kubernetes.** El Deployment referencia el Secret opcional `identidad-smtp`
para `SMTP_USER`/`SMTP_PASS`; host y puerto vienen del ConfigMap. Crearlo desde
un archivo privado con esas dos líneas, nunca con la clave como argumento:

```bash
kubectl create secret generic identidad-smtp --from-env-file=<archivo-privado>
kubectl rollout restart deployment/identidad
```

El log de arranque `identidad_correo` debe mostrar `"modo":"smtp"`. La
NetworkPolicy sólo restringe entrada: la salida al puerto SMTP depende de la
red del clúster. No escribir contraseñas en Git, issues ni logs.

Un error `smtp conexion`
requiere revisar DNS, conectividad, firewall y plazo; `smtp autenticacion`,
credenciales y permisos del proveedor; `smtp starttls`/`smtp tls`, certificado
y negociación TLS (por ejemplo, un proxy que intercepta). Ningún error por sí
solo demuestra que el clúster bloquee un puerto concreto; ante `smtp conexion`
persistente se puede probar `SMTP_PORT=465`.

`ServicioRecuperacion.Solicitar` normaliza y valida el correo y despacha el
resto en segundo plano, sin esperar consultas ni envío: la respuesta no revela
la existencia de la cuenta. Sólo se atienden cuentas activas y se aplican los
límites por cuenta. No hay cola ni tope de envíos simultáneos: el endpoint que
exponga este servicio (SUP-233) debe llevar contador por IP, como login y registro. El token tiene 256 bits
aleatorios, codificados en 43 caracteres base64url; sólo su SHA-256 se guarda en
`api.tokens_recuperacion`. Emitir uno nuevo invalida los pendientes de esa cuenta.
La vigencia configurada se usa tanto en PostgreSQL como en el texto y HTML del correo.

El enlace usa `IDENTIDAD_URL_RESTABLECER?token=...`. Se activa el servicio interno
cuando están disponibles correo, URL y permisos; los logs indican
`identidad_recuperacion_activa` o `identidad_recuperacion_deshabilitada`.
Esto no acredita endpoints públicos: solicitar recuperación es SUP-233 y
consumir el token/restablecer la contraseña corresponde a SUP-185/SUP-234.
No se presentan esas operaciones como implementadas ni como verificación de correo.

Para quien implemente el consumo (SUP-185/SUP-234): calcular
`security.HashTokenRecuperacion(token)` y marcarlo usado en una sola sentencia,
para que dos peticiones simultáneas no lo consuman dos veces. Sin fila, el token
es inválido, venció o ya se usó: responder lo mismo en los tres casos.

```sql
UPDATE api.tokens_recuperacion SET usado_el = now()
WHERE token_hash = $1 AND usado_el IS NULL AND expira_el > now()
RETURNING usuario_id;
```

`app_identity` ya tiene SELECT/INSERT/UPDATE en esa tabla, sin DELETE. El
frontend (SUP-250) lee `token` del enlace y lo envía al endpoint de restablecimiento.

## Salud y operación

`/_identidad/live` responde sin consultar SQL ni Redis. `/_identidad/ready`
comparte su presupuesto entre Ping SQL y Redis; no repite la auditoría de
permisos. Con los defaults, el arranque tiene 5 s, readiness 2 s y el apagado
15 s. Al apagar, HTTP y los envíos de recuperación en curso comparten el presupuesto de cierre.
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
| `internal/config` | `config.go` | Entorno validado al arrancar, plazos y límites fijos, correo |
| `internal/api` | `rutas.go`, `auth.go`, `admin.go`, `sesion.go`, `limites.go`, `errores.go` | Router y middlewares globales, endpoints, JWT y RBAC, contadores por IP, formato de error |
| `internal/services` | `auth.go`, `admin.go`, `recuperacion.go` | Casos de uso, bcrypt y tokens de recuperación |
| `internal/repositories` | `conexion.go`, `usuarios.go`, `recuperacion.go` | Conexión, verificación de permisos y SQL |
| `internal/domain` | `usuario.go`, `admin.go`, `repositorios.go` | Entidades, roles, DTO y contratos de repositorio |
| `internal/mailer` | `mailer.go` | Transporte SMTP y modo `log` |

Las pruebas siguen los mismos temas: `tests/unit` cubre configuración, sesión,
límites, servicios y handlers con dependencias aisladas; `tests/integration`
cubre el router completo, SMTP local y PostgreSQL y Redis reales opcionales.
`arquitectura_test.go` falla si Identidad importa otro servicio.

Las pruebas SMTP crean certificados efímeros y aíslan las raíces de confianza en
subprocesos mediante la API pública de `x509`; no modifican TLS de producción.
Las pruebas Google usan el validador real con firmas RSA y una respuesta JWKS
controlada, sin credenciales ni llamadas al proveedor. Ninguna reemplaza una
prueba de Google en navegador o la entrega de correo a una bandeja real.

PostgreSQL real requiere `IDENTIDAD_TEST_DB_URL` y `IDENTIDAD_TEST_ADMIN_URL`;
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
