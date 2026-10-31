"""Contratos de Catálogo contra la API de referencia, sólo sobre fixtures aislados."""
import argparse
import http.cookiejar
import json
import urllib.error
import urllib.request


def client():
    return urllib.request.build_opener(
        urllib.request.ProxyHandler({}),
        urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))


def request(base, path, method='GET', data=None, opener=None, headers=None):
    opener = opener or client()
    headers = dict(headers or {})
    if data is not None:
        headers['Content-Type'] = 'application/json'
    req = urllib.request.Request(base.rstrip('/') + path, method=method, headers=headers,
                                 data=None if data is None else json.dumps(data).encode())
    try:
        response = opener.open(req, timeout=8)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        return response.status, dict(response.headers), json.loads(response.read())


parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--api-url', default='http://127.0.0.1:18084')
parser.add_argument('--catalogo-url', default='http://127.0.0.1:18083')
mode = parser.add_mutually_exclusive_group()
mode.add_argument('--catalogo-only', action='store_true')
mode.add_argument('--database-down', action='store_true')
args = parser.parse_args()
checks = []
fresh = '00000000-0000-4000-8000-000000002671'
stale = '00000000-0000-4000-8000-000000002672'


def catalog(path, status=200, **kwargs):
    result = request(args.catalogo_url, path, **kwargs)
    assert result[0] == status, 'Unexpected Catalog status for ' + path
    checks.append(path + ': HTTP ' + str(status))
    return result[2]


if args.database_down:
    catalog('/_catalogo/live')
    catalog('/_catalogo/ready', 503)
    failure = catalog('/api/v1/productos', 500)
    assert failure == {'error': 'Error interno al obtener los productos del catálogo'}
    print(json.dumps({'mode': 'database-down', 'passed': len(checks), 'failed': 0}))
    raise SystemExit(0)

if args.catalogo_only:
    page = catalog('/api/v1/productos?q=Contrato&limit=5')
    assert page['total_registros'] == 2 and len(page['data']) == 2
    item = catalog('/api/v1/productos/' + fresh)
    assert item['precio'] == 990 and len(item['historial']) == 2
    catalog('/_catalogo/ready')
    catalog('/api/v1/auth/login', 404, method='POST', data={})
    catalog('/api/v1/scraper/trabajos/test-job', 404)
    print(json.dumps({'mode': 'without-api-and-redis', 'passed': len(checks), 'failed': 0}))
    raise SystemExit(0)


def compare(path, status=200, method='GET', opener=None, headers=None):
    old = request(args.api_url, path, method=method, opener=opener, headers=headers)
    new = request(args.catalogo_url, path, method=method, opener=opener, headers=headers)
    assert old[0] == new[0] == status, 'Status differs for ' + path
    # Endurecimiento deliberado: no publicar diagnósticos JWT/rol en 401/403.
    old_body = dict(old[2])
    if status in (401, 403):
        old_body.pop('detalle', None)
        assert 'detalle' not in new[2], 'Authentication diagnostic exposed'
    assert old_body == new[2], 'JSON differs for ' + path
    assert old[1].get('Content-Type') == new[1].get('Content-Type'), 'Content-Type differs'
    if not headers or 'Origin' not in headers:
        assert 'Access-Control-Allow-Origin' not in new[1]
        assert 'Access-Control-Allow-Credentials' not in new[1]
    else:
        assert new[1].get('Access-Control-Allow-Origin') == headers['Origin']
        assert new[1].get('Access-Control-Allow-Credentials') == 'true'
        assert 'Origin' in new[1].get('Vary', '')
    checks.append(method + ' ' + path + ': compatible')
    return new[2]


for path in (
    '/api/v1/productos',
    '/api/v1/productos?page=2&limit=5',
    '/api/v1/productos?page=invalid&limit=0&sort_by=unknown&order=invalid',
    '/api/v1/productos?q=Contrato&sort_by=nombre&order=desc',
    '/api/v1/productos?q=Contrato&marca=MarcaContrato',
    '/api/v1/productos?categoria=Lacteos&supermercado=Jumbo',
    '/api/v1/productos?precio_min=900&precio_max=1300&en_oferta=true',
    '/api/v1/productos?en_oferta=false&sort_by=marca',
    '/api/v1/productos?en_stock=false',
    '/api/v1/productos/buscar?q=Contrato&page=1&limit=5',
    '/api/v1/productos/' + fresh,
    '/api/v1/productos/' + stale,
):
    compare(path)
for path in ('/api/v1/productos?q=le', '/api/v1/productos/buscar', '/api/v1/productos/buscar?q=le'):
    compare(path, 400)
compare('/api/v1/productos/missing-product', 404)
compare('/api/v1/productos', 405, method='PUT')
compare('/api/v1/admin/productos', 401)
compare('/api/v1/admin/productos', 401, headers={'X-User-ID': 'forged-user', 'X-User-Role': 'admin'})
compare('/api/v1/admin/productos', 401, headers={'Cookie': 'jwt=invalid'})

origin = {'Origin': 'https://frontend.example', 'Access-Control-Request-Method': 'GET'}
for base in (args.api_url, args.catalogo_url):
    req = urllib.request.Request(base + '/api/v1/productos', method='OPTIONS', headers=origin)
    with client().open(req, timeout=5) as response:
        assert response.status == 204
        assert response.headers['Access-Control-Allow-Origin'] == origin['Origin']
        assert response.headers['Access-Control-Allow-Credentials'] == 'true'
checks.append('OPTIONS: CORS compatible')
compare('/api/v1/productos', headers={'Origin': origin['Origin']})
for rejected in ('https://evil.example', 'https://frontend.example.evil.test', 'null'):
    req = urllib.request.Request(args.catalogo_url + '/api/v1/admin/productos',
                                 method='OPTIONS', headers={'Origin': rejected, 'Access-Control-Request-Method': 'GET'})
    try:
        client().open(req, timeout=5)
        raise AssertionError('Untrusted preflight accepted')
    except urllib.error.HTTPError as error:
        with error:
            assert error.code == 403
            assert error.headers.get('Access-Control-Allow-Origin') is None
            assert error.headers.get('Access-Control-Allow-Credentials') is None
            assert 'Origin' in error.headers.get('Vary', '')
    checks.append('Rejected preflight: ' + rejected)

for email, expected in (('registered.sup267@example.test', 403), ('admin.sup267@example.test', 200)):
    opener = client()
    status, _, login = request(args.api_url, '/api/v1/auth/login', 'POST',
                               {'correo': email, 'password': 'Sup267FixturePassword123!'}, opener)
    assert status == 200 and login['correo'] == email, 'Identity fixture login failed'
    compare('/api/v1/admin/productos', expected, opener=opener)
    compare('/api/v1/admin/productos/', expected, opener=opener)
    if expected == 200:
        compare('/api/v1/admin/productos', opener=opener, headers={'Origin': origin['Origin']})
        status, blocked_headers, blocked_body = request(args.catalogo_url, '/api/v1/admin/productos',
            opener=opener, headers={'Origin': 'https://evil.example'})
        assert status == 403 and blocked_body['mensaje'] == 'Origen no permitido.'
        assert 'Access-Control-Allow-Origin' not in blocked_headers
        assert 'Access-Control-Allow-Credentials' not in blocked_headers
        checks.append('Untrusted origin rejected with real admin cookie')
        for path in (
            '/api/v1/admin/productos?sku=SUP267&en_stock=false&sort_by=sku',
            '/api/v1/admin/productos?en_stock=true&limit=5&sort_by=ultima_extraccion&order=desc',
            '/api/v1/admin/productos?q=Contrato&precio_min=900&precio_max=2000',
            '/api/v1/admin/productos?categoria=Despensa&supermercado=Jumbo&marca=MarcaContrato',
        ):
            compare(path, opener=opener)
        compare('/api/v1/admin/productos?q=le', 400, opener=opener)

detail = catalog('/api/v1/productos/' + fresh)
assert detail['precio_normal'] == 1290 and detail['precio_oferta'] == 990 and detail['en_stock']
assert [p['precio_normal'] for p in detail['historial']] == [1290, 1490]
# Brecha preexistente: la lectura actual no aplica caducidad a precios antiguos.
old = catalog('/api/v1/productos/' + stale)
assert old['precio'] == 1990 and not old['en_stock']
assert compare('/api/v1/productos/' + stale) == old
status, headers, _ = request(args.catalogo_url, '/api/v1/productos?limit=5',
                             headers={'X-Request-ID': 'contract-sup267'})
assert status == 200 and headers.get('X-Request-Id', headers.get('X-Request-ID')) == 'contract-sup267'
checks.append('Request correlation preserved')
print(json.dumps({'mode': 'api-catalog-contracts', 'passed': len(checks), 'failed': 0,
                  'checks': checks}, ensure_ascii=False, indent=2))
