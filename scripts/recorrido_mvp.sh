#!/usr/bin/env bash
# ==============================================================================
# scripts/recorrido_mvp.sh
#
# Recorrido de diagnóstico del flujo MVP a través del Gateway (http://127.0.0.1:8082
# por defecto). Ejecuta los 6 pasos principales usando curl:
#   1. Sesión de prueba con cookie jwt HS256 (modo por defecto) o registro local
#      (--registro-local / RECORRIDO_MODO_AUTH=registro)
#   2. Búsqueda de producto (GET /api/v1/productos?limit=5)
#   3. Creación de lista (POST /api/v1/listas)
#   4. Agregado de artículo (POST /api/v1/listas/{lista_id}/articulos)
#   5. Consulta al chat IA (POST /api/v1/chat)
#   6. Optimización de ruta de la lista (POST /api/v1/rutas/optimizar)
#
# No detiene la ejecución ante el primer fallo, no imprime cookies ni secretos,
# no toca la base de datos directamente ni modifica contenedores o volúmenes.
# ==============================================================================

set -uo pipefail

# ===== Constantes de configuración =====

readonly DIRECTORIO_RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly URL_BASE_GATEWAY="${GATEWAY_URL:-http://127.0.0.1:8082}"
readonly TIEMPO_ESPERA_SEGUNDOS=25
readonly UUID_RESPALDO="00000000-0000-0000-0000-000000000000"
readonly LONGITUD_MINIMA_JWT_SECRET=32
readonly ROL_PRUEBA="registrado"
readonly PROVEEDOR_PRUEBA="google"
readonly CONTRASENA_PRUEBA="PruebaMvp2026!"
readonly NOMBRE_USUARIO_PRUEBA="Usuario Recorrido MVP"
readonly NOMBRE_LISTA_PRUEBA="Lista recorrido MVP"
readonly MENSAJE_CHAT_PRUEBA="Busco arroz barato"

MODO_AUTH="${RECORRIDO_MODO_AUTH:-jwt}"
if [[ "${1:-}" == "--registro-local" ]]; then
  MODO_AUTH="registro"
elif [[ "${1:-}" == "--jwt" ]]; then
  MODO_AUTH="jwt"
fi

# ===== Archivos temporales y limpieza =====

ARCHIVO_COOKIES="$(mktemp /tmp/recorrido_mvp_cookies.XXXXXX)"
ARCHIVO_CUERPO="$(mktemp /tmp/recorrido_mvp_cuerpo.XXXXXX)"

# limpiar_temporales elimina el cookie jar y el archivo de cuerpo al terminar
# para no dejar tokens de sesión en disco.
limpiar_temporales() {
  rm -f "${ARCHIVO_COOKIES}" "${ARCHIVO_CUERPO}"
}
trap limpiar_temporales EXIT

# ===== Funciones auxiliares =====

# resumir_cuerpo devuelve los primeros 180 caracteres del cuerpo HTTP en una sola
# línea para informar el motivo de un fallo sin volcar respuestas extensas.
resumir_cuerpo() {
  if [[ ! -s "${ARCHIVO_CUERPO}" ]]; then
    printf "sin cuerpo en la respuesta"
    return
  fi
  tr '\r\n' ' ' < "${ARCHIVO_CUERPO}" | cut -c1-180
}

# extraer_campo_json lee un campo puntual desde el archivo de respuesta usando python3
# para evitar dependencias opcionales como jq.
extraer_campo_json() {
  local expresion_ruta="$1"
  python3 - "${ARCHIVO_CUERPO}" "${expresion_ruta}" <<'PY' 2>/dev/null || true
import json
import sys

ruta_archivo = sys.argv[1]
clave = sys.argv[2]

with open(ruta_archivo, "r", encoding="utf-8") as archivo:
    datos = json.load(archivo)

if clave == "primer_producto_id":
    elementos = datos.get("data") or []
    if elementos and isinstance(elementos[0], dict):
        print(elementos[0].get("id", ""))
elif clave == "productos_ids":
    elementos = datos.get("data") or []
    for elemento in elementos:
        if isinstance(elemento, dict) and elemento.get("id"):
            print(elemento["id"])
elif clave == "articulo_vigente":
    producto = datos.get("producto") or {}
    if datos.get("estado_producto") == "disponible" and producto.get("precio") is not None:
        print("si")
elif clave == "articulo_id":
    print(datos.get("id", ""))
elif clave == "total_productos":
    print(datos.get("total_registros", 0))
elif clave == "lista_id":
    print(datos.get("id", ""))
elif clave == "resumen_ruta":
    ruta = datos.get("ruta") or {}
    paradas = ruta.get("paradas") or []
    cadenas = ", ".join(p.get("cadena") or p.get("nombre") or "?" for p in paradas if isinstance(p, dict))
    total = ruta.get("total", 0)
    print(f"paradas={len(paradas)} ({cadenas}), total=${total}")
PY
}

# imprimir_resultado muestra una línea uniforme con el número de paso, estado y detalle.
imprimir_resultado() {
  local nombre_paso="$1"
  local estado_paso="$2"
  local detalle_paso="$3"
  printf "[%s] %s -> %s\n" "${estado_paso}" "${nombre_paso}" "${detalle_paso}"
}

# generar_cookie_jwt_prueba obtiene JWT_SECRET del entorno o del archivo .env
# raíz y firma un JWT HS256 válido para Identidad, Motor de rutas e IA.
# Imprime "<user_id>|<token>" por stdout o devuelve 1 si falta/es inválido.
generar_cookie_jwt_prueba() {
  local secreto_jwt="${JWT_SECRET:-}"
  local archivo_env="${DIRECTORIO_RAIZ}/.env"

  python3 - "${secreto_jwt}" "${archivo_env}" "${LONGITUD_MINIMA_JWT_SECRET}" "${ROL_PRUEBA}" "${PROVEEDOR_PRUEBA}" <<'PY'
import base64
import hashlib
import hmac
import json
import sys
import time
import uuid

secreto = sys.argv[1].strip()
ruta_env = sys.argv[2]
longitud_minima = int(sys.argv[3])
rol = sys.argv[4]
proveedor = sys.argv[5]

if not secreto:
    try:
        with open(ruta_env, "r", encoding="utf-8") as archivo:
            for linea in archivo:
                linea_limpia = linea.strip()
                if not linea_limpia or linea_limpia.startswith("#"):
                    continue
                if linea_limpia.startswith("JWT_SECRET="):
                    valor = linea_limpia.split("=", 1)[1].strip()
                    if (valor.startswith('"') and valor.endswith('"')) or (valor.startswith("'") and valor.endswith("'")):
                        valor = valor[1:-1]
                    secreto = valor
                    break
    except OSError:
        pass

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

# ejecutar_peticion realiza una llamada HTTP con curl y devuelve el código de estado
# (o "000" si no hubo conexión). Nunca imprime cabeceras Set-Cookie ni el cookie jar.
ejecutar_peticion() {
  local metodo_http="$1"
  local ruta_recurso="$2"
  local cuerpo_json="$3"
  local modo_cookies="$4"
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
  )

  if [[ "${modo_cookies}" == "guardar" ]]; then
    argumentos_curl+=(--cookie-jar "${ARCHIVO_COOKIES}")
  elif [[ "${modo_cookies}" == "enviar" ]]; then
    if [[ -n "${TOKEN_JWT_PRUEBA:-}" ]]; then
      argumentos_curl+=(--cookie "jwt=${TOKEN_JWT_PRUEBA}")
    else
      argumentos_curl+=(--cookie "${ARCHIVO_COOKIES}")
    fi
  fi

  if [[ -n "${cuerpo_json}" ]]; then
    argumentos_curl+=(--data "${cuerpo_json}")
  fi

  codigo_http="$(curl "${argumentos_curl[@]}" "${URL_BASE_GATEWAY}${ruta_recurso}" 2>/dev/null || true)"
  if [[ -z "${codigo_http}" ]]; then
    codigo_http="000"
  fi
  printf "%s" "${codigo_http}"
}

# ===== Ejecución del recorrido =====

printf "=== Recorrido MVP por Gateway: %s ===\n" "${URL_BASE_GATEWAY}"

TOKEN_JWT_PRUEBA=""
USUARIO_ID_PRUEBA=""

# ----- Paso 1: Sesión de prueba (cookie jwt por defecto o registro local) -----
if [[ "${MODO_AUTH}" == "registro" ]]; then
  MARCA_TIEMPO="$(date +%s)"
  CORREO_UNICO="recorrido_${MARCA_TIEMPO}_$$@ejemplo.local"
  CUERPO_REGISTRO="$(printf '{"correo":"%s","password":"%s","nombre_completo":"%s"}' \
    "${CORREO_UNICO}" "${CONTRASENA_PRUEBA}" "${NOMBRE_USUARIO_PRUEBA}")"
  CODIGO_REGISTRO="$(ejecutar_peticion "POST" "/api/v1/auth/register" "${CUERPO_REGISTRO}" "guardar")"

  if [[ "${CODIGO_REGISTRO}" == "201" ]]; then
    imprimir_resultado "1. Registro local" "OK" "HTTP 201 (usuario ${CORREO_UNICO} creado y sesión guardada en cookie jar)"
  else
    imprimir_resultado "1. Registro local" "FALLO" "HTTP ${CODIGO_REGISTRO} - $(resumir_cuerpo)"
  fi
else
  RESULTADO_JWT="$(generar_cookie_jwt_prueba 2>/dev/null || true)"
  if [[ -n "${RESULTADO_JWT}" && "${RESULTADO_JWT}" == *"|"* ]]; then
    USUARIO_ID_PRUEBA="${RESULTADO_JWT%%|*}"
    TOKEN_JWT_PRUEBA="${RESULTADO_JWT#*|}"
    imprimir_resultado "1. Cookie JWT de prueba" "OK" \
      "cookie jwt HS256 generada (user_id=${USUARIO_ID_PRUEBA}, rol=${ROL_PRUEBA}, provider=${PROVEEDOR_PRUEBA})"
  else
    imprimir_resultado "1. Cookie JWT de prueba" "FALLO" \
      "falta JWT_SECRET (>= ${LONGITUD_MINIMA_JWT_SECRET} bytes) en entorno o .env"
  fi
fi

# ----- Paso 2: Buscar un producto -----
PRODUCTO_ID_ENCONTRADO=""
PRODUCTOS_CANDIDATOS=()
CODIGO_PRODUCTOS="$(ejecutar_peticion "GET" "/api/v1/productos?q=arroz&limit=10" "" "ninguno")"

if [[ "${CODIGO_PRODUCTOS}" == "200" ]]; then
  while IFS= read -r id_candidato; do
    [[ -n "${id_candidato}" ]] && PRODUCTOS_CANDIDATOS+=("${id_candidato}")
  done < <(extraer_campo_json "productos_ids")
  PRODUCTO_ID_ENCONTRADO="${PRODUCTOS_CANDIDATOS[0]:-}"
  TOTAL_PRODUCTOS="$(extraer_campo_json "total_productos")"
  if [[ -n "${PRODUCTO_ID_ENCONTRADO}" ]]; then
    imprimir_resultado "2. Buscar producto" "OK" "HTTP 200 (total=${TOTAL_PRODUCTOS}, primer producto_id=${PRODUCTO_ID_ENCONTRADO})"
  else
    imprimir_resultado "2. Buscar producto" "FALLO" "HTTP 200 pero el catálogo devolvió 0 productos (sin producto_id para los pasos siguientes)"
  fi
else
  imprimir_resultado "2. Buscar producto" "FALLO" "HTTP ${CODIGO_PRODUCTOS} - $(resumir_cuerpo)"
fi

# ----- Paso 3: Crear una lista -----
LISTA_ID_CREADA=""
CUERPO_LISTA="$(printf '{"nombre":"%s"}' "${NOMBRE_LISTA_PRUEBA}")"
CODIGO_LISTA="$(ejecutar_peticion "POST" "/api/v1/listas" "${CUERPO_LISTA}" "enviar")"

if [[ "${CODIGO_LISTA}" == "201" ]]; then
  LISTA_ID_CREADA="$(extraer_campo_json "lista_id")"
  imprimir_resultado "3. Crear lista" "OK" "HTTP 201 (lista_id=${LISTA_ID_CREADA})"
else
  imprimir_resultado "3. Crear lista" "FALLO" "HTTP ${CODIGO_LISTA} - $(resumir_cuerpo)"
fi

# ----- Paso 4: Agregar un artículo -----
USA_UUID_RESPALDO=0
LISTA_ID_DESTINO="${LISTA_ID_CREADA}"
PRODUCTO_ID_DESTINO="${PRODUCTO_ID_ENCONTRADO}"
MOTIVO_RESPALDO=""

if [[ -z "${LISTA_ID_DESTINO}" ]]; then
  LISTA_ID_DESTINO="${UUID_RESPALDO}"
  USA_UUID_RESPALDO=1
  MOTIVO_RESPALDO="sin lista_id del paso 3"
fi

if [[ -z "${PRODUCTO_ID_DESTINO}" ]]; then
  PRODUCTO_ID_DESTINO="${UUID_RESPALDO}"
  USA_UUID_RESPALDO=1
  if [[ -n "${MOTIVO_RESPALDO}" ]]; then
    MOTIVO_RESPALDO="${MOTIVO_RESPALDO} y sin producto_id del paso 2"
  else
    MOTIVO_RESPALDO="sin producto_id del paso 2"
  fi
fi

if [[ "${USA_UUID_RESPALDO}" -eq 1 ]]; then
  CUERPO_ARTICULO="$(printf '{"producto_id":"%s","cantidad":1}' "${PRODUCTO_ID_DESTINO}")"
  CODIGO_ARTICULO="$(ejecutar_peticion "POST" "/api/v1/listas/${LISTA_ID_DESTINO}/articulos" "${CUERPO_ARTICULO}" "enviar")"
  imprimir_resultado "4. Agregar artículo" "NO CONCLUYENTE (UUID de respaldo)" \
    "HTTP ${CODIGO_ARTICULO} (${MOTIVO_RESPALDO}) - $(resumir_cuerpo)"
else
  # Prueba los candidatos del paso 2 hasta encontrar uno con precio vigente
  # para que el paso 6 (optimizar ruta) pueda calcular una ruta real.
  CODIGO_ARTICULO="000"
  ARTICULO_VIGENTE=""
  for candidato_id in "${PRODUCTOS_CANDIDATOS[@]}"; do
    CUERPO_ARTICULO="$(printf '{"producto_id":"%s","cantidad":1}' "${candidato_id}")"
    CODIGO_ARTICULO="$(ejecutar_peticion "POST" "/api/v1/listas/${LISTA_ID_DESTINO}/articulos" "${CUERPO_ARTICULO}" "enviar")"
    if [[ "${CODIGO_ARTICULO}" == "201" || "${CODIGO_ARTICULO}" == "200" ]]; then
      PRODUCTO_ID_DESTINO="${candidato_id}"
      ARTICULO_VIGENTE="$(extraer_campo_json "articulo_vigente")"
      if [[ "${ARTICULO_VIGENTE}" == "si" ]]; then
        break
      fi
      # Quita el artículo con precio vencido antes de probar el siguiente candidato.
      ARTICULO_ID_TEMP="$(extraer_campo_json "articulo_id")"
      if [[ -n "${ARTICULO_ID_TEMP}" ]]; then
        ejecutar_peticion "DELETE" "/api/v1/listas/${LISTA_ID_DESTINO}/articulos/${ARTICULO_ID_TEMP}" "" "enviar" >/dev/null
      fi
    else
      break
    fi
  done

  if [[ "${CODIGO_ARTICULO}" == "201" || "${CODIGO_ARTICULO}" == "200" ]]; then
    imprimir_resultado "4. Agregar artículo" "OK" "HTTP ${CODIGO_ARTICULO} (producto ${PRODUCTO_ID_DESTINO} agregado a lista ${LISTA_ID_DESTINO})"
  else
    imprimir_resultado "4. Agregar artículo" "FALLO" "HTTP ${CODIGO_ARTICULO} - $(resumir_cuerpo)"
  fi
fi

# ----- Paso 5: Consultar el chat -----
CUERPO_CHAT="$(printf '{"mensaje":"%s"}' "${MENSAJE_CHAT_PRUEBA}")"
CODIGO_CHAT="$(ejecutar_peticion "POST" "/api/v1/chat" "${CUERPO_CHAT}" "enviar")"

if [[ "${CODIGO_CHAT}" == "200" ]]; then
  imprimir_resultado "5. Consultar chat" "OK" "HTTP 200 (respuesta recibida del asistente)"
else
  imprimir_resultado "5. Consultar chat" "FALLO" "HTTP ${CODIGO_CHAT} - $(resumir_cuerpo)"
fi

# ----- Paso 6: Optimizar ruta de la lista -----
CUERPO_RUTA="$(printf '{"lista_id":"%s"}' "${LISTA_ID_DESTINO}")"
CODIGO_RUTA="$(ejecutar_peticion "POST" "/api/v1/rutas/optimizar" "${CUERPO_RUTA}" "enviar")"

if [[ "${USA_UUID_RESPALDO}" -eq 1 ]]; then
  imprimir_resultado "6. Optimizar ruta" "NO CONCLUYENTE (UUID de respaldo)" \
    "HTTP ${CODIGO_RUTA} (${MOTIVO_RESPALDO}) - $(resumir_cuerpo)"
elif [[ "${CODIGO_RUTA}" == "200" ]]; then
  RESUMEN_RUTA="$(extraer_campo_json "resumen_ruta")"
  imprimir_resultado "6. Optimizar ruta" "OK" "HTTP 200 (${RESUMEN_RUTA})"
else
  imprimir_resultado "6. Optimizar ruta" "FALLO" "HTTP ${CODIGO_RUTA} - $(resumir_cuerpo)"
fi

printf "=== Fin del recorrido MVP ===\n"
