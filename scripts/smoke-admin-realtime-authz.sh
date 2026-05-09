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
PARTICIPANT_EMAIL="security.realtime.${TEMP_STAMP}@teams.local"
PARTICIPANT_PASSWORD="security-realtime-secret"
TEMP_PLAYER_ID=""
ORGANIZER_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ORGANIZER_PASSWORD="${ADMIN_PASSWORD:-testing123}"

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
  local email="$1"
  local password="$2"
  local jar response status

  jar="$(new_temp_file)"
  response="$(curl -sS -o /dev/null -w '%{http_code}' \
    -c "${jar}" \
    -b "${jar}" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg email "${email}" --arg password "${password}" '{email:$email,password:$password}')" \
    "${EDGE_BASE_URL}/api/platform/session/login")"

  status="${response}"
  if [[ "${status}" != "200" ]]; then
    echo "session login failed for ${email}: status=${status}" >&2
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
  curl -sS -N --max-time 2 \
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

assert_status_detail() {
  local label="$1"
  local expected_status="$2"
  local expected_detail="$3"
  local capture_file="$4"
  local actual_status actual_detail

  actual_status="$(awk -F= '$1=="http_status"{print $2}' "${capture_file}")"
  if [[ "${actual_status}" != "${expected_status}" ]]; then
    echo "${label}: expected status ${expected_status}, got ${actual_status}" >&2
    cat "${capture_file}" >&2
    exit 1
  fi

  actual_detail="$(field_after_marker "${capture_file}" '---body---' | jq -r '.detail // empty')"
  if [[ "${actual_detail}" != *"${expected_detail}"* ]]; then
    echo "${label}: expected detail containing '${expected_detail}', got '${actual_detail}'" >&2
    cat "${capture_file}" >&2
    exit 1
  fi
}

assert_stream_ok() {
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
    echo "${label}: expected SSE payload or keepalive in body" >&2
    cat "${capture_file}" >&2
    exit 1
  fi
}

assert_organizer_not_blocked() {
  local label="$1"
  local capture_file="$2"
  local actual_status content_type detail body

  actual_status="$(awk -F= '$1=="http_status"{print $2}' "${capture_file}")"
  case "${actual_status}" in
    200)
      content_type="$(field_after_marker "${capture_file}" '---headers---' | grep -i '^Content-Type:' || true)"
      if [[ "${content_type}" != *"text/event-stream"* ]]; then
        echo "${label}: expected text/event-stream for 200 response, got '${content_type}'" >&2
        cat "${capture_file}" >&2
        exit 1
      fi
      ;;
    502)
      detail="$(field_after_marker "${capture_file}" '---body---' | jq -r '.message // empty')"
      if [[ "${detail}" != *"admin realtime"* ]]; then
        echo "${label}: expected admin realtime upstream failure message, got '${detail}'" >&2
        cat "${capture_file}" >&2
        exit 1
      fi
      ;;
    *)
      echo "${label}: expected status 200 or 502, got ${actual_status}" >&2
      cat "${capture_file}" >&2
      exit 1
      ;;
  esac

  body="$(field_after_marker "${capture_file}" '---body---')"
  if [[ "${actual_status}" == "200" && "${body}" != *"data:"* && "${body}" != *": keepalive"* ]]; then
    echo "${label}: expected SSE payload or keepalive in body" >&2
    cat "${capture_file}" >&2
    exit 1
  fi
}

assert_public_not_blocked() {
  local label="$1"
  local capture_file="$2"
  local actual_status content_type detail body

  actual_status="$(awk -F= '$1=="http_status"{print $2}' "${capture_file}")"
  case "${actual_status}" in
    200)
      content_type="$(field_after_marker "${capture_file}" '---headers---' | grep -i '^Content-Type:' || true)"
      if [[ "${content_type}" != *"text/event-stream"* ]]; then
        echo "${label}: expected text/event-stream for 200 response, got '${content_type}'" >&2
        cat "${capture_file}" >&2
        exit 1
      fi
      ;;
    502)
      detail="$(field_after_marker "${capture_file}" '---body---' | jq -r '.message // empty')"
      if [[ "${detail}" != *"participant realtime"* ]]; then
        echo "${label}: expected participant realtime upstream failure message, got '${detail}'" >&2
        cat "${capture_file}" >&2
        exit 1
      fi
      ;;
    *)
      echo "${label}: expected status 200 or 502, got ${actual_status}" >&2
      cat "${capture_file}" >&2
      exit 1
      ;;
  esac

  body="$(field_after_marker "${capture_file}" '---body---')"
  if [[ "${actual_status}" == "200" && "${body}" != *"data:"* && "${body}" != *": keepalive"* ]]; then
    echo "${label}: expected SSE payload or keepalive in body" >&2
    cat "${capture_file}" >&2
    exit 1
  fi
}

echo "admin realtime authz smoke: base_url=${EDGE_BASE_URL}"

if [[ -z "${ADMIN_TOKEN}" ]]; then
  echo "ADMIN_API_TOKEN must be set in ${PROD_ENV}." >&2
  exit 1
fi

create_player_body="$(
  jq -nc \
    --argjson team_id "${SECURITY_TEAM_ID}" \
    --arg display_name "Security Realtime Probe" \
    --arg email "${PARTICIPANT_EMAIL}" \
    --arg password "${PARTICIPANT_PASSWORD}" \
    --arg role "captain" \
    '{team_id:$team_id,display_name:$display_name,email:$email,password:$password,role:$role}'
)"
create_player_response="$(
  curl -sS -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "${create_player_body}" \
    "${EDGE_BASE_URL}/api/v2/admin/players"
)"
TEMP_PLAYER_ID="$(printf '%s' "${create_player_response}" | jq -er '.id')"
echo "created temporary participant player id=${TEMP_PLAYER_ID}"

anonymous_capture="$(new_temp_file)"
capture_stream_response "${anonymous_capture}" \
  "${EDGE_BASE_URL}/api/admin/realtime/game/status/stream"
assert_status_detail \
  "anonymous admin realtime" \
  "403" \
  "please authenticate before accessing organizer routes." \
  "${anonymous_capture}"
echo "anonymous admin realtime denied"

participant_cookie_jar="$(login_cookie_jar "${PARTICIPANT_EMAIL}" "${PARTICIPANT_PASSWORD}")"
participant_capture="$(new_temp_file)"
capture_stream_response "${participant_capture}" \
  -b "${participant_cookie_jar}" \
  "${EDGE_BASE_URL}/api/admin/realtime/game/status/stream"
assert_status_detail \
  "participant admin realtime" \
  "403" \
  "please authenticate as organizer." \
  "${participant_capture}"
echo "participant admin realtime denied"

organizer_cookie_jar="$(login_cookie_jar "${ORGANIZER_EMAIL}" "${ORGANIZER_PASSWORD}")"
organizer_capture="$(new_temp_file)"
capture_stream_response "${organizer_capture}" \
  -b "${organizer_cookie_jar}" \
  "${EDGE_BASE_URL}/api/admin/realtime/game/status/stream"
assert_organizer_not_blocked "organizer admin realtime" "${organizer_capture}"
echo "organizer admin realtime passed auth gate"

public_capture="$(new_temp_file)"
capture_stream_response "${public_capture}" \
  "${EDGE_BASE_URL}/api/platform/realtime/scoreboard/stream"
assert_public_not_blocked "public participant realtime" "${public_capture}"
echo "public participant realtime reachable"

echo "admin realtime authz smoke passed"
