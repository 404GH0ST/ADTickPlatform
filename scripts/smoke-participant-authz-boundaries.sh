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

EDGE_BASE_URL="${PROD_EDGE_BASE_URL:-$(derive_edge_base_url)}"
ADMIN_TOKEN="${ADMIN_API_TOKEN:-}"
SECURITY_TEAM_ID="${SECURITY_TEAM_ID:-101}"
TEMP_STAMP="$(date +%s)"
TEMP_EMAIL="security.probe.${TEMP_STAMP}@teams.local"
TEMP_PASSWORD="security-probe-secret"
DRAFT_CHALLENGE_NAME="security-draft-source-${TEMP_STAMP}"
QUEUED_CHALLENGE_NAME="security-queued-service-${TEMP_STAMP}"
TEMP_PLAYER_ID=""
DRAFT_CHALLENGE_ID=""
QUEUED_CHALLENGE_ID=""

if [[ -z "${ADMIN_TOKEN}" ]]; then
  echo "ADMIN_API_TOKEN must be set in ${PROD_ENV}." >&2
  exit 1
fi

cleanup() {
  if [[ -n "${TEMP_PLAYER_ID}" ]]; then
    curl_retry -X DELETE -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      "${EDGE_BASE_URL}/api/v2/admin/players/${TEMP_PLAYER_ID}" >/dev/null || true
  fi
  if [[ -n "${DRAFT_CHALLENGE_ID}" ]]; then
    curl_retry -X DELETE -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      "${EDGE_BASE_URL}/api/v2/admin/challenges/${DRAFT_CHALLENGE_ID}" >/dev/null || true
  fi
  if [[ -n "${QUEUED_CHALLENGE_ID}" ]]; then
    curl_retry -X DELETE -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      "${EDGE_BASE_URL}/api/v2/admin/challenges/${QUEUED_CHALLENGE_ID}" >/dev/null || true
  fi
}
trap cleanup EXIT

http_request() {
  local body_file status
  body_file="$(mktemp)"
  status="$(curl -sS -o "${body_file}" -w '%{http_code}' "$@")"
  printf '%s\n' "${status}"
  cat "${body_file}"
  rm -f "${body_file}"
}

curl_retry() {
  local attempts="${CURL_RETRY_ATTEMPTS:-5}"
  local delay="${CURL_RETRY_DELAY_SECONDS:-1}"
  local attempt exit_code=0

  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if curl -sS "$@"; then
      return 0
    fi
    exit_code=$?
    if (( attempt == attempts )); then
      return "${exit_code}"
    fi
    sleep "${delay}"
  done

  return "${exit_code}"
}

expect_problem() {
  local label="$1"
  local expected_status="$2"
  local expected_detail_substr="$3"
  shift 3

  local response status body detail
  response="$(http_request "$@")"
  status="$(printf '%s\n' "${response}" | sed -n '1p')"
  body="$(printf '%s\n' "${response}" | tail -n +2)"

  if [[ "${status}" != "${expected_status}" ]]; then
    echo "${label} failed: expected status ${expected_status}, got ${status}" >&2
    printf '%s\n' "${body}" >&2
    exit 1
  fi

  detail="$(printf '%s' "${body}" | jq -r '.detail // empty')"
  if [[ "${detail}" != *"${expected_detail_substr}"* ]]; then
    echo "${label} failed: expected detail containing '${expected_detail_substr}', got '${detail}'" >&2
    exit 1
  fi
}

echo "participant authz smoke: base_url=${EDGE_BASE_URL}"

create_player_body="$(
  jq -nc \
    --argjson team_id "${SECURITY_TEAM_ID}" \
    --arg display_name "Security Probe" \
    --arg email "${TEMP_EMAIL}" \
    --arg password "${TEMP_PASSWORD}" \
    --arg role "captain" \
    '{team_id:$team_id,display_name:$display_name,email:$email,password:$password,role:$role}'
)"
create_player_response="$(
  curl_retry -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "${create_player_body}" \
    "${EDGE_BASE_URL}/api/v2/admin/players"
)"
TEMP_PLAYER_ID="$(printf '%s' "${create_player_response}" | jq -er '.id')"
echo "created temporary player id=${TEMP_PLAYER_ID}"

participant_auth_body="$(
  jq -nc \
    --arg email "${TEMP_EMAIL}" \
    --arg password "${TEMP_PASSWORD}" \
    '{email:$email,password:$password}'
)"
participant_token="$(
  curl_retry -H 'Content-Type: application/json' \
    -d "${participant_auth_body}" \
    "${EDGE_BASE_URL}/api/v2/authenticate" |
    jq -er '.token'
)"
printf 'participant token prefix=%s...\n' "${participant_token:0:16}"

expect_problem \
  "participant token on admin route" \
  "403" \
  "please authenticate as organizer." \
  -H "Authorization: Bearer ${participant_token}" \
  "${EDGE_BASE_URL}/api/v2/admin/teams"
echo "participant token denied on admin route"

expect_problem \
  "admin token on participant route" \
  "403" \
  "please authenticate before access." \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  "${EDGE_BASE_URL}/api/v2/team/services"
echo "admin token denied on participant route"

curl_retry -X DELETE -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  "${EDGE_BASE_URL}/api/v2/admin/players/${TEMP_PLAYER_ID}" >/dev/null
TEMP_PLAYER_ID=""
expect_problem \
  "deleted player token on participant route" \
  "403" \
  "please authenticate before access." \
  -H "Authorization: Bearer ${participant_token}" \
  "${EDGE_BASE_URL}/api/v2/team/services"
echo "deleted player token denied"

create_player_response="$(
  curl_retry -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "${create_player_body}" \
    "${EDGE_BASE_URL}/api/v2/admin/players"
)"
TEMP_PLAYER_ID="$(printf '%s' "${create_player_response}" | jq -er '.id')"
participant_token="$(
  curl_retry -H 'Content-Type: application/json' \
    -d "${participant_auth_body}" \
    "${EDGE_BASE_URL}/api/v2/authenticate" |
    jq -er '.token'
)"
printf 'replacement participant token prefix=%s...\n' "${participant_token:0:16}"

team_services="$(
  curl_retry -H "Authorization: Bearer ${participant_token}" \
    "${EDGE_BASE_URL}/api/v2/team/services"
)"
locked_challenge_id="$(
  printf '%s' "${team_services}" | jq -er 'first(.[] | select(.unlocked == false) | .challenge_id)'
)"
echo "using locked challenge_id=${locked_challenge_id}"

expect_problem \
  "locked ssh session" \
  "400" \
  "service is not unlocked yet." \
  -X POST \
  -H "Authorization: Bearer ${participant_token}" \
  "${EDGE_BASE_URL}/api/v2/services/${locked_challenge_id}/ssh-session"
echo "locked service ssh denied"

expect_problem \
  "invalid unlock proof" \
  "400" \
  "unlock proof is invalid." \
  -X POST \
  -H "Authorization: Bearer ${participant_token}" \
  -H 'Content-Type: application/json' \
  -d '{"proof":"definitely-not-valid"}' \
  "${EDGE_BASE_URL}/api/v2/services/${locked_challenge_id}/unlock"
echo "invalid unlock proof denied"

draft_challenge_body="$(
  source_bundle_path="$(
    curl_retry -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      "${EDGE_BASE_URL}/api/v2/admin/challenges" |
      jq -er 'first(.[] | select(.source_bundle_path != null and .source_bundle_path != "") | .source_bundle_path)'
  )"
  jq -nc \
    --arg name "${DRAFT_CHALLENGE_NAME}" \
    --arg baseline_image "registry.local/sec-draft:baseline" \
    --arg checker_image "registry.local/sec-draft-checker:latest" \
    --arg source_bundle_path "${source_bundle_path}" \
    '{name:$name,baseline_image:$baseline_image,checker_image:$checker_image,source_bundle_path:$source_bundle_path}'
)"
draft_challenge_response="$(
  curl_retry -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "${draft_challenge_body}" \
    "${EDGE_BASE_URL}/api/v2/admin/challenges"
)"
DRAFT_CHALLENGE_ID="$(printf '%s' "${draft_challenge_response}" | jq -er '.id')"
echo "created draft challenge id=${DRAFT_CHALLENGE_ID}"

expect_problem \
  "draft source download" \
  "404" \
  "challenge source is unavailable." \
  -H "Authorization: Bearer ${participant_token}" \
  "${EDGE_BASE_URL}/api/v2/challenges/${DRAFT_CHALLENGE_ID}/source"
echo "draft source denied"

expect_problem \
  "draft service unlock" \
  "400" \
  "challenge id is invalid." \
  -X POST \
  -H "Authorization: Bearer ${participant_token}" \
  -H 'Content-Type: application/json' \
  -d '{"proof":"definitely-not-valid"}' \
  "${EDGE_BASE_URL}/api/v2/services/${DRAFT_CHALLENGE_ID}/unlock"
echo "draft service unlock denied"

expect_problem \
  "draft service ssh" \
  "400" \
  "challenge id is invalid." \
  -X POST \
  -H "Authorization: Bearer ${participant_token}" \
  "${EDGE_BASE_URL}/api/v2/services/${DRAFT_CHALLENGE_ID}/ssh-session"
echo "draft service ssh denied"

expect_problem \
  "draft service factory reset" \
  "400" \
  "challenge id is invalid." \
  -X POST \
  -H "Authorization: Bearer ${participant_token}" \
  "${EDGE_BASE_URL}/api/v2/services/${DRAFT_CHALLENGE_ID}/reset/factory"
echo "draft service factory reset denied"

expect_problem \
  "draft service restart" \
  "400" \
  "challenge id is invalid." \
  -X POST \
  -H "Authorization: Bearer ${participant_token}" \
  "${EDGE_BASE_URL}/api/v2/services/${DRAFT_CHALLENGE_ID}/reset/restart"
echo "draft service restart denied"

queued_challenge_body="$(
  challenge_template="$(
    curl_retry -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      "${EDGE_BASE_URL}/api/v2/admin/challenges" |
      jq -er 'first(.[] | {baseline_image, checker_image, source_bundle_path})'
  )"
  jq -nc \
    --arg name "${QUEUED_CHALLENGE_NAME}" \
    --argjson template "${challenge_template}" \
    '{
      name:$name,
      baseline_image:$template.baseline_image,
      checker_image:$template.checker_image,
      source_bundle_path:($template.source_bundle_path // "")
    }'
)"
queued_challenge_response="$(
  curl_retry -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "${queued_challenge_body}" \
    "${EDGE_BASE_URL}/api/v2/admin/challenges"
)"
QUEUED_CHALLENGE_ID="$(printf '%s' "${queued_challenge_response}" | jq -er '.id')"
echo "created queued challenge id=${QUEUED_CHALLENGE_ID}"

curl_retry -X POST -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  "${EDGE_BASE_URL}/api/v2/admin/challenges/${QUEUED_CHALLENGE_ID}/deploy" >/dev/null
echo "queued challenge deployed without reconcile"

queued_state="$(
  curl_retry -H "Authorization: Bearer ${participant_token}" \
    "${EDGE_BASE_URL}/api/v2/team/services" |
    jq -r --argjson challenge_id "${QUEUED_CHALLENGE_ID}" 'first(.[] | select(.challenge_id == $challenge_id) | .reset_cooldown) // ""'
)"
if [[ "${queued_state}" == "deploying" ]]; then
  expect_problem \
    "queued service unlock" \
    "400" \
    "service is not available yet." \
    -X POST \
    -H "Authorization: Bearer ${participant_token}" \
    -H 'Content-Type: application/json' \
    -d '{"proof":"definitely-not-valid"}' \
    "${EDGE_BASE_URL}/api/v2/services/${QUEUED_CHALLENGE_ID}/unlock"
  echo "queued service unlock denied"

  expect_problem \
    "queued service ssh" \
    "400" \
    "service is not available yet." \
    -X POST \
    -H "Authorization: Bearer ${participant_token}" \
    "${EDGE_BASE_URL}/api/v2/services/${QUEUED_CHALLENGE_ID}/ssh-session"
  echo "queued service ssh denied"

  expect_problem \
    "queued service factory reset" \
    "400" \
    "service is not available yet." \
    -X POST \
    -H "Authorization: Bearer ${participant_token}" \
    "${EDGE_BASE_URL}/api/v2/services/${QUEUED_CHALLENGE_ID}/reset/factory"
  echo "queued service factory reset denied"

  expect_problem \
    "queued service restart" \
    "400" \
    "service is not available yet." \
    -X POST \
    -H "Authorization: Bearer ${participant_token}" \
    "${EDGE_BASE_URL}/api/v2/services/${QUEUED_CHALLENGE_ID}/reset/restart"
  echo "queued service restart denied"
else
  echo "queued challenge reached ready state before denial checks; skipped queued-state assertions"
fi

echo "participant authz smoke passed"
