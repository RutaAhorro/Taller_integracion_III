#!/usr/bin/env bash
set -euo pipefail
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
export SUP272_IDENTITY=1 SUP271_OWNERSHIP=1 SUP270_RECOVERY=1
export SUP269_API_TEST_IMAGE=sup273-control:local
export SUP269_CATALOGO_TEST_IMAGE=sup271-catalogo:local
exec bash "$repo_root/scripts/smoke_catalogo_routing_compose.sh"
