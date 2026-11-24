"""SUP-271: permisos y migración reales, sólo sobre el Compose efímero del harness."""
import argparse
import json
import os
import re
import secrets
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
from pathlib import Path
from uuid import uuid4

analizador = argparse.ArgumentParser(description=__doc__)
analizador.add_argument('--project', required=True)
analizador.add_argument('--env-file', required=True)
analizador.add_argument('--gateway-url', required=True)
argumentos = analizador.parse_args()
assert argumentos.project.startswith('sup268-'), 'Sólo proyectos efímeros del harness'
raiz = Path(__file__).resolve().parents[3]
argumentosCompose = ['docker', 'compose', '--project-name', argumentos.project, '--env-file', argumentos.env_file,
                '-f', str(raiz / 'docker-compose.yml'), '-f', str(raiz / 'docker-compose.catalogo-routing-test.yml')]
entornoCompose = {claveIdempotencia: valor for claveIdempotencia, valor in os.environ.items() if claveIdempotencia not in {
    'DB_USER', 'DB_PASSWORD', 'DB_NAME', 'JWT_SECRET', 'GOOGLE_CLIENT_ID',
    'CATALOGO_CORS_ALLOWED_ORIGINS', 'GATEWAY_TRUSTED_PROXIES',
    'CATALOGO_INTERNAL_TOKEN', 'CATALOGO_CONSULTA_TOKEN', 'IDENTITY_DB_PASSWORD', 'SCRAPING_DB_PASSWORD', 'CATALOGO_DB_PASSWORD', 'IDENTIDAD_EDGE_SUBNET','IDENTIDAD_EDGE_RANGE', 'IDENTIDAD_GATEWAY_IP',
    'IDENTIDAD_CORS_ALLOWED_ORIGINS', 'IDENTIDAD_COOKIE_SECURE'}}
peticionesHTTPExitosas = 0
consultasSQLExitosas = 0


def compose(*comando, entrada=None, exito=True):
    resultado = subprocess.run(argumentosCompose + list(comando), input=entrada, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=entornoCompose)
    assert (resultado.returncode == 0) == exito, resultado.stderr
    return resultado


def sql(consultaSQL):
    return compose('exec', '-T', 'db', 'psql', '-X', '-U', 'sup268', '-d', 'sup268',
                   '-v', 'ON_ERROR_STOP=1', '-tA', '-c', consultaSQL).stdout.strip()


def runtime(rol, consultaSQL, exito=True):
    global consultasSQLExitosas
    nombreVariableCredencial = {'app_identity': 'IDENTITY_DB_PASSWORD', 'app_scraping': 'SCRAPING_DB_PASSWORD',
                  'app_catalogo': 'CATALOGO_DB_PASSWORD'}[rol]
    # La contraseña ya pertenece al contenedor DB de prueba. No se transmite
    # como argumento ni se imprime: psql la lee mediante PGPASSWORD.
    resultado = compose('exec', '-T', 'db', 'bash', '-c',
                     'export PGPASSWORD="${!1}"; exec psql -X -h 127.0.0.1 -U "$2" -d sup268 -v ON_ERROR_STOP=1 -tA -c "$3"',
                     '--', nombreVariableCredencial, rol, consultaSQL, exito=exito)
    if not exito:
        assert 'permission denied' in resultado.stderr, resultado.stderr
    consultasSQLExitosas += 1
    return resultado.stdout.strip()


# Control exige el token de servicio que usan worker y bot (SCRAPING_SERVICE_TOKEN).
def scraper_auth(ruta):
    return {'Authorization': 'Bearer ' + os.environ['SCRAPING_SERVICE_TOKEN']} if '/api/v1/scraper' in ruta else {}

def http(metodo, ruta, carga=None, estadoHTTP=200, cabeceras=None, origen=None):
    global peticionesHTTPExitosas
    peticion = urllib.request.Request((origen or argumentos.gateway_url) + ruta, method=metodo,
                                 data=json.dumps(carga).encode() if carga is not None else None,
                                 headers={'Content-Type': 'application/json', **scraper_auth(ruta), **(cabeceras or {})})
    try:
        respuesta = urllib.request.build_opener(urllib.request.ProxyHandler({})).open(peticion, timeout=20)
    except urllib.error.HTTPError as errorObtenido:
        respuesta = errorObtenido
    cuerpo = json.loads(respuesta.read())
    assert respuesta.status == estadoHTTP, (metodo, ruta, respuesta.status, cuerpo)
    peticionesHTTPExitosas += 1
    return cuerpo


permitidas = {
    # La auditoría administrativa se crea en 08 y sólo la escribe Identidad; las
    # direcciones del usuario (SUP-239, migración 11) también son de Identidad.
    'app_identity': {'api.usuarios', 'api.tokens_recuperacion', 'api.auditoria_admin_usuarios', 'api.direcciones_usuario'},
    'app_scraping': {'scraper.trabajos_scraper', 'scraper.entregas_ingesta', 'scraper.entregas_trabajos'},
    'app_catalogo': {'scraper.cadenas_supermercado', 'scraper.sucursales_supermercado',
                    'scraper.productos_crudos', 'scraper.capturas_precios', 'scraper.lotes_catalogo'},
}
tablas = sql("SELECT quote_ident(n.nspname)||'.'||quote_ident(c.relname) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname IN ('api','scraper','rutas') AND c.relkind IN ('r','p') ORDER BY 1;").splitlines()
for rol, tablasPropias in permitidas.items():
    assert runtime(rol, 'SELECT current_user') == rol
    for tabla in tablas:
        runtime(rol, 'SELECT count(*) FROM ' + tabla, exito=tabla in tablasPropias)
    for tabla in tablasPropias:
        # DELETE sólo donde el dueño lo necesita: purga de entregas (Control) y
        # direcciones que el propio usuario elimina (Identidad).
        runtime(rol, 'DELETE FROM ' + tabla + ' WHERE false',
                exito=(rol == 'app_scraping' and tabla == 'scraper.entregas_trabajos')
                or (rol == 'app_identity' and tabla == 'api.direcciones_usuario'))
        runtime(rol, 'TRUNCATE ' + tabla, exito=False)
    runtime(rol, 'CREATE TABLE public.sup271_forbidden (id INTEGER)', exito=False)
    runtime(rol, 'SET ROLE sup268', exito=False)
    runtime(rol, "SELECT nextval('api.categorias_id_seq')", exito=False)
runtime('app_catalogo', 'UPDATE scraper.capturas_precios SET precio_normal=precio_normal WHERE false', exito=False)
runtime('app_catalogo', 'UPDATE scraper.lotes_catalogo SET resultado=resultado WHERE false', exito=False)
runtime('app_scraping', 'UPDATE api.usuarios SET rol=rol WHERE false', exito=False)
runtime('app_identity', 'UPDATE scraper.trabajos_scraper SET estado=estado WHERE false', exito=False)
sql('CREATE TABLE api.sup271_future_probe (id INTEGER);')
for rol in permitidas:
    runtime(rol, 'SELECT * FROM api.sup271_future_probe', exito=False)
sql('DROP TABLE api.sup271_future_probe;')

assert sql("SELECT count(*) FROM pg_constraint WHERE contype='f' AND conrelid='scraper.trabajos_scraper'::regclass AND confrelid IN ('api.usuarios'::regclass,'scraper.cadenas_supermercado'::regclass);") == '0'
assert sql("SELECT count(*) FROM pg_constraint WHERE contype='f' AND conrelid IN ('scraper.entregas_ingesta'::regclass,'scraper.entregas_trabajos'::regclass) AND confrelid='scraper.trabajos_scraper'::regclass;") == '2'

# Migrar datos históricos y repetir la migración sin cambiar filas/contraseñas.
usuarioID, trabajoAnteriorID = (str(uuid4()) for _ in range(2))
cadena = sql("INSERT INTO scraper.cadenas_supermercado(nombre) VALUES ('SUP271 Snapshot') RETURNING id;").splitlines()[0]
sql(f"INSERT INTO api.usuarios(id,correo,nombre_completo) VALUES ('{usuarioID}','sup271.snapshot@example.test','Snapshot');")
trabajo = http('POST', '/api/v1/scraper/trabajos', {'cadena_id': int(cadena), 'disparado_por_usuario_id': usuarioID}, 201)
trabajoID = trabajo['id']
assert trabajo['cadena_nombre'] == 'SUP271 Snapshot'
sql(f"INSERT INTO scraper.trabajos_scraper(id,cadena_id,disparado_por_usuario_id,estado) VALUES ('{trabajoAnteriorID}',{cadena},'{usuarioID}','en_progreso');")
estadoPrevio = sql("SELECT md5(string_agg(rolpassword,',' ORDER BY rolname)) FROM pg_authid WHERE rolname IN ('app_identity','app_scraping','app_catalogo');")
migracion = (raiz / 'infrastructure/db/06_propiedad_permisos.sql').read_text()
compose('exec', '-T', 'db', 'psql', '-X', '-U', 'sup268', '-d', 'sup268', '-v', 'ON_ERROR_STOP=1', entrada=migracion)
estadoPosterior = sql("SELECT md5(string_agg(rolpassword,',' ORDER BY rolname)) FROM pg_authid WHERE rolname IN ('app_identity','app_scraping','app_catalogo');")
assert estadoPrevio == estadoPosterior
assert sql(f"SELECT cadena_nombre FROM scraper.trabajos_scraper WHERE id='{trabajoAnteriorID}';") == 'SUP271 Snapshot'
sql(f"DELETE FROM scraper.cadenas_supermercado WHERE id={cadena}; DELETE FROM api.usuarios WHERE id='{usuarioID}';")
compose('stop', 'catalogo')
try:
    for identificador in (trabajoID, trabajoAnteriorID):
        cuerpo = http('GET', '/api/v1/scraper/trabajos/' + identificador)
        assert cuerpo['cadena_nombre'] == 'SUP271 Snapshot'
        assert sql(f"SELECT disparado_por_usuario_id::text FROM scraper.trabajos_scraper WHERE id='{identificador}';") == usuarioID
        cuerpo = http('PUT', '/api/v1/scraper/trabajos/' + identificador + '/finalizar', {'estado': 'completado'})
        assert cuerpo['estado'] == 'completado' and cuerpo['cadena_nombre'] == 'SUP271 Snapshot'
finally:
    compose('start', 'catalogo')
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '60', 'catalogo')

# Configuraciones peligrosas deben abortar la migración atómicamente.
sql('CREATE ROLE sup271_parent NOLOGIN; GRANT sup271_parent TO app_identity;')
resultado = compose('exec', '-T', 'db', 'psql', '-X', '-U', 'sup268', '-d', 'sup268', '-v', 'ON_ERROR_STOP=1', entrada=migracion, exito=False)
assert 'no es un rol runtime aislado' in resultado.stderr
sql('REVOKE sup271_parent FROM app_identity; DROP ROLE sup271_parent;')
sql('GRANT SELECT ON scraper.entregas_trabajos TO PUBLIC;')
resultado = compose('exec', '-T', 'db', 'psql', '-X', '-U', 'sup268', '-d', 'sup268', '-v', 'ON_ERROR_STOP=1', entrada=migracion, exito=False)
assert 'Privilegio efectivo inesperado' in resultado.stderr
sql('REVOKE SELECT ON scraper.entregas_trabajos FROM PUBLIC;')
sql('GRANT SELECT(correo) ON api.usuarios TO PUBLIC;')
resultado = compose('exec', '-T', 'db', 'psql', '-X', '-U', 'sup268', '-d', 'sup268', '-v', 'ON_ERROR_STOP=1', entrada=migracion, exito=False)
assert 'Privilegio efectivo inesperado' in resultado.stderr
sql('REVOKE SELECT(correo) ON api.usuarios FROM PUBLIC;')
runtime('app_identity', 'SELECT count(*) FROM api.usuarios')
runtime('app_scraping', 'SELECT count(*) FROM scraper.trabajos_scraper')
runtime('app_catalogo', 'SELECT count(*) FROM scraper.productos_crudos')

salud = http('GET', '/health')
assert all(salud['checks'][claveIdempotencia] == 'ok' for claveIdempotencia in ('database', 'scraping_database'))

# H-01/H-02: probes reales contra los binarios y PostgreSQL del harness.
def catalog_probe(ruta, estadoHTTP=200):
    global peticionesHTTPExitosas
    resultado = compose('exec', '-T', 'catalogo', 'wget', '-S', '-O', '/dev/null',
                     'http://127.0.0.1:8080' + ruta, exito=estadoHTTP == 200)
    assert re.search(r'HTTP/1\.1 ' + str(estadoHTTP) + r'\b', resultado.stderr), resultado.stderr
    peticionesHTTPExitosas += 1


def container_state(servicio):
    identificador = compose('ps', '-q', servicio).stdout.strip()
    return subprocess.run(['docker', 'inspect', '--format',
                           '{{.Id}} {{.State.StartedAt}} {{.RestartCount}}', identificador],
                          check=True, text=True, stdout=subprocess.PIPE).stdout.strip()


def wait_for(condicion, tiempoEspera=40):
    fin = time.monotonic() + tiempoEspera
    while time.monotonic() < fin:
        if condicion():
            return
        time.sleep(.2)
    raise AssertionError('El escenario de probes no terminó dentro del plazo')


direccionAPI = 'http://127.0.0.1:' + os.getenv('SUP268_API_PORT', '18080')
estadoInicialAPI, estadoInicialCatalogo = container_state('control_scraping'), container_state('catalogo')
estadoInicialIdentidad = container_state('identidad')
compose('stop', 'db')
try:
    salud = http('GET', '/health', estadoHTTP=503, origen=direccionAPI)
    assert salud['status'] == 'error'
    assert all(salud['checks'][claveIdempotencia] == 'error' for claveIdempotencia in
               ('database', 'scraping_database'))
    http('GET', '/api/v1/health', estadoHTTP=503, origen=direccionAPI)
    http('GET', '/_api/live', origen=direccionAPI)
    catalog_probe('/_catalogo/ready', 503)
    catalog_probe('/_catalogo/live')
    direccionIdentidad = 'http://' + compose('port', 'identidad', '8080').stdout.strip()
    http('GET', '/_identidad/ready', estadoHTTP=503, origen=direccionIdentidad)
    http('GET', '/_identidad/live', origen=direccionIdentidad)
finally:
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'db')
http('GET', '/health', origen=direccionAPI)
http('GET', '/_api/live', origen=direccionAPI)
catalog_probe('/_catalogo/ready')
assert container_state('control_scraping') == estadoInicialAPI
assert container_state('catalogo') == estadoInicialCatalogo
assert container_state('identidad') == estadoInicialIdentidad
http('GET', '/_identidad/ready', origen=direccionIdentidad)

# Contar consultas reales en el log PostgreSQL del rol, no en un mock del probe.
# log_statement se activa sólo para Catálogo en esta DB efímera; no para el
# operador de credenciales ni para Identidad. Se restablece antes de continuar.
consultaPermisos = re.compile(r'\bSELECT\s+has_(?:schema|table|sequence|column)_privilege\(', re.I)
def privilege_queries():
    return len(consultaPermisos.findall(compose('logs', '--no-color', 'db').stdout))

consultasIniciales = privilege_queries()
sql("ALTER ROLE app_catalogo SET log_statement = 'all';")
try:
    compose('restart', 'catalogo')
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'catalogo')
    consultasArranque = privilege_queries()
    # 1 esquema + 5 SELECT + 5 INSERT + 2 UPDATE (tabla y columna) + 3 secuencias (SUP-243).
    assert consultasArranque - consultasIniciales == 16, 'validación inicial no ejecutada una vez'
    for _ in range(3):
        catalog_probe('/_catalogo/ready')
    catalog_probe('/_catalogo/live')
    assert privilege_queries() == consultasArranque, 'readiness repite las consultas de permisos'
finally:
    sql('ALTER ROLE app_catalogo RESET log_statement;')
    compose('restart', 'catalogo')
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'catalogo')

# El chequeo de arranque no se elimina: un grant ausente impide abrir HTTP.
registrosPrevios = compose('logs', '--no-color', 'catalogo').stdout
sql('REVOKE INSERT ON scraper.lotes_catalogo FROM app_catalogo;')
try:
    compose('restart', 'catalogo')
    wait_for(lambda: 'catalogo_storage_invalid' in
             compose('logs', '--no-color', 'catalogo').stdout[len(registrosPrevios):])
    compose('stop', 'catalogo')
finally:
    sql('GRANT INSERT ON scraper.lotes_catalogo TO app_catalogo;')
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'catalogo')
catalog_probe('/_catalogo/ready')

# Rollback real a SUP-270 con una credencial combinada temporal y mínima,
# sin ampliar los roles dedicados ni utilizar al administrador como runtime.
contrasenaRollback = secrets.token_hex(32)
sqlProvisionamiento = f"""CREATE ROLE sup271_rollback_api LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD '{contrasenaRollback}';
GRANT CONNECT ON DATABASE sup268 TO sup271_rollback_api;
GRANT USAGE ON SCHEMA api,scraper TO sup271_rollback_api;
GRANT SELECT,INSERT,UPDATE ON api.usuarios,scraper.trabajos_scraper,scraper.entregas_ingesta,scraper.entregas_trabajos TO sup271_rollback_api;
GRANT DELETE ON scraper.entregas_trabajos TO sup271_rollback_api;
"""
compose('exec', '-T', 'db', 'psql', '-X', '-U', 'sup268', '-d', 'sup268', '-v', 'ON_ERROR_STOP=1', entrada=sqlProvisionamiento)
with tempfile.TemporaryDirectory(dir=Path(argumentos.env_file).parent) as directorioPrivado:
    archivoAnulacion = Path(directorioPrivado) / 'rollback.yml'
    archivoAnulacion.write_text('services:\n  control_scraping:\n    healthcheck:\n      test: [CMD, wget, "-q", "--spider", "http://127.0.0.1:8080/health"]\n    image: sup270-api:local\n    build: !reset null\n    environment:\n      JWT_SECRET: ${JWT_SECRET:?}\n      DB_URL: postgres://sup271_rollback_api:' + contrasenaRollback + '@db:5432/sup268?sslmode=disable\n')
    archivoAnulacion.chmod(0o600)
    compose('-f', str(archivoAnulacion), 'up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'control_scraping')
    try:
        http('POST', '/api/v1/auth/register', {'correo': 'sup271.rollback@example.test',
             'password': 'Sup271FixturePassword123!', 'nombre_completo': 'Rollback SUP271'}, 201)
        http('POST', '/api/v1/auth/login', {'correo': 'sup271.rollback@example.test',
             'password': 'Sup271FixturePassword123!'})
        trabajo = http('POST', '/api/v1/scraper/trabajos', {'cadena_id': 1}, 201)
        ruta = '/api/v1/scraper/trabajos/' + trabajo['id']
        http('POST', ruta + '/ejecutar', {'spider': 'jumbo_rsc'}, 202)
        lote = {'sucursal_id': 1, 'productos': [{'sku': 'SUP271-ROLLBACK',
                 'producto': 'Rollback permisos SUP271', 'precio_normal': 1700}]}
        claveIdempotencia = {'Idempotency-Key': str(uuid4())}
        resultado = http('POST', ruta + '/productos', lote, cabeceras=claveIdempotencia)
        assert resultado['precios_registrados'] == 1
        http('POST', ruta + '/productos', lote, cabeceras=claveIdempotencia)
        trabajoFinalizado = http('PUT', ruta + '/finalizar', {'estado': 'completado'})
        assert trabajoFinalizado['elementos_extraidos'] == 1
    finally:
        compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'control_scraping')
sql('DROP OWNED BY sup271_rollback_api; DROP ROLE sup271_rollback_api;')
salud = http('GET', '/health')
assert all(salud['checks'][claveIdempotencia] == 'ok' for claveIdempotencia in ('database', 'scraping_database'))
assert sql("SELECT count(*) FROM scraper.capturas_precios cp JOIN scraper.productos_crudos pc ON pc.id=cp.producto_crudo_id WHERE pc.sku='SUP271-ROLLBACK';") == '1'
print(json.dumps({'task': 'SUP-271', 'passed': peticionesHTTPExitosas, 'failed': 0,
                  'sql_checks': consultasSQLExitosas, 'roles': list(permitidas), 'migration': 'idempotent',
                  'health': '503 on DB outage; live 200; recovery without restart',
                  'catalog_permission_queries': '16 at startup; 0 per readiness',
                  'rollback': 'SUP-270 and back with restricted credentials'}))
