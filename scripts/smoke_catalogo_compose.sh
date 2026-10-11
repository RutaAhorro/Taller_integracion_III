#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
test_dir=$(mktemp -d)
test_project="sup267-$(python3 -c 'import uuid; print(uuid.uuid4().hex[:10])')"
test_env="$test_dir/test.env"
test_ca=""
export SUP267_INIT_DIR="$test_dir/db-init"
export SUP267_REFERENCE_API_DIR="$test_dir/reference-api"
# La API actual ya no tiene lector. Comparar contra el merge de SUP-267.
reference_ref="${SUP267_REFERENCE_REF:-1ae1f8662e1d0fa972b04700ec7a84c9e037565c}"

compose() {
  env -u SUP267_DB_PASSWORD -u SUP267_READER_PASSWORD -u SUP267_JWT_SECRET \
    docker compose --project-name "$test_project" --env-file "$test_env" \
    -f "$repo_root/docker-compose.catalogo-test.yml" "$@"
}
cleanup() {
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$test_dir"
  if [[ -n "$test_ca" ]]; then rm -f "$test_ca"; fi
}
finish() {
  status=$?
  if (( status != 0 )); then
    compose logs --no-color --tail 40 api catalogo || true
  fi
  cleanup
  exit "$status"
}
trap finish EXIT

mkdir -p "$SUP267_REFERENCE_API_DIR"
git -C "$repo_root" archive "$reference_ref" backend/api | tar -x -C "$SUP267_REFERENCE_API_DIR" --strip-components=2

mkdir -p "$SUP267_INIT_DIR"
chmod 755 "$test_dir" "$SUP267_INIT_DIR"
cp "$repo_root"/infrastructure/db/*.sql "$SUP267_INIT_DIR/"
cp "$repo_root/backend/catalogo/tests/fixtures.sql" "$SUP267_INIT_DIR/zz_sup267_fixtures.sql"
python3 - "$test_env" "$SUP267_INIT_DIR/zzz_catalog_reader.sql" <<'PY'
from pathlib import Path
import secrets, sys
password, reader, jwt = (secrets.token_hex(24) for _ in range(3))
env = Path(sys.argv[1])
env.write_text('SUP267_DB_PASSWORD=' + password + '\nSUP267_READER_PASSWORD=' + reader + '\nSUP267_JWT_SECRET=' + jwt + '\n')
env.chmod(0o600)
Path(sys.argv[2]).write_text("CREATE ROLE sup267_catalog_reader LOGIN PASSWORD '" + reader + "';\n"
    "GRANT CONNECT ON DATABASE sup267 TO sup267_catalog_reader;\n"
    "GRANT USAGE ON SCHEMA scraper TO sup267_catalog_reader;\n"
    "GRANT SELECT ON scraper.productos_crudos, scraper.capturas_precios, "
    "scraper.cadenas_supermercado, scraper.sucursales_supermercado TO sup267_catalog_reader;\n")
PY
chmod 644 "$SUP267_INIT_DIR"/*.sql

# Respeta el origen y utiliza una copia única dentro del contexto permitido a
# BuildKit. El cleanup sólo elimina el archivo creado por esta ejecución.
source_ca="${SUP267_CA_BUNDLE:-/etc/ssl/certs/ca-certificates.crt}"
if [[ ! -s "$source_ca" ]]; then
  echo 'SUP267_CA_BUNDLE debe apuntar a un bundle CA existente y no vacío.' >&2
  exit 1
fi
test_ca=$(mktemp "$repo_root/.sup267-ca.XXXXXXXX.crt")
cp "$source_ca" "$test_ca"
export SUP267_CA_BUNDLE="./${test_ca##*/}"
compose config --quiet
if [[ "${SUP267_SKIP_BUILD:-0}" != 1 ]]; then
  compose build --build-arg HTTP_PROXY --build-arg HTTPS_PROXY --build-arg NO_PROXY \
    --build-arg http_proxy --build-arg https_proxy --build-arg no_proxy api catalogo
fi
compose up -d --wait --wait-timeout 180

api_url="http://127.0.0.1:${SUP267_API_PORT:-18084}"
catalogo_url="http://127.0.0.1:${SUP267_CATALOGO_PORT:-18083}"
python3 "$repo_root/backend/catalogo/tests/smoke_contracts.py" \
  --api-url "$api_url" --catalogo-url "$catalogo_url"

# El usuario del proceso Catálogo sólo puede leer las cuatro tablas consultadas.
compose exec -T db psql -U sup267 -d sup267 -v ON_ERROR_STOP=1 <<'SQL'
DO $$ BEGIN
  IF has_table_privilege('sup267_catalog_reader', 'api.usuarios', 'SELECT')
     OR has_table_privilege('sup267_catalog_reader', 'scraper.productos_crudos', 'INSERT')
     OR has_table_privilege('sup267_catalog_reader', 'scraper.capturas_precios', 'UPDATE') THEN
    RAISE EXCEPTION 'Catalog reader has unexpected privileges';
  END IF;
END $$;
SQL

# Catálogo sirve datos con API y Redis detenidos, y conserva liveness sin DB.
compose stop api redis >/dev/null
python3 "$repo_root/backend/catalogo/tests/smoke_contracts.py" \
  --catalogo-url "$catalogo_url" --catalogo-only
compose stop db >/dev/null
python3 "$repo_root/backend/catalogo/tests/smoke_contracts.py" \
  --catalogo-url "$catalogo_url" --database-down
