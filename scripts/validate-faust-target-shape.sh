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
ADMIN_TOKEN="${ADMIN_API_TOKEN:-dev-admin-token}"

if [[ "${API_URL}" == *"api-gateway"* ]] && [[ -n "${PUBLIC_URL}" ]]; then
  API_URL="${PUBLIC_URL}"
fi

TARGET_TEAM_COUNT="${TARGET_TEAM_COUNT:-10}"
TARGET_SERVICE_COUNT="${TARGET_SERVICE_COUNT:-5}"
TARGET_TICK_INTERVAL_SECONDS="${TARGET_TICK_INTERVAL_SECONDS:-300}"
TARGET_MATCH_DURATION_HOURS="${TARGET_MATCH_DURATION_HOURS:-24}"
TARGET_CHALLENGE_PREFIX="${TARGET_CHALLENGE_PREFIX:-faust-service}"
TARGET_VALIDATION_ARTIFACT="${TARGET_VALIDATION_ARTIFACT:-.runtime/faust-target-shape-validation.json}"

usage() {
  cat <<'EOF'
Usage: scripts/validate-faust-target-shape.sh

Validates that the live stack matches the intended Faust-style contest shape.

Environment overrides:
  TARGET_TEAM_COUNT=10
  TARGET_SERVICE_COUNT=5
  TARGET_TICK_INTERVAL_SECONDS=300
  TARGET_MATCH_DURATION_HOURS=24
  TARGET_CHALLENGE_PREFIX=faust-service
  TARGET_VALIDATION_ARTIFACT=.runtime/faust-target-shape-validation.json
  AD_PLATFORM_API_URL
  AD_PLATFORM_PUBLIC_BASE_URL
  ADMIN_API_TOKEN
EOF
}

if [[ "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

mkdir -p "$(dirname "${TARGET_VALIDATION_ARTIFACT}")"

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

deployments="$(
  curl_json "list deployments" \
    "${API_URL}/api/v2/admin/deployments" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
challenges="$(
  curl_json "list challenges" \
    "${API_URL}/api/v2/challenges"
)"
game_status="$(
  curl_json "game status" \
    "${API_URL}/api/v2/game/status"
)"
admin_scheduler="$(
  curl_json "scheduler status" \
    "${API_URL}/api/v2/admin/game/scheduler" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
admin_match="$(
  curl_json "match status" \
    "${API_URL}/api/v2/admin/game/match" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
public_scoreboard="$(
  curl_json "scoreboard" \
    "${API_URL}/api/v2/scoreboard"
)"

if ! printf '%s\n' "${challenges}" | jq -e \
  --arg prefix "${TARGET_CHALLENGE_PREFIX}" \
  --argjson expected "${TARGET_SERVICE_COUNT}" \
  'map(select(.name | startswith($prefix))) | length == $expected' >/dev/null; then
  echo "target challenge count/prefix mismatch" >&2
  printf '%s\n' "${challenges}" >&2
  exit 1
fi

if ! printf '%s\n' "${deployments}" | jq -e \
  --arg prefix "${TARGET_CHALLENGE_PREFIX}" \
  --argjson expected_services "${TARGET_SERVICE_COUNT}" \
  --argjson expected_teams "${TARGET_TEAM_COUNT}" '
    map(select(.challenge_name | startswith($prefix))) as $rows
    | ($rows | length) == $expected_services
      and all($rows[]; .status == "completed")
      and all($rows[]; .ready_team_count == $expected_teams)
      and all($rows[]; .failed_team_count == 0)
  ' >/dev/null; then
  echo "deployment shape mismatch" >&2
  printf '%s\n' "${deployments}" >&2
  exit 1
fi

if ! printf '%s\n' "${admin_scheduler}" | jq -e \
  --argjson expected "${TARGET_TICK_INTERVAL_SECONDS}" \
  '.interval_seconds == $expected and .state == "running"' >/dev/null; then
  echo "scheduler shape mismatch" >&2
  printf '%s\n' "${admin_scheduler}" >&2
  exit 1
fi

if ! printf '%s\n' "${admin_match}" | jq -e '.state == "running" and .accepting_submissions == true' >/dev/null; then
  echo "match is not running" >&2
  printf '%s\n' "${admin_match}" >&2
  exit 1
fi

if ! printf '%s\n' "${game_status}" | jq -e '.match.state == "running" and .scheduler.state == "running"' >/dev/null; then
  echo "public game status mismatch" >&2
  printf '%s\n' "${game_status}" >&2
  exit 1
fi

if ! printf '%s\n' "${public_scoreboard}" | jq -e --argjson expected "${TARGET_TEAM_COUNT}" 'length == $expected' >/dev/null; then
  echo "scoreboard row count mismatch" >&2
  printf '%s\n' "${public_scoreboard}" >&2
  exit 1
fi

match_duration_seconds="$(
  printf '%s\n' "${admin_match}" | jq -r '
    if (.scheduled_start_at // "") != "" and (.scheduled_end_at // "") != "" then
      ((.scheduled_end_at | fromdateiso8601) - (.scheduled_start_at | fromdateiso8601))
    else
      0
    end
  '
)"
expected_duration_seconds=$((TARGET_MATCH_DURATION_HOURS * 3600))
if [[ "${match_duration_seconds}" != "${expected_duration_seconds}" ]]; then
  echo "match duration mismatch: got ${match_duration_seconds}, want ${expected_duration_seconds}" >&2
  printf '%s\n' "${admin_match}" >&2
  exit 1
fi

jq -n \
  --arg generated_at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
  --arg api_url "${API_URL}" \
  --argjson target_team_count "${TARGET_TEAM_COUNT}" \
  --argjson target_service_count "${TARGET_SERVICE_COUNT}" \
  --argjson target_tick_interval_seconds "${TARGET_TICK_INTERVAL_SECONDS}" \
  --argjson target_match_duration_hours "${TARGET_MATCH_DURATION_HOURS}" \
  --argjson deployments "$(printf '%s' "${deployments}")" \
  --argjson challenges "$(printf '%s' "${challenges}")" \
  --argjson scheduler "$(printf '%s' "${admin_scheduler}")" \
  --argjson match "$(printf '%s' "${admin_match}")" \
  --argjson game_status "$(printf '%s' "${game_status}")" \
  --argjson scoreboard "$(printf '%s' "${public_scoreboard}")" \
  '{
    generated_at: $generated_at,
    api_url: $api_url,
    target_shape: {
      team_count: $target_team_count,
      service_count: $target_service_count,
      tick_interval_seconds: $target_tick_interval_seconds,
      match_duration_hours: $target_match_duration_hours
    },
    deployments: $deployments,
    challenges: $challenges,
    scheduler: $scheduler,
    match: $match,
    game_status: $game_status,
    scoreboard: $scoreboard
  }' > "${TARGET_VALIDATION_ARTIFACT}"

echo "faust target-shape validation passed"
printf '  %s\n' \
  "teams: ${TARGET_TEAM_COUNT}" \
  "services: ${TARGET_SERVICE_COUNT}" \
  "scheduler interval: ${TARGET_TICK_INTERVAL_SECONDS}s" \
  "match duration: ${TARGET_MATCH_DURATION_HOURS}h" \
  "artifact: ${TARGET_VALIDATION_ARTIFACT}"
