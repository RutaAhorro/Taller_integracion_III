"""SUP-269: contratos de escritura, contador, concurrencia y fallos atómicos."""
import argparse
import concurrent.futures
import json
import time
from uuid import uuid4
from pathlib import Path
import urllib.error
import urllib.request

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--gateway-url', required=True)
parser.add_argument('--reference-url')
parser.add_argument('--state-file', required=True)
parser.add_argument('--mode', choices=['normal', 'counter-failure', 'catalog-down', 'ingesta-rollback'], required=True)
args = parser.parse_args()
passed = 0

def request(base, path, status=200, method='GET', data=None):
    global passed
    headers = {'Content-Type': 'application/json', 'X-Request-ID': 'sup269-contract'}
    # SUP-270 permite 202 si recuperación tomó la lease antes del handler.
    # Repetir sólo ese lote con una clave estable y exigir resultado final 200;
    # las aserciones posteriores siguen exigiendo 12 capturas/contador exactos.
    durable_ingest = status == 200 and method == 'POST' and path.endswith('/productos')
    if durable_ingest:
        headers['Idempotency-Key'] = str(uuid4())
    deadline = time.monotonic() + 20
    while True:
        req = urllib.request.Request(base + path, method=method,
            data=None if data is None else json.dumps(data).encode(), headers=headers)
        try:
            response = urllib.request.build_opener(urllib.request.ProxyHandler({})).open(req, timeout=20)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            body = json.loads(response.read())
            expected = {status} if isinstance(status, int) else set(status)
            if durable_ingest:
                expected.add(202)
            assert response.status in expected, (path, status, response.status, body)
            pending = durable_ingest and response.status == 202
            if pending:
                assert body.get('codigo') == 'entrega_pendiente', body
        passed += 1
        if not pending:
            return body
        assert time.monotonic() < deadline, 'El lote aceptado no obtuvo confirmación dentro del plazo'
        time.sleep(.05)

def item(sku, **fields):
    return {'sku': sku, 'producto': 'Producto Ingesta SUP269', 'precio_normal': 1500, **fields}

def batch(base, path, products, status=200, **fields):
    return request(base, path, status, 'POST', {'sucursal_id': 1, 'productos': products, **fields})

def job(base):
    result = request(base, '/api/v1/scraper/trabajos', 201, 'POST', {'cadena_id': 1, 'supermercado': 'Jumbo'})
    return '/api/v1/scraper/trabajos/' + result['id']

if args.mode == 'normal':
    current = job(args.gateway_url)
    reference = job(args.reference_url)
    for method in ['insert', 'update']:
        results = []
        for base, path, sku in [(args.gateway_url, current, 'SUP269-CURRENT'), (args.reference_url, reference, 'SUP269-REFERENCE')]:
            result = batch(base, path + '/productos', [item(sku)])
            assert result['trabajo_id'] == path.rsplit('/', 1)[1]
            result.pop('trabajo_id')
            results.append(result)
        assert results[0] == results[1], (method, results)
    assert request(args.gateway_url, current)['elementos_extraidos'] == 2
    assert request(args.reference_url, reference)['elementos_extraidos'] == 2
    batch(args.gateway_url, '/api/v1/scraper/productos', [item('SUP269-STOCK', en_stock=False, precio=1000)])
    result = batch(args.gateway_url, '/api/v1/scraper/productos', [item('  ', producto='  '), item('SUP269-SKIP')])
    assert result['insertados'] == 1 and result['precios_registrados'] == 1 and len(result['errores']) == 1
    result = batch(args.gateway_url, current + '/productos', [item('SUP269-ROLLBACK-FIRST'), item('SUP269-ROLLBACK-FAIL', producto='x' * 256)], 400)
    assert result['codigo'] == 'lote_rechazado' and 'detalle' not in result
    assert request(args.gateway_url, current)['elementos_extraidos'] == 2
    request(args.gateway_url, '/internal/v1/ingestas', 404, 'POST', {})
    batch(args.gateway_url, current + '/productos', [item('SUP269-NO-BRANCH')], 404, sucursal_id=999999)
    batch(args.gateway_url, current + '/productos', [item('SKU')] * 1001, 400)
    missing = '/api/v1/scraper/trabajos/00000000-0000-0000-0000-000000000000/productos'
    batch(args.gateway_url, missing, [item('SKU')], 404)
    concurrent_job = job(args.gateway_url)
    def hit(i):
        return batch(args.gateway_url, concurrent_job + '/productos', [item('SUP269-CONCURRENT')])
    with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:
        results = list(pool.map(hit, range(12)))
    assert sum(r['insertados'] for r in results) == 1 and sum(r['actualizados'] for r in results) == 11
    assert request(args.gateway_url, concurrent_job)['elementos_extraidos'] == 12
    def resolve_new_source(i):
        return request(args.gateway_url, '/api/v1/scraper/productos', 200, 'POST',
            {'supermercado': 'SUP269 Auto Source', 'productos': [item('SUP269-AUTO-' + str(i))]})
    with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:
        automatic = list(pool.map(resolve_new_source, range(6)))
    branches = {r['sucursal_id'] for r in automatic}
    assert len(branches) == 1
    batch(args.gateway_url, current + '/productos', [item('SUP269-CROSS-CHAIN')], 404, sucursal_id=next(iter(branches)))
    request(args.gateway_url, concurrent_job + '/finalizar', 200, 'PUT', {'estado': 'completado'})
    batch(args.gateway_url, concurrent_job + '/productos', [item('SKU')], 409)
    file = Path(args.state_file)
    file.write_text(json.dumps({'job': current}))
    file.chmod(0o600)
elif args.mode == 'counter-failure':
    pending = job(args.gateway_url)
    result = batch(args.gateway_url, pending + '/productos', [item('SUP269-COUNTER-PENDING', producto='Contador pendiente SUP269')], 409)
    assert result['codigo'] == 'contador_pendiente' and result['catalogo_confirmado'] is True
    assert request(args.gateway_url, pending)['elementos_extraidos'] == 0
    products = request(args.gateway_url, '/api/v1/productos?q=Contador%20pendiente%20SUP269')
    assert len(products['data']) == 1 and products['data'][0]['precio_normal'] == 1500
elif args.mode == 'ingesta-rollback':
    legacy = job(args.gateway_url)
    result = batch(args.gateway_url, legacy + '/productos', [item('SUP269-ROLLBACK-WRITER', producto='Rollback escritura SUP269')])
    assert result['insertados'] == 1 and result['precios_registrados'] == 1
    assert request(args.gateway_url, legacy)['elementos_extraidos'] == 1
    products = request(args.gateway_url, '/api/v1/productos?q=Rollback%20escritura%20SUP269')
    assert len(products['data']) == 1
    health = request(args.gateway_url, '/_gateway/dependencies')
    assert health['checks']['catalogo'] == 'ok'
else:
    current = json.loads(Path(args.state_file).read_text())['job']
    for path in [current + '/productos', '/api/v1/scraper/productos']:
        body = batch(args.gateway_url, path, [item('SUP269-DOWN')], (503, 504))
        assert body['codigo'] in {'catalogo_unavailable', 'catalogo_timeout'} and 'detalle' not in body
    body = request(args.gateway_url, '/api/v1/scraper/trabajos', (503, 504), 'POST', {'cadena_id': 1})
    assert body['codigo'] in {'catalogo_unavailable', 'catalogo_timeout'}
    assert request(args.gateway_url, current)['elementos_extraidos'] == 2

print(json.dumps({'task': 'SUP-269', 'mode': args.mode, 'passed': passed, 'failed': 0}))
