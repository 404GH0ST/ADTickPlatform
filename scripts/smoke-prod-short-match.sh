#!/usr/bin/env bash
# Production short-match smoke. This is the event-shaped production smoke:
# 10 teams, a 3-minute (180s) scheduler tick, and 6 challenges (2 pwn + 1 crypto
# over TCP, plus flask/nodejs/php web). It drives the live edge via the
# scheduler and writes the short-match artifact that go-live-check and the
# host-recovery smoke consume.
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
  echo "POSTGRES_DSN_HOST_ENFORCEMENT or POSTGRES_DSN must be set in ${PROD_ENV} for the prod short-match smoke." >&2
  exit 1
fi

export AD_PLATFORM_API_URL="${EDGE_BASE_URL}"
export ORGANIZER_SMOKE_READY_URL="${EDGE_BASE_URL}/healthz"
export POSTGRES_DSN
export FLEET_SMOKE_TEAM_COUNT="${FLEET_SMOKE_TEAM_COUNT:-10}"
export FLEET_SMOKE_TICK_INTERVAL_SECONDS="${FLEET_SMOKE_TICK_INTERVAL_SECONDS:-180}"
export FLEET_SMOKE_TARGET_TICKS="${FLEET_SMOKE_TARGET_TICKS:-1}"
# Downstream smokes (go-live-check, host-recovery) read this path.
export FLEET_SMOKE_ARTIFACT_FILE="${ORGANIZER_SMOKE_ARTIFACT_FILE:-.runtime/last-prod-short-match.env}"

echo "production short match smoke (event shape): edge=${EDGE_BASE_URL} teams=${FLEET_SMOKE_TEAM_COUNT} interval=${FLEET_SMOKE_TICK_INTERVAL_SECONDS}s"
"${ROOT_DIR}/scripts/smoke-organizer-fleet-flow.sh"
