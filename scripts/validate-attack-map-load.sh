#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin curl
require_bin jq

PROD_ENV="${PROD_ENV:-deploy/compose/prod.env}"
load_env_file "${PROD_ENV}"

EDGE_BASE_URL="${ATTACK_MAP_LOAD_BASE_URL:-$(derive_edge_base_url)}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUTPUT_DIR="${ATTACK_MAP_LOAD_OUTPUT_DIR:-.runtime/attack-map-load-${TIMESTAMP}}"

mkdir -p "${OUTPUT_DIR}"

export AD_PLATFORM_API_URL="${EDGE_BASE_URL}"
export ATTACK_MAP_LOAD_ARTIFACT_FILE="${ATTACK_MAP_LOAD_ARTIFACT_FILE:-${OUTPUT_DIR}/attack-map-load.env}"
export ATTACK_MAP_LOAD_REPORT_FILE="${ATTACK_MAP_LOAD_REPORT_FILE:-${OUTPUT_DIR}/attack-map-load-report.json}"

# Default validation profile for release-candidate rehearsal runs.
export TEAM_COUNT="${TEAM_COUNT:-12}"
export ATTACK_MAP_LOAD_ROUNDS="${ATTACK_MAP_LOAD_ROUNDS:-2}"
export ATTACK_MAP_LOAD_RESET="${ATTACK_MAP_LOAD_RESET:-true}"
export ATTACK_MAP_LOAD_BUILD_IMAGES="${ATTACK_MAP_LOAD_BUILD_IMAGES:-true}"
export ATTACK_MAP_LOAD_MIN_SUBMISSIONS_PER_SECOND="${ATTACK_MAP_LOAD_MIN_SUBMISSIONS_PER_SECOND:-5}"
export ATTACK_MAP_LOAD_MAX_FLAG_FETCH_P95_MS="${ATTACK_MAP_LOAD_MAX_FLAG_FETCH_P95_MS:-1000}"
export ATTACK_MAP_LOAD_MAX_SUBMISSION_P95_MS="${ATTACK_MAP_LOAD_MAX_SUBMISSION_P95_MS:-1500}"
export ATTACK_MAP_LOAD_MAX_RECOMPUTE_MS="${ATTACK_MAP_LOAD_MAX_RECOMPUTE_MS:-5000}"
export ATTACK_MAP_LOAD_MAX_ATTACK_FEED_LAG_MS="${ATTACK_MAP_LOAD_MAX_ATTACK_FEED_LAG_MS:-3000}"
export ATTACK_MAP_LOAD_ATTACK_FEED_TIMEOUT_MS="${ATTACK_MAP_LOAD_ATTACK_FEED_TIMEOUT_MS:-15000}"
export ATTACK_MAP_LOAD_ATTACK_FEED_POLL_INTERVAL_MS="${ATTACK_MAP_LOAD_ATTACK_FEED_POLL_INTERVAL_MS:-250}"

echo "attack-map load validation: edge=${EDGE_BASE_URL} output=${OUTPUT_DIR}"
wait_for_http "${EDGE_BASE_URL}/healthz" 60 "edge healthz"
wait_for_http "${EDGE_BASE_URL}/api/v2/challenges" 60 "public challenges"

"${ROOT_DIR}/scripts/simulate-attack-map-load.sh" "${TEAM_COUNT}" "${TEAM_PREFIX:-}" "${TEAM_EMAIL_DOMAIN:-}" "${TEAM_START_INDEX:-}"

{
  echo "attack-map load validation completed at ${TIMESTAMP}."
  echo
  echo "Base URL:"
  echo "- ${EDGE_BASE_URL}"
  echo
  echo "Threshold profile:"
  echo "- min submissions/sec: ${ATTACK_MAP_LOAD_MIN_SUBMISSIONS_PER_SECOND}"
  echo "- max flag fetch p95 ms: ${ATTACK_MAP_LOAD_MAX_FLAG_FETCH_P95_MS}"
  echo "- max submission p95 ms: ${ATTACK_MAP_LOAD_MAX_SUBMISSION_P95_MS}"
  echo "- max scoreboard recompute ms: ${ATTACK_MAP_LOAD_MAX_RECOMPUTE_MS}"
  echo "- max attack feed lag ms: ${ATTACK_MAP_LOAD_MAX_ATTACK_FEED_LAG_MS}"
  echo
  echo "Artifacts in this directory:"
  echo "- attack-map-load.env"
  echo "- attack-map-load-report.json"
} > "${OUTPUT_DIR}/README.txt"

{
  echo "commit=$(git rev-parse HEAD)"
  echo "short_commit=$(git rev-parse --short HEAD)"
  echo "generated_at=${TIMESTAMP}"
} > "${OUTPUT_DIR}/git-revision.txt"

if command -v sha256sum >/dev/null 2>&1 && [[ -f "${PROD_ENV}" ]]; then
  sha256sum "${PROD_ENV}" > "${OUTPUT_DIR}/prod-env.sha256"
fi

echo "attack-map load validation passed:"
printf '  %s\n' \
  "${OUTPUT_DIR}/attack-map-load-report.json" \
  "${OUTPUT_DIR}/attack-map-load.env" \
  "${OUTPUT_DIR}/README.txt"
