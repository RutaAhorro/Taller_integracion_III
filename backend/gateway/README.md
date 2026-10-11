# Gateway — SUP-264 a SUP-268

Servicio Go independiente que enruta productos y administración de productos a
Catálogo, y autenticación/scraping a la API. No incluye reglas de negocio.
La Gateway no tiene conexión PostgreSQL, Redis ni dependencias de negocio.

## Ejecutar

Requiere Go 1.26 y un backend en ejecución. Desde `backend/gateway`:

```bash
BACKEND_URL=http://127.0.0.1:8080 CATALOGO_URL=http://127.0.0.1:8083 go run ./cmd/gateway
```

Escucha por defecto en el puerto 8082. Para validar localmente:

```bash
curl -fsS http://127.0.0.1:8082/_gateway/live
curl -fsS http://127.0.0.1:8082/_gateway/ready
curl -fsS http://127.0.0.1:8082/_gateway/dependencies
curl -fsS http://127.0.0.1:8082/api/v1/health
curl -fsS 'http://127.0.0.1:8082/api/v1/productos?page=1&limit=5'
```

`/_gateway/live` y `/_gateway/ready` comprueban el proceso y el router configurado.
Las rutas `/api/v1/productos` y `/api/v1/admin/productos`, con sus subrutas, se
envían a Catálogo para todos los métodos; el resto se envía a la API. No se
normalizan rutas ni se hace fallback entre destinos. En `/api/v1/health`, verificar también los
campos de PostgreSQL y Redis: un HTTP 200 no demuestra que esas dependencias estén
disponibles.

`/_gateway/dependencies` consulta en paralelo la salud de la API y de Catálogo,
con un límite común y sin seguir redirects. Responde 503 si alguno falla. Este
diagnóstico no es el readiness probe: retirar Gateway porque falla un dominio
impediría acceder también a los otros. Los tres endpoints aceptan GET/HEAD.

## Configuración

| Variable | Valor por defecto | Uso |
|---|---|---|
| `BACKEND_URL` | Obligatoria | Origen HTTP/HTTPS, por ejemplo `http://127.0.0.1:8080`; sin credenciales, prefijo de ruta, query o fragmento |
| `CATALOGO_URL` | Vacío | Origen HTTP/HTTPS de Catálogo; vacío sólo para rollback con una API anterior a SUP-268 que conserve el lector |
| `GATEWAY_LISTEN_ADDR` | `:8082` | Dirección de escucha; usar `127.0.0.1:8082` para restringirla a la máquina local |
| `GATEWAY_REQUEST_TIMEOUT` | `30s` | Tiempo máximo de la solicitud proxificada |
| `GATEWAY_DIAL_TIMEOUT` | `5s` | Límite para conexión y handshake TLS con el backend |
| `GATEWAY_RESPONSE_HEADER_TIMEOUT` | `10s` | Espera de headers después de enviar la solicitud al backend |
| `GATEWAY_READ_HEADER_TIMEOUT` | `5s` | Lectura de headers del cliente |
| `GATEWAY_IDLE_TIMEOUT` | `60s` | Conexión entrante inactiva |
| `GATEWAY_SHUTDOWN_TIMEOUT` | `10s` | Espera de solicitudes activas al recibir SIGINT/SIGTERM |
| `GATEWAY_READINESS_TIMEOUT` | `2s` | Límite conjunto de las consultas de diagnóstico de dependencias |
| `GATEWAY_TRUSTED_PROXIES` | Vacío | IPs/CIDRs de proxies de entrada confiables, separados por coma |

Los timeouts deben ser duraciones positivas de Go, como `500ms` o `30s`. Una
configuración inválida impide el arranque. El servidor también limita la lectura
del request completo al tiempo configurado de solicitud. No apuntar `BACKEND_URL`
a la propia Gateway. Los HTTPS del backend conservan verificación TLS; las
conexiones internas no usan automáticamente el proxy de salida de la máquina.

## Transparencia y seguridad de la entrada

- Conserva método, path codificado, query, cuerpo, Host, códigos y representación
  de respuesta. Propaga cookies, Authorization, CORS, redirects y headers de la
  aplicación. Los headers de salto HTTP se gestionan con `httputil.ReverseProxy`.
- CORS, autenticación, autorización y rate limiting continúan en cada servicio;
  esta tarea no introduce contadores duplicados ni cambia permisos.
- Acepta un `X-Request-ID` de hasta 128 caracteres ASCII alfanuméricos, `.`, `_`
  o `-`; genera uno si falta o es inválido. Lo propaga al backend, respuesta y
  logs JSON. No registra query, cookies, Authorization ni cuerpos.
- Elimina `X-User-ID`, `X-Role` y `X-User-Role` del cliente: no constituyen identidad verificada.
- Reconstruye los headers `X-Forwarded-*`. Sin proxies confiables configurados,
  usa la IP de la conexión y el protocolo real de entrada. Si el peer inmediato
  es confiable, recorre la cadena de IPs desde la derecha hasta el primer salto
  no confiable, para que un cliente no pueda falsificar la primera IP. Se detiene
  también ante un salto inválido, sin saltarlo ni inspeccionar su prefijo. Así,
  `invalid_string, 203.0.113.5` conserva la IP real que anexó el proxy confiable.
- Antes de colocarla detrás de Nginx/Ingress, configurar los CIDRs/IPs reales de
  ese proxy; no confiar en todas las redes. De lo contrario el rate limiter
  existente verá la IP del proxy en vez de la del cliente. El backend también
  debe configurar sus proxies confiables al integrar el despliegue.

Los GET/HEAD/OPTIONS/TRACE reutilizan conexiones al backend. Para los demás
métodos se abre una conexión HTTP/1 por solicitud: se evita que el transporte
reenvíe una escritura ante un fallo de una conexión reutilizada, incluso con
`Idempotency-Key`. Esto agrega costo de conexión a las escrituras. Las pruebas de
integración verifican el comportamiento funcional; el rendimiento bajo carga
debe medirse en el entorno de prueba. La Gateway no implementa reintentos de negocio.

Si no se puede conectar al backend, devuelve HTTP 502 con `backend_unavailable`.
Un timeout antes de empezar la respuesta devuelve HTTP 504 con `backend_timeout`.
Ambos contienen `request_id`, sin detalles internos en la respuesta. Los códigos
de error enviados por el backend se conservan. Una respuesta cuyo cuerpo ya
empezó no puede cambiar a 504: al vencer el plazo se interrumpe la transferencia.
El cierre del cliente cancela la solicitud al backend.

## Pruebas

Desde `backend/gateway`:

```bash
go test -race ./...
go vet ./...
go build ./cmd/gateway
```

Las pruebas usan servidores HTTP locales y cubren transparencia de requests y
respuestas, cookies, estados de error del backend, compresión, redirects, IPs de
proxies confiables, correlación sin credenciales en logs, conexión fallida,
timeout, cancelación, escrituras sin replay, enrutamiento por dominio y diagnóstico
de dependencias separado de readiness.

Desde la raíz, el humo usa Docker Compose y Python 3 estándar:

```bash
bash scripts/smoke_catalogo_routing_compose.sh
```

Utiliza fixtures, credenciales y recursos aislados; los elimina al terminar.
Cubre contratos frente a una API anterior al corte, frontend, cookies, roles,
registro/perfil, trabajos/ingesta, tres réplicas de Catálogo, fallos independientes
y rollback con la imagen anterior. No inicia workers ni spiders reales.

No basta con una salida exitosa de los tests del proxy para afirmar compatibilidad
del backend real. Registrar resultados de ambas suites y del humo por separado.

## Docker, integración y retorno al estado anterior

Desde la raíz del repositorio:

```bash
docker build --target test -t gateway-tests backend/gateway
docker build -t gateway backend/gateway
bash scripts/smoke_gateway_compose.sh
```

La imagen ejecuta un binario estático como UID/GID 65532. Compose y Kubernetes
añaden filesystem de solo lectura, recursos y probes. Gateway no recibe secretos
JWT, acceso Redis ni credenciales PostgreSQL.

Los clientes usan `api:8080`. Desde SUP-268, Gateway reenvía consultas de productos
a `catalogo:8080`; autenticación/scraping van a `go_service:8080` en Compose y
`api-backend:8080` en Kubernetes. API ya no contiene el lector de Catálogo.

La [guía de SUP-268](../../docs/Guias/GUIA_CATALOGO_ENRUTAMIENTO.md)
describe configuración, pruebas aisladas, orden de activación y rollback.
El script prueba también el frontend tras detener Gateway y devolver el alias
`api` a la API original. Los manifiestos no implican un despliegue automático en
el clúster de la universidad.
