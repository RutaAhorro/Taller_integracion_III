#!/usr/bin/env bash
# ==============================================================================
# scripts/demo/probar_ruta.sh
#
# Valida contra el stack local (Gateway + Motor de rutas + Catálogo + PostgreSQL)
# que POST /api/v1/rutas/optimizar calcula una combinación multi-tienda coherente
# usando datos reales del catálogo.
#
# Flujo:
#   1. Consulta de SOLO LECTURA a la BD local (docker exec bd_supermercados psql,
#      sin imprimir credenciales) para elegir hasta 6 productos de categorías
#      comunes (arroz, aceite, fideos, leche, azúcar, harina/salsa) cuyo EAN
#      exista en al menos 3 cadenas con precio_normal >= 100.
#   2. Genera una cookie JWT HS256 de prueba (reutilizando la lógica de
#      scripts/recorrido_mvp.sh, sin imprimir secretos ni cookies).
#   3. Crea una lista de prueba, agrega los productos seleccionados por su
#      producto_id de Catálogo y llama a POST /api/v1/rutas/optimizar.
#   4. Imprime el JSON real del endpoint y un resumen de paradas, ahorro frente
#      a una sola tienda y orden de visita.
#
# Variables opcionales:
#   GATEWAY_URL        URL base del Gateway (por defecto: http://127.0.0.1:8082)
#   CONTENEDOR_BD      Nombre del contenedor PostgreSQL (por defecto: bd_supermercados)
#   NOMBRE_LISTA_DEMO  Nombre de la lista de prueba
# ==============================================================================

set -euo pipefail

# ===== Constantes de configuración =====

readonly DIRECTORIO_RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly URL_BASE_GATEWAY="${GATEWAY_URL:-http://127.0.0.1:8082}"
readonly NOMBRE_CONTENEDOR_BD="${CONTENEDOR_BD:-bd_supermercados}"
readonly NOMBRE_LISTA="${NOMBRE_LISTA_DEMO:-Lista demo multi-tienda}"
readonly TIEMPO_ESPERA_SEGUNDOS=25
readonly LONGITUD_MINIMA_JWT_SECRET=32
readonly PRECIO_MINIMO_VALIDO=100
readonly ROL_PRUEBA="registrado"
readonly PROVEEDOR_PRUEBA="google"

# ===== Archivos temporales y limpieza =====

ARCHIVO_CUERPO="$(mktemp /tmp/probar_ruta_cuerpo.XXXXXX)"
ARCHIVO_PRODUCTOS="$(mktemp /tmp/probar_ruta_productos.XXXXXX)"

# limpiar_temporales elimina archivos temporales al finalizar para no dejar
# respuestas ni metadatos en /tmp.
limpiar_temporales() {
  rm -f "${ARCHIVO_CUERPO}" "${ARCHIVO_PRODUCTOS}"
}
trap limpiar_temporales EXIT

# ===== Funciones auxiliares =====

# leer_variable_env obtiene el valor de una clave desde el entorno o desde .env
# sin imprimirla ni exportar el archivo completo.
leer_variable_env() {
  local nombre_clave="$1"
  local archivo_env="${DIRECTORIO_RAIZ}/.env"

  python3 - "${nombre_clave}" "${archivo_env}" <<'PY'
import os
import sys

clave = sys.argv[1]
ruta_env = sys.argv[2]

valor = os.environ.get(clave, "").strip()
if not valor and os.path.isfile(ruta_env):
    with open(ruta_env, "r", encoding="utf-8") as archivo:
        for linea in archivo:
            linea_limpia = linea.strip()
            if not linea_limpia or linea_limpia.startswith("#"):
                continue
            if linea_limpia.startswith(clave + "="):
                crudo = linea_limpia.split("=", 1)[1].strip()
                if (crudo.startswith('"') and crudo.endswith('"')) or (
                    crudo.startswith("'") and crudo.endswith("'")
                ):
                    crudo = crudo[1:-1]
                valor = crudo
                break

if valor:
    sys.stdout.write(valor)
PY
}

# generar_cookie_jwt_prueba firma un JWT HS256 de prueba con JWT_SECRET del
# entorno o .env, replicando el formato aceptado por Identidad y Motor de rutas.
# Devuelve "<usuario_id>|<token>" por stdout sin registrar el token en pantalla.
generar_cookie_jwt_prueba() {
  local secreto_jwt
  secreto_jwt="$(leer_variable_env "JWT_SECRET")"

  python3 - "${secreto_jwt}" "${LONGITUD_MINIMA_JWT_SECRET}" "${ROL_PRUEBA}" "${PROVEEDOR_PRUEBA}" <<'PY'
import base64
import hashlib
import hmac
import json
import sys
import time
import uuid

secreto = sys.argv[1].strip()
longitud_minima = int(sys.argv[2])
rol = sys.argv[3]
proveedor = sys.argv[4]

if not secreto or len(secreto.encode("utf-8")) < longitud_minima:
    sys.exit(1)

def codificar_b64url(datos: bytes) -> str:
    return base64.urlsafe_b64encode(datos).rstrip(b"=").decode("ascii")

ahora = int(time.time())
usuario_id = str(uuid.uuid4())
encabezado = {"alg": "HS256", "typ": "JWT"}
carga = {
    "user_id": usuario_id,
    "rol": rol,
    "provider": proveedor,
    "iat": ahora,
    "nbf": ahora,
    "exp": ahora + 3600,
}

segmento_encabezado = codificar_b64url(json.dumps(encabezado, separators=(",", ":")).encode("utf-8"))
segmento_carga = codificar_b64url(json.dumps(carga, separators=(",", ":")).encode("utf-8"))
mensaje = f"{segmento_encabezado}.{segmento_carga}".encode("ascii")
firma = hmac.new(secreto.encode("utf-8"), mensaje, hashlib.sha256).digest()
token = f"{segmento_encabezado}.{segmento_carga}.{codificar_b64url(firma)}"

print(f"{usuario_id}|{token}")
PY
}

# seleccionar_productos_multicadena consulta PostgreSQL en modo SOLO LECTURA y
# escribe hasta 6 productos de categorías comunes presentes en 3+ cadenas con
# precio_normal >= 100 (formato TSV: categoria, ean, producto_id, titulo, precios).
seleccionar_productos_multicadena() {
  local usuario_bd
  local nombre_bd
  usuario_bd="$(leer_variable_env "DB_USER")"
  nombre_bd="$(leer_variable_env "DB_NAME")"
  usuario_bd="${usuario_bd:-postgres}"
  nombre_bd="${nombre_bd:-supermercados_db}"

  docker exec -i "${NOMBRE_CONTENEDOR_BD}" \
    psql -q -U "${usuario_bd}" -d "${nombre_bd}" -v ON_ERROR_STOP=1 -AtF $'\t' <<SQL
SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY;

WITH ultimas_capturas AS (
    SELECT DISTINCT ON (captura.producto_crudo_id)
           captura.producto_crudo_id,
           captura.precio_normal,
           captura.precio_oferta,
           captura.esta_disponible
    FROM scraper.capturas_precios AS captura
    WHERE captura.precio_normal >= ${PRECIO_MINIMO_VALIDO}
    ORDER BY captura.producto_crudo_id, captura.capturado_el DESC, captura.id DESC
),
productos_con_precio AS (
    SELECT producto.id AS producto_id,
           producto.ean_gtin,
           producto.titulo_crudo,
           cadena.nombre AS cadena,
           captura.precio_normal::int AS precio_normal,
           COALESCE(captura.precio_oferta, captura.precio_normal)::int AS precio_efectivo
    FROM scraper.productos_crudos AS producto
    JOIN scraper.sucursales_supermercado AS sucursal ON sucursal.id = producto.sucursal_id
    JOIN scraper.cadenas_supermercado AS cadena ON cadena.id = sucursal.cadena_id
    JOIN ultimas_capturas AS captura ON captura.producto_crudo_id = producto.id
    WHERE producto.ean_gtin IS NOT NULL
      AND length(trim(producto.ean_gtin)) >= 8
      AND COALESCE(producto.en_stock, true) = true
      AND COALESCE(captura.esta_disponible, true) = true
      AND sucursal.esta_activa = true
),
eans_tres_cadenas AS (
    SELECT ean_gtin,
           min(titulo_crudo) AS titulo,
           min(producto_id::text) AS producto_id,
           (max(precio_efectivo) - min(precio_efectivo)) AS diferencia_precio,
           string_agg(
             DISTINCT cadena || ':$' || precio_efectivo::text,
             ' | '
             ORDER BY cadena || ':$' || precio_efectivo::text
           ) AS precios_por_cadena
    FROM productos_con_precio
    GROUP BY ean_gtin
    HAVING count(DISTINCT cadena) >= 3
),
categorizados AS (
    SELECT CASE
             WHEN titulo ~* '^arroz|\barroz\b' THEN 'arroz'
             WHEN titulo ~* '^aceite|\baceite\b' AND titulo !~* '(sardina|at[uú]n|jurel|caballa)' THEN 'aceite'
             WHEN titulo ~* '(fideo|tallarin|spaghetti|espiral|corbatita|fettuc)' THEN 'fideos'
             WHEN titulo ~* '^leche|\bleche\b' AND titulo !~* '(crema|condensada|dulce)' THEN 'leche'
             WHEN titulo ~* '(az[uú]car|azucar)' AND titulo !~* '(sin az|cero az|0% az|libre de az)' THEN 'azucar'
             WHEN titulo ~* '^harina|\bharina\b' THEN 'harina'
             WHEN titulo ~* 'salsa de tomate' THEN 'salsa_tomate'
           END AS categoria,
           ean_gtin,
           producto_id,
           titulo,
           diferencia_precio,
           precios_por_cadena
    FROM eans_tres_cadenas
),
mejores_por_categoria AS (
    SELECT categoria,
           ean_gtin,
           producto_id,
           titulo,
           precios_por_cadena,
           row_number() OVER (
             PARTITION BY categoria
             ORDER BY diferencia_precio DESC, ean_gtin
           ) AS posicion
    FROM categorizados
    WHERE categoria IS NOT NULL
)
SELECT categoria, ean_gtin, producto_id, titulo, precios_por_cadena
FROM mejores_por_categoria
WHERE posicion = 1
ORDER BY array_position(
  ARRAY['arroz', 'aceite', 'fideos', 'leche', 'azucar', 'harina', 'salsa_tomate'],
  categoria
)
LIMIT 6;
SQL
}

# ejecutar_peticion realiza una llamada HTTP autenticada con la cookie JWT en
# memoria, guarda el cuerpo en ARCHIVO_CUERPO y devuelve el código HTTP.
ejecutar_peticion() {
  local metodo_http="$1"
  local ruta_recurso="$2"
  local cuerpo_json="$3"
  local codigo_http=""

  : > "${ARCHIVO_CUERPO}"

  local argumentos_curl=(
    --silent
    --show-error
    --max-time "${TIEMPO_ESPERA_SEGUNDOS}"
    --output "${ARCHIVO_CUERPO}"
    --write-out "%{http_code}"
    --request "${metodo_http}"
    --header "Content-Type: application/json"
    --cookie "jwt=${TOKEN_JWT_PRUEBA}"
  )

  if [[ -n "${cuerpo_json}" ]]; then
    argumentos_curl+=(--data "${cuerpo_json}")
  fi

  codigo_http="$(curl "${argumentos_curl[@]}" "${URL_BASE_GATEWAY}${ruta_recurso}" 2>/dev/null || true)"
  if [[ -z "${codigo_http}" ]]; then
    codigo_http="000"
  fi
  printf "%s" "${codigo_http}"
}

# ===== Ejecución principal =====

printf "=== Demo de optimización multi-tienda: %s ===\n" "${URL_BASE_GATEWAY}"

# ----- Paso 1: Selección de productos en BD local (solo lectura) -----
seleccionar_productos_multicadena > "${ARCHIVO_PRODUCTOS}"

CANTIDAD_PRODUCTOS="$(grep -c '.' "${ARCHIVO_PRODUCTOS}" || true)"
if [[ "${CANTIDAD_PRODUCTOS}" -lt 1 ]]; then
  printf "ERROR: No se encontraron productos comunes en 3+ cadenas con precio_normal >= %d.\n" "${PRECIO_MINIMO_VALIDO}" >&2
  exit 1
fi

printf "\n1. Productos seleccionados (consulta SOLO LECTURA en %s):\n" "${NOMBRE_CONTENEDOR_BD}"
while IFS=$'\t' read -r categoria ean_producto producto_id titulo_producto precios_cadenas; do
  printf "   - [%s] EAN %s | %s\n     producto_id=%s | %s\n" \
    "${categoria}" "${ean_producto}" "${titulo_producto}" "${producto_id}" "${precios_cadenas}"
done < "${ARCHIVO_PRODUCTOS}"

# ----- Paso 2: Cookie JWT de prueba (sin imprimir token) -----
RESULTADO_JWT="$(generar_cookie_jwt_prueba || true)"
if [[ -z "${RESULTADO_JWT}" || "${RESULTADO_JWT}" != *"|"* ]]; then
  printf "ERROR: Falta JWT_SECRET (>= %d bytes) en entorno o .env.\n" "${LONGITUD_MINIMA_JWT_SECRET}" >&2
  exit 1
fi
USUARIO_ID_PRUEBA="${RESULTADO_JWT%%|*}"
TOKEN_JWT_PRUEBA="${RESULTADO_JWT#*|}"
printf "\n2. Sesión de prueba preparada (user_id=%s, rol=%s).\n" "${USUARIO_ID_PRUEBA}" "${ROL_PRUEBA}"

# ----- Paso 3: Crear lista y agregar productos -----
CUERPO_LISTA="$(printf '{"nombre":"%s"}' "${NOMBRE_LISTA}")"
CODIGO_LISTA="$(ejecutar_peticion "POST" "/api/v1/listas" "${CUERPO_LISTA}")"
if [[ "${CODIGO_LISTA}" != "201" ]]; then
  printf "ERROR: POST /api/v1/listas devolvió HTTP %s: %s\n" "${CODIGO_LISTA}" "$(cat "${ARCHIVO_CUERPO}")" >&2
  exit 1
fi

LISTA_ID="$(python3 - "${ARCHIVO_CUERPO}" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as archivo:
    print(json.load(archivo).get("id", ""))
PY
)"
printf "\n3. Lista creada: %s (agregando %d productos)...\n" "${LISTA_ID}" "${CANTIDAD_PRODUCTOS}"

ARTICULOS_VENCIDOS=0
while IFS=$'\t' read -r categoria ean_producto producto_id titulo_producto precios_cadenas; do
  CUERPO_ARTICULO="$(printf '{"producto_id":"%s","cantidad":1}' "${producto_id}")"
  CODIGO_ARTICULO="$(ejecutar_peticion "POST" "/api/v1/listas/${LISTA_ID}/articulos" "${CUERPO_ARTICULO}")"
  ESTADO_PRODUCTO="$(python3 - "${ARCHIVO_CUERPO}" <<'PY'
import json, sys
try:
    with open(sys.argv[1], "r", encoding="utf-8") as archivo:
        datos = json.load(archivo)
    print(datos.get("estado_producto", "?"))
except Exception:
    print("?")
PY
)"
  if [[ "${CODIGO_ARTICULO}" != "201" && "${CODIGO_ARTICULO}" != "200" ]]; then
    printf "   ERROR agregando %s (%s): HTTP %s\n" "${titulo_producto}" "${producto_id}" "${CODIGO_ARTICULO}" >&2
    exit 1
  fi
  if [[ "${ESTADO_PRODUCTO}" != "disponible" ]]; then
    ARTICULOS_VENCIDOS=$((ARTICULOS_VENCIDOS + 1))
  fi
  printf "   + [%s] %s -> HTTP %s (estado_producto=%s)\n" \
    "${categoria}" "${titulo_producto}" "${CODIGO_ARTICULO}" "${ESTADO_PRODUCTO}"
done < "${ARCHIVO_PRODUCTOS}"

# ----- Paso 4: Llamar a POST /api/v1/rutas/optimizar -----
CUERPO_RUTA="$(printf '{"lista_id":"%s"}' "${LISTA_ID}")"
CODIGO_RUTA="$(ejecutar_peticion "POST" "/api/v1/rutas/optimizar" "${CUERPO_RUTA}")"

printf "\n4. Respuesta de POST /api/v1/rutas/optimizar (HTTP %s):\n" "${CODIGO_RUTA}"
python3 - "${ARCHIVO_CUERPO}" "${CODIGO_RUTA}" "${ARTICULOS_VENCIDOS}" <<'PY'
import json
import sys

ruta_archivo = sys.argv[1]
codigo_http = sys.argv[2]
vencidos = int(sys.argv[3])

with open(ruta_archivo, "r", encoding="utf-8") as archivo:
    texto = archivo.read()

try:
    datos = json.loads(texto)
    print(json.dumps(datos, indent=2, ensure_ascii=False))
except json.JSONDecodeError:
    print(texto)
    sys.exit(1)

if codigo_http != "200":
    if codigo_http == "422" and vencidos > 0:
        print(
            "\nDIAGNÓSTICO: Los productos con EAN del dump local tienen capturas más antiguas "
            "que CATALOGO_DISPONIBILIDAD_ANTIGUEDAD_MAXIMA (por defecto 48h).\n"
            "Para probar con un dump histórico en local, reinicia solo Catálogo con una ventana mayor:\n"
            "  CATALOGO_DISPONIBILIDAD_ANTIGUEDAD_MAXIMA=2160h docker compose up -d --no-deps catalogo",
            file=sys.stderr,
        )
    sys.exit(1)

ruta = datos.get("ruta") or {}
paradas = ruta.get("paradas") or []
base = datos.get("base_una_tienda")
ahorro = datos.get("ahorro")
total = ruta.get("total", 0)
distancia = ruta.get("distancia_km", 0)

print("\n=== Resumen de optimización ===")
print(f"Paradas elegidas : {len(paradas)}")
for parada in paradas:
    articulos = parada.get("articulos") or []
    print(
        f"  {parada.get('orden')}. {parada.get('cadena')} - {parada.get('nombre')} "
        f"(subtotal=${parada.get('subtotal', 0):,}, {len(articulos)} artículos)"
    )
print(f"Total ruta       : ${total:,} CLP")
print(f"Distancia total  : {distancia} km")
if isinstance(base, dict):
    print(
        f"Base una tienda  : {base.get('cadena')} - {base.get('nombre')} "
        f"(${base.get('total', 0):,} CLP)"
    )
if ahorro is not None:
    porcentaje = (ahorro / base["total"] * 100.0) if isinstance(base, dict) and base.get("total") else 0.0
    print(f"Ahorro estimado  : ${ahorro:,} CLP ({porcentaje:.1f}%)")
PY
