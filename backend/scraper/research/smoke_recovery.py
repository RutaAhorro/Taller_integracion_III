"""SUP-270: fallos reales sobre los contenedores efímeros del harness.

Nunca usar este script contra un clúster: detiene servicios y vacía el Redis
del proyecto de prueba. El harness crea y elimina esos recursos por ejecución.
"""
import argparse
import concurrent.futures
import json
import os
import re
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from unittest.mock import patch
from uuid import uuid4

from redis import Redis
from scraper_core.api_client import ScraperAPIClient
from scraper_core.outbox import Outbox
from scraper_core.migrate_queue import migrate_one
from scraper_core.worker import ReliableQueue, QUEUE_NAME, GROUP, DEAD_LETTER, JOB_CLOSED_PREFIX, handle_message
from scraper_core.storage import SpoolCollector

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--project', required=True)
parser.add_argument('--env-file', required=True)
parser.add_argument('--gateway-url', required=True)
parser.add_argument('--state-dir', required=True)
parser.add_argument('--replicas-only',action='store_true')
args = parser.parse_args()
assert args.project.startswith('sup268-'), 'Sólo proyectos efímeros del harness'
root = Path(__file__).resolve().parents[3]
compose_args = ['docker', 'compose', '--project-name', args.project, '--env-file', args.env_file,
                '-f', str(root / 'docker-compose.yml'), '-f', str(root / 'docker-compose.catalogo-routing-test.yml')]
passed = 0
compose_env = {k:v for k,v in os.environ.items() if k not in {
    'DB_USER','DB_PASSWORD','DB_NAME','JWT_SECRET','GOOGLE_CLIENT_ID',
    'CATALOGO_CORS_ALLOWED_ORIGINS','GATEWAY_TRUSTED_PROXIES','SUP268_READER_PASSWORD',
    'CATALOGO_INTERNAL_TOKEN','SUP269_COORDINATOR_PASSWORD',
    'IDENTITY_DB_PASSWORD','SCRAPING_DB_PASSWORD','CATALOGO_DB_PASSWORD', 'IDENTIDAD_EDGE_SUBNET','IDENTIDAD_EDGE_RANGE', 'IDENTIDAD_GATEWAY_IP',
    'IDENTIDAD_CORS_ALLOWED_ORIGINS', 'IDENTIDAD_COOKIE_SECURE'}}


def compose(*command, input=None):
    return subprocess.run(compose_args + list(command), input=input, text=True, check=True,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE,env=compose_env).stdout


def sql(query):
    return compose('exec', '-T', 'db', 'psql', '-U', 'sup268', '-d', 'sup268', '-At', '-v', 'ON_ERROR_STOP=1', input=query).strip()


def request(path, data=None, status=200, method=None, key=None):
    global passed
    headers = {'Content-Type': 'application/json', 'X-Request-ID': 'sup270-recovery'}
    if key: headers['Idempotency-Key'] = key
    req = urllib.request.Request(args.gateway_url + path, data=None if data is None else json.dumps(data).encode(),
                                 method=method or ('POST' if data is not None else 'GET'), headers=headers)
    try:
        response = urllib.request.build_opener(urllib.request.ProxyHandler({})).open(req, timeout=25)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        result = json.loads(response.read())
        assert response.status in ((status,) if isinstance(status, int) else status), (path, response.status, result)
        if key and key != 'invalid': assert response.headers.get('Idempotency-Key') == key
    passed += 1
    return result


def wait_for(predicate, timeout=40):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        if predicate(): return
        time.sleep(.2)
    raise AssertionError('No se completó la recuperación dentro del plazo')


def job():
    return request('/api/v1/scraper/trabajos', {'cadena_id': 1}, 201)['id']


def batch(sku):
    return {'sucursal_id': 1, 'productos': [{'sku': sku, 'producto': 'Recuperación SUP270', 'precio_normal': 1250}]}


def path(work_id): return '/api/v1/scraper/trabajos/' + work_id
def captures(sku):
    return int(sql("SELECT count(*) FROM scraper.capturas_precios cp JOIN scraper.productos_crudos p ON p.id=cp.producto_crudo_id WHERE p.sku='" + sku + "';"))


if args.replicas_only:
    key=str(uuid4())
    payload=json.dumps({'lote_id':key,'sucursal_id':1,'productos':batch('SUP270-MULTIPOD')['productos']})
    def hit(i):
        # El secreto se lee dentro del contenedor; no se copia ni imprime.
        output=compose('exec','-T','--index',str(i%3+1),'catalogo','sh','-c',
                       'wget -q -O - --header "Authorization: Bearer $CATALOGO_INTERNAL_TOKEN" --header "Content-Type: application/json" --post-data "$1" http://127.0.0.1:8080/internal/v1/ingestas',
                       'sup270',payload)
        return json.loads(output)
    with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:
        results=list(pool.map(hit,range(12)))
    assert all(result==results[0] for result in results)
    assert results[0]['insertados']==1 and captures('SUP270-MULTIPOD')==1
    assert sql("SELECT count(*) FROM scraper.lotes_catalogo WHERE id='"+key+"';")=='1'
    print(json.dumps({'task':'SUP-270','mode':'same-key-three-catalog-replicas','internal_http_checks':12,'failed':0}))
    sys.exit(0)


# Misma clave y documento devuelve el resultado original, también al finalizar.
work, key = job(), str(uuid4())
payload = batch('SUP270-REPLAY')
original = request(path(work) + '/productos', payload, key=key)
assert request(path(work) + '/productos', payload, key=key) == original
assert captures('SUP270-REPLAY') == 1
changed = batch('SUP270-CHANGED')
assert request(path(work) + '/productos', changed, 409, key=key)['codigo'] == 'idempotency_conflict'
other = job()
assert request(path(other) + '/productos', payload, 409, key=key)['codigo'] == 'idempotency_conflict'
assert request(path(work))['elementos_extraidos'] == 1
assert request(path(work) + '/finalizar', {'estado':'completado','elementos_extraidos':999}, method='PUT')['elementos_extraidos'] == 1
request(path(work) + '/finalizar', {'estado':'completado'}, method='PUT')
assert request(path(work) + '/productos', payload, key=key) == original
request(path(work) + '/productos', payload, 409, key=str(uuid4()))
request('/api/v1/scraper/productos', payload, 400, key='invalid')

# Contención de la misma clave: captura y contador únicos entre solicitudes.
work, key = job(), str(uuid4())
payload = batch('SUP270-CONCURRENT-KEY')
with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:
    results = list(pool.map(lambda _: request(path(work) + '/productos', payload, (200,202), key=key), range(12)))
assert any('insertados' in r for r in results)
assert captures('SUP270-CONCURRENT-KEY') == 1
assert request(path(work))['elementos_extraidos'] == 1

# Commit remoto + fallo local: reiniciar API y recuperar sin intervención cliente.
work, key = job(), str(uuid4())
payload = batch('SUP270-COUNTER-RECOVERY')
sql("CREATE FUNCTION scraper.sup270_counter_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id='" + work + "' AND NEW.elementos_extraidos>OLD.elementos_extraidos THEN RAISE EXCEPTION 'simulated counter failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER sup270_counter_failure BEFORE UPDATE ON scraper.trabajos_scraper FOR EACH ROW EXECUTE FUNCTION scraper.sup270_counter_failure();")
assert request(path(work) + '/productos', payload, 409, key=key)['catalogo_confirmado'] is True
assert captures('SUP270-COUNTER-RECOVERY') == 1
assert request(path(work) + '/finalizar', {'estado':'completado'}, 409, 'PUT')['codigo'] == 'entregas_pendientes'
compose('stop', 'go_service')
sql('DROP TRIGGER sup270_counter_failure ON scraper.trabajos_scraper; DROP FUNCTION scraper.sup270_counter_failure();')
compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'go_service')
wait_for(lambda: sql("SELECT estado FROM scraper.entregas_ingesta WHERE id='" + key + "';") == 'confirmada')
assert request(path(work))['elementos_extraidos'] == 1
assert captures('SUP270-COUNTER-RECOVERY') == 1
assert request(path(work) + '/productos', payload, key=key)['insertados'] == 1

# Caída antes del envío: la entrada existe antes del HTTP a Catálogo.
work, key = job(), str(uuid4())
compose('stop', 'catalogo')
request(path(work) + '/productos', batch('SUP270-CATALOG-RECOVERY'), (503,504), key=key)
assert sql("SELECT estado FROM scraper.entregas_ingesta WHERE id='" + key + "';") == 'pendiente'
compose('restart', 'go_service')
compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'catalogo', 'go_service')
wait_for(lambda: sql("SELECT estado FROM scraper.entregas_ingesta WHERE id='" + key + "';") == 'confirmada')
assert captures('SUP270-CATALOG-RECOVERY') == 1
assert request(path(work))['elementos_extraidos'] == 1

# Un dato inválido revierte todo y queda fallido en el primer intento.
work, key = job(), str(uuid4())
bad = batch('SUP270-ATOMIC-FIRST')
bad['productos'].append({'sku':'SUP270-ATOMIC-FAIL','producto':'x'*256,'precio_normal':1000})
assert request(path(work) + '/productos', bad, 400, key=key)['codigo'] == 'lote_rechazado'
assert captures('SUP270-ATOMIC-FIRST') == 0
assert sql("SELECT count(*) FROM scraper.lotes_catalogo WHERE id='" + key + "';") == '0'
assert sql("SELECT estado || ':' || intentos FROM scraper.entregas_ingesta WHERE id='" + key + "';") == 'fallida:1'
assert request(path(work) + '/productos', bad, 409, key=key)['codigo'] == 'entrega_fallida'
# Una entrega fallida y otra pendiente no impiden cerrar como fallido.
pending_key = str(uuid4())
compose('stop', 'catalogo')
request(path(work) + '/productos', batch('SUP270-CLOSE-PENDING'), (503,504), key=pending_key)
assert request(path(work) + '/finalizar', {'estado':'completado'}, 409, 'PUT')['codigo'] == 'entregas_fallidas'
request(path(work) + '/finalizar', {'estado':'fallido'}, method='PUT')
assert request(path(work))['estado'] == 'fallido'
compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'catalogo')
wait_for(lambda: sql("SELECT estado FROM scraper.entregas_ingesta WHERE id='" + pending_key + "';") == 'confirmada')
assert request(path(work))['elementos_extraidos'] == 1
assert request(path(work) + '/productos', batch('SUP270-CLOSE-PENDING'), key=pending_key)['insertados'] == 1
assert captures('SUP270-ATOMIC-FIRST') == 0

# Un fallo realmente transitorio conserva backoff y presupuesto de 10 intentos.
work, key = job(), str(uuid4())
sql("CREATE FUNCTION scraper.sup270_transient_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.sku='SUP270-TRANSIENT' THEN RAISE EXCEPTION 'transient fixture' USING ERRCODE='40001'; END IF; RETURN NEW; END $$; CREATE TRIGGER sup270_transient_failure BEFORE INSERT ON scraper.productos_crudos FOR EACH ROW EXECUTE FUNCTION scraper.sup270_transient_failure();")
request(path(work) + '/productos', batch('SUP270-TRANSIENT'), 503, key=key)
assert sql("SELECT estado FROM scraper.entregas_ingesta WHERE id='" + key + "';") == 'pendiente'
sql("UPDATE scraper.entregas_ingesta SET intentos=9,proximo_intento=now(),lease_hasta=NULL WHERE id='" + key + "';")
wait_for(lambda: sql("SELECT estado FROM scraper.entregas_ingesta WHERE id='" + key + "';") == 'fallida')
assert sql("SELECT intentos FROM scraper.entregas_ingesta WHERE id='" + key + "';") == '10'
sql('DROP TRIGGER sup270_transient_failure ON scraper.productos_crudos; DROP FUNCTION scraper.sup270_transient_failure();')
request(path(work) + '/finalizar', {'estado':'fallido'}, method='PUT')

# Redis bloqueado no puede mantener una transacción SQL ni impedir el cierre.
test_env = dict(line.split('=', 1) for line in Path(args.env_file).read_text().splitlines() if '=' in line)
go_env = dict(os.environ)
go_env['SUP270_PUBLISHER_TEST_DB_URL'] = ('postgres://app_scraping:' + test_env['SCRAPING_DB_PASSWORD'] +
    '@127.0.0.1:' + os.getenv('SUP270_DB_PORT', '18078') + '/sup268?sslmode=disable')
subprocess.run(['go', 'test', '-race', '-count=1', '-run', '^TestPublisherStalledRedisAllowsJobFinalization$', '-v',
                './internal/scraping/internal/repositories'], cwd=root/'backend/api', env=go_env, check=True)

# Cola SQL aceptada durante caída de Redis y reconstruida tras pérdida total.
work = job()
compose('stop', 'redis')
execution = {'spider':'jumbo_rsc'}
request(path(work) + '/ejecutar', execution, 202)
request(path(work) + '/ejecutar', execution, 202)
request(path(work) + '/ejecutar', {'spider':'cugat_rsc'}, 409)
assert sql("SELECT count(*) FROM scraper.entregas_trabajos WHERE trabajo_id='" + work + "';") == '1'
compose('up', '-d', '--wait', '--wait-timeout', '90', 'redis')
redis = Redis.from_url('redis://127.0.0.1:' + os.getenv('SUP270_REDIS_PORT','18079'), decode_responses=True)
wait_for(lambda: redis.xlen(QUEUE_NAME) == 1)
redis.flushdb()  # Sólo Redis propio del proyecto efímero.
compose('restart', 'go_service')
wait_for(lambda: redis.xlen(QUEUE_NAME) == 1)

# Recuperación por lease; propietario antiguo no puede renovar ni hacer ACK.
first, second = ReliableQueue(redis, 'sup270-old', 10000), ReliableQueue(redis, 'sup270-new', 10000)
first.ensure_group()
message = first.receive(); assert message
message_id = message[0]
# Un reinicio rápido conserva prioridad: no toma un trabajo nuevo mientras
# el interrumpido todavía tiene lease. Después reclama el mensaje original.
second_work = job(); request(path(second_work) + '/ejecutar', execution, 202)
wait_for(lambda: redis.xlen(QUEUE_NAME) == 2)
assert second.receive() is None
redis.xclaim(QUEUE_NAME, GROUP, first.consumer, 0, [message_id], idle=11000, justid=True)
reclaimed = second.receive(); assert reclaimed[0] == message_id
assert not first.renew(message_id) and not first.ack(message_id, message[1]['payload'],work)
before = redis.xpending_range(QUEUE_NAME,GROUP,message_id,message_id,1)[0]['times_delivered']
assert second.renew(message_id)
assert redis.xpending_range(QUEUE_NAME,GROUP,message_id,message_id,1)[0]['times_delivered'] == before
request(path(work) + '/finalizar', {'estado':'fallido'}, method='PUT')
assert second.ack(message_id,message[1]['payload'],work,'lease_review')
assert redis.xpending(QUEUE_NAME,GROUP)['pending'] == 0 and redis.xlen(DEAD_LETTER) == 1
assert sql("SELECT count(*) FROM scraper.entregas_trabajos WHERE trabajo_id='" + work + "';") == '0'
# El publisher que conservaba una lectura anterior no recrea el mensaje tras ACK.
source = (root/'backend/api/internal/scraping/internal/repositories/job_publisher.go').read_text()
publish_script = re.search(r'var publishJob = redis.NewScript\(`(.*?)`\)', source, re.S).group(1)
assert redis.ttl(JOB_CLOSED_PREFIX + work) > 86000
assert redis.eval(publish_script, 3, QUEUE_NAME, 'scraper:jobs:index', JOB_CLOSED_PREFIX + work, work, message[1]['payload']) is None
assert redis.xlen(QUEUE_NAME) == 1
new_message = second.receive(); assert json.loads(new_message[1]['payload'])['trabajo_id'] == second_work
request(path(second_work) + '/finalizar', {'estado':'fallido'}, method='PUT')
assert second.ack(new_message[0], new_message[1]['payload'], second_work)

# Proceso muere después del commit HTTP y antes de marcar entregado/ACK.
work = job()
request(path(work) + '/ejecutar', execution, 202)
wait_for(lambda: redis.hget('scraper:jobs:index',work))
os.environ['SCRAPER_STATE_DIR'] = args.state_dir
script = """
import os
from redis import Redis
from scraper_core.worker import ReliableQueue
from scraper_core.outbox import Outbox
from scraper_core.api_client import ScraperAPIClient
q=ReliableQueue(Redis.from_url(os.environ['TEST_REDIS_URL'],decode_responses=True),'sup270-killed',10000)
q.ensure_group(); msg=q.receive(); assert msg
spool=Outbox(os.environ['TEST_WORK_ID'],os.environ['SCRAPER_STATE_DIR'])
spool.add({'sku':'SUP270-KILL','producto':'Recuperación SUP270','precio_normal':1250},'Jumbo'); spool.seal()
client=ScraperAPIClient(os.environ['TEST_API_URL'])
class LostReply:
 def ingest_batch(self,**data):
  client.ingest_batch(**data)
  os._exit(73)
spool.replay(LostReply())
"""
child_env = dict(os.environ, TEST_REDIS_URL='redis://127.0.0.1:' + os.getenv('SUP270_REDIS_PORT','18079'),
                 TEST_WORK_ID=work, TEST_API_URL=args.gateway_url+'/api/v1/scraper/productos')
result = subprocess.run([sys.executable,'-c',script],env=child_env,timeout=25)
assert result.returncode == 73 and captures('SUP270-KILL') == 1
pending = redis.xpending_range(QUEUE_NAME,GROUP,'-','+',1)[0]
redis.xclaim(QUEUE_NAME,GROUP,'sup270-killed',0,[pending['message_id']],idle=11000,justid=True)
queue = ReliableQueue(redis,'sup270-recovered',10000)
message = queue.receive(); assert message
def recovered_crawl(job, cancelled=None):
    spool=Outbox(work,args.state_dir); spool.set_progress('crawl_done',1); spool.close(); return {}
with patch('scraper_core.worker.ScrapyCommandExecutor', return_value=recovered_crawl):
    handle_message(queue,message,ScraperAPIClient(args.gateway_url+'/api/v1/scraper/productos'))
assert redis.xpending(QUEUE_NAME,GROUP)['pending'] == 0
assert request(path(work))['estado'] == 'completado'
assert request(path(work))['elementos_extraidos'] == 1 and captures('SUP270-KILL') == 1
assert not (Path(args.state_dir)/work).exists(), 'spool exitoso debe retirarse después del ACK'

# GC repara un crash posterior al ACK sin borrar un spool con mensaje pendiente.
spool = Outbox(work, args.state_dir); spool.close()
gc_work = job(); request(path(gc_work) + '/ejecutar', execution, 202)
wait_for(lambda: redis.hget('scraper:jobs:index', gc_work))
spool = Outbox(gc_work, args.state_dir); spool.close()
api = ScraperAPIClient(args.gateway_url + '/api/v1/scraper/productos')
collector = SpoolCollector(args.state_dir)
assert collector.collect(api, queue) >= 1
assert not (Path(args.state_dir)/work).exists()
assert (Path(args.state_dir)/gc_work).exists()
message = queue.receive(); assert json.loads(message[1]['payload'])['trabajo_id'] == gc_work
request(path(gc_work) + '/finalizar', {'estado':'fallido'}, method='PUT')
assert queue.ack(message[0], message[1]['payload'], gc_work)

# Tres crawls agotados: confirmar fallo en API y guardar dead-letter una vez.
work = job(); request(path(work)+'/ejecutar',execution,202)
wait_for(lambda: redis.hget('scraper:jobs:index',work))
spool=Outbox(work,args.state_dir); spool.set_progress('crawl_attempts',3); spool.close()
queue=ReliableQueue(redis,'sup270-dead',10000)
message=queue.receive(); assert message
handle_message(queue,message,ScraperAPIClient(args.gateway_url+'/api/v1/scraper/productos'))
assert request(path(work))['estado']=='fallido'
assert redis.xpending(QUEUE_NAME,GROUP)['pending']==0 and redis.xlen(DEAD_LETTER)==2

# Pipeline y señales Scrapy reales, sobre data: local; ningún supermercado.
work=job(); request(path(work)+'/ejecutar',execution,202)
wait_for(lambda: redis.hget('scraper:jobs:index',work))
queue=ReliableQueue(redis,'sup270-fixture',10000)
message=queue.receive(); assert message
def fixture_crawl(job,cancelled=None):
    env=dict(os.environ,SCRAPER_API_URL=args.gateway_url+'/api/v1/scraper/productos')
    result=subprocess.run([sys.executable,str(Path(__file__).with_name('recovery_fixture.py'))],env=env,capture_output=True,text=True,timeout=30)
    assert result.returncode==0,result.stderr
    return {}
with patch('scraper_core.worker.ScrapyCommandExecutor',return_value=fixture_crawl):
    handle_message(queue,message,ScraperAPIClient(args.gateway_url+'/api/v1/scraper/productos'))
assert request(path(work))['estado']=='completado'
assert request(path(work))['elementos_extraidos']==2
assert captures('SUP270-SCRAPY-1')==1 and captures('SUP270-SCRAPY-2')==1

# Corte de lista anterior: retirar sólo después de aceptación SQL y luego
# confirmar el mensaje sin volver a ejecutar un trabajo terminal.
work=job()
legacy=json.dumps({'trabajo_id':work,'spider':'jumbo_rsc'})
redis.lpush('scraper:jobs',legacy)
client=ScraperAPIClient(args.gateway_url+'/api/v1/scraper/productos')
assert migrate_one(redis,client)==work and redis.llen('scraper:jobs')==1
assert migrate_one(redis,client,apply=True)==work and redis.llen('scraper:jobs')==0
assert sql("SELECT count(*) FROM scraper.entregas_trabajos WHERE trabajo_id='"+work+"';")=='1'
wait_for(lambda: redis.hget('scraper:jobs:index',work))
queue=ReliableQueue(redis,'sup270-legacy',10000)
message=queue.receive(); assert message
request(path(work)+'/finalizar',{'estado':'completado'},method='PUT')
with patch('scraper_core.worker.ScrapyCommandExecutor') as executor:
    handle_message(queue,message,client)
    executor.assert_not_called()
assert redis.xpending(QUEUE_NAME,GROUP)['pending']==0

# El rol de cada servicio no puede leer el ledger del otro dominio.
assert sql("SELECT has_table_privilege('app_scraping','scraper.lotes_catalogo','SELECT') OR has_table_privilege('app_catalogo','scraper.entregas_ingesta','SELECT') OR has_table_privilege('app_catalogo','scraper.entregas_trabajos','SELECT');")=='f'
# Tablas ausentes: ambos binarios impiden arrancar antes de exponer HTTP.
catalog_logs_before = len(compose('logs', '--no-color', 'catalogo'))
compose('stop', 'go_service', 'catalogo')
sql('ALTER TABLE scraper.entregas_ingesta RENAME TO sup270_restore_ingesta; ALTER TABLE scraper.lotes_catalogo RENAME TO sup270_restore_catalogo;')
compose('up', '-d', '--no-deps', 'go_service', 'catalogo')
wait_for(lambda: 'aplicar 05_recuperacion_scraping.sql' in compose('logs', '--no-color', 'go_service'))
wait_for(lambda: 'catalogo_storage_invalid' in compose('logs', '--no-color', 'catalogo')[catalog_logs_before:])
compose('stop', 'go_service', 'catalogo')
sql('ALTER TABLE scraper.sup270_restore_ingesta RENAME TO entregas_ingesta; ALTER TABLE scraper.sup270_restore_catalogo RENAME TO lotes_catalogo;')
compose('up','-d','--no-deps','--wait','--wait-timeout','90','go_service','catalogo')
request('/health')
print(json.dumps({'task':'SUP-270','http_checks':passed,'recovery_scenarios':18,'failed':0}))
