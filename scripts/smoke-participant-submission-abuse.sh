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
TEMP_EMAIL="security.submit.${TEMP_STAMP}@teams.local"
TEMP_PASSWORD="security-submit-secret"
ORGANIZER_EMAIL="security.submit.organizer.${TEMP_STAMP}@teams.local"
ORGANIZER_PASSWORD="security-submit-organizer-secret"
TEMP_PLAYER_ID=""
TEMP_ORGANIZER_ID=""

if [[ -z "${ADMIN_TOKEN}" ]]; then
  echo "ADMIN_API_TOKEN must be set in ${PROD_ENV}." >&2
  exit 1
fi

cleanup() {
  if [[ -n "${TEMP_PLAYER_ID}" ]]; then
    curl -fsS -X DELETE -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      "${EDGE_BASE_URL}/api/v2/admin/players/${TEMP_PLAYER_ID}" >/dev/null || true
  fi
  if [[ -n "${TEMP_ORGANIZER_ID}" ]]; then
    curl -fsS -X DELETE -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      "${EDGE_BASE_URL}/api/v2/admin/players/${TEMP_ORGANIZER_ID}" >/dev/null || true
  fi
}
trap cleanup EXIT

wait_for_http "${EDGE_BASE_URL}/healthz" 60 "edge healthz"

http_status_body() {
  local body_file status
  body_file="$(mktemp)"
  status="$(curl -sS -o "${body_file}" -w '%{http_code}' "$@")"
  printf '%s\n' "${status}"
  cat "${body_file}"
  rm -f "${body_file}"
}

expect_problem() {
  local label="$1" expected_status="$2" expected_detail_substr="$3"
  shift 3

  local response status body detail
  response="$(http_status_body "$@")"
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

echo "participant submission abuse smoke: base_url=${EDGE_BASE_URL}"

create_player_body="$(
  jq -nc \
    --argjson team_id "${SECURITY_TEAM_ID}" \
    --arg display_name "Security Submit Probe" \
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

create_organizer_body="$(
  jq -nc \
    --argjson team_id "${SECURITY_TEAM_ID}" \
    --arg display_name "Security Submit Organizer" \
    --arg email "${ORGANIZER_EMAIL}" \
    --arg password "${ORGANIZER_PASSWORD}" \
    --arg role "organizer" \
    '{team_id:$team_id,display_name:$display_name,email:$email,password:$password,role:$role}'
)"
create_organizer_response="$(
  curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "${create_organizer_body}" \
    "${EDGE_BASE_URL}/api/v2/admin/players"
)"
TEMP_ORGANIZER_ID="$(printf '%s' "${create_organizer_response}" | jq -er '.id')"
echo "created temporary organizer id=${TEMP_ORGANIZER_ID}"

organizer_token="$(
  curl -fsS -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg email "${ORGANIZER_EMAIL}" --arg password "${ORGANIZER_PASSWORD}" '{email:$email,password:$password}')" \
    "${EDGE_BASE_URL}/api/v2/authenticate" |
    jq -er '.token'
)"
printf 'organizer token prefix=%s...\n' "${organizer_token:0:16}"

expect_problem \
  "admin token on submit" \
  "403" \
  "please authenticate before submit." \
  -X POST \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"flags":["FLAGv1.demo"]}' \
  "${EDGE_BASE_URL}/api/v2/submit"
echo "admin token denied on submit"

expect_problem \
  "organizer token on submit" \
  "403" \
  "please authenticate before submit." \
  -X POST \
  -H "Authorization: Bearer ${organizer_token}" \
  -H 'Content-Type: application/json' \
  -d '{"flags":["FLAGv1.demo"]}' \
  "${EDGE_BASE_URL}/api/v2/submit"
echo "organizer token denied on submit"

oversized_flags="$(
  jq -nc --argjson max "${MAX_SUBMIT_FLAGS_PER_REQUEST:-128}" '
    [range(0; ($max + 1)) | "FLAGv1.oversized.\(.)"]
  '
)"
expect_problem \
  "oversized submission batch" \
  "400" \
  "too many flags in one request." \
  -X POST \
  -H "Authorization: Bearer ${participant_token}" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --argjson flags "${oversized_flags}" '{flags:$flags}')" \
  "${EDGE_BASE_URL}/api/v2/submit"
echo "oversized submission batch denied"

submit_status_dir="$(mktemp -d)"
for attempt in $(seq 1 48); do
  {
    body_file="${submit_status_dir}/body.${attempt}"
    status_file="${submit_status_dir}/status.${attempt}"
    status="$(curl -sS -o "${body_file}" -w '%{http_code}' \
      -X POST \
      -H "Authorization: Bearer ${participant_token}" \
      -H 'Content-Type: application/json' \
      -d '{"flags":["FLAGv1.invalid"]}' \
      "${EDGE_BASE_URL}/api/v2/submit" || true)"
    printf '%s' "${status:-000}" > "${status_file}"
  } &
  if (( attempt % 12 == 0 )); then
    wait
  fi
done
wait

submit_429_seen=0
for attempt in $(seq 1 48); do
  status="$(cat "${submit_status_dir}/status.${attempt}")"
  body="$(cat "${submit_status_dir}/body.${attempt}")"
  if [[ "${status}" == "200" ]]; then
    continue
  fi
  if [[ "${status}" == "400" ]]; then
    detail="$(printf '%s' "${body}" | jq -r '.detail // empty')"
    if [[ "${detail}" == "contest has not started yet." || "${detail}" == "contest is over." ]]; then
      continue
    fi
  fi
  if [[ "${status}" == "429" ]]; then
    submit_429_seen=1
    detail="$(printf '%s' "${body}" | jq -r '.detail // empty')"
    if [[ "${detail}" != *"calm down"* ]]; then
      echo "unexpected submit rate-limit detail: ${detail}" >&2
      rm -rf "${submit_status_dir}"
      exit 1
    fi
    continue
  fi
  echo "submit attempt ${attempt} failed with unexpected status ${status}" >&2
  printf '%s\n' "${body}" >&2
  rm -rf "${submit_status_dir}"
  exit 1
done
rm -rf "${submit_status_dir}"
if [[ "${submit_429_seen}" -ne 1 ]]; then
  echo "submit rate limit did not trigger during the burst" >&2
  exit 1
fi
echo "submit rate limit triggered"

curl -fsS -X DELETE -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  "${EDGE_BASE_URL}/api/v2/admin/players/${TEMP_PLAYER_ID}" >/dev/null
TEMP_PLAYER_ID=""

expect_problem \
  "deleted participant token on submit" \
  "403" \
  "please authenticate before submit." \
  -X POST \
  -H "Authorization: Bearer ${participant_token}" \
  -H 'Content-Type: application/json' \
  -d '{"flags":["FLAGv1.invalid"]}' \
  "${EDGE_BASE_URL}/api/v2/submit"
echo "deleted participant token denied on submit"

echo "participant submission abuse smoke passed"
