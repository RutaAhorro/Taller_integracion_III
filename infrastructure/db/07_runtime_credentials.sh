#!/usr/bin/env bash
# Sólo inicialización de un volumen nuevo. Para uno existente, ejecutar
# expresamente tras 06_propiedad_permisos.sql con el operador PostgreSQL.
# El entrypoint puede ejecutar o sourcear este archivo (ConfigMap 0644).
# El subshell evita modificar sus opciones/variables cuando se sourcea.
(
set -euo pipefail
for credential in IDENTITY_DB_PASSWORD SCRAPING_DB_PASSWORD CATALOGO_DB_PASSWORD; do
  credential_value=${!credential:-}
  if (( ${#credential_value} < 32 )); then
    echo "$credential debe contener al menos 32 caracteres." >&2
    exit 1
  fi
done
unset credential_value
if [[ "$IDENTITY_DB_PASSWORD" == "$SCRAPING_DB_PASSWORD" || "$IDENTITY_DB_PASSWORD" == "$CATALOGO_DB_PASSWORD" ||
      "$SCRAPING_DB_PASSWORD" == "$CATALOGO_DB_PASSWORD" || "$IDENTITY_DB_PASSWORD" == "${POSTGRES_PASSWORD:-}" ||
      "$SCRAPING_DB_PASSWORD" == "${POSTGRES_PASSWORD:-}" || "$CATALOGO_DB_PASSWORD" == "${POSTGRES_PASSWORD:-}" ]]; then
  echo 'Las tres contraseñas runtime deben ser distintas entre sí y del administrador.' >&2
  exit 1
fi
# SUP-240: el motor de rutas es opcional; sin RUTAS_DB_PASSWORD, app_rutas queda sin LOGIN.
rutas_password=${RUTAS_DB_PASSWORD:-}
if [[ -n "$rutas_password" ]]; then
  if (( ${#rutas_password} < 32 )); then
    echo "RUTAS_DB_PASSWORD debe contener al menos 32 caracteres." >&2
    exit 1
  fi
  for other in "$IDENTITY_DB_PASSWORD" "$SCRAPING_DB_PASSWORD" "$CATALOGO_DB_PASSWORD" "${POSTGRES_PASSWORD:-}"; do
    if [[ "$rutas_password" == "$other" ]]; then
      echo 'RUTAS_DB_PASSWORD debe ser distinta de las demás contraseñas.' >&2
      exit 1
    fi
  done
fi
unset rutas_password other
psql -X --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" --set ON_ERROR_STOP=1 <<'SQL'
\getenv identity_password IDENTITY_DB_PASSWORD
\getenv scraping_password SCRAPING_DB_PASSWORD
\getenv catalogo_password CATALOGO_DB_PASSWORD
\set rutas_password ''
\getenv rutas_password RUTAS_DB_PASSWORD
BEGIN;
SET LOCAL password_encryption = 'scram-sha-256';
SELECT format('ALTER ROLE app_identity LOGIN PASSWORD %L', :'identity_password') \gexec
SELECT format('ALTER ROLE app_scraping LOGIN PASSWORD %L', :'scraping_password') \gexec
SELECT format('ALTER ROLE app_catalogo LOGIN PASSWORD %L', :'catalogo_password') \gexec
SELECT format('ALTER ROLE app_rutas LOGIN PASSWORD %L', :'rutas_password') WHERE :'rutas_password' <> '' \gexec
COMMIT;
SQL
)
