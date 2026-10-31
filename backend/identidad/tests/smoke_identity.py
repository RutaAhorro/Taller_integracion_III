"""SUP-272: contratos, sesiones cruzadas, fallos y ataques en un Compose efímero."""
import argparse
import base64
import concurrent.futures
import hashlib
import hmac
import http.cookiejar
import json
import os
import re
import subprocess
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
assert args.project.startswith('sup268-'), 'Sólo recursos del harness aislado'
root = Path(__file__).resolve().parents[3]
env = {k: v for k, v in os.environ.items() if k not in {
    'DB_USER','DB_PASSWORD','DB_NAME','JWT_SECRET','GOOGLE_CLIENT_ID',
    'CATALOGO_CORS_ALLOWED_ORIGINS','GATEWAY_TRUSTED_PROXIES','CATALOGO_INTERNAL_TOKEN',
    'IDENTITY_DB_PASSWORD','SCRAPING_DB_PASSWORD','CATALOGO_DB_PASSWORD',
    'IDENTIDAD_EDGE_SUBNET','IDENTIDAD_EDGE_RANGE','IDENTIDAD_GATEWAY_IP','IDENTIDAD_CORS_ALLOWED_ORIGINS','IDENTIDAD_COOKIE_SECURE'}}
compose_args = ['docker','compose','--project-name',args.project,'--env-file',args.env_file,
                '-f',str(root/'docker-compose.yml'),'-f',str(root/'docker-compose.catalogo-routing-test.yml')]
identity_url = 'identidad' # Resolver puerto efímero después de cada recreación.
reference_url = 'http://127.0.0.1:' + os.getenv('SUP272_REFERENCE_PORT','18087')
api_url = 'http://127.0.0.1:' + os.getenv('SUP268_API_PORT','18080')
frontend_url = 'http://127.0.0.1:' + os.getenv('SUP268_FRONTEND_PORT','13000')
passed = 0

def compose(*command, success=True, input=None):
    result = subprocess.run(compose_args+list(command), env=env, input=input, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    assert (result.returncode == 0) == success, result.stderr
    return result.stdout

def sql(statement):
    return compose('exec','-T','db','psql','-X','-U','sup268','-d','sup268','-v','ON_ERROR_STOP=1','-tA','-c',statement).strip()

def client():
    return urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

# Control exige el token de servicio que usan worker y bot (SCRAPING_SERVICE_TOKEN).
def scraper_auth(path):
    return {'Authorization': 'Bearer ' + os.environ['SCRAPING_SERVICE_TOKEN']} if '/api/v1/scraper' in path else {}

def request(path, status=200, method='GET', data=None, opener=None, headers=None, base=None):
    global passed
    if base == identity_url: base = 'http://' + compose('port','identidad','8080').strip()
    req = urllib.request.Request((base or args.gateway_url)+path,method=method,
        data=json.dumps(data).encode() if data is not None else None,
        headers={'Content-Type':'application/json',**scraper_auth(path),**(headers or {})})
    try: response = (opener or client()).open(req,timeout=15)
    except urllib.error.HTTPError as e: response = e
    with response:
        body = response.read()
        assert response.status == status, (path,status,response.status,body.decode())
        result = json.loads(body) if body else None
        passed += 1
        return result,response.headers

def reset_login_counters():
    # Únicamente Redis del proyecto efímero, nunca FLUSHDB ni estado real.
    script="local keys=redis.call('KEYS','rate_limit:login:*'); for _,key in ipairs(keys) do redis.call('DEL',key) end; return #keys"
    compose('exec','-T','redis','redis-cli','EVAL',script,'0')

def state(service):
    ids = compose('ps','-q',service).split()
    return subprocess.run(['docker','inspect','--format','{{.Id}} {{.State.StartedAt}} {{.RestartCount}}',*ids],
        check=True,text=True,stdout=subprocess.PIPE).stdout.strip()

def check_kubernetes_probes(ready=True):
    global passed
    # Ejecutar los comandos del Deployment, no una copia, en la imagen runtime.
    manifest=root/'infrastructure/k8s/identidad/deployment.yaml'
    commands=[json.loads(line.split('command:',1)[1]) for line in manifest.read_text().splitlines()
              if line.strip().startswith('command:')]
    assert len(commands)==3 and sum(command[-1].endswith('/ready') for command in commands)==1
    for command in commands:
        assert command[-1].startswith('http://127.0.0.1:8080/_identidad/')
        compose('exec','-T','identidad',*command,success=ready or command[-1].endswith('/live'))
        passed+=1

def wait(predicate, timeout=30):
    end=time.monotonic()+timeout
    while time.monotonic()<end:
        if predicate(): return
        time.sleep(.2)
    raise AssertionError('No se completó el escenario dentro del plazo')

reset_login_counters()
user_id = sql("SELECT id FROM api.usuarios WHERE correo='registered.sup267@example.test';")
# Credenciales anteriores siguen sirviendo a Identidad y a Catálogo.
old_client = client()
old_body,old_headers=request('/api/v1/auth/login',method='POST',data={'correo':'admin.sup267@example.test','password':'Sup267FixturePassword123!'},opener=old_client,base=reference_url)
new_client = client()
new_body,new_headers=request('/api/v1/auth/login',method='POST',data={'correo':'admin.sup267@example.test','password':'Sup267FixturePassword123!'},opener=new_client)
assert old_body == new_body
for headers in (old_headers,new_headers):
    cookie=headers.get('Set-Cookie','')
    assert 'HttpOnly' in cookie and 'Path=/' in cookie and 'Max-Age=86400' in cookie
frontend_client=client()
frontend_body,_=request('/api/v1/auth/login',method='POST',data={'correo':'admin.sup267@example.test','password':'Sup267FixturePassword123!'},opener=frontend_client,base=frontend_url)
assert frontend_body == new_body
request('/api/v1/auth/me',opener=frontend_client,base=frontend_url)
request('/api/v1/admin/productos',opener=frontend_client,base=frontend_url)
request('/api/v1/auth/me',opener=old_client)
request('/api/v1/admin/productos',opener=old_client)
request('/api/v1/auth/me',opener=new_client,base=reference_url)
request('/api/v1/admin/productos',opener=new_client)
request('/api/v1/auth/me',401,headers={'X-User-ID':user_id,'X-Role':'admin','X-User-Role':'admin'})
request('/api/v1/auth/me',404,base=api_url)
request('/api/v1/auth/login',404,method='POST',data={},base=api_url)
request('/api/v1/productos',404,base=identity_url)
request('/api/v1/scraper/trabajos',404,method='POST',data={},base=identity_url)
request('/_identidad/ready',404) # Probes privados no se exponen por Gateway.
request('/_identidad/ready',base=identity_url)
request('/_identidad/live',base=identity_url)
check_kubernetes_probes()

# Firma, algoritmo y expiración. No se imprimen la clave ni los tokens.
fixture_env=dict(line.split('=',1) for line in Path(args.env_file).read_text().splitlines() if '=' in line)
key=fixture_env['JWT_SECRET'].encode()
def token(claims, alg='HS256', secret=key):
    encode=lambda obj: base64.urlsafe_b64encode(json.dumps(obj,separators=(',',':')).encode()).rstrip(b'=')
    value=encode({'alg':alg,'typ':'JWT'})+b'.'+encode(claims)
    digest={'HS256':hashlib.sha256,'HS384':hashlib.sha384,'HS512':hashlib.sha512}[alg]
    return (value+b'.'+base64.urlsafe_b64encode(hmac.new(secret,value,digest).digest()).rstrip(b'=')).decode()
claims={'user_id':user_id,'rol':'registrado','provider':'local','exp':int(time.time())+3600}
for value in [token(claims,secret=b'wrong-signature'),token(claims,'HS384'),token(claims,'HS512'),
              token({k:v for k,v in claims.items() if k!='exp'}),token({**claims,'exp':int(time.time())-10}),
              token({**claims,'nbf':int(time.time())+7200}),'malformed-token']:
    request('/api/v1/auth/me',401,headers={'Cookie':'jwt='+value})
request('/api/v1/admin/productos',403,headers={'Cookie':'jwt='+token(claims),'X-Role':'admin'})
request('/api/v1/auth/me',opener=old_client,headers={'X-User-ID':'forged','X-Role':'registrado'})

# CORS: rechazar antes de ejecutar handlers y conservar preflight legítimo.
for method in ('GET','PUT','OPTIONS'):
    body,headers=request('/api/v1/auth/me',403,method=method,data={} if method=='PUT' else None,opener=new_client,headers={'Origin':'https://evil.example'})
    assert headers.get('Access-Control-Allow-Origin') is None and headers.get('Access-Control-Allow-Credentials') is None
body,headers=request('/api/v1/auth/me',opener=new_client,headers={'Origin':'https://frontend.example','X-Request-ID':'sup272-correlation'})
assert headers.get('Access-Control-Allow-Origin')=='https://frontend.example'
assert headers.get('Access-Control-Allow-Credentials')=='true' and 'Origin' in headers.get('Vary','')
assert headers.get('X-Request-ID')=='sup272-correlation'
request('/api/v1/auth/me',204,method='OPTIONS',headers={'Origin':'https://frontend.example'})
request('/api/v1/auth/google',400,method='POST',data={})
request('/api/v1/auth/google',503,method='POST',data={'id_token':'malformed-token'}) # SSO no configurado en harness.
request('/api/v1/auth/register',400,method='POST',data={'correo':'too-long@example.test','password':'A1!'+('a'*70),'nombre_completo':'Fixture'})
request('/api/v1/auth/register',400,method='POST',data={'correo':'huge@example.test','password':'A1!'+('a'*70000),'nombre_completo':'Fixture'})

# Carrera real de registro: una creación, el resto conflictos controlados.
email='sup272.race.'+uuid4().hex+'@example.test'
def register_race(_):
    req=urllib.request.Request(args.gateway_url+'/api/v1/auth/register',method='POST',headers={'Content-Type':'application/json'},data=json.dumps({'correo':email,'password':'Sup272FixturePassword123!','nombre_completo':'Concurrent Fixture'}).encode())
    try: response=client().open(req,timeout=15)
    except urllib.error.HTTPError as e: response=e
    with response: return response.status,json.loads(response.read())
with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool: results=list(pool.map(register_race,range(8)))
assert sum(code==201 for code,_ in results)==1 and sum(code==409 for code,_ in results)==7, results
passed+=8
assert sql("SELECT count(*) FROM api.usuarios WHERE correo='"+email+"';")=='1'

# Profile updates derive the ID and role exclusively from JWT.
registered=client()
request('/api/v1/auth/register',201,method='POST',data={'correo':'sup272.profile@example.test','password':'Sup272FixturePassword123!','nombre_completo':'Profile'},opener=registered)
body,_=request('/api/v1/auth/me',method='PUT',data={'nombre_completo':'Updated Profile','rol':'admin','esta_activo':False,'cuota_tokens_ia':999999,'user_id':old_body['id']},opener=registered)
assert body['rol']=='registrado' and body['esta_activo'] is True and body['nombre_completo']=='Updated Profile'
for name in ('@@@;;;', ' \t ', '@ @'):
    request('/api/v1/auth/me',400,method='PUT',data={'nombre_completo':name},opener=registered)
    assert sql("SELECT nombre_completo FROM api.usuarios WHERE correo='sup272.profile@example.test';")=='Updated Profile'
request('/api/v1/auth/register',400,method='POST',data={'correo':'sup272.empty-name@example.test','password':'Sup272FixturePassword123!','nombre_completo':'@@@;;;'})
assert sql("SELECT count(*) FROM api.usuarios WHERE correo='sup272.empty-name@example.test';")=='0'
for name,expected in [('a'*99+'ñ'+'z','a'*99+'ñ'), ('áéíóúñ'*20,('áéíóúñ'*20)[:100])]:
    body,_=request('/api/v1/auth/me',method='PUT',data={'nombre_completo':name},opener=registered)
    assert body['nombre_completo']==expected
    assert sql("SELECT nombre_completo FROM api.usuarios WHERE correo='sup272.profile@example.test';")==expected
request('/api/v1/admin/productos',403,opener=registered)
# Cambiar contraseña o correo exige la contraseña actual (una cookie robada no basta).
request('/api/v1/auth/me',403,method='PUT',data={'password':'Sup272NewPassword123!'},opener=registered)
request('/api/v1/auth/me',403,method='PUT',data={'correo':'sup272.hijack@example.test','current_password':'WrongPassword123!'},opener=registered)
assert sql("SELECT count(*) FROM api.usuarios WHERE correo='sup272.hijack@example.test';")=='0'
request('/api/v1/auth/me',method='PUT',data={'password':'Sup272NewPassword123!','current_password':'Sup272FixturePassword123!'},opener=registered)
reset_login_counters()
request('/api/v1/auth/login',method='POST',data={'correo':'sup272.profile@example.test','password':'Sup272NewPassword123!'})
# Los inicios de sesión correctos no consumen el límite: un NAT compartido no bloquea a usuarios legítimos.
reset_login_counters()
for _ in range(8): request('/api/v1/auth/login',method='POST',data={'correo':'admin.sup267@example.test','password':'Sup267FixturePassword123!'})
assert compose('exec','-T','redis','redis-cli','--raw','KEYS','rate_limit:login:*').split()==[] or all(
    compose('exec','-T','redis','redis-cli','--raw','GET',key).strip()=='0' for key in compose('exec','-T','redis','redis-cli','--raw','KEYS','rate_limit:login:*').split())
# Registro y Google también quedan limitados por IP.
assert compose('exec','-T','redis','redis-cli','--raw','KEYS','rate_limit:register:*').split(), 'registro sin rate limit'
assert compose('exec','-T','redis','redis-cli','--raw','KEYS','rate_limit:google:*').split(), 'Google sin rate limit'
# Real SSO row: password nil cannot panic at local login.
sql("INSERT INTO api.usuarios(correo,nombre_completo,google_id,password_hash,rol,esta_activo,cuota_tokens_ia) VALUES ('sup272.sso@example.test','SSO Fixture','google-fixture',NULL,'registrado',true,1000);")
reset_login_counters()
request('/api/v1/auth/login',401,method='POST',data={'correo':'sup272.sso@example.test','password':'Sup272FixturePassword123!'})
reset_login_counters()
for _ in range(5): request('/api/v1/auth/login',401,method='POST',data={'correo':'unknown@example.test','password':'WrongPassword123!'})
request('/api/v1/auth/login',429,method='POST',data={'correo':'unknown@example.test','password':'WrongPassword123!'})
for value in ['203.0.113.10','invalid_string, 203.0.113.11']:
    request('/api/v1/auth/login',429,method='POST',data={'correo':'unknown@example.test','password':'WrongPassword123!'},headers={'X-Forwarded-For':value})
keys=compose('exec','-T','redis','redis-cli','--raw','KEYS','rate_limit:login:*').split()
gateway_id=compose('ps','-q','gateway').strip()
gateway_inspect=json.loads(subprocess.run(['docker','inspect',gateway_id],check=True,text=True,stdout=subprocess.PIPE).stdout)[0]
gateway_ips={v['IPAddress'] for v in gateway_inspect['NetworkSettings']['Networks'].values()}
assert len(keys)==1 and keys[0].removeprefix('rate_limit:login:') not in gateway_ips, 'Todos los usuarios limitados como Gateway'
ttl=int(compose('exec','-T','redis','redis-cli','PTTL',keys[0]))
assert 0<ttl<=900000
compose('exec','-T','redis','redis-cli','PERSIST',keys[0])
request('/api/v1/auth/login',429,method='POST',data={'correo':'unknown@example.test','password':'WrongPassword123!'})
assert int(compose('exec','-T','redis','redis-cli','PTTL',keys[0])) > 0, 'Una clave sin TTL quedó bloqueada para siempre'
reset_login_counters()
# Otro contenedor de Compose no es Gateway y no puede falsificar su IP.
result=subprocess.run(compose_args+['exec','-T','control_scraping','wget','-S','-O','/dev/null',
    '--header','Content-Type: application/json','--header','X-Forwarded-For: 203.0.113.99',
    '--post-data',json.dumps({'correo':'unknown@example.test','password':'WrongPassword123!'}),
    'http://identidad:8080/api/v1/auth/login'],env=env,text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
assert result.returncode != 0 and re.search(r'HTTP/1\.1 401\b',result.stderr), result.stderr
passed += 1
assert compose('exec','-T','redis','redis-cli','--raw','GET','rate_limit:login:203.0.113.99').strip() == ''
api_id=compose('ps','-q','control_scraping').strip()
api_inspect=json.loads(subprocess.run(['docker','inspect',api_id],check=True,text=True,stdout=subprocess.PIPE).stdout)[0]
api_ips={v['IPAddress'] for v in api_inspect['NetworkSettings']['Networks'].values()}
assert any(compose('exec','-T','redis','redis-cli','--raw','GET','rate_limit:login:'+ip).strip() == '1' for ip in api_ips)
reset_login_counters()

# El navegador entra por Nginx: el sufijo real prevalece ante XFF malformado.
for _ in range(5): request('/api/v1/auth/login',401,method='POST',data={'correo':'unknown@example.test','password':'WrongPassword123!'},base=frontend_url)
request('/api/v1/auth/login',429,method='POST',data={'correo':'unknown@example.test','password':'WrongPassword123!'},base=frontend_url,headers={'X-Forwarded-For':'invalid_string, 203.0.113.77'})
frontend_id=compose('ps','-q','frontend').strip()
frontend_inspect=json.loads(subprocess.run(['docker','inspect',frontend_id],check=True,text=True,stdout=subprocess.PIPE).stdout)[0]
frontend_ips={v['IPAddress'] for v in frontend_inspect['NetworkSettings']['Networks'].values()}
keys=compose('exec','-T','redis','redis-cli','--raw','KEYS','rate_limit:login:*').split()
assert len(keys)==1 and keys[0].removeprefix('rate_limit:login:') not in (gateway_ips | frontend_ips), 'Se limitó la IP de un proxy en lugar del navegador'
reset_login_counters()

# SQL error remains local: client receives generic 500 without table/SQL traces.
sql("CREATE FUNCTION api.sup272_fail_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'SUP272 private SQL diagnostic'; END $$; CREATE TRIGGER sup272_fail_insert BEFORE INSERT ON api.usuarios FOR EACH ROW EXECUTE FUNCTION api.sup272_fail_insert();")
try:
    body,_=request('/api/v1/auth/register',500,method='POST',data={'correo':'sup272.error@example.test','password':'Sup272FixturePassword123!','nombre_completo':'Error Fixture'})
    assert 'detalle' not in body and 'SUP272' not in json.dumps(body)
finally: sql('DROP TRIGGER sup272_fail_insert ON api.usuarios; DROP FUNCTION api.sup272_fail_insert();')

# Redis outage: fail closed, independent liveness, same processes recover.
before=state('identidad')
compose('stop','redis')
try:
    request('/_identidad/ready',503,base=identity_url)
    request('/_identidad/live',base=identity_url)
    check_kubernetes_probes(ready=False)
    start=time.monotonic()
    body,_=request('/api/v1/auth/login',500,method='POST',data={'correo':'admin.sup267@example.test','password':'Sup267FixturePassword123!'})
    assert time.monotonic()-start<4 and 'detalle' not in body
    request('/api/v1/admin/productos',opener=new_client)
    request('/health') # Coordinación puede persistir en SQL sin Redis.
    request('/_gateway/ready')
finally: compose('up','-d','--no-deps','--wait','--wait-timeout','90','redis')
request('/_identidad/ready',base=identity_url)
check_kubernetes_probes()
assert state('identidad')==before

# Los mismos comandos distinguen caída SQL de caída del proceso y se recuperan.
compose('stop','db')
try:
    check_kubernetes_probes(ready=False)
finally: compose('up','-d','--no-deps','--wait','--wait-timeout','90','db')
check_kubernetes_probes()
assert state('identidad')==before

# Single-domain outages never cause fallback or withdraw the edge Gateway.
# Catálogo valida el JWT localmente: la caída de Identidad no bloquea su administración.
compose('stop','identidad')
try:
    body,_=request('/api/v1/auth/me',502,opener=new_client); assert body['error']=='identidad_unavailable'
    request('/api/v1/admin/productos',opener=new_client)
    request('/api/v1/scraper/trabajos',201,method='POST',data={'cadena_id':1})
    request('/_gateway/ready')
    body,_=request('/_gateway/dependencies',503); assert body['checks']['backend']=='ok' and body['checks']['catalogo']=='ok' and body['checks']['identidad']=='error'
finally: compose('up','-d','--no-deps','--wait','--wait-timeout','90','identidad')
request('/api/v1/auth/me',opener=new_client)
compose('stop','control_scraping')
try:
    request('/api/v1/auth/me',opener=new_client)
    request('/api/v1/admin/productos',opener=new_client)
    request('/_gateway/ready')
    body,_=request('/_gateway/dependencies',503); assert body['checks']['identidad']=='ok' and body['checks']['backend']=='error'
finally: compose('up','-d','--no-deps','--wait','--wait-timeout','90','control_scraping')

# Independent replicas: only Identity is recreated/scaled; sessions survive.
api_state,gateway_state=state('control_scraping'),state('gateway')
compose('up','-d','--no-deps','--scale','identidad=3','--wait','--wait-timeout','90','identidad')
assert len(compose('ps','-q','identidad').split())==3
assert state('control_scraping')==api_state and state('gateway')==gateway_state
for _ in range(12): request('/api/v1/auth/me',opener=new_client)
reset_login_counters()
def concurrent_login(_):
    req=urllib.request.Request(args.gateway_url+'/api/v1/auth/login',method='POST',headers={'Content-Type':'application/json'},data=json.dumps({'correo':'unknown@example.test','password':'WrongPassword123!'}).encode())
    try: response=client().open(req,timeout=15)
    except urllib.error.HTTPError as e: response=e
    with response: response.read(); return response.status
with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool: statuses=list(pool.map(concurrent_login,range(8)))
assert statuses.count(401)==5 and statuses.count(429)==3, statuses
passed+=8
reset_login_counters()
request('/api/v1/auth/me',method='PUT',data={'telefono':'+56927227227'},opener=registered)
compose('up','-d','--no-deps','--scale','identidad=1','--wait','--wait-timeout','90','identidad')

# Rollback to approved SUP-271 requires old image plus DSN and route switch.
env['SUP272_LEGACY_API_IMAGE'] = 'sup271-api:reference'
compose('-f',str(root/'docker-compose.identidad-rollback.yml'),'up','-d','--no-deps','--wait','--wait-timeout','90','control_scraping','gateway')
compose('stop','identidad')
try:
    request('/api/v1/auth/me',opener=new_client)
    # El rollback sustituye Identidad, no Catálogo: la lectura administrativa
    # sigue disponible porque Catálogo valida el JWT localmente.
    request('/api/v1/admin/productos',opener=new_client)
    request('/api/v1/auth/register',201,method='POST',data={'correo':'sup272.rollback@example.test','password':'Sup272FixturePassword123!','nombre_completo':'Rollback Fixture'})
finally:
    compose('up','-d','--no-deps','--wait','--wait-timeout','90','identidad','control_scraping','gateway')
request('/api/v1/auth/me',opener=new_client)

# La configuración de cookie Secure también se verifica con el proceso real.
reset_login_counters()
env['IDENTIDAD_COOKIE_SECURE'] = 'true'
compose('up','-d','--no-deps','--wait','--wait-timeout','90','identidad')
try:
    _,headers=request('/api/v1/auth/login',method='POST',data={'correo':'admin.sup267@example.test','password':'Sup267FixturePassword123!'})
    assert 'Secure' in headers.get('Set-Cookie','') and 'SameSite=Lax' in headers.get('Set-Cookie','')
finally:
    env.pop('IDENTIDAD_COOKIE_SECURE')
    compose('up','-d','--no-deps','--wait','--wait-timeout','90','identidad')
request('/api/v1/auth/me',opener=new_client)

# Missing grants cannot silently serve HTTP on startup.
previous=compose('logs','--no-color','identidad')
sql('REVOKE UPDATE ON api.usuarios FROM app_identity;')
try:
    compose('restart','identidad')
    wait(lambda:'identidad_storage_invalid' in compose('logs','--no-color','identidad')[len(previous):])
    compose('stop','identidad')
finally:
    sql('GRANT UPDATE ON api.usuarios TO app_identity;')
    compose('up','-d','--no-deps','--wait','--wait-timeout','90','identidad')
request('/_identidad/ready',base=identity_url)
request('/api/v1/auth/me',opener=new_client)

# Runtime separation: inspect only environment variable NAMES.
for service,forbidden in [('control_scraping',{'IDENTITY_DB_URL','IDENTITY_DB_PASSWORD','GOOGLE_CLIENT_ID','JWT_SECRET'}),('identidad',{'SCRAPING_DB_URL','CATALOGO_INTERNAL_TOKEN','DB_URL','DB_PASSWORD'})]:
    identifier=compose('ps','-q',service).strip()
    names=json.loads(subprocess.run(['docker','inspect',identifier],check=True,text=True,stdout=subprocess.PIPE).stdout)[0]['Config']['Env']
    assert not forbidden.intersection(value.split('=',1)[0] for value in names), 'Credencial de otro dominio en runtime'
reset_login_counters()
print(json.dumps({'task':'SUP-272','passed':passed,'failed':0,'contracts':'legacy and new sessions compatible',
    'security':'JWT/CORS/IDOR/SSO nil/rate IP spoof/concurrent register/SQL errors',
    'isolation':'independent outages and three Identity replicas','rollback':'SUP-271 and back'}))
