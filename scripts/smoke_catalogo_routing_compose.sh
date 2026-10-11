#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
test_dir=$(mktemp -d)
test_project="sup268-$(python3 -c 'import uuid; print(uuid.uuid4().hex[:10])')"
test_env="$test_dir/test.env"
test_ca=""
export SUP268_INIT_DIR="$test_dir/db-init"
export SUP268_REFERENCE_API_DIR="$test_dir/reference-api"
export SUP268_LEGACY_API_IMAGE=sup267-api:local
export SUP269_LEGACY_API_IMAGE=sup268-api:local
reference_ref="${SUP268_REFERENCE_REF:-1ae1f8662e1d0fa972b04700ec7a84c9e037565c}"
export SUP269_REFERENCE_API_DIR="$test_dir/sup268-api"
ingesta_reference_ref="${SUP269_REFERENCE_REF:-b465d0ad278830a47a0c84a532e7d3f8a6ca7397}"

compose() {
  env -u DB_USER -u DB_PASSWORD -u DB_NAME -u JWT_SECRET -u GOOGLE_CLIENT_ID \
    -u CATALOGO_CORS_ALLOWED_ORIGINS -u GATEWAY_TRUSTED_PROXIES -u SUP268_READER_PASSWORD \
    -u CATALOGO_INTERNAL_TOKEN -u SUP269_COORDINATOR_PASSWORD \
    -u IDENTITY_DB_PASSWORD -u SCRAPING_DB_PASSWORD -u CATALOGO_DB_PASSWORD \
    docker compose --project-name "$test_project" --env-file "$test_env" \
    -f "$repo_root/docker-compose.yml" -f "$repo_root/docker-compose.catalogo-routing-test.yml" "$@"
}
finish() {
  status=$?
  if (( status != 0 )); then
    compose logs --no-color --tail 40 go_service catalogo gateway frontend || true
    # Diagnóstico de init sin volcar credenciales ni el log SQL completo.
    compose exec -T db psql -X -U sup268 -d sup268 -tA -c \
      "SELECT rolname,rolcanlogin,rolpassword IS NOT NULL FROM pg_authid WHERE rolname IN ('app_identity','app_scraping','app_catalogo');" || true
    compose logs --no-color db | python3 -c 'import sys; print("".join(line for line in sys.stdin if "running /docker-entrypoint-initdb.d/" in line or "sourcing /docker-entrypoint-initdb.d/" in line or "ERROR:" in line or "debe contener al menos" in line or "contraseñas runtime deben" in line))' || true
  fi
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$test_dir"
  if [[ -n "$test_ca" ]]; then rm -f "$test_ca"; fi
  exit "$status"
}
trap finish EXIT

mkdir -p "$SUP268_INIT_DIR" "$SUP268_REFERENCE_API_DIR" "$SUP269_REFERENCE_API_DIR"
git -C "$repo_root" archive "$reference_ref" backend/api | tar -x -C "$SUP268_REFERENCE_API_DIR" --strip-components=2
git -C "$repo_root" archive "$ingesta_reference_ref" backend/api | tar -x -C "$SUP269_REFERENCE_API_DIR" --strip-components=2
additional_builds=()
if [[ "${SUP271_OWNERSHIP:-0}" == 1 ]]; then
  export SUP271_REFERENCE_API_DIR="$test_dir/sup270-api"
  mkdir -p "$SUP271_REFERENCE_API_DIR"
  git -C "$repo_root" archive "${SUP271_REFERENCE_REF:-42b6ca448875f03e63e31d51f44fccc5cee3de91}" backend/api | tar -x -C "$SUP271_REFERENCE_API_DIR" --strip-components=2
  additional_builds=(reference_permissions_api)
fi
chmod 755 "$test_dir" "$SUP268_INIT_DIR"
cp "$repo_root"/infrastructure/db/*.sql "$SUP268_INIT_DIR/"
cp "$repo_root/infrastructure/db/07_runtime_credentials.sh" "$SUP268_INIT_DIR/"
cp "$repo_root/backend/catalogo/tests/fixtures.sql" "$SUP268_INIT_DIR/zz_sup268_fixtures.sql"
python3 - "$test_env" <<'PY'
from pathlib import Path
import secrets, sys
password, reader, jwt, internal, coordinator, identity = (secrets.token_hex(24) for _ in range(6))
p = Path(sys.argv[1])
p.write_text('DB_USER=sup268\nDB_NAME=sup268\nDB_PASSWORD=' + password + '\nJWT_SECRET=' + jwt +
    '\nCATALOGO_DB_PASSWORD=' + reader + '\nCATALOGO_INTERNAL_TOKEN=' + internal +
    '\nSCRAPING_DB_PASSWORD=' + coordinator + '\nIDENTITY_DB_PASSWORD=' + identity +
    '\nGOOGLE_CLIENT_ID=\nGATEWAY_TRUSTED_PROXIES=\n' +
    'CATALOGO_CORS_ALLOWED_ORIGINS=https://frontend.example,http://localhost:3000\n')
p.chmod(0o600)
PY
chmod 644 "$SUP268_INIT_DIR"/*.sql
chmod 755 "$SUP268_INIT_DIR/07_runtime_credentials.sh"
source_ca="${SUP268_CA_BUNDLE:-/etc/ssl/certs/ca-certificates.crt}"
if [[ ! -s "$source_ca" ]]; then echo 'SUP268_CA_BUNDLE debe ser un bundle CA no vacío.' >&2; exit 1; fi
test_ca=$(mktemp "$repo_root/.sup268-ca.XXXXXXXX.crt")
cp "$source_ca" "$test_ca"
export SUP268_CA_BUNDLE="./${test_ca##*/}"

compose config --quiet
if [[ "${SUP268_SKIP_BUILD:-0}" != 1 ]]; then
  compose build --build-arg HTTP_PROXY --build-arg HTTPS_PROXY --build-arg NO_PROXY \
    --build-arg http_proxy --build-arg https_proxy --build-arg no_proxy \
    go_service catalogo gateway frontend reference_api reference_ingesta_api "${additional_builds[@]}"
fi
compose up -d --wait --wait-timeout 180 db redis
compose exec -T db psql -X -U sup268 -d sup268 -v ON_ERROR_STOP=1 <<'SQL'
DO $$ BEGIN
  IF (SELECT count(*) FROM pg_authid WHERE rolname IN ('app_identity','app_scraping','app_catalogo')
      AND rolcanlogin AND rolpassword IS NOT NULL) <> 3 THEN
    RAISE EXCEPTION 'La inicialización no provisionó los tres roles runtime: revisar scripts 06/07';
  END IF;
END $$;
SQL
compose up -d --wait --wait-timeout 180 go_service catalogo gateway frontend reference_api
api_url="http://127.0.0.1:${SUP268_API_PORT:-18080}"
gateway_url="http://127.0.0.1:${SUP268_GATEWAY_PORT:-18082}"
frontend_url="http://127.0.0.1:${SUP268_FRONTEND_PORT:-13000}"
reference_url="http://127.0.0.1:${SUP268_REFERENCE_PORT:-18084}"
smoke="$repo_root/backend/gateway/tests/smoke_catalogo_routing.py"
python3 "$repo_root/backend/catalogo/tests/smoke_contracts.py" --api-url "$reference_url" --catalogo-url "$gateway_url"
python3 "$smoke" --session-file "$test_dir/admin.cookies" --api-url "$api_url" --gateway-url "$gateway_url" --frontend-url "$frontend_url" --mode normal
python3 "$repo_root/backend/catalogo/tests/smoke_ingesta.py" --gateway-url "$gateway_url" --reference-url "$reference_url" --state-file "$test_dir/ingesta.json" --mode normal
compose exec -T db psql -U sup268 -d sup268 -v ON_ERROR_STOP=1 <<'SQL'
CREATE FUNCTION scraper.sup269_fail_counter() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.elementos_extraidos > OLD.elementos_extraidos THEN RAISE EXCEPTION 'SUP269 simulated counter failure'; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER sup269_fail_counter BEFORE UPDATE ON scraper.trabajos_scraper
FOR EACH ROW EXECUTE FUNCTION scraper.sup269_fail_counter();
SQL
python3 "$repo_root/backend/catalogo/tests/smoke_ingesta.py" --gateway-url "$gateway_url" --state-file "$test_dir/ingesta.json" --mode counter-failure
compose exec -T db psql -U sup268 -d sup268 -v ON_ERROR_STOP=1 <<'SQL'
DROP TRIGGER sup269_fail_counter ON scraper.trabajos_scraper;
DROP FUNCTION scraper.sup269_fail_counter();
SQL
compose exec -T db psql -U sup268 -d sup268 -v ON_ERROR_STOP=1 < "$repo_root/backend/catalogo/tests/verify_ingesta_ownership.sql"
if [[ "${SUP271_OWNERSHIP:-0}" == 1 ]]; then
  python3 "$repo_root/backend/api/tests/smoke_data_ownership.py" \
    --project "$test_project" --env-file "$test_env" --gateway-url "$gateway_url"
fi
if [[ "${SUP270_RECOVERY:-0}" == 1 ]]; then
  PYTHONPATH="$repo_root/backend/scraper" python3 "$repo_root/backend/scraper/research/smoke_recovery.py" \
    --project "$test_project" --env-file "$test_env" --gateway-url "$gateway_url" --state-dir "$test_dir/worker-state"
fi

api_before=$(compose ps -q go_service)
gateway_before=$(compose ps -q gateway)
compose up -d --no-deps --scale catalogo=3 --wait --wait-timeout 90 catalogo
test "$(compose ps -q catalogo | wc -l)" -eq 3
test "$(compose ps -q go_service)" = "$api_before"
test "$(compose ps -q gateway)" = "$gateway_before"
if [[ "${SUP270_RECOVERY:-0}" == 1 ]]; then
  PYTHONPATH="$repo_root/backend/scraper" python3 "$repo_root/backend/scraper/research/smoke_recovery.py" \
    --project "$test_project" --env-file "$test_env" --gateway-url "$gateway_url" --state-dir "$test_dir/worker-state" --replicas-only
fi
python3 "$smoke" --session-file "$test_dir/admin.cookies" --gateway-url "$gateway_url" --mode load
compose logs --no-color catalogo > "$test_dir/catalogo.log"
python3 - "$test_dir/catalogo.log" <<'PY'
from pathlib import Path
import json, sys
pods = {line.split('|')[0].strip() for line in Path(sys.argv[1]).read_text().splitlines()
        if 'sup268-load-' in line and 'http_request' in line}
assert pods, 'Load did not reach Catalog'
print(json.dumps({'catalog_replicas': 3, 'replicas_observed_serving_load': len(pods),
                  'api_replicas': 1, 'gateway_replicas': 1}))
PY
compose stop catalogo >/dev/null
python3 "$smoke" --session-file "$test_dir/admin.cookies" --gateway-url "$gateway_url" --mode catalog-down
python3 "$repo_root/backend/catalogo/tests/smoke_ingesta.py" --gateway-url "$gateway_url" --state-file "$test_dir/ingesta.json" --mode catalog-down
compose start catalogo >/dev/null
compose up -d --no-deps --scale catalogo=3 --wait --wait-timeout 90 catalogo
compose stop go_service reference_api redis >/dev/null
python3 "$smoke" --session-file "$test_dir/admin.cookies" --gateway-url "$gateway_url" --frontend-url "$frontend_url" --mode api-down

compose up -d --wait --wait-timeout 90 redis
compose -f "$repo_root/docker-compose.catalogo-ingesta-rollback.yml" up -d --wait --wait-timeout 180 go_service
python3 "$repo_root/backend/catalogo/tests/smoke_ingesta.py" --gateway-url "$gateway_url" --state-file "$test_dir/ingesta.json" --mode ingesta-rollback
compose -f "$repo_root/docker-compose.catalogo-rollback.yml" up -d --wait --wait-timeout 180 go_service gateway
compose -f "$repo_root/docker-compose.catalogo-rollback.yml" up -d --no-deps --force-recreate --wait --wait-timeout 60 frontend
compose stop catalogo >/dev/null
python3 "$smoke" --session-file "$test_dir/admin.cookies" --api-url "$api_url" --gateway-url "$gateway_url" --frontend-url "$frontend_url" --mode rollback
