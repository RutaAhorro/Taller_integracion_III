# SUP-271: propiedad de datos y permisos del backend

SUP-271 parte del merge aprobado de SUP-270 (`42b6ca4`, PR #97) en
`backend/refactor/sup-271-separar-permisos-datos-vmatus`. Define los roles de
runtime y quita las referencias SQL que cruzaban desde trabajos a otros dominios.
No crea bases nuevas: la separación corresponde al propietario de cada tabla.

## Arquitectura vigente

```mermaid
flowchart LR
    Clientes[Frontend, bot y worker] --> Gateway
    Gateway -->|auth y scraper| API[API: un binario]
    Gateway -->|productos y admin/productos| Catalogo[Catálogo]
    API --> Identidad[Identidad: módulo privado]
    API --> Scraping[Coordinación: módulo privado]
    Identidad -->|app_identity| Usuarios[(api.usuarios)]
    Scraping -->|app_scraping| Trabajos[(Trabajos y outboxes)]
    Scraping -->|HTTP + token interno| Catalogo
    Catalogo -->|app_catalogo| Productos[(Cadenas, sucursales, productos, precios y recibos)]
    Scraping --> Redis[Redis Streams]
    Redis --> Worker[Worker: spool SQLite persistente]
    Worker -->|resultados y cierre HTTP| Gateway
```

Los tres grupos de tablas siguen en PostgreSQL/PostGIS. `api` y `scraper` son
nombres físicos heredados; `scraper` contiene tanto tablas de Catálogo como de
coordinación. Un esquema no equivale automáticamente a un servicio. No se copian
datos ni se introducen vistas compartidas, joins externos o entidades comunes.

Identidad y coordinación conservan el mismo proceso/Deployment de API, con
dos pools GORM separados. El proceso tiene ambas credenciales: esta etapa
impide que cada adaptador use el rol del otro, pero no aísla una vulneración
completa del proceso. SUP-272/273 pueden separar esos procesos si el equipo
decide avanzar. Catálogo ya tiene proceso, imagen, Deployment y HPA propios.

## Matriz de runtime

| Propietario / rol | Tablas | Permisos |
| --- | --- | --- |
| Identidad / `app_identity` | `api.usuarios` | SELECT, INSERT, UPDATE |
| Coordinación / `app_scraping` | `scraper.trabajos_scraper`, `scraper.entregas_ingesta`, `scraper.entregas_trabajos` | SELECT, INSERT, UPDATE; DELETE sólo en `entregas_trabajos` |
| Catálogo / `app_catalogo` | `scraper.productos_crudos` | SELECT, INSERT, UPDATE |
| Catálogo / `app_catalogo` | `scraper.cadenas_supermercado`, `scraper.sucursales_supermercado`, `scraper.capturas_precios`, `scraper.lotes_catalogo` | SELECT, INSERT |
| Gateway y worker | Ninguna tabla PostgreSQL | Sin credencial PostgreSQL |

Sólo Catálogo necesita USAGE en las secuencias de cadenas, sucursales y capturas.
Las capturas y recibos son append-only para el runtime. Sólo `app_scraping` puede
borrar del outbox `scraper.entregas_trabajos` para purgar ejecuciones terminales,
como exige SUP-270. No puede borrar trabajos ni entregas de ingesta. Ningún rol
puede truncar, crear tablas, administrar roles, heredar membresías o migrar el esquema.
La desactivación de un usuario usa UPDATE, no DELETE. Retención y borrado requieren
un procedimiento del operador; no se añaden grants para tareas aún inexistentes.

Los nombres `app_identity`, `app_scraping` y `app_catalogo` quedan reservados
para esta aplicación y son globales al clúster PostgreSQL. Se crean NOLOGIN,
NOINHERIT y sin privilegios de administrador; provisionar la contraseña activa
LOGIN. La migración conserva contraseñas existentes al repetirse. Los objetos
siguen perteneciendo al operador de migraciones, no a estos roles.

El resto de `api.*` (preferencias, perfiles, tarjetas, misiones, catálogo
normalizado, equivalencias, recetas y mapeos IA) y `rutas.*` queda fuera del
runtime implementado. Se conserva con el operador hasta definir sus servicios
y contratos. No se modifica IA conversacional ni motor de rutas ni se conceden
permisos preventivos a los tres roles actuales.

## Referencias entre dominios

- `trabajos_scraper.cadena_id` conserva el ID devuelto por Catálogo mediante
  HTTP al iniciar el trabajo. `cadena_nombre` almacena el nombre en ese momento.
  La migración rellena el nombre de trabajos históricos que conservan su cadena.
  Consultar y finalizar un trabajo usa ese snapshot; sólo un snapshot vacío
  conserva la consulta HTTP auxiliar para compatibilidad histórica.
- `disparado_por_usuario_id` conserva un UUID de auditoría. No acredita sesión,
  existencia actual del usuario ni autorización; las rutas de scraping mantienen
  su contrato actual. No debe usarse para conceder permisos. Una futura ruta
  autenticada debe tomar el actor del JWT validado, no confiar en un UUID del body.
- Se eliminan sólo las FK desde trabajos hacia cadenas y usuarios. Eliminar esos
  registros mediante un operador ya no elimina trabajos ni modifica el actor.
  Se conservan FK internas de Catálogo y de entregas hacia sus propios trabajos.
- Los `cadena_id`/`sucursal_id` de las entregas permanecen referencias/snapshots
  resueltos por HTTP, sin transacción ni joins SQL con Catálogo. Una referencia
  cuyo destino desapareció sigue el manejo de fallos de SUP-270.

Las FK de funcionalidades de IA/rutas todavía no extraídas no se eliminan en
esta tarea. Evitar crear nuevas dependencias de runtime hacia ellas.

## Configuración

API requiere `IDENTITY_DB_URL` con usuario `app_identity` y `SCRAPING_DB_URL`
con usuario `app_scraping`, base y contraseña explícitos. No utiliza `DB_URL`
como fallback. Los errores de configuración/conexión no incluyen el DSN.
Cada pool tiene máximo 10 conexiones, 5 inactivas y vida máxima de 30 minutos.
Al iniciar se verifican migración/tablas y permisos mínimos de ambos módulos.
`/health` conserva `checks.database` e incorpora los dos estados individuales.

Catálogo sigue usando `DB_URL`, inyectado exclusivamente con `app_catalogo`.
Readiness comprueba permisos de sus tablas/secuencias; un lector usado para
comparar contratos no necesita los grants de ingesta. No se ejecuta AutoMigrate.
Con una API y el HPA de Catálogo de hasta cuatro réplicas, reservar hasta
20 + 4×10 = 60 conexiones, además de operadores y otros procesos. Si aumenta
el HPA de API, contar 20 por réplica. Ajustar PostgreSQL/pools con datos de carga.

Compose recibe `IDENTITY_DB_PASSWORD`, `SCRAPING_DB_PASSWORD` y
`CATALOGO_DB_PASSWORD` por `.env` privado. Generar cada valor por separado con
`openssl rand -hex 32`; no reutilizar el de PostgreSQL administrador. El formato
hexadecimal permite construir las URL sin caracteres que necesiten escape.
Sólo DB recibe las tres contraseñas para inicializar roles; API recibe las dos
URL de sus módulos y Catálogo la suya. Gateway/worker no reciben estos valores.

Kubernetes requiere tres Secrets privados, cada uno con claves `DB_URL` y
`DB_PASSWORD` consistentes: `identity-db-secret`, `scraping-db-secret` y
`catalogo-db-secret`. El pod DB usa DB_PASSWORD para inicialización; cada runtime
recibe únicamente las URL que utiliza. No hay valores reales ni contraseñas de
ejemplo nuevas en Git. Crear los Secrets con el procedimiento del equipo o
`kubectl create secret generic <nombre> --from-env-file=<archivo-privado>`.
Las URL deben apuntar al Service DB/base reales y mantener la política TLS del
entorno. No imprimir Secrets ni usar `docker compose config` sin `--quiet` al
diagnosticar credenciales.

## Migración y corte

1. Integrar y verificar SUP-270 antes de desplegar SUP-271. Conservar imágenes
   de rollback, respaldo PostgreSQL y un inventario de permisos/constraints.
   Pausar productores y worker durante el corte de credenciales/migración.
2. Aplicar `05_recuperacion_scraping.sql` si falta. Ejecutar
   `06_propiedad_permisos.sql` con el operador y `psql -X -v ON_ERROR_STOP=1`.
   Tiene transacción, lock timeout de 5 s y statement timeout de 60 s; una
   contención exige reprogramar el corte, no dejar el esquema a medias.
3. La migración crea el snapshot, elimina las dos dependencias externas de
   trabajos y concede únicamente la matriz. Rechaza roles privilegiados,
   membresías, propiedad de objetos y permisos públicos/defaults que crucen
   dominios. Si falla, auditar con el operador; no revoca permisos de otros
   usuarios ni convierte un administrador existente en un rol runtime.
4. Provisionar tres contraseñas distintas. En psql interactivo:

   ```text
   \password app_identity
   \password app_scraping
   \password app_catalogo
   ALTER ROLE app_identity LOGIN;
   ALTER ROLE app_scraping LOGIN;
   ALTER ROLE app_catalogo LOGIN;
   ```

   Para Compose nuevo, `07_runtime_credentials.sh` lo
   realiza usando variables del contenedor DB y SCRAM-SHA-256. En un volumen
   existente, ejecutarlo expresamente después de la migración, con las variables
   nuevas y el operador autorizado. No regenerar roles/contraseñas al arrancar API.
5. Crear/actualizar los Secrets privados antes de actualizar workloads. Incorporar
   los scripts nuevos al ConfigMap `db-init-scripts` para futuros volúmenes.
   Los scripts de init sólo se ejecutan al crear un volumen: actualizar el
   ConfigMap o `.env` no migra ni rota una base existente.
6. Actualizar Catálogo y API con credenciales dedicadas. Verificar readiness,
   registro/login/perfil, consultas públicas y administrativas, inicio/ingesta/
   cierre de trabajo, reintento del mismo lote y ausencia de permisos cruzados.
7. Reactivar worker/productores y observar pendientes SQL/Streams, errores y
   conexiones. Confirmar que los pods nuevos no contienen credenciales del
   administrador. El administrador queda reservado a DB y a operación manual.

Cambiar `.env` o un Secret no rota por sí solo PostgreSQL: coordinar ALTER ROLE
y recreación/rollout de los clientes. No revocar ni borrar los usuarios anteriores
durante este PR; inventariar consumidores restantes y retirar accesos con el
operador después de validar el corte. No reutilizar esos usuarios en runtime nuevo.

## Rollback

SUP-270 conserva la misma ingesta, outboxes, worker y Streams. Para volver a sus
imágenes, pausar productores, detener loops de API y restaurar la configuración
de conexión única que esa imagen espera, con un rol temporal aprobado que reúna
los permisos de Identidad y coordinación, incluido DELETE exclusivamente en
`scraper.entregas_trabajos`. Usar la imagen aprobada `42b6ca4` con los fixes de
cierre, publisher y recuperación. El Catálogo de SUP-270 puede conservar
`app_catalogo`; no necesita usuarios ni trabajos. No conceder ese acceso combinado
a los pods SUP-271 ni ampliar los roles dedicados de esta etapa.

Mantener la columna nueva y las tablas/recibos/volúmenes: los binarios anteriores
ignoran el snapshot extra y no requieren FK para operar. Restaurar las FK sólo
si el equipo decide recuperar las cascadas y tras verificar que no hay referencias
externas huérfanas; esta tarea no las recrea automáticamente ni borra la auditoría.
Volver a SUP-269 o anterior también requiere drenar la recuperación de SUP-270,
como indica `GUIA_SCRAPING_RECUPERACION.md`.

## Validación

```bash
bash scripts/smoke_data_ownership_compose.sh
```

El harness usa los scripts de migración/credenciales reales y roles dedicados,
sin grants alternativos escritos sólo para tests. Comprueba acceso TCP positivo
y denegaciones reales sobre tablas de los tres esquemas, DELETE/TRUNCATE/DDL,
SET ROLE, secuencias y tablas futuras. Repite la migración, preserva contraseñas,
rechaza membresías/permisos públicos (también grants por columna) y verifica trabajos históricos después de
eliminar cadena/usuario, incluso con Catálogo detenido. También ejecuta los
contratos, recuperación y rollbacks heredados. Restaura la imagen de API de
SUP-270 con un rol temporal que sólo reúne usuarios y coordinación, prueba
registro/login/cola/ingesta idempotente/cierre y vuelve a SUP-271 sin ampliar
los tres roles dedicados.

La validación usa bases, redes y volúmenes temporales. No modifica PostgreSQL
local del usuario ni el clúster universitario. La aplicación de la migración,
Secret/ConfigMap y el rollback de credenciales en el clúster requieren validación
del operador durante despliegue; este trabajo no los ejecuta.

Comprobaciones locales completadas:

- 200 resultados Go: 91 unitarios de API, 108 de Catálogo y una integración
  de publisher con PostgreSQL real (omitida en la corrida unitaria y ejecutada
  explícitamente en el harness). Race detector, vet y compilación `CGO_ENABLED=0`
  correctos; 25 pruebas Python aprobadas.
- 352 comprobaciones HTTP: 338 de contratos/recuperación anteriores y 14 de
  propiedad de datos y rollback SUP-271 → SUP-270 → SUP-271.
- 118 comprobaciones de acceso SQL por rol, además de las aserciones de
  migración, conservación de referencias, snapshots e idempotencia.
- Inicialización de un PostgreSQL nuevo con el script 07 no ejecutable (0644),
  como en ConfigMap: roles habilitados y opciones del entrypoint conservadas.
- 20 recursos Kubernetes válidos con esquema estricto 1.32; workflows,
  scripts, archivos Python y diff validados. Recursos temporales eliminados.

La integración local usó binarios nuevos compilados desde fuente sobre imágenes
runtime verificadas y Python instalado. No reconstruyó los builders completos
de producción ni la imagen del worker. CI configura las construcciones de API,
Catálogo, Gateway y frontend; el worker se valida mediante sus pruebas Python.
Las cifras anteriores corresponden a la validación local. Los checks de GitHub
validan el PR por separado. No se publicaron imágenes ni se desplegó en el clúster.
