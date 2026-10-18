# Motor de rutas

Servicio FastAPI dueño de `rutas.*` (decisión de SUP-240, alineada con
`GUIA_BACKEND_GO.md` §4 y `PLAN_IMPLEMENTACION_CAPAS.md`). Hoy expone el CRUD de
listas de compra; la optimización de rutas (OR-Tools + OTP) se agregará sobre
el mismo servicio y el mismo rol de base de datos.

## API (tras el Gateway)

| Método | Ruta | Respuesta |
| --- | --- | --- |
| GET | `/api/v1/listas` | `{"listas": [...]}` del usuario, más recientes primero |
| POST | `/api/v1/listas` | 201 con la lista creada. Cuerpo `{"nombre": "..."}` |
| GET | `/api/v1/listas/{id}` | Lista con `articulos` |
| PUT | `/api/v1/listas/{id}` | Cuerpo con `nombre` y/o `estado` (`activa`, `completada`, `archivada`) |
| DELETE | `/api/v1/listas/{id}` | 204; sus artículos se borran en cascada |

- La sesión sale sólo de la cookie HttpOnly `jwt` emitida por Identidad (HS256,
  `exp` obligatorio, `user_id` UUID). Se ignoran `Authorization`, `X-User-ID` y
  cualquier dueño enviado en el cuerpo.
- Una lista ajena o un id mal formado responden 404, sin revelar si existe.
- Nombre: 1 a 150 caracteres tras quitar caracteres de control y espacios extra.
  Máximo 100 listas por usuario (409 al superarlo).
- Errores con el envoltorio común `{"estado", "mensaje", "detalle"?}`; las
  validaciones nunca repiten el valor recibido.
- Agregar o quitar artículos llega con SUP-241 (`/api/v1/listas/{id}/articulos`).

Probes internos: `/_rutas/live` (sin SQL) y `/_rutas/ready` (ping SQL, 2 s).

## Configuración

| Variable | Requisito |
| --- | --- |
| `RUTAS_DB_URL` | DSN con usuario `app_rutas`, base y contraseña explícitos |
| `JWT_SECRET` | La misma clave de Identidad y Catálogo, al menos 32 bytes |
| `CORS_ALLOWED_ORIGINS` | Orígenes HTTP(S) exactos del frontend, separados por comas |
| `RUTAS_DB_POOL_MAX` | Conexiones máximas por proceso, 5 por defecto (1 a 20) |

Al arrancar valida una vez los grants de `06_propiedad_permisos.sql`; sin ellos
no abre HTTP. No ejecuta migraciones.

## Datos y despliegue

1. Aplicar `infrastructure/db/06_propiedad_permisos.sql` como operador: crea
   `app_rutas` (NOLOGIN), le da CRUD sobre `rutas.listas_compras` y lectura de
   `rutas.articulos_lista_compras`, y retira la FK de listas hacia `api.usuarios`.
   Es idempotente.
2. Activar LOGIN: en volúmenes nuevos, `07_runtime_credentials.sh` lo hace si
   existe `RUTAS_DB_PASSWORD`; en la base existente, `\password app_rutas` en psql.
3. Kubernetes: Secret `rutas-db-secret` con `DB_URL` y `DB_PASSWORD`, luego
   `kubectl apply -f infrastructure/k8s/motor-rutas/` y el Deployment del Gateway
   con `RUTAS_URL=http://motor-rutas:8001`.
4. Compose: definir `RUTAS_DB_PASSWORD` y `RUTAS_URL=http://motor-rutas:8001`
   en `.env`, y levantar `docker compose --profile rutas up -d motor_rutas gateway`.
   Sin `RUTAS_URL`, el Gateway no enruta `/api/v1/listas` (404).

## Pruebas

```bash
pip install -r requirements.txt
python -m unittest discover -s tests -t .
docker build --target test backend/motor_rutas   # lo mismo en la CI
```

`tests/test_repositorio_postgres.py` corre contra una base desechable sólo si
existen `RUTAS_TEST_DB_URL` (app_rutas) y `RUTAS_TEST_ADMIN_URL` (operador):
prueba los grants reales, el aislamiento por usuario y el borrado en cascada.
