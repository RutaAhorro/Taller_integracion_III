#!/usr/bin/env bash
# ==============================================================================
# scripts/preparar_entorno.sh
#
# Prepara el entorno local de desarrollo de RutaAhorro:
#   1. Verifica que estén instalados docker compose, git, bash, curl, openssl y jq.
#   2. Crea `.env` a partir de `.env.example` solo si `.env` no existe,
#      generando secretos aleatorios con `openssl rand -hex 24` sin imprimirlos.
#   3. Garantiza que `.env` esté incluido en `.gitignore`.
#   4. Informa qué variables deben completarse a mano.
# ==============================================================================

set -euo pipefail

# ===== Constantes =====

readonly DIRECTORIO_RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly ARCHIVO_PLANTILLA="${DIRECTORIO_RAIZ}/.env.example"
readonly ARCHIVO_ENTORNO="${DIRECTORIO_RAIZ}/.env"
readonly ARCHIVO_GITIGNORE="${DIRECTORIO_RAIZ}/.gitignore"
readonly BYTES_SECRETO=24

readonly VARIABLES_SECRETAS=(
  "DB_PASSWORD"
  "IDENTITY_DB_PASSWORD"
  "CATALOGO_DB_PASSWORD"
  "SCRAPING_DB_PASSWORD"
  "RUTAS_DB_PASSWORD"
  "JWT_SECRET"
  "CATALOGO_INTERNAL_TOKEN"
  "CATALOGO_CONSULTA_TOKEN"
  "SCRAPING_SERVICE_TOKEN"
)

# ===== Funciones auxiliares =====

# verificar_herramientas comprueba la disponibilidad de los binarios requeridos
# sin instalar paquetes. Falla listando todas las herramientas ausentes.
verificar_herramientas() {
  local herramientas_faltantes=()
  local comandos_simples=("git" "bash" "curl" "openssl" "jq")
  local comando=""

  for comando in "${comandos_simples[@]}"; do
    if ! command -v "${comando}" >/dev/null 2>&1; then
      herramientas_faltantes+=("${comando}")
    fi
  done

  if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
    herramientas_faltantes+=("docker compose")
  fi

  if [[ "${#herramientas_faltantes[@]}" -gt 0 ]]; then
    printf "ERROR: Faltan herramientas requeridas:\n" >&2
    local faltante=""
    for faltante in "${herramientas_faltantes[@]}"; do
      printf "  - %s\n" "${faltante}" >&2
    done
    printf "Instálalas y vuelve a ejecutar este script.\n" >&2
    exit 1
  fi

  printf "[OK] Herramientas verificadas: docker compose, git, bash, curl, openssl, jq.\n"
}

# asegurar_gitignore verifica que exista una entrada exacta `.env` en `.gitignore`
# y la agrega solo si falta.
asegurar_gitignore() {
  if [[ -f "${ARCHIVO_GITIGNORE}" ]] && grep -qx '\.env' "${ARCHIVO_GITIGNORE}"; then
    printf "[OK] .env ya figura en .gitignore.\n"
    return
  fi

  printf "\n.env\n" >> "${ARCHIVO_GITIGNORE}"
  printf "[OK] Se agregó .env a .gitignore.\n"
}

# generar_archivo_entorno copia `.env.example` a `.env` solo si no existe y
# reemplaza las variables secretas con valores hexadecimales aleatorios.
# Nunca imprime los secretos generados en pantalla.
generar_archivo_entorno() {
  if [[ -f "${ARCHIVO_ENTORNO}" ]]; then
    printf "[OMITIDO] .env ya existe; no se sobrescribió.\n"
    return
  fi

  if [[ ! -f "${ARCHIVO_PLANTILLA}" ]]; then
    printf "ERROR: No se encontró la plantilla %s.\n" "${ARCHIVO_PLANTILLA}" >&2
    exit 1
  fi

  local archivo_temporal=""
  archivo_temporal="$(mktemp)"
  cp "${ARCHIVO_PLANTILLA}" "${archivo_temporal}"

  local nombre_variable=""
  local valor_secreto=""
  for nombre_variable in "${VARIABLES_SECRETAS[@]}"; do
    valor_secreto="$(openssl rand -hex "${BYTES_SECRETO}")"
    # Se usa awk pasando el secreto por entorno interno del subproceso para no
    # exponerlo en la línea de comandos ni en stdout.
    SECRETO_TEMPORAL="${valor_secreto}" awk -v clave="${nombre_variable}" '
      BEGIN { reemplazo = clave "=" ENVIRON["SECRETO_TEMPORAL"] }
      $0 ~ ("^" clave "=") { print reemplazo; next }
      { print }
    ' "${archivo_temporal}" > "${archivo_temporal}.nuevo"
    mv "${archivo_temporal}.nuevo" "${archivo_temporal}"
  done

  chmod 600 "${archivo_temporal}"
  mv "${archivo_temporal}" "${ARCHIVO_ENTORNO}"
  printf "[OK] Se creó .env desde .env.example con %d secretos aleatorios (no mostrados por seguridad).\n" "${#VARIABLES_SECRETAS[@]}"
}

# imprimir_pasos_manuales recuerda qué variables deben completarse a mano.
imprimir_pasos_manuales() {
  printf "\nCompleta a mano en .env antes de levantar Docker Compose:\n"
  printf "  - GEMINI_API_KEY       (requerida para que POST /api/v1/chat responda)\n"
  printf "  - GOOGLE_CLIENT_ID     (requerida para login con Google; se inyecta también en frontend)\n"
  printf "  - ADMIN_EMAILS         (correos de administradores separados por coma)\n"
  printf "  - GROQ_API_KEY         (opcional, respaldo de Gemini en el chat IA)\n"
  printf "\nNota: si luego cambias GOOGLE_CLIENT_ID o VITE_API_URL, reconstruye el frontend con:\n"
  printf "  docker compose up -d --build frontend\n"
}

# ===== Ejecución principal =====

verificar_herramientas
asegurar_gitignore
generar_archivo_entorno
imprimir_pasos_manuales
