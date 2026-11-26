# Guía de despliegue del MVP en Kubernetes (`student-vmatus`)

Esta guía describe cómo desplegar el MVP completo de RutaAhorro en el clúster Kubernetes (namespace `student-vmatus`) usando los manifiestos de `infrastructure/k8s/`, las imágenes publicadas en GHCR por `.github/workflows/docker-publish.yml` y los scripts de migración de `infrastructure/db/`.

> [!WARNING]
> **No apliques `infrastructure/k8s/db/secret.yaml` directamente.** El archivo versionado contiene credenciales estáticas históricas; crea o rota `db-secret` directamente en el clúster con `kubectl create secret` usando valores nuevos generados con `openssl rand -hex 32`.

---

## 1. Prerrequisitos y publicación de imágenes

Antes de aplicar los manifiestos, asegúrate de que el workflow `.github/workflows/docker-publish.yml` haya publicado la etiqueta `:develop` (o el tag que vayas a desplegar) de todas las imágenes en `ghcr.io/danirc2024/`:

- `taller_integracion_iii-gateway:develop`
- `taller_integracion_iii-identidad:develop`
- `taller_integracion_iii-catalogo:develop`
- `taller_integracion_iii-control-scraping:develop`
- `taller_integracion_iii-motor-rutas:develop`
- `taller_integracion_iii-ia-conversacional:develop`
- `taller_integracion_iii-frontend:develop`
- `taller_integracion_iii-scraper-go:develop`
- `taller_integracion_iii-bot:develop` (opcional para el MVP)

Define la variable de comodidad para los comandos:

```bash
export NS=student-vmatus
```

---

## 2. Lista completa de Secrets

Genera cada secreto/token independiente con `openssl rand -hex 32` (64 caracteres hexadecimales, sin espacios ni caracteres especiales de URL). **Ningún token compartido debe reutilizar `JWT_SECRET`.**

> [!IMPORTANT]
> Sustituye cada `<...>` por el valor real al ejecutar el comando en tu terminal. Nunca guardes estos valores en git ni en ConfigMaps.

### 2.1. Base de datos y JWT (`db-secret`)

Usado por `db` (`envFrom`) y para `JWT_SECRET` en `identidad`, `catalogo`, `motor-rutas` e `ia-conversacional`.

```bash
kubectl create secret generic db-secret -n "$NS" \
  --from-literal=POSTGRES_USER='<usuario-admin-postgres>' \
  --from-literal=POSTGRES_PASSWORD='<password-admin-postgres>' \
  --from-literal=POSTGRES_DB='supermercados' \
  --from-literal=JWT_SECRET='<jwt-secret-hex-64>'
```

### 2.2. Credenciales PostgreSQL por rol de servicio

Cada microservicio usa su propio rol con privilegio mínimo:

```bash
# Identidad (rol app_identity)
kubectl create secret generic identity-db-secret -n "$NS" \
  --from-literal=DB_PASSWORD='<password-app-identity>' \
  --from-literal=DB_URL='postgres://app_identity:<password-app-identity>@db:5432/supermercados?sslmode=disable'

# Catálogo (rol app_catalogo)
kubectl create secret generic catalogo-db-secret -n "$NS" \
  --from-literal=DB_PASSWORD='<password-app-catalogo>' \
  --from-literal=DB_URL='postgres://app_catalogo:<password-app-catalogo>@db:5432/supermercados?sslmode=disable'

# Control de scraping (rol app_scraping)
kubectl create secret generic scraping-db-secret -n "$NS" \
  --from-literal=DB_PASSWORD='<password-app-scraping>' \
  --from-literal=DB_URL='postgres://app_scraping:<password-app-scraping>@db:5432/supermercados?sslmode=disable'

# Motor de rutas (rol app_rutas)
kubectl create secret generic rutas-db-secret -n "$NS" \
  --from-literal=DB_PASSWORD='<password-app-rutas>' \
  --from-literal=DB_URL='postgres://app_rutas:<password-app-rutas>@db:5432/supermercados?sslmode=disable'
```

### 2.3. Tokens internos entre servicios

```bash
# Catálogo <-> Control (token de ingesta) y Catálogo <-> Motor/IA (consulta-token)
kubectl create secret generic catalogo-internal-secret -n "$NS" \
  --from-literal=token='<catalogo-internal-token-hex-64>' \
  --from-literal=consulta-token='<catalogo-consulta-token-hex-64>'

# Control de scraping <-> Scraper Go / Bot / CronJob
kubectl create secret generic scraping-service-secret -n "$NS" \
  --from-literal=token='<scraping-service-token-hex-64>'
```

### 2.4. IA conversacional (`ia-conversacional-secret`)

Claves de proveedores LLM (nunca en ConfigMap). Se recomienda proveer al menos una (`GEMINI_API_KEY` es el primario; `GROQ_API_KEY` es el respaldo):

```bash
kubectl create secret generic ia-conversacional-secret -n "$NS" \
  --from-literal=GEMINI_API_KEY='<gemini-api-key>' \
  --from-literal=GROQ_API_KEY='<groq-api-key>'
```

### 2.5. Administradores de Identidad (`identidad-admin-secret`, opcional)

Lista de correos separados por coma que obtienen rol `admin` al iniciar sesión con Google (`email_verified=true`). Referenciado con `optional: true`:

```bash
kubectl create secret generic identidad-admin-secret -n "$NS" \
  --from-literal=ADMIN_EMAILS='<admin1@ejemplo.cl,admin2@ejemplo.cl>'
```

### 2.6. Bot de Discord (`bot-secret`, opcional fuera del flujo mínimo MVP)

```bash
kubectl create secret generic bot-secret -n "$NS" \
  --from-literal=token='<discord-bot-token>' \
  --from-literal=channel_id='<discord-channel-id>'
```

---

## 3. Variables nuevas y actualizadas para el MVP

| Servicio | Archivo | Variables clave |
| --- | --- | --- |
| IA conversacional | `infrastructure/k8s/ia-conversacional/configmap.yaml` | `IA_CORS_ALLOWED_ORIGINS`, `CATALOGO_INTERNAL_URL=http://catalogo:8080` |
| IA conversacional | `infrastructure/k8s/ia-conversacional/deployment.yaml` | `JWT_SECRET` (`db-secret`), `CATALOGO_CONSULTA_TOKEN` (`catalogo-internal-secret`, `optional: true`), `GEMINI_API_KEY` y `GROQ_API_KEY` (`ia-conversacional-secret`, `optional: true`) |
| Gateway | `infrastructure/k8s/gateway/configmap.yaml` | `IA_URL=http://ia-conversacional:8002`, `GATEWAY_IA_PREFIXES=/api/v1/chat`, `GATEWAY_RESPONSE_HEADER_TIMEOUT=20s` (además de `RUTAS_URL=http://motor-rutas:8001` en `deployment.yaml` y `GATEWAY_RUTAS_PREFIXES=/api/v1/listas,/api/v1/rutas`) |
| Motor de rutas | `infrastructure/k8s/motor-rutas/configmap.yaml` | `CATALOGO_INTERNAL_URL=http://catalogo:8080`, `RUTAS_DB_POOL_MAX`, `RUTAS_MAX_LISTAS`, `RUTAS_CATALOGO_TIMEOUT`, `RUTAS_RADIO_KM`, `RUTAS_MAX_SUCURSALES`, `RUTAS_MAX_PARADAS`, `RUTAS_MAX_ARTICULOS`, `RUTAS_FACTOR_DISTANCIA`, `RUTAS_UMBRAL_AHORRO` |
| Motor de rutas | `infrastructure/k8s/motor-rutas/deployment.yaml` | `CATALOGO_CONSULTA_TOKEN` (`catalogo-internal-secret`, obligatorio), `RUTAS_DB_URL` (`rutas-db-secret`), `JWT_SECRET` (`db-secret`) |
| Identidad | `infrastructure/k8s/identidad/deployment.yaml` | `GOOGLE_CLIENT_ID` y `ADMIN_EMAILS` (`identidad-admin-secret`, `optional: true`) |
| Catálogo | `infrastructure/k8s/catalogo/configmap.yaml` | `CATALOGO_DISPONIBILIDAD_ANTIGUEDAD_MAXIMA=48h` (ya presente) |

---

## 4. Google Cloud Console y coherencia de `GOOGLE_CLIENT_ID`

Para que el inicio de sesión con Google funcione desde el navegador y sea validado por Identidad:

1. En **Google Cloud Console → APIs y servicios → Credenciales → ID de cliente OAuth 2.0 (Aplicación web)**, configura los siguientes **Orígenes de JavaScript autorizados**:
   - `https://miweb-vmatus.dev.censei.cl`
   - `https://api-vmatus.dev.censei.cl`
2. Verifica que `GOOGLE_CLIENT_ID` en `infrastructure/k8s/identidad/deployment.yaml` y `VITE_GOOGLE_CLIENT_ID` en `infrastructure/k8s/frontend/deployment.yaml` tengan **exactamente el mismo valor** (terminado en `.apps.googleusercontent.com`).
   - En el contenedor de Frontend (`frontend/web/entrypoint.sh`), `VITE_GOOGLE_CLIENT_ID` se inyecta en tiempo de arranque sustituyendo `__VITE_GOOGLE_CLIENT_ID__` en los estáticos servidos por Nginx, por lo que basta con actualizar la variable en el Deployment y reiniciar el pod (no hace falta reconstruir la imagen).
   - Si ambos valores difieren, Identidad rechazará el token de Google por audiencia (`aud`) inválida.

---

## 5. Orden de despliegue

Aplica los recursos respetando las dependencias (persistencia y datos primero, luego microservicios internos, Gateway, Frontend e Ingress):

```bash
# 1. Almacenamiento y ConfigMap de inicialización de BD (solo para volumen nuevo)
kubectl apply -n "$NS" -f infrastructure/k8s/db/pvc.yaml
kubectl apply -n "$NS" -f infrastructure/k8s/db/configmap.yaml
kubectl apply -n "$NS" -f infrastructure/k8s/redis/pvc.yaml

# 2. PostgreSQL/PostGIS y Redis (crear antes los Secrets de la sección 2)
kubectl apply -n "$NS" -f infrastructure/k8s/db/service.yaml
kubectl apply -n "$NS" -f infrastructure/k8s/db/deployment.yaml
kubectl apply -n "$NS" -f infrastructure/k8s/redis/service.yaml
kubectl apply -n "$NS" -f infrastructure/k8s/redis/deployment.yaml
kubectl rollout status deployment/db -n "$NS"
kubectl rollout status deployment/redis -n "$NS"

# 3. Aplicar migraciones SQL si la BD ya existía (ver sección 6)

# 4. Servicios núcleo: Identidad, Catálogo y Control de scraping
kubectl apply -n "$NS" -f infrastructure/k8s/identidad/configmap.yaml \
  -f infrastructure/k8s/identidad/service.yaml \
  -f infrastructure/k8s/identidad/networkpolicy.yaml \
  -f infrastructure/k8s/identidad/deployment.yaml

kubectl apply -n "$NS" -f infrastructure/k8s/catalogo/configmap.yaml \
  -f infrastructure/k8s/catalogo/service.yaml \
  -f infrastructure/k8s/catalogo/deployment.yaml

kubectl apply -n "$NS" -f infrastructure/k8s/control-scraping/configmap.yaml \
  -f infrastructure/k8s/control-scraping/service.yaml \
  -f infrastructure/k8s/control-scraping/networkpolicy.yaml \
  -f infrastructure/k8s/control-scraping/deployment.yaml

kubectl rollout status deployment/identidad -n "$NS"
kubectl rollout status deployment/catalogo -n "$NS"
kubectl rollout status deployment/control-scraping -n "$NS"

# 5. Motor de rutas e IA conversacional (dependen de Catálogo y BD)
# Ambos usan strategy: type: Recreate para actualizarse sin requerir un pod extra bajo la cuota.
kubectl apply -n "$NS" -f infrastructure/k8s/motor-rutas/configmap.yaml \
  -f infrastructure/k8s/motor-rutas/service.yaml \
  -f infrastructure/k8s/motor-rutas/deployment.yaml

kubectl apply -n "$NS" -f infrastructure/k8s/ia-conversacional/configmap.yaml \
  -f infrastructure/k8s/ia-conversacional/service.yaml \
  -f infrastructure/k8s/ia-conversacional/deployment.yaml

kubectl rollout status deployment/motor-rutas -n "$NS"
kubectl rollout status deployment/ia-conversacional -n "$NS"

# 6. Gateway, Frontend e Ingress
kubectl apply -n "$NS" -f infrastructure/k8s/gateway/configmap.yaml \
  -f infrastructure/k8s/gateway/api-service.yaml \
  -f infrastructure/k8s/gateway/deployment.yaml

kubectl apply -n "$NS" -f infrastructure/k8s/frontend/service.yaml \
  -f infrastructure/k8s/frontend/deployment.yaml

kubectl apply -n "$NS" -f infrastructure/k8s/ingress/api-ingress.yaml \
  -f infrastructure/k8s/ingress/frontend-ingress.yaml

kubectl rollout status deployment/api-gateway -n "$NS"
kubectl rollout status deployment/frontend -n "$NS"

# 7. Worker de scraping y opcionales (Bot y CronJob)
kubectl apply -n "$NS" -f infrastructure/k8s/scraper-go/pvc.yaml \
  -f infrastructure/k8s/scraper-go/deployment.yaml
# Opcionales según presupuesto de cuota (ver sección 8):
# kubectl apply -n "$NS" -f infrastructure/k8s/bot/deployment.yaml
# kubectl apply -n "$NS" -f infrastructure/k8s/control-scraping/cronjob-configmap.yaml \
#   -f infrastructure/k8s/control-scraping/cronjob.yaml
```

---

## 6. Migraciones de base de datos en el clúster

Los scripts en `/docker-entrypoint-initdb.d` (montados desde el ConfigMap `db-init-scripts`) **sólo se ejecutan al inicializar un volumen PostgreSQL vacío**. En una base de datos existente, las migraciones deben aplicarse manualmente con el usuario administrador (`POSTGRES_USER`), **nunca** con los roles de aplicación (`app_*`) ni recreando el PVC.

1. Obtén el nombre del pod de PostgreSQL:

   ```bash
   export DB_POD=$(kubectl get pod -n "$NS" -l app=db -o jsonpath='{.items[0].metadata.name}')
   ```

2. Aplica los scripts SQL pendientes en orden estricto (todos son idempotentes y usan `-X -v ON_ERROR_STOP=1` para detenerse al primer error):

   ```bash
   for archivo in \
     infrastructure/db/03_schema_rutas.sql \
     infrastructure/db/04_migracion_nuevas_cadenas.sql \
     infrastructure/db/05_recuperacion_scraping.sql \
     infrastructure/db/05_tokens_recuperacion.sql \
     infrastructure/db/06_propiedad_permisos.sql \
     infrastructure/db/07_respuestas_bloqueadas.sql \
     infrastructure/db/08_grants_catalogo_fuentes.sql \
     infrastructure/db/08_normalizar_rol_admin_y_auditoria.sql \
     infrastructure/db/09_plan_y_ultimo_acceso.sql \
     infrastructure/db/10_restringir_rol_usuarios.sql \
     infrastructure/db/11_articulos_lista_producto.sql \
     infrastructure/db/11_direcciones_usuario.sql \
     infrastructure/db/12_lotes_articulos_idempotencia.sql \
     infrastructure/db/13_actualizar_sucursales_acuenta_cugat.sql \
     infrastructure/db/14_carritos_persistentes.sql
   do
     echo "Aplicando $archivo..."
     kubectl exec -i -n "$NS" "$DB_POD" -- sh -c \
       'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < "$archivo"
   done
   ```

3. Si el rol `app_rutas` es nuevo o necesitas sincronizar contraseñas de roles runtime en una base ya existente, ejecuta el script de credenciales dentro del pod (lee las variables inyectadas por los Secrets del Deployment `db`):

   ```bash
   kubectl exec -i -n "$NS" "$DB_POD" -- bash < infrastructure/db/07_runtime_credentials.sh
   ```

4. Si necesitas regenerar el ConfigMap `db-init-scripts` para un volumen nuevo, revisa `scripts/sync_db_init_configmap.py` y `infrastructure/db/README.md`.

---

## 7. Verificación posterior

### 7.1. Estado de pods y sondas de salud

```bash
kubectl get pods -n "$NS"
kubectl describe resourcequota -n "$NS"
```

Comprueba los endpoints de salud externos a través del Ingress:

```bash
curl -fsS https://api-vmatus.dev.censei.cl/health
curl -fsS https://api-vmatus.dev.censei.cl/_gateway/ready
curl -fsS -I https://miweb-vmatus.dev.censei.cl/
```

Comprueba los endpoints internos de readiness/health (por ejemplo, con `port-forward` o desde un pod temporal):

- Identidad: `/_identidad/ready` (puerto `8080`)
- Catálogo: `/_catalogo/ready` (puerto `8080`)
- Control de scraping: `/_scraping/ready` (puerto `8080`)
- Motor de rutas: `/_rutas/ready` (puerto `8001`)
- IA conversacional: `/health` (puerto `8002`, verifica que `productos_cargados > 0`)

### 7.2. Recorrido funcional del MVP

Con `JWT_SECRET` exportado en tu shell (sin imprimirlo en pantalla), ejecuta el recorrido de diagnóstico contra el Gateway del clúster:

```bash
GATEWAY_URL=https://api-vmatus.dev.censei.cl ./scripts/recorrido_mvp.sh
```

También verifica manualmente desde el navegador en `https://miweb-vmatus.dev.censei.cl`:
1. Inicio de sesión con Google (y acceso al panel de usuarios si tu correo está en `ADMIN_EMAILS`).
2. Búsqueda de un producto con precios por cadena.
3. Creación de lista, agregado de artículos y cálculo de ruta óptima.
4. Consulta en el chat de IA con tarjetas de productos recomendados.

---

## 8. Tabla de recursos contra la cuota del namespace

### Cuota y `LimitRange` del namespace `student-vmatus`

- **ResourceQuota:** `limits.cpu = 2000m`, `limits.memory = 2G` (≈ `1907 MiB`), `pods = 12`.
- **LimitRange por contenedor:** CPU mínima `25m`, máxima `500m`, límite por defecto `100m`; memoria mínima `64Mi`, máxima `512Mi`, límite por defecto `128Mi`.

### Recursos configurados en los manifiestos base

| Servicio | Tipo | Request CPU | Request Mem | Limit CPU | Limit Mem |
| --- | --- | ---: | ---: | ---: | ---: |
| `db` (PostgreSQL/PostGIS) | Deployment | 50m | 64Mi | 250m | 300Mi |
| `redis` | Deployment | 25m | 64Mi | 100m | 100Mi |
| `control-scraping` | Deployment | 50m | 64Mi | 200m | 160Mi |
| `identidad` | Deployment | 50m | 64Mi | 200m | 160Mi |
| `catalogo` | Deployment | 50m | 64Mi | 200m | 160Mi |
| `api-gateway` | Deployment | 30m | 64Mi | 150m | 96Mi |
| `frontend` | Deployment | 30m | 64Mi | 100m | 150Mi |
| `scraper-go` | Deployment | 50m | 64Mi | 500m | 128Mi |
| `bot` (Discord, opcional) | Deployment | 50m | 64Mi | 100m | 128Mi |
| `motor-rutas` | Deployment | 50m | 64Mi | 100m | 128Mi |
| `ia-conversacional` | Deployment | 25m | 96Mi | 100m | 192Mi |
| **Suma 11 Deployments (todo encendido)** | **11 pods** | **460m** | **736Mi** | **2000m** | **1702Mi** |
| **Suma 9 Deployments (sin `bot` ni `scraper-go`)** | **9 pods** | **360m** | **608Mi** | **1400m** | **1446Mi** |
| `scraping-scheduler` (efímero) | CronJob | 50m | 64Mi | 200m | 128Mi |

### Comparación contra la cuota

Estado previo en `student-vmatus` (sin `ia-conversacional` y con `motor-rutas` en 0 réplicas): `1800m` CPU, `1382Mi` memoria y `9` pods. Tras desplegar `motor-rutas` (`100m / 128Mi`) e `ia-conversacional` (`100m / 192Mi`), existen dos escenarios operativos:

| Recurso | Cuota `student-vmatus` | Escenario 1: todo encendido (`11` pods) | Margen (todo encendido) | Escenario 2: `bot` y `scraper-go` en 0 réplicas (`9` pods) | Margen (escenario recomendado) |
| --- | ---: | ---: | ---: | ---: | ---: |
| **Limits CPU** | `2000m` | `2000m` | **`0m` (sin margen)** | `1400m` | **`+600m`** |
| **Limits Memoria** | `2G` (`1907Mi`) | `1702Mi` | `+205Mi` | `1446Mi` | `+461Mi` |
| **Pods** | `12` | `11` | `+1 pod` | `9` | `+3 pods` |

> [!WARNING]
> **Sin margen con los 11 pods encendidos:** con todo activo se consume el **100 % de la cuota de CPU (`2000m / 2000m`)**. Aunque `motor-rutas` e `ia-conversacional` usan `strategy: type: Recreate` para evitar crear un pod extra durante un rollout, cualquier actualización con `RollingUpdate` o la ejecución del CronJob (`200m`) fallará por cuota.
>
> **Recomendación operativa:** apagar `bot` y `scraper-go` después de completar el scraping y encenderlos únicamente cuando se necesiten:
>
> ```bash
> # Apagar tras el scraping (libera 600m CPU, 256Mi memoria y 2 pods)
> kubectl scale deployment/bot deployment/scraper-go --replicas=0 -n "$NS"
>
> # Encender solo cuando se necesite ejecutar scraping o usar el bot
> kubectl scale deployment/bot deployment/scraper-go --replicas=1 -n "$NS"
> ```
