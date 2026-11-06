# Base de Datos — Esquemas y Migraciones

Este directorio contiene las definiciones DDL del esquema PostgreSQL (PostGIS) estructurado en 3 esquemas principales (`api`, `scraper`, `rutas`), los datos de seed iniciales y las scripts de migración.

---

## Orden de ejecución en una base nueva

Compose (`docker-compose.yml`) monta esta carpeta en
`/docker-entrypoint-initdb.d`. Kubernetes monta ahí el ConfigMap
`db-init-scripts`, que no está versionado y debe contener estos mismos archivos. Al crear un **volumen
vacío**, PostgreSQL ejecuta los `.sql` y `.sh` **en orden alfabético**, una sola
vez. Sobre una base existente no se ejecuta nada: ver la sección de migraciones
más abajo.

El orden real es este:

| # | Archivo | Qué hace |
| --- | --- | --- |
| 1 | `01_schema_api.sql` | Esquema `api` |
| 2 | `02_schema_scraper.sql` | Esquema `scraper` |
| 3 | `03_indices_optimizacion.sql` | Índices sobre `scraper.productos_crudos` |
| 4 | `03_schema_rutas.sql` | Esquema `rutas` |
| 5 | `04_migracion_nuevas_cadenas.sql` | Cadenas A Cuenta y Cugat |
| 6 | `05_recuperacion_scraping.sql` | Recibos y outboxes de scraping (SUP-270) |
| 7 | `05_tokens_recuperacion.sql` | Tokens de recuperación de contraseña (SUP-184) |
| 8 | `06_propiedad_permisos.sql` | Roles runtime y permisos por tabla |
| 9 | `07_respuestas_bloqueadas.sql` | Columna de respuestas bloqueadas del scraper |
| 10 | `07_runtime_credentials.sh` | Contraseñas de los roles runtime |
| 11 | `08_grants_catalogo_fuentes.sql` | Permiso de Catálogo sobre `esta_activa` (SUP-243) |
| 12 | `08_normalizar_rol_admin_y_auditoria.sql` | Rol `admin` y auditoría (SUP-242) |
| 13 | `09_plan_y_ultimo_acceso.sql` | Plan y último acceso de usuarios (SUP-242) |
| 14 | `init.sql` | Extensiones y **datos iniciales**. Va al final porque `i` se ordena después de los dígitos |

**Números repetidos (`03_`, `05_`, `07_`, `08_`).** Son históricos: se agregaron
scripts en paralelo con el mismo número. No causan errores porque el orden
alfabético es siempre el mismo y respeta las dependencias. No se renombran
porque el código de Identidad, Control y Motor, los tests y las bases ya
desplegadas los identifican por su nombre actual.

**Regla para scripts nuevos:** usar el siguiente número libre (hoy `10_`) y
nunca repetir uno existente. Si el script depende de otro, debe ordenarse
después de él.

**Finales de línea:** `.gitattributes` fuerza LF en `.sh` y `.sql`. Con CRLF,
`07_runtime_credentials.sh` falla dentro del contenedor (`bash\r`).

**Pendiente:** `init.sql` carga datos fijos de productos y precios. Se revisa
en el barrido de Catálogo (SUP-349).

---

## 📁 Estructura de Archivos

Las migraciones recientes de la refactorización se aplican expresamente a
volúmenes existentes, como operador y con `psql -X -v ON_ERROR_STOP=1`:

| Archivo | Uso |
| --- | --- |
| [05_recuperacion_scraping.sql](05_recuperacion_scraping.sql) | Recibos idempotentes de Catálogo y outboxes de coordinación (SUP-270). |
| [05_tokens_recuperacion.sql](05_tokens_recuperacion.sql) | Tokens de restablecimiento de contraseña de Identidad (SUP-184). Aplicar antes de re-ejecutar 06, que concede sus permisos. |
| [06_propiedad_permisos.sql](06_propiedad_permisos.sql) | Roles runtime mínimos, snapshot de cadena y eliminación de FK externas desde trabajos (SUP-271). Desde SUP-240 también `app_rutas` (listas) y la FK de listas hacia usuarios. Idempotente: re-ejecutarlo en la base existente. |
| [07_respuestas_bloqueadas.sql](07_respuestas_bloqueadas.sql) | Agrega la columna `scraper.trabajos_scraper.respuestas_bloqueadas` para el conteo de bloqueos del worker Go. Idempotente; aplicar antes de desplegar el Control que envía el campo. |
| [07_runtime_credentials.sh](07_runtime_credentials.sh) | Inicializar/rotar contraseñas de los roles mediante variables privadas; `RUTAS_DB_PASSWORD` es opcional. No se ejecuta automáticamente sobre un volumen existente. |
| [08_grants_catalogo_fuentes.sql](08_grants_catalogo_fuentes.sql) | `UPDATE` de `app_catalogo` acotado a la columna `esta_activa` de `scraper.cadenas_supermercado` para administrar el flag de fuentes de scraping (SUP-243). El `06` vigente ya lo concede; este script lo reaplica de forma idempotente en bases existentes. |
| [08_normalizar_rol_admin_y_auditoria.sql](08_normalizar_rol_admin_y_auditoria.sql) | Normaliza el alias histórico `administrador` a `admin` y crea `api.auditoria_admin_usuarios` con permisos para Identidad (SUP-242). Aplicar después de 06; el `06` vigente restaura esos grants si se re-ejecuta sobre una base ya migrada. |
| [09_plan_y_ultimo_acceso.sql](09_plan_y_ultimo_acceso.sql) | Agrega `api.usuarios.plan` (`basico`/`premium`) y `ultimo_acceso`, usados por el panel administrativo (SUP-242). Aplicar después de 08. Idempotente; no requiere grants nuevos porque el permiso es de tabla. El DDL y el `VALIDATE CONSTRAINT` corren en transacciones separadas para no retener el `AccessExclusiveLock` durante el escaneo. |

Ver [datos y permisos del backend](../../docs/Guias/GUIA_DATOS_Y_PERMISOS_BACKEND.md)
para orden de corte, Secrets, auditoría y rollback. No usar un rol runtime para
migraciones ni recrear el volumen para aplicar estos scripts.

| Archivo | Descripción | Cuándo usar |
|---|---|---|
| [`init.sql`](init.sql) | Script completo de inicialización con esquemas y datos seed de prueba (Jumbo, Lider, Santa Isabel, A Cuenta, Cugat). | Se ejecuta automáticamente al construir la BD desde cero (`docker-compose up`). |
| [`01_schema_api.sql`](01_schema_api.sql) | Esquema `api.*` (usuarios, catálogo maestro normalizado, recetas, misiones). | Referencia DDL / auto-init. |
| [`02_schema_scraper.sql`](02_schema_scraper.sql) | Esquema `scraper.*` (cadenas, sucursales, trabajos, productos crudos, capturas de precios). | Referencia DDL / auto-init. |
| [`03_schema_rutas.sql`](03_schema_rutas.sql) | Esquema `rutas.*` (listas de compras, optimizaciones y paradas). | Referencia DDL / auto-init. |
| [`03_indices_optimizacion.sql`](03_indices_optimizacion.sql) | Índices adicionales de rendimiento para consultas frecuentes. | Referencia DDL / auto-init. |
| [`04_migracion_nuevas_cadenas.sql`](04_migracion_nuevas_cadenas.sql) | **Script de migración**: Agrega A Cuenta y Cugat a bases de datos en producción o activas. | **Ejecución manual en bases de datos existentes** (ej. clúster Kubernetes UCT). |

---

## 🚀 Guía de Migración para BD Existente (Clúster UCT)

Si la base de datos en el clúster ya fue desplegada previamente y no contiene las cadenas **A Cuenta** y **Cugat**, debes aplicar el script `04_migracion_nuevas_cadenas.sql`.

### 🛡️ Características de la Migración
- **Idempotente**: Puede ejecutarse múltiples veces de forma segura sin duplicar registros (`ON CONFLICT`).
- **Atómica**: Ejecutada dentro de un bloque `BEGIN ... COMMIT`. Si ocurre un error, se realiza rollback automático.
- **Ajuste de Secuencias**: Actualiza las secuencias de `id` para evitar colisiones con la auto-creación de cadenas desde la API Go.

---

## 🛠️ Instrucciones de Ejecución

### Opción A: Desde Pod/Contenedor de PostgreSQL (Kubernetes u Orden de Consola)

```bash
psql -U $DB_USER -d $DB_NAME -f infrastructure/db/04_migracion_nuevas_cadenas.sql
```

### Opción B: Mediante `kubectl exec` hacia el Pod del Clúster UCT

```bash
kubectl exec -i -n <namespace> <pod-postgres> -- psql -U <usuario> -d <bd> < infrastructure/db/04_migracion_nuevas_cadenas.sql
```

### Opción C: Usando Docker Compose en local/servidor

```bash
docker exec -i bd_supermercados psql -U postgres -d bd_supermercados < infrastructure/db/04_migracion_nuevas_cadenas.sql
```

---

## 🔍 Verificación Post-Migración

El script emite automáticamente un listado de confirmación al terminar. También puedes verificar ejecutando:

```sql
SELECT id, nombre, url_sitio_web, esta_activa FROM scraper.cadenas_supermercado ORDER BY id;
```

**Resultado esperado:**
- ID 1: Jumbo
- ID 2: Lider
- ID 3: Unimarc
- ID 4: Santa Isabel
- ID 5: A Cuenta
- ID 6: Cugat

## SUP-270: recuperación y permisos antes del despliegue

Para una base existente, aplicar `05_recuperacion_scraping.sql` mediante el
operador de PostgreSQL **antes** de desplegar las imágenes nuevas. Los scripts
de inicialización de Compose sólo se ejecutan al crear un volumen vacío.

Coordinación necesita SELECT/INSERT/UPDATE en sus dos outboxes y DELETE
**únicamente** en `scraper.entregas_trabajos`, para retirar ejecuciones terminales.
Si la migración ya está aplicada, el fix del PR #97 sólo requiere este permiso
adicional en el rol de coordinación; no agrega nuevas tablas. Catálogo conserva
SELECT/INSERT en `scraper.lotes_catalogo`, sin acceso al ledger de coordinación.

La API verifica tablas y permiso DELETE al arrancar. Si falta SQL o permisos,
impide iniciar con un diagnóstico controlado. Catálogo sigue vivo y devuelve
readiness 503 si falta su tabla; no sirve tráfico hasta completar la migración.
Orden del rollout: SQL y permisos, Catálogo, API, migración de cola, worker.

Ver [procedimiento, retención y rollback](../../docs/Guias/GUIA_SCRAPING_RECUPERACION.md).
