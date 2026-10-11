# Integración de Gateway en desarrollo y pruebas — SUP-265

> SUP-268 activa Catálogo como destino separado y cambia readiness. El procedimiento
> vigente está en [Enrutamiento y despliegue de Catálogo](GUIA_CATALOGO_ENRUTAMIENTO.md).
> Esta guía conserva el contexto de SUP-265; un rollback completo ahora requiere
> también una imagen de API anterior a SUP-268, mediante `SUP268_LEGACY_API_IMAGE`.

## Resultado y alcance

El frontend (Nginx o Vite), bot y worker usan `api:8080`. Ese nombre corresponde a
Gateway, que reenvía al monolito actual. Identidad, catálogo, scraping, JWT,
autorización y rate limiting siguen en la API; esta etapa no migra tablas ni
extrae dominios. El puerto local 8082 permite consultar Gateway directamente y
8080 conserva el acceso de diagnóstico a la API.

| Entorno | Entrada | Destino interno de Gateway |
|---|---|---|
| Docker Compose | Alias `api:8080` → Gateway | `go_service:8080` (alias adicional `api-backend`) |
| Kubernetes | Service `api:8080` → pods Gateway | Service `api-backend:8080` → pods API |

`BACKEND_URL` nunca debe apuntar a `api:8080`: crearía un bucle. El Service
`gateway` permite probar el proxy antes de cambiar el selector del Service `api`.

## Desarrollo local

Requiere Docker con BuildKit y Compose **2.24.4 o superior**, Python 3 y Go 1.26
para pruebas fuera de Docker. Los comandos se ejecutan desde la raíz del repo.
Preparar `.env` desde `.env.example` solo si no existe; completar `DB_USER`,
`DB_PASSWORD`, `DB_NAME` y una clave `JWT_SECRET` local generada con
`openssl rand -hex 32`. Conservar las credenciales de volúmenes ya existentes.
No subir `.env` ni claves a Git. Google OAuth requiere además `GOOGLE_CLIENT_ID`;
no se necesita para las pruebas de registro/login con contraseña.

```bash
docker compose config --quiet
docker compose up -d --build --wait db redis go_service gateway frontend
curl -fsS http://127.0.0.1:8082/_gateway/ready
curl -fsS 'http://127.0.0.1:3000/api/v1/productos?limit=5'
```

Para conservar la IP del cliente en login, configurar `GATEWAY_TRUSTED_PROXIES`
con la IP real del contenedor Nginx/Vite. Puede consultarse con:

```bash
docker inspect frontend_web --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}'
docker compose up -d --force-recreate gateway frontend
```

Actualizar `.env` antes del segundo comando. Si cambia la IP del proxy, actualizar
la lista. No confiar en todo el subnet compartido: bot o worker también viven allí.
Vacío es una configuración conservadora, pero agrupa clientes bajo la IP del
proxy y afecta la cuota de login. Gateway reconstruye X-Forwarded-For y elimina
headers de identidad falsificables. La API conserva la configuración actual de
Gin; restringir sus proxies y el acceso directo al backend forma parte del
endurecimiento posterior, antes de exponer el entorno fuera de desarrollo/prueba.

Para hot reload del frontend usar el target `development` ya documentado en
Compose. El proxy de Vite también apunta a `api:8080`. Bot y worker se levantan
explícitamente cuando se necesita probarlos; no son necesarios para el humo.

## Comprobación aislada y CI

```bash
bash scripts/smoke_gateway_compose.sh
```

El script construye las tres imágenes y crea un proyecto, red y volumen únicos,
con credenciales sintéticas. Copia las semillas SQL con permisos legibles y usa
los puertos de loopback 18080, 18082 y 13000. Se pueden cambiar mediante
`SUP265_BACKEND_PORT`, `SUP265_GATEWAY_PORT` y `SUP265_FRONTEND_PORT`.
No levanta el bot ni spiders reales, ni modifica la base local existente.

Compara respuestas directas y proxificadas: catálogo/búsqueda/detalle, salud,
Swagger, CORS, registro/login/cookies/perfil, permisos, trabajos e ingesta con
fixtures. Comprueba HTML y Nginx → Gateway → API. Después devuelve `api` al
monolito, detiene Gateway y comprueba el frontend y los contratos otra vez.
Elimina sus contenedores, red, volumen y archivos temporales al finalizar,
también ante un fallo. `SUP265_SKIP_BUILD=1` reutiliza imágenes ya construidas.

En redes con proxy de salida, el build acepta los argumentos estándar
`HTTP_PROXY`, `HTTPS_PROXY` y sus variantes en minúscula. El override de prueba
admite `SUP265_BUILD_NETWORK` y `SUP265_CA_BUNDLE` para un bundle público de CA
confiables; se monta como secreto BuildKit `network_ca` durante las descargas,
sin incorporarlo a la imagen final ni desactivar TLS o firmas APK. La resolución
DNS del proxy debe funcionar dentro de BuildKit. En una máquina con Docker
anidado puede requerir un mapping `--add-host` del proxy al construir las imágenes
por separado y luego ejecutar el humo con `SUP265_SKIP_BUILD=1`.

El workflow `Gateway checks` ejecuta pruebas Go con race detector, construcción
de imagen, humo Compose/rollback y validación estricta de los cinco manifiestos
afectados con kubeconform (schema Kubernetes 1.32.0). Este schema de CI no afirma
la versión instalada en la universidad. El workflow de publicación agrega la
imagen `ghcr.io/danirc2024/taller_integracion_iii-gateway` al integrarse en
`develop-refactor`, `develop` o `main`; los PR solo ejecutan comprobaciones.

Validación local de SUP-265 (2026-10-09): pruebas Gateway con race detector y vet
aprobadas (64 resultados, incluyendo subpruebas y la regresión de X-Forwarded-For);
imágenes Gateway, API y frontend
construidas; 23 comprobaciones de contratos y 10 de rollback aprobadas, además de
HTML/ruteo del frontend; cinco manifiestos válidos y workflows validados con
actionlint. El proyecto de prueba se eliminó sin detener los servicios locales.
No se ejecutó GitHub Actions ni se desplegó en el clúster desde esta validación.

El build existente del frontend emite 15 errores TypeScript en componentes que
esta tarea no modifica. Su script actual `tsc -b || true && vite build` continúa
y genera los estáticos. El humo verifica HTML y proxy HTTP; no demuestra que el
frontend pase una comprobación estricta de tipos. Corregir ese problema en una
tarea de frontend independiente, sin desactivar más comprobaciones.

## Activación gradual en el clúster de prueba

No aplicar recursivamente toda la carpeta `infrastructure/k8s`: el selector de
`api` se cambia **al final**. Antes de comenzar, registrar namespace, versión,
selector e imagen de la API actual. Confirmar acceso GHCR a la imagen de Gateway,
configurar los proxies reales (Ingress y Nginx/Vite que llaman a Gateway) en
`infrastructure/k8s/gateway/configmap.yaml` y fijar la imagen a un SHA publicado
o digest probado, en lugar del tag mutable de integración. Usar el namespace
real en todos los comandos (`<namespace>` es un placeholder).

1. Aplicar `infrastructure/k8s/api/backend-service.yaml`: selecciona `app: api`,
   sin cambiar el acceso actual.
2. Aplicar `infrastructure/k8s/gateway/` y esperar `rollout status deployment/gateway`.
   Desde SUP-268 startup/liveness/readiness comprueban la Gateway; el diagnóstico
   de API, PostgreSQL, Redis y Catálogo está en `/_gateway/dependencies`.
3. Probar por `kubectl -n <namespace> port-forward service/gateway 8082:8080`:
   readiness y rutas de lectura; comparar con la API directa. Ejecutar fixtures
   de escritura solo contra una base aislada de prueba.
4. Aplicar **únicamente** `infrastructure/k8s/api/service.yaml` para cambiar
   `selector.app` de `api` a `gateway`. Ingress y URLs de clientes se conservan.
5. Comprobar productos, login/perfil, correlación X-Request-ID, IP del cliente y
   errores desde el frontend/Ingress real. Reiniciar los pods Nginx que hayan
   resuelto `api` previamente si es necesario; el ClusterIP de `api` no cambia.

Gateway parte con una réplica, requests de 50m CPU/64Mi y límites de 200m/128Mi.
Son valores iniciales de prueba, no capacidad garantizada. No se agrega un HPA:
la API sigue siendo un único binario. El escalado de Catálogo pertenece a SUP-268.

## Rollback

En Compose, conservar la misma `.env` y el mismo proyecto/volumen:

```bash
docker compose -f docker-compose.yml -f docker-compose.gateway-rollback.yml up -d --wait go_service gateway frontend
docker compose -f docker-compose.yml -f docker-compose.gateway-rollback.yml up -d --no-deps --force-recreate --wait frontend
docker compose -f docker-compose.yml -f docker-compose.gateway-rollback.yml stop gateway
```

Nginx resuelve el upstream al arrancar; cambiar `depends_on` no obliga a Compose
a recrearlo. Por eso se recrea explícitamente el frontend después de mover el
alias. Recrear también bot/worker con el override si estaban ejecutándose. El alias
`api` vuelve al monolito; los clientes mantienen sus URLs. Para reactivar el proxy,
levantar los mismos servicios con **solo** `docker-compose.yml`, sin el override.
Recrear clientes para que Nginx vuelva a resolver DNS. No borrar el volumen.

En Kubernetes, devolver el selector del Service `api` a la API actual:

```bash
kubectl -n <namespace> patch service api --type merge -p '{"spec":{"selector":{"app":"api"}}}'
```

Comprobar los endpoints del Service y repetir el humo de lectura/frontend antes
de reducir o eliminar Gateway. No hay restauración de datos: no se cambió su
propiedad ni esquema. El rollback Compose se prueba automáticamente; el cambio
y rollback del clúster deben verificarse en la universidad con sus credenciales.

## Trabajo y revisión

Rama: `backend/refactor/sup-265-integrar-gateway-vmatus`. Parte de SUP-264 mientras
su PR está pendiente e incluye la corrección de X-Forwarded-For del commit
`4badb76`, ya publicada en la rama de SUP-264. Antes de abrir el PR hacia `develop-refactor`, incorporar
SUP-264; si se integró mediante squash, reubicar solo los commits de SUP-265 sobre
la integración para evitar duplicar el cambio anterior. El autor abre el PR
manualmente. No hacer commit ni push hasta contar con su autorización.
