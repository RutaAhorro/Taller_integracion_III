#!/usr/bin/env bash
set -euo pipefail
# SUP-273 añade corte/rollback y réplicas de Control al harness completo vigente.
export SUP273_CONTROL=1
export SUP269_API_TEST_IMAGE=sup273-control:local
exec bash "$(dirname "${BASH_SOURCE[0]}")/smoke_identity_compose.sh"
