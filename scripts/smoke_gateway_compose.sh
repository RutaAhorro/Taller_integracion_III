#!/usr/bin/env bash
set -euo pipefail
# Compatibilidad del comando anterior: el lector salió de API en SUP-268.
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
export SUP268_SKIP_BUILD="${SUP268_SKIP_BUILD:-${SUP265_SKIP_BUILD:-0}}"
export SUP268_API_PORT="${SUP268_API_PORT:-${SUP265_BACKEND_PORT:-18080}}"
export SUP268_GATEWAY_PORT="${SUP268_GATEWAY_PORT:-${SUP265_GATEWAY_PORT:-18082}}"
export SUP268_FRONTEND_PORT="${SUP268_FRONTEND_PORT:-${SUP265_FRONTEND_PORT:-13000}}"
export SUP268_BUILD_NETWORK="${SUP268_BUILD_NETWORK:-${SUP265_BUILD_NETWORK:-default}}"
export SUP268_CA_BUNDLE="${SUP268_CA_BUNDLE:-${SUP265_CA_BUNDLE:-/etc/ssl/certs/ca-certificates.crt}}"
exec bash "$repo_root/scripts/smoke_catalogo_routing_compose.sh" "$@"
