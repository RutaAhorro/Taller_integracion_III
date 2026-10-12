"""SUP-273: contratos públicos, aislamiento, probes, réplicas y rollback real."""
import argparse
import concurrent.futures
import http.cookiejar
import json
import os
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
assert args.project.startswith('sup268-'), 'Sólo el proyecto efímero del harness'
root = Path(__file__).resolve().parents[3]
env = {k: v for k, v in os.environ.items() if k not in {
    'DB_USER', 'DB_PASSWORD', 'DB_NAME', 'JWT_SECRET', 'GOOGLE_CLIENT_ID',
    'IDENTITY_DB_PASSWORD', 'SCRAPING_DB_PASSWORD', 'CATALOGO_DB_PASSWORD',
    'CATALOGO_INTERNAL_TOKEN', 'GATEWAY_TRUSTED_PROXIES', 'IDENTIDAD_GATEWAY_IP',
    'IDENTIDAD_EDGE_SUBNET', 'IDENTIDAD_EDGE_RANGE', 'IDENTIDAD_COOKIE_SECURE'}}
compose_args = ['docker', 'compose', '--project-name', args.project, '--env-file', args.env_file,
                '-f', str(root/'docker-compose.yml'), '-f', str(root/'docker-compose.catalogo-routing-test.yml')]
passed = 0
probes_passed = 0


def compose(*command, success=True, input=None):
    result = subprocess.run(compose_args+list(command), env=env, input=input, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    assert (result.returncode == 0) == success, result.stderr
    return result.stdout


def sql(statement):
    return compose('exec', '-T', 'db', 'psql', '-X', '-U', 'sup268', '-d', 'sup268',
                   '-v', 'ON_ERROR_STOP=1', '-tA', '-c', statement).strip()


# Control exige el token de servicio que usan worker y bot (SCRAPING_SERVICE_TOKEN).
def scraper_auth(path):
    return {'Authorization': 'Bearer ' + os.environ['SCRAPING_SERVICE_TOKEN']} if '/api/v1/scraper' in path else {}

def client():
    return urllib.request.build_opener(urllib.request.ProxyHandler({}),
        urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))


def request(path, status=200, method='GET', data=None, opener=None, headers=None, direct=False):
    global passed
    base = 'http://' + compose('port', 'control_scraping', '8080').strip() if direct else args.gateway_url
    req = urllib.request.Request(base+path, method=method,
        data=json.dumps(data).encode() if data is not None else None,
        headers={'Content-Type': 'application/json', **scraper_auth(path), **(headers or {})})
    try:
        response = (opener or client()).open(req, timeout=15)
    except urllib.error.HTTPError as e:
        response = e
    with response:
        body = response.read()
        assert response.status == status, (path, status, response.status, body.decode())
        passed += 1
        return json.loads(body) if body else None, response.headers


def state(service):
    ids = compose('ps', '-q', service).split()
    return subprocess.run(['docker', 'inspect', '--format', '{{.Id}} {{.State.StartedAt}} {{.RestartCount}}', *ids],
                          check=True, text=True, stdout=subprocess.PIPE).stdout.strip()


def probes(ready=True):
    global probes_passed
    manifest = root/'infrastructure/k8s/control-scraping/deployment.yaml'
    commands = [json.loads(line.split('command:', 1)[1]) for line in manifest.read_text().splitlines()
                if line.strip().startswith('command:')]
    assert len(commands) == 3 and sum(c[-1].endswith('/ready') for c in commands) == 1
    for command in commands:
        assert command[-1].startswith('http://127.0.0.1:8080/_scraping/')
        compose('exec', '-T', 'control_scraping', *command, success=ready or command[-1].endswith('/live'))
        probes_passed += 1


admin = client()
request('/api/v1/auth/login', method='POST', data={'correo': 'admin.sup267@example.test',
    'password': 'Sup267FixturePassword123!'}, opener=admin)
for path in ['/_scraping/live', '/_scraping/ready']:
    request(path, direct=True)
    request(path, 404)  # Los probes no se publican por Gateway.
request('/api/v1/auth/me', 404, direct=True)
request('/api/v1/productos', 404, direct=True)
request('/api/v1/scrapertools', 404)
body, _ = request('/_gateway/dependencies')
assert body['checks']['scraping'] == body['checks']['scraping_database'] == 'ok'
spec, _ = request('/swagger/doc.json')
assert '/api/v1/scraper/trabajos' in spec['paths']
assert not any(p.startswith(('/_', '/api/v1/auth', '/api/v1/productos', '/api/v1/admin/productos')) for p in spec['paths'])
probes()

# Rechazar Origin antes de una escritura, conservando cookies/headers del frontend permitido.
for origin in ['https://evil.example', 'null', 'https://frontend.example.evil.test']:
    _, headers = request('/api/v1/scraper/trabajos', 403, method='POST', data={'cadena_id': 1},
                         headers={'Origin': origin})
    assert headers.get('Access-Control-Allow-Origin') is None
for invalid in ('', 'Bearer ' + 'x' * 48, os.environ['SCRAPING_SERVICE_TOKEN']):
    for direct in (False, True):
        _, headers = request('/api/v1/scraper/trabajos', 401, method='POST', data={'cadena_id': 1},
                             headers={'Authorization': invalid}, direct=direct)
        assert headers.get('WWW-Authenticate', '').startswith('Bearer')
_, headers = request('/api/v1/scraper/trabajos', 204, method='OPTIONS',
    headers={'Origin': 'https://frontend.example'})
assert headers.get('Access-Control-Allow-Origin') == 'https://frontend.example'
assert 'Idempotency-Key' in headers.get('Access-Control-Allow-Headers', '')

# Caída independiente: sesiones y Catálogo siguen funcionando; no hay fallback.
identifier = compose('ps', '-q', 'control_scraping').strip()
compose('stop', 'control_scraping')
exit_code = subprocess.run(['docker', 'inspect', '--format', '{{.State.ExitCode}}', identifier],
                           check=True, text=True, stdout=subprocess.PIPE).stdout.strip()
assert exit_code == '0', 'El shutdown de Control no fue limpio'
try:
    body, _ = request('/api/v1/scraper/trabajos', 502, method='POST', data={'cadena_id': 1})
    assert body['error'] == 'scraping_unavailable'
    request('/api/v1/auth/me', opener=admin)
    request('/api/v1/admin/productos', opener=admin)
    request('/_gateway/ready')
finally:
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'control_scraping')

# SQL es readiness; Redis puede degradarse conservando el outbox durable.
before = state('control_scraping')
compose('stop', 'redis')
try:
    probes()
    body, _ = request('/_scraping/ready', direct=True)
    assert body['checks']['redis'] == 'error' and body['checks']['database'] == 'ok'
finally:
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'redis')
probes()
compose('stop', 'db')
try:
    probes(ready=False)
finally:
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'db')
probes()
assert state('control_scraping') == before

# Escalar sólo Control: usar puertos efímeros en las réplicas y restaurar el debug port.
others = {service: state(service) for service in ['gateway', 'identidad', 'catalogo']}
with tempfile.TemporaryDirectory(dir=Path(args.env_file).parent) as private_dir:
    scale = Path(private_dir)/'scale.yml'
    scale.write_text('services:\n  control_scraping:\n    ports: !override ["127.0.0.1::8080"]\n')
    compose('-f', str(scale), 'up', '-d', '--no-deps', '--scale', 'control_scraping=3',
            '--wait', '--wait-timeout', '90', 'control_scraping')
    try:
        assert len(compose('ps', '-q', 'control_scraping').split()) == 3
        assert all(state(service) == initial for service, initial in others.items())
        job, _ = request('/api/v1/scraper/trabajos', 201, method='POST', data={'cadena_id': 1})
        path = '/api/v1/scraper/trabajos/' + job['id']
        key = str(uuid4())
        sku = 'SUP273-' + uuid4().hex
        payload = {'sucursal_id': 1, 'productos': [{'sku': sku, 'producto': 'Control replicas SUP273',
                   'precio_normal': 1234, 'en_stock': True}]}

        # Alcanzar determinísticamente las tres réplicas en sus puertos efímeros;
        # la confirmación final entra por Gateway. No depender del reparto DNS Docker.
        replica_urls = ['http://' + compose('port', '--index', str(i), 'control_scraping', '8080').strip()
                        for i in range(1, 4)]

        def deliver(index):
            base = args.gateway_url if index is None else replica_urls[index % 3]
            req = urllib.request.Request(base+path+'/productos', method='POST',
                headers={'Content-Type': 'application/json', 'Idempotency-Key': key, **scraper_auth(path)},
                data=json.dumps(payload).encode())
            try:
                response = client().open(req, timeout=20)
            except urllib.error.HTTPError as e:
                response = e
            with response:
                assert response.status in (200, 202), (response.status, response.read())
                return response.status

        with concurrent.futures.ThreadPoolExecutor(max_workers=12) as pool:
            results = list(pool.map(deliver, range(18)))
        passed += len(results)
        deadline = time.monotonic()+30
        while True:
            result = deliver(None)
            passed += 1
            if result == 200:
                break
            assert time.monotonic() < deadline, 'El lote no alcanzó confirmación final'
            time.sleep(.2)
        request(path+'/finalizar', method='PUT', data={'estado': 'completado'})
        job, _ = request(path)
        assert job['elementos_extraidos'] == 1 and job['estado'] == 'completado'
        assert sql("SELECT count(*) FROM scraper.capturas_precios cp JOIN scraper.productos_crudos pc "
                   "ON pc.id=cp.producto_crudo_id WHERE pc.sku='"+sku+"';") == '1'
    finally:
        compose('up', '-d', '--no-deps', '--scale', 'control_scraping=1', '--wait', '--wait-timeout', '90', 'control_scraping')
assert all(state(service) == initial for service, initial in others.items())

# Volver a la imagen aprobada SUP-272 sin mover tablas/outboxes ni rotar secretos.
env['SUP273_LEGACY_API_IMAGE'] = 'sup272-api:reference'
compose('-f', str(root/'docker-compose.scraping-rollback.yml'), 'up', '-d', '--no-deps',
        '--wait', '--wait-timeout', '90', 'control_scraping', 'gateway')
try:
    request('/api/v1/auth/me', opener=admin)
    request('/api/v1/admin/productos', opener=admin)
    job, _ = request('/api/v1/scraper/trabajos', 201, method='POST', data={'cadena_id': 1})
    request('/api/v1/scraper/trabajos/'+job['id'])
    body, _ = request('/_gateway/dependencies')
    assert body['checks']['backend'] == 'ok' and 'scraping' not in body['checks']
finally:
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'control_scraping', 'gateway')
request('/api/v1/auth/me', opener=admin)
probes()

identifier = compose('ps', '-q', 'control_scraping').strip()
inspected = json.loads(subprocess.run(['docker', 'inspect', identifier], check=True,
                       text=True, stdout=subprocess.PIPE).stdout)[0]
names = {value.split('=', 1)[0] for value in inspected['Config']['Env']}
assert not names.intersection({'JWT_SECRET', 'IDENTITY_DB_URL', 'GOOGLE_CLIENT_ID', 'DB_URL', 'DB_PASSWORD'})
assert inspected['Config']['User'] == '65532:65532'
assert inspected['HostConfig']['ReadonlyRootfs'] is True
print(json.dumps({'task': 'SUP-273', 'passed': passed, 'exec_probe_checks': probes_passed, 'failed': 0,
    'replicas': 'three Control replicas, same key, one price/counter', 'rollback': 'SUP-272 and back'}))
