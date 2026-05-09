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
require_bin mktemp
require_bin grep

EDGE_BASE_URL="${PROD_EDGE_BASE_URL:-$(derive_edge_base_url)}"
ADMIN_TOKEN="${ADMIN_API_TOKEN:-}"
SECURITY_TEAM_ID="${SECURITY_TEAM_ID:-101}"
TEMP_STAMP="$(date +%s)"
ORGANIZER_EMAIL="security.realtime.health.${TEMP_STAMP}@teams.local"
ORGANIZER_PASSWORD="security-realtime-health-secret"
TEMP_PLAYER_ID=""
REALTIME_HEALTH_ARTIFACT_FILE="${REALTIME_HEALTH_ARTIFACT_FILE:-}"

cleanup_files=()

cleanup() {
  local path=""
  if [[ -n "${TEMP_PLAYER_ID}" && -n "${ADMIN_TOKEN}" ]]; then
    curl -sS -X DELETE -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      "${EDGE_BASE_URL}/api/v2/admin/players/${TEMP_PLAYER_ID}" >/dev/null || true
  fi
  for path in "${cleanup_files[@]:-}"; do
    [[ -n "${path}" ]] && rm -f "${path}" || true
  done
}
trap cleanup EXIT

new_temp_file() {
  local path
  path="$(mktemp)"
  cleanup_files+=("${path}")
  printf '%s\n' "${path}"
}

login_cookie_jar() {
  local jar response status
  jar="$(new_temp_file)"
  response="$(curl -sS -o /dev/null -w '%{http_code}' \
    -c "${jar}" \
    -b "${jar}" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg email "${ORGANIZER_EMAIL}" --arg password "${ORGANIZER_PASSWORD}" '{email:$email,password:$password}')" \
    "${EDGE_BASE_URL}/api/platform/session/login")"
  status="${response}"
  if [[ "${status}" != "200" ]]; then
    echo "organizer session login failed: status=${status}" >&2
    exit 1
  fi
  printf '%s\n' "${jar}"
}

capture_stream_response() {
  local output_file="$1"
  shift

  local status_file header_file body_file curl_status
  status_file="$(new_temp_file)"
  header_file="$(new_temp_file)"
  body_file="$(new_temp_file)"

  set +e
  curl -sS -N --max-time 3 \
    -D "${header_file}" \
    -o "${body_file}" \
    -w '%{http_code}' \
    "$@" >"${status_file}"
  curl_status=$?
  set -e

  {
    printf 'curl_status=%s\n' "${curl_status}"
    printf 'http_status=%s\n' "$(cat "${status_file}")"
    printf '%s\n' '---headers---'
    cat "${header_file}"
    printf '%s\n' '---body---'
    cat "${body_file}"
  } > "${output_file}"
}

field_after_marker() {
  local file="$1"
  local marker="$2"
  awk -v marker="${marker}" '
    found && $0 ~ /^---/ { exit }
    found { print }
    $0 == marker { found=1 }
  ' "${file}"
}

assert_stream_healthy() {
  local label="$1"
  local capture_file="$2"
  local actual_status content_type body

  actual_status="$(awk -F= '$1=="http_status"{print $2}' "${capture_file}")"
  if [[ "${actual_status}" != "200" ]]; then
    echo "${label}: expected status 200, got ${actual_status}" >&2
    cat "${capture_file}" >&2
    exit 1
  fi

  content_type="$(field_after_marker "${capture_file}" '---headers---' | grep -i '^Content-Type:' || true)"
  if [[ "${content_type}" != *"text/event-stream"* ]]; then
    echo "${label}: expected text/event-stream, got '${content_type}'" >&2
    cat "${capture_file}" >&2
    exit 1
  fi

  body="$(field_after_marker "${capture_file}" '---body---')"
  if [[ "${body}" != *"data:"* && "${body}" != *": keepalive"* ]]; then
    echo "${label}: expected SSE payload or keepalive" >&2
    cat "${capture_file}" >&2
    exit 1
  fi
}

wait_for_http "${EDGE_BASE_URL}/healthz" 60 "edge healthz"

echo "realtime health smoke: base_url=${EDGE_BASE_URL}"

if [[ -z "${ADMIN_TOKEN}" ]]; then
  echo "ADMIN_API_TOKEN must be set in ${PROD_ENV}." >&2
  exit 1
fi

create_player_body="$(
  jq -nc \
    --argjson team_id "${SECURITY_TEAM_ID}" \
    --arg display_name "Security Realtime Health Organizer" \
    --arg email "${ORGANIZER_EMAIL}" \
    --arg password "${ORGANIZER_PASSWORD}" \
    --arg role "organizer" \
    '{team_id:$team_id,display_name:$display_name,email:$email,password:$password,role:$role}'
)"
create_player_response="$(
  curl -sS -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "${create_player_body}" \
    "${EDGE_BASE_URL}/api/v2/admin/players"
)"
TEMP_PLAYER_ID="$(printf '%s' "${create_player_response}" | jq -er '.id')"

organizer_cookie_jar="$(login_cookie_jar)"

public_scoreboard_capture="$(new_temp_file)"
capture_stream_response "${public_scoreboard_capture}" \
  "${EDGE_BASE_URL}/api/platform/realtime/scoreboard/stream"
assert_stream_healthy "public realtime scoreboard" "${public_scoreboard_capture}"
echo "public realtime scoreboard healthy"

public_attacks_capture="$(new_temp_file)"
capture_stream_response "${public_attacks_capture}" \
  "${EDGE_BASE_URL}/api/platform/realtime/attacks/stream"
assert_stream_healthy "public realtime attacks" "${public_attacks_capture}"
echo "public realtime attacks healthy"

admin_status_capture="$(new_temp_file)"
capture_stream_response "${admin_status_capture}" \
  -b "${organizer_cookie_jar}" \
  "${EDGE_BASE_URL}/api/admin/realtime/game/status/stream"
assert_stream_healthy "admin realtime game status" "${admin_status_capture}"
echo "admin realtime game status healthy"

admin_scoreboard_capture="$(new_temp_file)"
capture_stream_response "${admin_scoreboard_capture}" \
  -b "${organizer_cookie_jar}" \
  "${EDGE_BASE_URL}/api/admin/realtime/game/scoreboard/stream"
assert_stream_healthy "admin realtime scoreboard" "${admin_scoreboard_capture}"
echo "admin realtime scoreboard healthy"

if [[ -n "${REALTIME_HEALTH_ARTIFACT_FILE}" ]]; then
  mkdir -p "$(dirname "${REALTIME_HEALTH_ARTIFACT_FILE}")"
  jq -n \
    --arg generated_at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
    --arg base_url "${EDGE_BASE_URL}" \
    '{
      generated_at: $generated_at,
      base_url: $base_url,
      streams: {
        public_scoreboard: "healthy",
        public_attacks: "healthy",
        admin_game_status: "healthy",
        admin_scoreboard: "healthy"
      }
    }' > "${REALTIME_HEALTH_ARTIFACT_FILE}"
fi

echo "realtime health smoke passed"
