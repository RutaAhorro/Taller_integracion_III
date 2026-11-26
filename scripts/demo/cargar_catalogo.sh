#!/usr/bin/env bash
# ==============================================================================
# scripts/demo/cargar_catalogo.sh
#
# Carga un dump `.sql.gz` de catálogo (esquema `scraper.*`) en el contenedor
# PostgreSQL de Docker Compose de forma segura, transaccional y repetible.
#
# Validaciones previas a la carga:
#   - Integridad gzip (`gzip -t`).
#   - Solo bloques `COPY scraper.<tabla> ... FROM stdin;` (además de comentarios
#     y directivas `SET` / `SELECT pg_catalog.set_config` de pg_dump).
#   - Rechaza cualquier línea con `\!`, `\i`, `\o`, `DROP`, `ALTER`, `CREATE`,
#     `GRANT`, `DO` o tablas `api.*`.
#   - Si el último bloque COPY quedó sin cerrar (falta `\.`), lo cierra al cargar.
#
# Carga en UNA transacción:
#   TRUNCATE scraper.cadenas_supermercado CASCADE -> COPY -> limpieza de capturas
#   con precio_normal < 100 y productos sin captura.
# ==============================================================================

set -euo pipefail

# ===== Constantes =====

readonly DIRECTORIO_RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly PRECIO_MINIMO_VALIDO=100

# ===== Funciones auxiliares =====

# mostrar_uso imprime la forma correcta de invocar el script.
mostrar_uso() {
  printf "Uso: bash scripts/demo/cargar_catalogo.sh <ruta_al_archivo.sql.gz>\n" >&2
}

# validar_dump verifica gzip y revisa el SQL descomprimido con awk.
# Devuelve:
#   0 -> válido y cerrado correctamente
#   2 -> válido, pero falta la línea `\.` al final del último bloque COPY
#   1 -> inválido o rechazado por seguridad
validar_dump() {
  local ruta_archivo="$1"

  if ! gzip -t "${ruta_archivo}" 2>/dev/null; then
    printf "ERROR: El archivo no es un gzip válido o está dañado: %s\n" "${ruta_archivo}" >&2
    return 1
  fi

  local estado_awk=0
  gzip -dc "${ruta_archivo}" | awk '
    BEGIN {
      en_copia = 0
      cantidad_copias = 0
      error_detectado = 0
    }
    # Bloquear metacomandos peligrosos de psql en cualquier parte del archivo.
    /\\!|\\i[[:space:]]|\\o[[:space:]]|^\\(i|o)$/ {
      printf("ERROR seguridad (línea %d): metacomando psql no permitido.\n", NR) > "/dev/stderr"
      error_detectado = 1
      exit 1
    }
    # Dentro de un bloque COPY solo se permiten filas de datos hasta el cierre `\.`.
    en_copia == 1 {
      if ($0 == "\\.") {
        en_copia = 0
      }
      next
    }
    # Fuera de COPY: permitir líneas vacías y comentarios SQL.
    /^[[:space:]]*$/ || /^[[:space:]]*--/ {
      next
    }
    # Fuera de COPY: rechazar sentencias peligrosas o referencias a api.*.
    toupper($0) ~ /(^|[^A-Z0-9_])(DROP|ALTER|CREATE|GRANT|DO)([^A-Z0-9_]|$)/ || tolower($0) ~ /(^|[^a-z0-9_])api\./ {
      printf("ERROR seguridad (línea %d): sentencia o esquema no permitido: %s\n", NR, substr($0, 1, 100)) > "/dev/stderr"
      error_detectado = 1
      exit 1
    }
    # Fuera de COPY: aceptar únicamente COPY de tablas scraper.* desde stdin.
    /^COPY[[:space:]]+scraper\.[a-zA-Z0-9_]+.*FROM[[:space:]]+stdin;[[:space:]]*$/ {
      en_copia = 1
      cantidad_copias++
      next
    }
    # Fuera de COPY: aceptar directivas benignas generadas por pg_dump.
    /^SET[[:space:]]+[a-zA-Z0-9_]+[[:space:]]*=/ || /^SELECT[[:space:]]+pg_catalog\.set_config\(/ {
      next
    }
    # Cualquier otra sentencia SQL fuera de COPY se rechaza.
    {
      printf("ERROR validación (línea %d): línea no permitida fuera de COPY scraper.*: %s\n", NR, substr($0, 1, 100)) > "/dev/stderr"
      error_detectado = 1
      exit 1
    }
    END {
      if (error_detectado == 1) {
        exit 1
      }
      if (cantidad_copias == 0) {
        printf("ERROR validación: el archivo no contiene ningún bloque COPY scraper.*.\n") > "/dev/stderr"
        exit 1
      }
      if (en_copia == 1) {
        exit 2
      }
    }
  ' || estado_awk=$?

  return "${estado_awk}"
}

# construir_flujo_sql genera la transacción completa por stdout para psql.
construir_flujo_sql() {
  local ruta_archivo="$1"
  local falta_cierre_copia="$2"

  printf "BEGIN;\n"
  printf "TRUNCATE scraper.cadenas_supermercado CASCADE;\n"
  if [[ "${falta_cierre_copia}" -eq 1 ]]; then
    gzip -dc "${ruta_archivo}" | awk '1; END { print "\\." }'
  else
    gzip -dc "${ruta_archivo}"
  fi
  printf "\nDELETE FROM scraper.capturas_precios WHERE precio_normal < %d;\n" "${PRECIO_MINIMO_VALIDO}"
  printf "DELETE FROM scraper.productos_crudos AS producto WHERE NOT EXISTS (SELECT 1 FROM scraper.capturas_precios AS captura WHERE captura.producto_crudo_id = producto.id);\n"
  printf "COMMIT;\n"
}

# ===== Ejecución principal =====

if [[ "$#" -ne 1 ]]; then
  mostrar_uso
  exit 1
fi

readonly RUTA_DUMP="$1"

if [[ ! -f "${RUTA_DUMP}" ]]; then
  printf "ERROR: No existe el archivo: %s\n" "${RUTA_DUMP}" >&2
  exit 1
fi

printf "Validando dump %s ...\n" "${RUTA_DUMP}"
ESTADO_VALIDACION=0
validar_dump "${RUTA_DUMP}" || ESTADO_VALIDACION=$?

FALTA_CIERRE=0
if [[ "${ESTADO_VALIDACION}" -eq 0 ]]; then
  printf "[OK] Validación aprobada (COPY scraper.* cerrado correctamente).\n"
elif [[ "${ESTADO_VALIDACION}" -eq 2 ]]; then
  FALTA_CIERRE=1
  printf "[OK] Validación aprobada (el último bloque COPY venía sin cierre '\\.'; se agregará al cargar).\n"
else
  printf "[RECHAZADO] El archivo no pasó la validación y no se modificó la base de datos.\n" >&2
  exit 1
fi

printf "Cargando catálogo en una sola transacción vía Docker Compose ...\n"
cd "${DIRECTORIO_RAIZ}"

construir_flujo_sql "${RUTA_DUMP}" "${FALTA_CIERRE}" | \
  docker compose exec -T db sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' >/dev/null

printf "[OK] Catálogo cargado y limpiado correctamente.\n"
docker compose exec -T db sh -c \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT (SELECT count(*) FROM scraper.cadenas_supermercado) AS cadenas, (SELECT count(*) FROM scraper.productos_crudos) AS productos, (SELECT count(*) FROM scraper.capturas_precios) AS capturas;"'
