#!/usr/bin/env bash
set -euo pipefail
# SUP-269 extiende los contratos, carga, fallos y rollback de SUP-268.
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
exec bash "$repo_root/scripts/smoke_catalogo_routing_compose.sh" "$@"
