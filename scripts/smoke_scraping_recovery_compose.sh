#!/usr/bin/env bash
set -euo pipefail
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
export SUP270_RECOVERY=1
export SUP269_API_TEST_IMAGE=sup270-api:local
export SUP269_CATALOGO_TEST_IMAGE=sup270-catalogo:local
exec bash "$repo_root/scripts/smoke_catalogo_routing_compose.sh"
