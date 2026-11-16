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

analizador = argparse.ArgumentParser(description=__doc__)
analizador.add_argument('--project', required=True)
analizador.add_argument('--env-file', required=True)
analizador.add_argument('--gateway-url', required=True)
argumentos = analizador.parse_args()
assert argumentos.project.startswith('sup268-'), 'Sólo el proyecto efímero del harness'
raiz = Path(__file__).resolve().parents[3]
entorno = {nombre_variable: valor_variable for nombre_variable, valor_variable in os.environ.items() if nombre_variable not in {
    'DB_USER', 'DB_PASSWORD', 'DB_NAME', 'JWT_SECRET', 'GOOGLE_CLIENT_ID',
    'IDENTITY_DB_PASSWORD', 'SCRAPING_DB_PASSWORD', 'CATALOGO_DB_PASSWORD',
    'CATALOGO_INTERNAL_TOKEN', 'GATEWAY_TRUSTED_PROXIES', 'IDENTIDAD_GATEWAY_IP',
    'IDENTIDAD_EDGE_SUBNET', 'IDENTIDAD_EDGE_RANGE', 'IDENTIDAD_COOKIE_SECURE'}}
argumentosCompose = ['docker', 'compose', '--project-name', argumentos.project, '--env-file', argumentos.env_file,
                '-f', str(raiz/'docker-compose.yml'), '-f', str(raiz/'docker-compose.catalogo-routing-test.yml')]
verificacionesPasadas = 0
sondeosExitosos = 0


def compose(*comando, exito=True, entrada=None):
    resultado = subprocess.run(argumentosCompose+list(comando), env=entorno, input=entrada, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    assert (resultado.returncode == 0) == exito, resultado.stderr
    return resultado.stdout


def sql(consultaSQL):
    return compose('exec', '-T', 'db', 'psql', '-X', '-U', 'sup268', '-d', 'sup268',
                   '-v', 'ON_ERROR_STOP=1', '-tA', '-c', consultaSQL).strip()


# Control exige el token de servicio que usan worker y bot (SCRAPING_SERVICE_TOKEN).
def scraper_auth(ruta):
    return {'Authorization': 'Bearer ' + os.environ['SCRAPING_SERVICE_TOKEN']} if '/api/v1/scraper' in ruta else {}

def client():
    return urllib.request.build_opener(urllib.request.ProxyHandler({}),
        urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))


def request(ruta, estadoHTTP=200, metodo='GET', datos=None, clienteHTTP=None, cabeceras=None, directo=False):
    global verificacionesPasadas
    origen = 'http://' + compose('port', 'control_scraping', '8080').strip() if directo else argumentos.gateway_url
    peticion = urllib.request.Request(origen+ruta, method=metodo,
        data=json.dumps(datos).encode() if datos is not None else None,
        headers={'Content-Type': 'application/json', **scraper_auth(ruta), **(cabeceras or {})})
    try:
        respuesta = (clienteHTTP or client()).open(peticion, timeout=15)
    except urllib.error.HTTPError as errorHTTP:
        respuesta = errorHTTP
    with respuesta:
        cuerpo = respuesta.read()
        assert respuesta.status == estadoHTTP, (ruta, estadoHTTP, respuesta.status, cuerpo.decode())
        verificacionesPasadas += 1
        return json.loads(cuerpo) if cuerpo else None, respuesta.headers


def state(servicio):
    identificadores = compose('ps', '-q', servicio).split()
    return subprocess.run(['docker', 'inspect', '--format', '{{.Id}} {{.State.StartedAt}} {{.RestartCount}}', *identificadores],
                          check=True, text=True, stdout=subprocess.PIPE).stdout.strip()


def probes(listo=True):
    global sondeosExitosos
    manifiesto = raiz/'infrastructure/k8s/control-scraping/deployment.yaml'
    comandos = [json.loads(linea.split('command:', 1)[1]) for linea in manifiesto.read_text().splitlines()
                if linea.strip().startswith('command:')]
    assert len(comandos) == 3 and sum(comando[-1].endswith('/ready') for comando in comandos) == 1
    for comando in comandos:
        assert comando[-1].startswith('http://127.0.0.1:8080/_scraping/')
        compose('exec', '-T', 'control_scraping', *comando, exito=listo or comando[-1].endswith('/live'))
        sondeosExitosos += 1


clienteAdministrador = client()
request('/api/v1/auth/login', metodo='POST', datos={'correo': 'admin.sup267@example.test',
    'password': 'Sup267FixturePassword123!'}, clienteHTTP=clienteAdministrador)
for ruta in ['/_scraping/live', '/_scraping/ready']:
    request(ruta, directo=True)
    request(ruta, 404)  # Los probes no se publican por Gateway.
request('/api/v1/auth/me', 404, directo=True)
request('/api/v1/productos', 404, directo=True)
request('/api/v1/scrapertools', 404)
cuerpo, _ = request('/_gateway/dependencies')
assert cuerpo['checks']['scraping'] == cuerpo['checks']['scraping_database'] == 'ok'
especificacionSwagger, _ = request('/swagger/doc.json')
assert '/api/v1/scraper/trabajos' in especificacionSwagger['paths']
assert not any(ruta.startswith(('/_', '/api/v1/auth', '/api/v1/productos', '/api/v1/admin/productos')) for ruta in especificacionSwagger['paths'])
probes()

# Rechazar Origin antes de una escritura, conservando cookies/headers del frontend permitido.
for origen in ['https://evil.example', 'null', 'https://frontend.example.evil.test']:
    _, cabeceras = request('/api/v1/scraper/trabajos', 403, metodo='POST', datos={'cadena_id': 1},
                         cabeceras={'Origin': origen})
    assert cabeceras.get('Access-Control-Allow-Origin') is None
for invalido in ('', 'Bearer ' + 'x' * 48, os.environ['SCRAPING_SERVICE_TOKEN']):
    for directo in (False, True):
        _, cabeceras = request('/api/v1/scraper/trabajos', 401, metodo='POST', datos={'cadena_id': 1},
                             cabeceras={'Authorization': invalido}, directo=directo)
        assert cabeceras.get('WWW-Authenticate', '').startswith('Bearer')
_, cabeceras = request('/api/v1/scraper/trabajos', 204, metodo='OPTIONS',
    cabeceras={'Origin': 'https://frontend.example'})
assert cabeceras.get('Access-Control-Allow-Origin') == 'https://frontend.example'
assert 'Idempotency-Key' in cabeceras.get('Access-Control-Allow-Headers', '')

# Caída independiente: sesiones y Catálogo siguen funcionando; no hay fallback.
identificador = compose('ps', '-q', 'control_scraping').strip()
compose('stop', 'control_scraping')
codigoSalida = subprocess.run(['docker', 'inspect', '--format', '{{.State.ExitCode}}', identificador],
                           check=True, text=True, stdout=subprocess.PIPE).stdout.strip()
assert codigoSalida == '0', 'El shutdown de Control no fue limpio'
try:
    cuerpo, _ = request('/api/v1/scraper/trabajos', 502, metodo='POST', datos={'cadena_id': 1})
    assert cuerpo['error'] == 'scraping_unavailable'
    request('/api/v1/auth/me', clienteHTTP=clienteAdministrador)
    request('/api/v1/admin/productos', clienteHTTP=clienteAdministrador)
    request('/_gateway/ready')
finally:
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'control_scraping')

# SQL es readiness; Redis puede degradarse conservando el outbox durable.
estadoPrevio = state('control_scraping')
compose('stop', 'redis')
try:
    probes()
    cuerpo, _ = request('/_scraping/ready', directo=True)
    assert cuerpo['checks']['redis'] == 'error' and cuerpo['checks']['database'] == 'ok'
finally:
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'redis')
probes()
compose('stop', 'db')
try:
    probes(listo=False)
finally:
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'db')
probes()
assert state('control_scraping') == estadoPrevio

# Escalar sólo Control: usar puertos efímeros en las réplicas y restaurar el debug port.
otrosServicios = {servicio: state(servicio) for servicio in ['gateway', 'identidad', 'catalogo']}
with tempfile.TemporaryDirectory(dir=Path(argumentos.env_file).parent) as directorioPrivado:
    configuracionEscala = Path(directorioPrivado)/'scale.yml'
    configuracionEscala.write_text('services:\n  control_scraping:\n    ports: !override ["127.0.0.1::8080"]\n')
    compose('-f', str(configuracionEscala), 'up', '-d', '--no-deps', '--scale', 'control_scraping=3',
            '--wait', '--wait-timeout', '90', 'control_scraping')
    try:
        assert len(compose('ps', '-q', 'control_scraping').split()) == 3
        assert all(state(servicio) == estadoInicial for servicio, estadoInicial in otrosServicios.items())
        trabajo, _ = request('/api/v1/scraper/trabajos', 201, metodo='POST', datos={'cadena_id': 1})
        ruta = '/api/v1/scraper/trabajos/' + trabajo['id']
        claveIdempotencia = str(uuid4())
        sku = 'SUP273-' + uuid4().hex
        carga = {'sucursal_id': 1, 'productos': [{'sku': sku, 'producto': 'Control replicas SUP273',
                   'precio_normal': 1234, 'en_stock': True}]}

        # Alcanzar determinísticamente las tres réplicas en sus puertos efímeros;
        # la confirmación final entra por Gateway. No depender del reparto DNS Docker.
        direccionesReplicas = ['http://' + compose('port', '--index', str(indice), 'control_scraping', '8080').strip()
                        for indice in range(1, 4)]

        def deliver(indice):
            origen = argumentos.gateway_url if indice is None else direccionesReplicas[indice % 3]
            peticion = urllib.request.Request(origen+ruta+'/productos', method='POST',
                headers={'Content-Type': 'application/json', 'Idempotency-Key': claveIdempotencia, **scraper_auth(ruta)},
                data=json.dumps(carga).encode())
            try:
                respuesta = client().open(peticion, timeout=20)
            except urllib.error.HTTPError as errorHTTP:
                respuesta = errorHTTP
            with respuesta:
                assert respuesta.status in (200, 202), (respuesta.status, respuesta.read())
                return respuesta.status

        with concurrent.futures.ThreadPoolExecutor(max_workers=12) as grupoTrabajadores:
            resultados = list(grupoTrabajadores.map(deliver, range(18)))
        verificacionesPasadas += len(resultados)
        plazo = time.monotonic()+30
        while True:
            resultado = deliver(None)
            verificacionesPasadas += 1
            if resultado == 200:
                break
            assert time.monotonic() < plazo, 'El lote no alcanzó confirmación final'
            time.sleep(.2)
        request(ruta+'/finalizar', metodo='PUT', datos={'estado': 'completado'})
        trabajo, _ = request(ruta)
        assert trabajo['elementos_extraidos'] == 1 and trabajo['estado'] == 'completado'
        assert sql("SELECT count(*) FROM scraper.capturas_precios cp JOIN scraper.productos_crudos pc "
                   "ON pc.id=cp.producto_crudo_id WHERE pc.sku='"+sku+"';") == '1'
    finally:
        compose('up', '-d', '--no-deps', '--scale', 'control_scraping=1', '--wait', '--wait-timeout', '90', 'control_scraping')
assert all(state(servicio) == estadoInicial for servicio, estadoInicial in otrosServicios.items())

# Volver a la imagen aprobada SUP-272 sin mover tablas/outboxes ni rotar secretos.
entorno['SUP273_LEGACY_API_IMAGE'] = 'sup272-api:reference'
compose('-f', str(raiz/'docker-compose.scraping-rollback.yml'), 'up', '-d', '--no-deps',
        '--wait', '--wait-timeout', '90', 'control_scraping', 'gateway')
try:
    request('/api/v1/auth/me', clienteHTTP=clienteAdministrador)
    request('/api/v1/admin/productos', clienteHTTP=clienteAdministrador)
    trabajo, _ = request('/api/v1/scraper/trabajos', 201, metodo='POST', datos={'cadena_id': 1})
    request('/api/v1/scraper/trabajos/'+trabajo['id'])
    cuerpo, _ = request('/_gateway/dependencies')
    assert cuerpo['checks']['backend'] == 'ok' and 'scraping' not in cuerpo['checks']
finally:
    compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'control_scraping', 'gateway')
request('/api/v1/auth/me', clienteHTTP=clienteAdministrador)

probes()

identificador = compose('ps', '-q', 'control_scraping').strip()
datosInspeccion = json.loads(subprocess.run(['docker', 'inspect', identificador], check=True,
                       text=True, stdout=subprocess.PIPE).stdout)[0]
nombresVariables = {valor.split('=', 1)[0] for valor in datosInspeccion['Config']['Env']}
assert not nombresVariables.intersection({'JWT_SECRET', 'IDENTITY_DB_URL', 'GOOGLE_CLIENT_ID', 'DB_URL', 'DB_PASSWORD'})
assert datosInspeccion['Config']['User'] == '65532:65532'
assert datosInspeccion['HostConfig']['ReadonlyRootfs'] is True
print(json.dumps({'task': 'SUP-273', 'passed': verificacionesPasadas, 'exec_probe_checks': sondeosExitosos, 'failed': 0,
    'replicas': 'three Control replicas, same key, one price/counter', 'rollback': 'SUP-272 and back'}))
