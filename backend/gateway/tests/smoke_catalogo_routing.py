"""SUP-268: rutas públicas, fallos independientes, carga y rollback aislados."""
import argparse
import concurrent.futures
import http.cookiejar
import json
import urllib.error
import urllib.request
import uuid

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--api-url', default='http://127.0.0.1:18080')
parser.add_argument('--gateway-url', default='http://127.0.0.1:18082')
parser.add_argument('--frontend-url', default='http://127.0.0.1:13000')
parser.add_argument('--mode', choices=['normal', 'load', 'catalog-down', 'api-down', 'rollback'], required=True)
parser.add_argument('--session-file', required=True, help='Archivo temporal privado para reutilizar una sesión real.')
args = parser.parse_args()
checks = []

def client(jar=None):
    return urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(jar if jar is not None else http.cookiejar.CookieJar()))

def admin_session():
    jar = http.cookiejar.LWPCookieJar(args.session_file)
    jar.load(ignore_discard=True, ignore_expires=True)
    return client(jar)

def request(base, path, status=200, method='GET', data=None, opener=None, headers=None):
    headers = dict(headers or {})
    if data is not None:
        headers['Content-Type'] = 'application/json'
    req = urllib.request.Request(base + path, method=method, headers=headers,
        data=None if data is None else json.dumps(data).encode())
    try:
        response = (opener or client()).open(req, timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        body = response.read()
        assert response.status == status, f'{path}: expected {status}, got {response.status}: {body}'
        if 'json' in response.headers.get('Content-Type', ''):
            body = json.loads(body)
        return body, dict(response.headers)

def check(base, path, status=200, **kwargs):
    result = request(base, path, status, **kwargs)
    checks.append(path + ': HTTP ' + str(status))
    return result

def login(email='admin.sup267@example.test'):
    jar = http.cookiejar.LWPCookieJar(args.session_file) if email == 'admin.sup267@example.test' else http.cookiejar.CookieJar()
    opener = client(jar)
    body, _ = check(args.gateway_url, '/api/v1/auth/login', method='POST',
        data={'correo': email, 'password': 'Sup267FixturePassword123!'}, opener=opener)
    assert body['correo'] == email
    if isinstance(jar, http.cookiejar.LWPCookieJar):
        jar.save(ignore_discard=True, ignore_expires=True)
    return opener

def catalog(base):
    body, headers = check(base, '/api/v1/productos?q=Contrato&limit=5')
    assert body['total_registros'] == 2 and len(body['data']) == 2
    assert headers.get('X-Request-Id') or headers.get('X-Request-ID')

if args.mode == 'normal':
    for path in ['/api/v1/productos', '/api/v1/productos/buscar?q=Contrato', '/api/v1/productos/id', '/api/v1/admin/productos']:
        check(args.api_url, path, 404)
    catalog(args.gateway_url)
    catalog(args.frontend_url)
    html, _ = check(args.frontend_url, '/')
    assert b'<html' in html.lower()
    check(args.gateway_url, '/_gateway/ready')
    health, _ = check(args.gateway_url, '/_gateway/dependencies')
    assert all(value == 'ok' for value in health['checks'].values()) and 'catalogo' in health['checks']
    admin = login()
    check(args.gateway_url, '/api/v1/admin/productos', opener=admin)
    check(args.frontend_url, '/api/v1/admin/productos', opener=admin)
    registered = login('registered.sup267@example.test')
    check(args.gateway_url, '/api/v1/admin/productos', 403, opener=registered)
    email = 'sup268.' + uuid.uuid4().hex[:8] + '@example.test'
    new_user = client()
    _, headers = check(args.gateway_url, '/api/v1/auth/register', 201, method='POST', opener=new_user,
          data={'correo':email, 'password':'Sup268FixturePassword123!', 'nombre_completo':'Fixture SUP268'})
    assert 'HttpOnly' in headers.get('Set-Cookie', '')
    check(args.gateway_url, '/api/v1/auth/me', opener=new_user)
    profile, _ = check(args.gateway_url, '/api/v1/auth/me', method='PUT', opener=new_user,
        data={'nombre_completo':'Fixture SUP268 actualizado', 'telefono':'+56911112222', 'rol':'admin'})
    assert profile['rol'] == 'registrado' and profile['nombre_completo'] == 'Fixture SUP268 actualizado'
    check(args.gateway_url, '/api/v1/admin/productos', 403, opener=new_user)
    job, _ = check(args.gateway_url, '/api/v1/scraper/trabajos', 201, method='POST', data={'cadena_id':1, 'supermercado':'Jumbo'})
    path = '/api/v1/scraper/trabajos/' + job['id']
    check(args.gateway_url, path + '/ejecutar', 400, method='POST', data={'spider':'invalid_spider'})
    check(args.gateway_url, path + '/ejecutar', 202, method='POST', data={'spider':'jumbo_rsc'})
    result, _ = check(args.gateway_url, path + '/productos', method='POST', data={'sucursal_id':1, 'supermercado':'Jumbo',
        'productos':[{'sku':'SUP268-PIPELINE', 'producto':'Leche Pipeline SUP268', 'precio_normal':1234, 'en_stock':True}]})
    assert result['precios_registrados'] == 1
    products, _ = check(args.gateway_url, '/api/v1/productos/buscar?q=Pipeline')
    assert any(p['nombre'] == 'Leche Pipeline SUP268' for p in products['data'])
    check(args.gateway_url, path + '/finalizar', method='PUT', data={'estado':'completado', 'elementos_extraidos':1})
    job, _ = check(args.gateway_url, path)
    assert job['estado'] == 'completado' and job['elementos_extraidos'] == 1
    check(args.gateway_url, '/api/v1/scraper/productos', method='POST', data={'sucursal_id':1, 'supermercado':'Jumbo',
        'productos':[{'sku':'SUP268-DIRECT', 'producto':'Leche Directa SUP268', 'precio_normal':1200, 'en_stock':True}]})
    swagger, _ = check(args.gateway_url, '/swagger/doc.json')
    assert all(p in swagger['paths'] for p in ['/api/v1/productos','/api/v1/admin/productos','/api/v1/auth/login'])
elif args.mode == 'load':
    def hit(i):
        body, headers = request(args.gateway_url, '/api/v1/productos?q=Contrato&limit=5',
            headers={'X-Request-ID': 'sup268-load-' + str(i)})
        assert len(body['data']) == 2
        assert (headers.get('X-Request-Id') or headers.get('X-Request-ID')) == 'sup268-load-' + str(i)
    with concurrent.futures.ThreadPoolExecutor(max_workers=24) as pool:
        list(pool.map(hit, range(120)))
    checks.extend(['Load request passed'] * 120)
elif args.mode == 'catalog-down':
    admin = admin_session()
    check(args.gateway_url, '/api/v1/auth/me', opener=admin)
    error, _ = check(args.gateway_url, '/api/v1/productos', 502)
    assert error['error'] == 'catalogo_unavailable'
    check(args.gateway_url, '/_gateway/ready')
    health, _ = check(args.gateway_url, '/_gateway/dependencies', 503)
    assert health['checks']['backend'] == 'ok' and health['checks']['catalogo'] == 'error'
elif args.mode == 'api-down':
    catalog(args.gateway_url)
    catalog(args.frontend_url)
    check(args.gateway_url, '/api/v1/admin/productos', opener=admin_session())
    error, _ = check(args.gateway_url, '/api/v1/auth/me', 502)
    assert error['error'] == 'backend_unavailable'
    check(args.gateway_url, '/_gateway/ready')
    health, _ = check(args.gateway_url, '/_gateway/dependencies', 503)
    assert health['checks']['catalogo'] == 'ok' and health['checks']['backend'] == 'error'
elif args.mode == 'rollback':
    catalog(args.gateway_url)
    catalog(args.frontend_url)
    check(args.api_url, '/api/v1/productos')
    admin = login()
    check(args.gateway_url, '/api/v1/admin/productos', opener=admin)
    health, _ = check(args.gateway_url, '/_gateway/dependencies')
    assert 'catalogo' not in health['checks']

print(json.dumps({'task':'SUP-268','mode':args.mode,'passed':len(checks),'failed':0},ensure_ascii=False))
