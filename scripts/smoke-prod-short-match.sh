#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin curl
require_bin docker
require_bin jq
require_bin psql

PROD_ENV="${PROD_ENV:-deploy/compose/prod.env}"
load_env_file "${PROD_ENV}"

EDGE_BASE_URL="${PROD_EDGE_BASE_URL:-$(derive_edge_base_url)}"
POSTGRES_DSN="${POSTGRES_DSN_HOST_ENFORCEMENT:-${POSTGRES_DSN:-}}"

if [[ -z "${POSTGRES_DSN}" ]]; then
  echo "POSTGRES_DSN_HOST_ENFORCEMENT or POSTGRES_DSN must be set in ${PROD_ENV} for prod short-match smoke." >&2
  exit 1
fi

export AD_PLATFORM_API_URL="${EDGE_BASE_URL}"
export ORGANIZER_SMOKE_READY_URL="${EDGE_BASE_URL}/healthz"
export ORGANIZER_SMOKE_SKIP_STACK_BOOTSTRAP=true
export ORGANIZER_SMOKE_USE_SCHEDULER="${ORGANIZER_SMOKE_USE_SCHEDULER:-true}"
export ORGANIZER_SMOKE_TARGET_TICKS="${ORGANIZER_SMOKE_TARGET_TICKS:-1}"
export ORGANIZER_SMOKE_SCHEDULER_INTERVAL_SECONDS="${ORGANIZER_SMOKE_SCHEDULER_INTERVAL_SECONDS:-5}"
export ORGANIZER_SMOKE_SCHEDULER_WAIT_SECONDS="${ORGANIZER_SMOKE_SCHEDULER_WAIT_SECONDS:-90}"
export ORGANIZER_SMOKE_ARTIFACT_FILE="${ORGANIZER_SMOKE_ARTIFACT_FILE:-.runtime/last-prod-short-match.env}"
export POSTGRES_DSN

echo "production short match smoke: edge=${EDGE_BASE_URL} target_ticks=${ORGANIZER_SMOKE_TARGET_TICKS} interval=${ORGANIZER_SMOKE_SCHEDULER_INTERVAL_SECONDS}s"
"${ROOT_DIR}/scripts/smoke-organizer-created-flow.sh"
