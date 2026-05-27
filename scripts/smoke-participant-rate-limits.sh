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
ADMIN_TOKEN="$(resolve_admin_api_token "${ROOT_DIR}/.runtime/backend-stack.env")"
SECURITY_TEAM_ID="${SECURITY_TEAM_ID:-101}"
TEMP_STAMP="$(date +%s)"
TEMP_EMAIL="security.rate.${TEMP_STAMP}@teams.local"
TEMP_PASSWORD="security-rate-secret"
INVALID_EMAIL="security.rate.invalid.${TEMP_STAMP}@teams.local"
TEMP_PLAYER_ID=""
RATE_LIMIT_DRAFT_CHALLENGE_ID=""
RATE_LIMIT_DRAFT_CHALLENGE_NAME="security-rate-draft-${TEMP_STAMP}"

wait_for_http "${EDGE_BASE_URL}/healthz" 60 "edge healthz"

cleanup() {
  if [[ -n "${TEMP_PLAYER_ID}" ]]; then
    curl -fsS -X DELETE -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      "${EDGE_BASE_URL}/api/v2/admin/players/${TEMP_PLAYER_ID}" >/dev/null || true
  fi
  if [[ -n "${RATE_LIMIT_DRAFT_CHALLENGE_ID}" ]]; then
    curl -fsS -X DELETE -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      "${EDGE_BASE_URL}/api/v2/admin/challenges/${RATE_LIMIT_DRAFT_CHALLENGE_ID}" >/dev/null || true
  fi
}
trap cleanup EXIT

http_status_body() {
  local body_file status
  body_file="$(mktemp)"
  status="$(curl -sS -o "${body_file}" -w '%{http_code}' "$@")"
  printf '%s\n' "${status}"
  cat "${body_file}"
  rm -f "${body_file}"
}

expect_status() {
  local label="$1" expected="$2"
  shift 2
  local response status body
  response="$(http_status_body "$@")"
  status="$(printf '%s\n' "${response}" | sed -n '1p')"
  body="$(printf '%s\n' "${response}" | tail -n +2)"
  if [[ "${status}" != "${expected}" ]]; then
    echo "${label} failed: expected status ${expected}, got ${status}" >&2
    printf '%s\n' "${body}" >&2
    exit 1
  fi
  printf '%s\n' "${body}"
}

echo "participant rate-limit smoke: base_url=${EDGE_BASE_URL}"

auth_429=""
auth_429_seen=0
for attempt in 1 2 3 4 5 6 7 8 9 10; do
  response="$(http_status_body \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg email "${INVALID_EMAIL}" --arg password "wrong-password" '{email:$email,password:$password}')" \
    "${EDGE_BASE_URL}/api/v2/authenticate")"
  status="$(printf '%s\n' "${response}" | sed -n '1p')"
  body="$(printf '%s\n' "${response}" | tail -n +2)"
  if [[ "${status}" == "403" ]]; then
    continue
  fi
  if [[ "${status}" == "429" ]]; then
    auth_429_seen=1
    auth_429="${body}"
    break
  fi
  echo "invalid auth attempt ${attempt} failed with unexpected status ${status}" >&2
  printf '%s\n' "${body}" >&2
  exit 1
done
if [[ "${auth_429_seen}" -ne 1 ]]; then
  echo "auth brute-force rate limit did not trigger within 10 attempts" >&2
  exit 1
fi
auth_detail="$(printf '%s' "${auth_429}" | jq -r '.detail // empty')"
if [[ "${auth_detail}" != *"calm down"* ]]; then
  echo "unexpected auth rate-limit detail: ${auth_detail}" >&2
  exit 1
fi
echo "auth brute-force rate limit triggered"

create_player_body="$(
  jq -nc \
    --argjson team_id "${SECURITY_TEAM_ID}" \
    --arg display_name "Security Rate Probe" \
    --arg email "${TEMP_EMAIL}" \
    --arg password "${TEMP_PASSWORD}" \
    --arg role "captain" \
    '{team_id:$team_id,display_name:$display_name,email:$email,password:$password,role:$role}'
)"
create_player_response="$(
  curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "${create_player_body}" \
    "${EDGE_BASE_URL}/api/v2/admin/players"
)"
TEMP_PLAYER_ID="$(printf '%s' "${create_player_response}" | jq -er '.id')"
echo "created temporary player id=${TEMP_PLAYER_ID}"

participant_token="$(
  curl -fsS -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg email "${TEMP_EMAIL}" --arg password "${TEMP_PASSWORD}" '{email:$email,password:$password}')" \
    "${EDGE_BASE_URL}/api/v2/authenticate" |
    jq -er '.token'
)"
printf 'participant token prefix=%s...\n' "${participant_token:0:16}"

team_services_body=""
team_services_429_seen=0
for attempt in 1 2 3 4 5 6 7; do
  response="$(http_status_body -H "Authorization: Bearer ${participant_token}" "${EDGE_BASE_URL}/api/v2/team/services")"
  status="$(printf '%s\n' "${response}" | sed -n '1p')"
  body="$(printf '%s\n' "${response}" | tail -n +2)"
  if [[ "${status}" == "200" ]]; then
    team_services_body="${body}"
    continue
  fi
  if [[ "${status}" == "429" ]]; then
    team_services_429_seen=1
    break
  fi
  echo "team/services attempt ${attempt} failed with unexpected status ${status}" >&2
  printf '%s\n' "${body}" >&2
  exit 1
done
if [[ "${team_services_429_seen}" -ne 1 ]]; then
  echo "team/services rate limit did not trigger within 7 attempts" >&2
  exit 1
fi
echo "team/services rate limit triggered"

if [[ -z "${team_services_body}" ]]; then
  team_services_body="$(
    curl -fsS -H "Authorization: Bearer ${participant_token}" "${EDGE_BASE_URL}/api/v2/team/services"
  )"
fi
locked_challenge_id="$(printf '%s' "${team_services_body}" | jq -er 'first(.[] | select(.unlocked == false) | .challenge_id)')"
echo "using locked challenge_id=${locked_challenge_id}"

draft_challenge_body="$(
  source_bundle_path="$(
    curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      "${EDGE_BASE_URL}/api/v2/admin/challenges" |
      jq -er 'first(.[] | select(.source_bundle_path != null and .source_bundle_path != "") | .source_bundle_path)'
  )"
  jq -nc \
    --arg name "${RATE_LIMIT_DRAFT_CHALLENGE_NAME}" \
    --arg baseline_image "registry.local/security-rate-draft:baseline" \
    --arg checker_image "registry.local/security-rate-draft-checker:latest" \
    --arg source_bundle_path "${source_bundle_path}" \
    '{name:$name,baseline_image:$baseline_image,checker_image:$checker_image,source_bundle_path:$source_bundle_path}'
)"
draft_challenge_response="$(
  curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "${draft_challenge_body}" \
    "${EDGE_BASE_URL}/api/v2/admin/challenges"
)"
RATE_LIMIT_DRAFT_CHALLENGE_ID="$(printf '%s' "${draft_challenge_response}" | jq -er '.id')"
echo "created rate-limit draft challenge id=${RATE_LIMIT_DRAFT_CHALLENGE_ID}"

unlock_429_seen=0
for attempt in 1 2 3 4 5 6 7 8 9 10 11; do
  response="$(http_status_body \
    -X POST \
    -H "Authorization: Bearer ${participant_token}" \
    -H 'Content-Type: application/json' \
    -d '{"proof":"definitely-not-valid"}' \
    "${EDGE_BASE_URL}/api/v2/services/${locked_challenge_id}/unlock")"
  status="$(printf '%s\n' "${response}" | sed -n '1p')"
  body="$(printf '%s\n' "${response}" | tail -n +2)"
  if [[ "${status}" == "400" ]]; then
    continue
  fi
  if [[ "${status}" == "429" ]]; then
    unlock_429_seen=1
    break
  fi
  echo "unlock attempt ${attempt} failed with unexpected status ${status}" >&2
  printf '%s\n' "${body}" >&2
  exit 1
done
if [[ "${unlock_429_seen}" -ne 1 ]]; then
  echo "unlock rate limit did not trigger within 11 attempts" >&2
  exit 1
fi
echo "unlock rate limit triggered"

ssh_429_seen=0
for attempt in 1 2 3 4 5 6 7; do
  response="$(http_status_body \
    -X POST \
    -H "Authorization: Bearer ${participant_token}" \
    "${EDGE_BASE_URL}/api/v2/services/${RATE_LIMIT_DRAFT_CHALLENGE_ID}/ssh-session")"
  status="$(printf '%s\n' "${response}" | sed -n '1p')"
  body="$(printf '%s\n' "${response}" | tail -n +2)"
  if [[ "${status}" == "400" ]]; then
    continue
  fi
  if [[ "${status}" == "429" ]]; then
    ssh_429_seen=1
    break
  fi
  echo "ssh-session attempt ${attempt} failed with unexpected status ${status}" >&2
  printf '%s\n' "${body}" >&2
  exit 1
done
if [[ "${ssh_429_seen}" -ne 1 ]]; then
  echo "ssh-session rate limit did not trigger within 7 attempts" >&2
  exit 1
fi
echo "ssh-session rate limit triggered"

factory_reset_429_seen=0
for attempt in 1 2 3 4; do
  response="$(http_status_body \
    -X POST \
    -H "Authorization: Bearer ${participant_token}" \
    "${EDGE_BASE_URL}/api/v2/services/${RATE_LIMIT_DRAFT_CHALLENGE_ID}/reset/factory")"
  status="$(printf '%s\n' "${response}" | sed -n '1p')"
  body="$(printf '%s\n' "${response}" | tail -n +2)"
  if [[ "${status}" == "400" ]]; then
    continue
  fi
  if [[ "${status}" == "429" ]]; then
    factory_reset_429_seen=1
    break
  fi
  echo "factory reset attempt ${attempt} failed with unexpected status ${status}" >&2
  printf '%s\n' "${body}" >&2
  exit 1
done
if [[ "${factory_reset_429_seen}" -ne 1 ]]; then
  echo "factory reset rate limit did not trigger within 4 attempts" >&2
  exit 1
fi
echo "factory reset rate limit triggered"

restart_429_seen=0
for attempt in 1 2 3 4 5 6 7; do
  response="$(http_status_body \
    -X POST \
    -H "Authorization: Bearer ${participant_token}" \
    "${EDGE_BASE_URL}/api/v2/services/${RATE_LIMIT_DRAFT_CHALLENGE_ID}/reset/restart")"
  status="$(printf '%s\n' "${response}" | sed -n '1p')"
  body="$(printf '%s\n' "${response}" | tail -n +2)"
  if [[ "${status}" == "400" ]]; then
    continue
  fi
  if [[ "${status}" == "429" ]]; then
    restart_429_seen=1
    break
  fi
  echo "restart attempt ${attempt} failed with unexpected status ${status}" >&2
  printf '%s\n' "${body}" >&2
  exit 1
done
if [[ "${restart_429_seen}" -ne 1 ]]; then
  echo "restart rate limit did not trigger within 7 attempts" >&2
  exit 1
fi
echo "restart rate limit triggered"

echo "participant rate-limit smoke passed"
