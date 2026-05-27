#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

load_default_env_files
PROD_ENV="${PROD_ENV:-${ROOT_DIR}/deploy/compose/prod.env}"
load_env_file_override "${PROD_ENV}"

require_bin curl
require_bin jq

API_URL="${AD_PLATFORM_API_URL:-http://localhost:8080}"
PUBLIC_URL="${AD_PLATFORM_PUBLIC_BASE_URL:-}"
ADMIN_TOKEN="$(resolve_admin_api_token "${ROOT_DIR}/.runtime/backend-stack.env")"

if [[ "${API_URL}" == *"api-gateway"* ]] && [[ -n "${PUBLIC_URL}" ]]; then
  API_URL="${PUBLIC_URL}"
fi

TARGET_TICK_INTERVAL_SECONDS="${TARGET_TICK_INTERVAL_SECONDS:-300}"
TARGET_MATCH_DURATION_HOURS="${TARGET_MATCH_DURATION_HOURS:-24}"
TARGET_START_MATCH="${TARGET_START_MATCH:-true}"
TARGET_START_SCHEDULER="${TARGET_START_SCHEDULER:-true}"
TARGET_FINALIZE_ARTIFACT="${TARGET_FINALIZE_ARTIFACT:-.runtime/faust-target-shape-finalize.json}"

usage() {
  cat <<'EOF'
Usage: scripts/finalize-faust-target-shape.sh

Applies the runtime settings for the Faust target shape without recreating teams or challenges.

Environment overrides:
  TARGET_TICK_INTERVAL_SECONDS=300
  TARGET_MATCH_DURATION_HOURS=24
  TARGET_START_MATCH=true|false
  TARGET_START_SCHEDULER=true|false
  TARGET_FINALIZE_ARTIFACT=.runtime/faust-target-shape-finalize.json
  AD_PLATFORM_API_URL
  AD_PLATFORM_PUBLIC_BASE_URL
  ADMIN_API_TOKEN
EOF
}

if [[ "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

mkdir -p "$(dirname "${TARGET_FINALIZE_ARTIFACT}")"

curl_json() {
  local label="$1"
  shift

  local response_file
  response_file="$(mktemp)"

  local status
  status="$(curl -sS -o "${response_file}" -w '%{http_code}' "$@")"
  if [[ "${status}" -lt 200 || "${status}" -ge 300 ]]; then
    echo "${label} failed (status=${status}):" >&2
    cat "${response_file}" >&2
    rm -f "${response_file}"
    exit 1
  fi

  cat "${response_file}"
  rm -f "${response_file}"
}

wait_for_operator_surface() {
  local base_url="$1"

  if wait_for_http "${base_url}/healthz" 60 "edge healthz"; then
    wait_for_http "${base_url}/api/v2/challenges" 60 "public challenges"
    return 0
  fi

  wait_for_http "${base_url}/api/v2/challenges" 60 "public challenges"
}

wait_for_operator_surface "${API_URL}"

scheduler_response="$(
  curl_json "update scheduler interval" \
    -X PUT "${API_URL}/api/v2/admin/game/scheduler/interval" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --argjson interval_seconds "${TARGET_TICK_INTERVAL_SECONDS}" '{interval_seconds:$interval_seconds}')"
)"

match_start_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
match_end_at="$(date -u -d "+${TARGET_MATCH_DURATION_HOURS} hours" +"%Y-%m-%dT%H:%M:%SZ")"
match_schedule_response="$(
  curl_json "update match schedule" \
    -X PUT "${API_URL}/api/v2/admin/game/match/schedule" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg scheduled_start_at "${match_start_at}" --arg scheduled_end_at "${match_end_at}" '{scheduled_start_at:$scheduled_start_at,scheduled_end_at:$scheduled_end_at}')"
)"

match_response='null'
scheduler_start_response='null'

if [[ "${TARGET_START_MATCH}" == "true" ]]; then
  match_response="$(
    curl_json "start match" \
      -X POST "${API_URL}/api/v2/admin/game/match/start" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}"
  )"
fi

if [[ "${TARGET_START_SCHEDULER}" == "true" ]]; then
  scheduler_start_response="$(
    curl_json "start scheduler" \
      -X POST "${API_URL}/api/v2/admin/game/scheduler/start" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}"
  )"
fi

game_status="$(
  curl_json "game status" \
    "${API_URL}/api/v2/game/status"
)"

jq -n \
  --arg generated_at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
  --arg api_url "${API_URL}" \
  --argjson scheduler "$(printf '%s' "${scheduler_response}")" \
  --argjson match_schedule "$(printf '%s' "${match_schedule_response}")" \
  --argjson match_start "${match_response}" \
  --argjson scheduler_start "${scheduler_start_response}" \
  --argjson game_status "$(printf '%s' "${game_status}")" \
  '{
    generated_at: $generated_at,
    api_url: $api_url,
    scheduler: $scheduler,
    match_schedule: $match_schedule,
    match_start: $match_start,
    scheduler_start: $scheduler_start,
    game_status: $game_status
  }' > "${TARGET_FINALIZE_ARTIFACT}"

echo "faust target-shape runtime finalized"
printf '  %s\n' \
  "scheduler interval: ${TARGET_TICK_INTERVAL_SECONDS}s" \
  "match window: ${match_start_at} -> ${match_end_at}" \
  "artifact: ${TARGET_FINALIZE_ARTIFACT}"
