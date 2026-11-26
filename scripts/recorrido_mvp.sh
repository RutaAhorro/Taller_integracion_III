#!/usr/bin/env bash
# ==============================================================================
# scripts/recorrido_mvp.sh
#
# Recorrido de diagnóstico del flujo MVP a través del Gateway (http://127.0.0.1:8082
# por defecto). Ejecuta los 5 pasos principales usando curl y un cookie jar temporal:
#   1. Registro local (POST /api/v1/auth/register)
#   2. Búsqueda de producto (GET /api/v1/productos?limit=5)
#   3. Creación de lista (POST /api/v1/listas)
#   4. Agregado de artículo (POST /api/v1/listas/{lista_id}/articulos)
#   5. Consulta al chat IA (POST /api/v1/chat)
#
# No detiene la ejecución ante el primer fallo, no imprime cookies ni secretos,
# no toca la base de datos directamente ni modifica contenedores o volúmenes.
# ==============================================================================

set -uo pipefail

# ===== Constantes de configuración =====

readonly URL_BASE_GATEWAY="${GATEWAY_URL:-http://127.0.0.1:8082}"
readonly TIEMPO_ESPERA_SEGUNDOS=25
readonly UUID_RESPALDO="00000000-0000-0000-0000-000000000000"
readonly CONTRASENA_PRUEBA="PruebaMvp2026!"
readonly NOMBRE_USUARIO_PRUEBA="Usuario Recorrido MVP"
readonly NOMBRE_LISTA_PRUEBA="Lista recorrido MVP"
readonly MENSAJE_CHAT_PRUEBA="Busco arroz barato"

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
elif clave == "total_productos":
    print(datos.get("total_registros", 0))
elif clave == "lista_id":
    print(datos.get("id", ""))
PY
}

# imprimir_resultado muestra una línea uniforme con el número de paso, estado y detalle.
imprimir_resultado() {
  local nombre_paso="$1"
  local estado_paso="$2"
  local detalle_paso="$3"
  printf "[%s] %s -> %s\n" "${estado_paso}" "${nombre_paso}" "${detalle_paso}"
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
    argumentos_curl+=(--cookie "${ARCHIVO_COOKIES}")
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

MARCA_TIEMPO="$(date +%s)"
CORREO_UNICO="recorrido_${MARCA_TIEMPO}_$$@ejemplo.local"

# ----- Paso 1: Registro local -----
CUERPO_REGISTRO="$(printf '{"correo":"%s","password":"%s","nombre_completo":"%s"}' \
  "${CORREO_UNICO}" "${CONTRASENA_PRUEBA}" "${NOMBRE_USUARIO_PRUEBA}")"
CODIGO_REGISTRO="$(ejecutar_peticion "POST" "/api/v1/auth/register" "${CUERPO_REGISTRO}" "guardar")"

if [[ "${CODIGO_REGISTRO}" == "201" ]]; then
  imprimir_resultado "1. Registro local" "OK" "HTTP 201 (usuario ${CORREO_UNICO} creado y sesión guardada en cookie jar)"
else
  imprimir_resultado "1. Registro local" "FALLO" "HTTP ${CODIGO_REGISTRO} - $(resumir_cuerpo)"
fi

# ----- Paso 2: Buscar un producto -----
PRODUCTO_ID_ENCONTRADO=""
CODIGO_PRODUCTOS="$(ejecutar_peticion "GET" "/api/v1/productos?limit=5" "" "ninguno")"

if [[ "${CODIGO_PRODUCTOS}" == "200" ]]; then
  PRODUCTO_ID_ENCONTRADO="$(extraer_campo_json "primer_producto_id")"
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

CUERPO_ARTICULO="$(printf '{"producto_id":"%s","cantidad":1}' "${PRODUCTO_ID_DESTINO}")"
CODIGO_ARTICULO="$(ejecutar_peticion "POST" "/api/v1/listas/${LISTA_ID_DESTINO}/articulos" "${CUERPO_ARTICULO}" "enviar")"

if [[ "${USA_UUID_RESPALDO}" -eq 1 ]]; then
  imprimir_resultado "4. Agregar artículo" "NO CONCLUYENTE (UUID de respaldo)" \
    "HTTP ${CODIGO_ARTICULO} (${MOTIVO_RESPALDO}) - $(resumir_cuerpo)"
elif [[ "${CODIGO_ARTICULO}" == "201" || "${CODIGO_ARTICULO}" == "200" ]]; then
  imprimir_resultado "4. Agregar artículo" "OK" "HTTP ${CODIGO_ARTICULO} (artículo agregado a lista ${LISTA_ID_DESTINO})"
else
  imprimir_resultado "4. Agregar artículo" "FALLO" "HTTP ${CODIGO_ARTICULO} - $(resumir_cuerpo)"
fi

# ----- Paso 5: Consultar el chat -----
CUERPO_CHAT="$(printf '{"mensaje":"%s"}' "${MENSAJE_CHAT_PRUEBA}")"
CODIGO_CHAT="$(ejecutar_peticion "POST" "/api/v1/chat" "${CUERPO_CHAT}" "enviar")"

if [[ "${CODIGO_CHAT}" == "200" ]]; then
  imprimir_resultado "5. Consultar chat" "OK" "HTTP 200 (respuesta recibida del asistente)"
else
  imprimir_resultado "5. Consultar chat" "FALLO" "HTTP ${CODIGO_CHAT} - $(resumir_cuerpo)"
fi

printf "=== Fin del recorrido MVP ===\n"
