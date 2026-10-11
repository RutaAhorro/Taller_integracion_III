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

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--project', required=True)
parser.add_argument('--env-file', required=True)
parser.add_argument('--gateway-url', required=True)
args = parser.parse_args()
assert args.project.startswith('sup268-'), 'Sólo proyectos efímeros del harness'
root = Path(__file__).resolve().parents[3]
compose_args = ['docker', 'compose', '--project-name', args.project, '--env-file', args.env_file,
                '-f', str(root / 'docker-compose.yml'), '-f', str(root / 'docker-compose.catalogo-routing-test.yml')]
compose_env = {key: value for key, value in os.environ.items() if key not in {
    'DB_USER', 'DB_PASSWORD', 'DB_NAME', 'JWT_SECRET', 'GOOGLE_CLIENT_ID',
    'CATALOGO_CORS_ALLOWED_ORIGINS', 'GATEWAY_TRUSTED_PROXIES',
    'CATALOGO_INTERNAL_TOKEN', 'IDENTITY_DB_PASSWORD', 'SCRAPING_DB_PASSWORD', 'CATALOGO_DB_PASSWORD'}}
http_passed = 0
sql_passed = 0


def compose(*command, input=None, success=True):
    result = subprocess.run(compose_args + list(command), input=input, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=compose_env)
    assert (result.returncode == 0) == success, result.stderr
    return result


def sql(statement):
    return compose('exec', '-T', 'db', 'psql', '-X', '-U', 'sup268', '-d', 'sup268',
                   '-v', 'ON_ERROR_STOP=1', '-tA', '-c', statement).stdout.strip()


def runtime(role, statement, success=True):
    global sql_passed
    credential = {'app_identity': 'IDENTITY_DB_PASSWORD', 'app_scraping': 'SCRAPING_DB_PASSWORD',
                  'app_catalogo': 'CATALOGO_DB_PASSWORD'}[role]
    # La contraseña ya pertenece al contenedor DB de prueba. No se transmite
    # como argumento ni se imprime: psql la lee mediante PGPASSWORD.
    result = compose('exec', '-T', 'db', 'bash', '-c',
                     'export PGPASSWORD="${!1}"; exec psql -X -h 127.0.0.1 -U "$2" -d sup268 -v ON_ERROR_STOP=1 -tA -c "$3"',
                     '--', credential, role, statement, success=success)
    if not success:
        assert 'permission denied' in result.stderr, result.stderr
    sql_passed += 1
    return result.stdout.strip()


def http(method, path, payload=None, status=200, headers=None, base=None):
    global http_passed
    req = urllib.request.Request((base or args.gateway_url) + path, method=method,
                                 data=json.dumps(payload).encode() if payload is not None else None,
                                 headers={'Content-Type': 'application/json', **(headers or {})})
    try:
        response = urllib.request.build_opener(urllib.request.ProxyHandler({})).open(req, timeout=20)
    except urllib.error.HTTPError as error:
        response = error
    body = json.loads(response.read())
    assert response.status == status, (method, path, response.status, body)
    http_passed += 1
    return body


allowed = {
    'app_identity': {'api.usuarios'},
    'app_scraping': {'scraper.trabajos_scraper', 'scraper.entregas_ingesta', 'scraper.entregas_trabajos'},
    'app_catalogo': {'scraper.cadenas_supermercado', 'scraper.sucursales_supermercado',
                    'scraper.productos_crudos', 'scraper.capturas_precios', 'scraper.lotes_catalogo'},
}
tables = sql("SELECT quote_ident(n.nspname)||'.'||quote_ident(c.relname) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname IN ('api','scraper','rutas') AND c.relkind IN ('r','p') ORDER BY 1;").splitlines()
for role, owned in allowed.items():
    assert runtime(role, 'SELECT current_user') == role
    for table in tables:
        runtime(role, 'SELECT count(*) FROM ' + table, success=table in owned)
    for table in owned:
        runtime(role, 'DELETE FROM ' + table + ' WHERE false',
                success=role == 'app_scraping' and table == 'scraper.entregas_trabajos')
        runtime(role, 'TRUNCATE ' + table, success=False)
    runtime(role, 'CREATE TABLE public.sup271_forbidden (id INTEGER)', success=False)
    runtime(role, 'SET ROLE sup268', success=False)
    runtime(role, "SELECT nextval('api.categorias_id_seq')", success=False)
runtime('app_catalogo', 'UPDATE scraper.capturas_precios SET precio_normal=precio_normal WHERE false', success=False)
runtime('app_catalogo', 'UPDATE scraper.lotes_catalogo SET resultado=resultado WHERE false', success=False)
runtime('app_scraping', 'UPDATE api.usuarios SET rol=rol WHERE false', success=False)
runtime('app_identity', 'UPDATE scraper.trabajos_scraper SET estado=estado WHERE false', success=False)
sql('CREATE TABLE api.sup271_future_probe (id INTEGER);')
for role in allowed:
    runtime(role, 'SELECT * FROM api.sup271_future_probe', success=False)
sql('DROP TABLE api.sup271_future_probe;')

assert sql("SELECT count(*) FROM pg_constraint WHERE contype='f' AND conrelid='scraper.trabajos_scraper'::regclass AND confrelid IN ('api.usuarios'::regclass,'scraper.cadenas_supermercado'::regclass);") == '0'
assert sql("SELECT count(*) FROM pg_constraint WHERE contype='f' AND conrelid IN ('scraper.entregas_ingesta'::regclass,'scraper.entregas_trabajos'::regclass) AND confrelid='scraper.trabajos_scraper'::regclass;") == '2'

# Migrar datos históricos y repetir la migración sin cambiar filas/contraseñas.
user_id, old_job_id = (str(uuid4()) for _ in range(2))
chain = sql("INSERT INTO scraper.cadenas_supermercado(nombre) VALUES ('SUP271 Snapshot') RETURNING id;").splitlines()[0]
sql(f"INSERT INTO api.usuarios(id,correo,nombre_completo) VALUES ('{user_id}','sup271.snapshot@example.test','Snapshot');")
job = http('POST', '/api/v1/scraper/trabajos', {'cadena_id': int(chain), 'disparado_por_usuario_id': user_id}, 201)
job_id = job['id']
assert job['cadena_nombre'] == 'SUP271 Snapshot'
sql(f"INSERT INTO scraper.trabajos_scraper(id,cadena_id,disparado_por_usuario_id,estado) VALUES ('{old_job_id}',{chain},'{user_id}','en_progreso');")
before = sql("SELECT md5(string_agg(rolpassword,',' ORDER BY rolname)) FROM pg_authid WHERE rolname IN ('app_identity','app_scraping','app_catalogo');")
migration = (root / 'infrastructure/db/06_propiedad_permisos.sql').read_text()
compose('exec', '-T', 'db', 'psql', '-X', '-U', 'sup268', '-d', 'sup268', '-v', 'ON_ERROR_STOP=1', input=migration)
after = sql("SELECT md5(string_agg(rolpassword,',' ORDER BY rolname)) FROM pg_authid WHERE rolname IN ('app_identity','app_scraping','app_catalogo');")
assert before == after
assert sql(f"SELECT cadena_nombre FROM scraper.trabajos_scraper WHERE id='{old_job_id}';") == 'SUP271 Snapshot'
sql(f"DELETE FROM scraper.cadenas_supermercado WHERE id={chain}; DELETE FROM api.usuarios WHERE id='{user_id}';")
compose('stop', 'catalogo')
try:
    for identifier in (job_id, old_job_id):
        body = http('GET', '/api/v1/scraper/trabajos/' + identifier)
        assert body['cadena_nombre'] == 'SUP271 Snapshot'
        assert sql(f"SELECT disparado_por_usuario_id::text FROM scraper.trabajos_scraper WHERE id='{identifier}';") == user_id
        body = http('PUT', '/api/v1/scraper/trabajos/' + identifier + '/finalizar', {'estado': 'completado'})
        assert body['estado'] == 'completado' and body['cadena_nombre'] == 'SUP271 Snapshot'
finally:
    compose('start', 'catalogo')
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '60', 'catalogo')

# Configuraciones peligrosas deben abortar la migración atómicamente.
sql('CREATE ROLE sup271_parent NOLOGIN; GRANT sup271_parent TO app_identity;')
result = compose('exec', '-T', 'db', 'psql', '-X', '-U', 'sup268', '-d', 'sup268', '-v', 'ON_ERROR_STOP=1', input=migration, success=False)
assert 'no es un rol runtime aislado' in result.stderr
sql('REVOKE sup271_parent FROM app_identity; DROP ROLE sup271_parent;')
sql('GRANT SELECT ON scraper.entregas_trabajos TO PUBLIC;')
result = compose('exec', '-T', 'db', 'psql', '-X', '-U', 'sup268', '-d', 'sup268', '-v', 'ON_ERROR_STOP=1', input=migration, success=False)
assert 'Privilegio efectivo inesperado' in result.stderr
sql('REVOKE SELECT ON scraper.entregas_trabajos FROM PUBLIC;')
sql('GRANT SELECT(correo) ON api.usuarios TO PUBLIC;')
result = compose('exec', '-T', 'db', 'psql', '-X', '-U', 'sup268', '-d', 'sup268', '-v', 'ON_ERROR_STOP=1', input=migration, success=False)
assert 'Privilegio efectivo inesperado' in result.stderr
sql('REVOKE SELECT(correo) ON api.usuarios FROM PUBLIC;')
runtime('app_identity', 'SELECT count(*) FROM api.usuarios')
runtime('app_scraping', 'SELECT count(*) FROM scraper.trabajos_scraper')
runtime('app_catalogo', 'SELECT count(*) FROM scraper.productos_crudos')

health = http('GET', '/health')
assert all(health['checks'][key] == 'ok' for key in ('database', 'identity_database', 'scraping_database'))

# H-01/H-02: probes reales contra los binarios y PostgreSQL del harness.
def catalog_probe(path, status=200):
    global http_passed
    result = compose('exec', '-T', 'catalogo', 'wget', '-S', '-O', '/dev/null',
                     'http://127.0.0.1:8080' + path, success=status == 200)
    assert re.search(r'HTTP/1\.1 ' + str(status) + r'\b', result.stderr), result.stderr
    http_passed += 1


def container_state(service):
    identifier = compose('ps', '-q', service).stdout.strip()
    return subprocess.run(['docker', 'inspect', '--format',
                           '{{.Id}} {{.State.StartedAt}} {{.RestartCount}}', identifier],
                          check=True, text=True, stdout=subprocess.PIPE).stdout.strip()


def wait_for(predicate, timeout=40):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        if predicate():
            return
        time.sleep(.2)
    raise AssertionError('El escenario de probes no terminó dentro del plazo')


api_url = 'http://127.0.0.1:' + os.getenv('SUP268_API_PORT', '18080')
before_api, before_catalog = container_state('go_service'), container_state('catalogo')
compose('stop', 'db')
try:
    health = http('GET', '/health', status=503, base=api_url)
    assert health['status'] == 'error'
    assert all(health['checks'][key] == 'error' for key in
               ('database', 'identity_database', 'scraping_database'))
    http('GET', '/api/v1/health', status=503, base=api_url)
    http('GET', '/_api/live', base=api_url)
    catalog_probe('/_catalogo/ready', 503)
    catalog_probe('/_catalogo/live')
finally:
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'db')
http('GET', '/health', base=api_url)
http('GET', '/_api/live', base=api_url)
catalog_probe('/_catalogo/ready')
assert container_state('go_service') == before_api
assert container_state('catalogo') == before_catalog

# Contar consultas reales en el log PostgreSQL del rol, no en un mock del probe.
# log_statement se activa sólo para Catálogo en esta DB efímera; no para el
# operador de credenciales ni para Identidad. Se restablece antes de continuar.
privilege_query = re.compile(r'\bSELECT\s+has_(?:schema|table|sequence)_privilege\(', re.I)
def privilege_queries():
    return len(privilege_query.findall(compose('logs', '--no-color', 'db').stdout))

initial_queries = privilege_queries()
sql("ALTER ROLE app_catalogo SET log_statement = 'all';")
try:
    compose('restart', 'catalogo')
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'catalogo')
    startup_queries = privilege_queries()
    assert startup_queries - initial_queries == 15, 'validación inicial no ejecutada una vez'
    for _ in range(3):
        catalog_probe('/_catalogo/ready')
    catalog_probe('/_catalogo/live')
    assert privilege_queries() == startup_queries, 'readiness repite las consultas de permisos'
finally:
    sql('ALTER ROLE app_catalogo RESET log_statement;')
    compose('restart', 'catalogo')
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'catalogo')

# El chequeo de arranque no se elimina: un grant ausente impide abrir HTTP.
previous_logs = compose('logs', '--no-color', 'catalogo').stdout
sql('REVOKE INSERT ON scraper.lotes_catalogo FROM app_catalogo;')
try:
    compose('restart', 'catalogo')
    wait_for(lambda: 'catalogo_storage_invalid' in
             compose('logs', '--no-color', 'catalogo').stdout[len(previous_logs):])
    compose('stop', 'catalogo')
finally:
    sql('GRANT INSERT ON scraper.lotes_catalogo TO app_catalogo;')
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'catalogo')
catalog_probe('/_catalogo/ready')

# Rollback real a SUP-270 con una credencial combinada temporal y mínima,
# sin ampliar los roles dedicados ni utilizar al administrador como runtime.
rollback_password = secrets.token_hex(32)
provision = f"""CREATE ROLE sup271_rollback_api LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD '{rollback_password}';
GRANT CONNECT ON DATABASE sup268 TO sup271_rollback_api;
GRANT USAGE ON SCHEMA api,scraper TO sup271_rollback_api;
GRANT SELECT,INSERT,UPDATE ON api.usuarios,scraper.trabajos_scraper,scraper.entregas_ingesta,scraper.entregas_trabajos TO sup271_rollback_api;
GRANT DELETE ON scraper.entregas_trabajos TO sup271_rollback_api;
"""
compose('exec', '-T', 'db', 'psql', '-X', '-U', 'sup268', '-d', 'sup268', '-v', 'ON_ERROR_STOP=1', input=provision)
with tempfile.TemporaryDirectory(dir=Path(args.env_file).parent) as private_dir:
    overlay = Path(private_dir) / 'rollback.yml'
    overlay.write_text('services:\n  go_service:\n    image: sup270-api:local\n    build: !reset null\n    environment:\n      DB_URL: postgres://sup271_rollback_api:' + rollback_password + '@db:5432/sup268?sslmode=disable\n')
    overlay.chmod(0o600)
    compose('-f', str(overlay), 'up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'go_service')
    try:
        http('POST', '/api/v1/auth/register', {'correo': 'sup271.rollback@example.test',
             'password': 'Sup271FixturePassword123!', 'nombre_completo': 'Rollback SUP271'}, 201)
        http('POST', '/api/v1/auth/login', {'correo': 'sup271.rollback@example.test',
             'password': 'Sup271FixturePassword123!'})
        job = http('POST', '/api/v1/scraper/trabajos', {'cadena_id': 1}, 201)
        path = '/api/v1/scraper/trabajos/' + job['id']
        http('POST', path + '/ejecutar', {'spider': 'jumbo_rsc'}, 202)
        batch = {'sucursal_id': 1, 'productos': [{'sku': 'SUP271-ROLLBACK',
                 'producto': 'Rollback permisos SUP271', 'precio_normal': 1700}]}
        key = {'Idempotency-Key': str(uuid4())}
        result = http('POST', path + '/productos', batch, headers=key)
        assert result['precios_registrados'] == 1
        http('POST', path + '/productos', batch, headers=key)
        finished = http('PUT', path + '/finalizar', {'estado': 'completado'})
        assert finished['elementos_extraidos'] == 1
    finally:
        compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'go_service')
sql('DROP OWNED BY sup271_rollback_api; DROP ROLE sup271_rollback_api;')
health = http('GET', '/health')
assert all(health['checks'][key] == 'ok' for key in ('database', 'identity_database', 'scraping_database'))
assert sql("SELECT count(*) FROM scraper.capturas_precios cp JOIN scraper.productos_crudos pc ON pc.id=cp.producto_crudo_id WHERE pc.sku='SUP271-ROLLBACK';") == '1'
print(json.dumps({'task': 'SUP-271', 'passed': http_passed, 'failed': 0,
                  'sql_checks': sql_passed, 'roles': list(allowed), 'migration': 'idempotent',
                  'health': '503 on DB outage; live 200; recovery without restart',
                  'catalog_permission_queries': '15 at startup; 0 per readiness',
                  'rollback': 'SUP-270 and back with restricted credentials'}))
