#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"
load_default_env_files
load_env_file_override "${ROOT_DIR}/.runtime/backend-stack.env"

require_bin curl
require_bin jq

API_URL="${AD_PLATFORM_API_URL:-http://127.0.0.1:8080}"
ADMIN_TOKEN="${ADMIN_API_TOKEN:-dev-admin-token}"
EMAIL="${AD_PLATFORM_EMAIL:-alpha.captain@example.com}"
PASSWORD="${AD_PLATFORM_PASSWORD:-alpha-secret}"
TEAM_ID="${AD_PLATFORM_TEAM_ID:-}"
UNIQUE_SUFFIX="$(date +%s)"
CHALLENGE_NAME="${RUNTIME_SMOKE_CHALLENGE_NAME:-runtime-smoke-${UNIQUE_SUFFIX}}"
BASELINE_IMAGE="${RUNTIME_SMOKE_BASELINE_IMAGE:-registry.local/${CHALLENGE_NAME}:baseline}"
CHECKER_IMAGE="${RUNTIME_SMOKE_CHECKER_IMAGE:-registry.local/${CHALLENGE_NAME}-checker:latest}"

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

existing_challenges="$(curl_json "challenge list" "${API_URL}/api/v2/admin/challenges" -H "Authorization: Bearer ${ADMIN_TOKEN}")"

SERVICE_SUBNET_OCTET="${RUNTIME_SMOKE_SERVICE_SUBNET_OCTET:-}"
if [[ -z "${SERVICE_SUBNET_OCTET}" ]]; then
  used_subnets="$(printf '%s' "${existing_challenges}" | jq -r '.[].service_subnet_octet')"
  for candidate in $(seq 50 254); do
    if ! printf '%s\n' "${used_subnets}" | grep -qx "${candidate}"; then
      SERVICE_SUBNET_OCTET="${candidate}"
      break
    fi
  done
fi
if [[ -z "${SERVICE_SUBNET_OCTET}" ]]; then
  echo "could not allocate a free service_subnet_octet for runtime smoke" >&2
  exit 1
fi

SERVICE_PORT="${RUNTIME_SMOKE_SERVICE_PORT:-$((30000 + SERVICE_SUBNET_OCTET))}"

echo "admin runtime smoke: api=${API_URL} challenge=${CHALLENGE_NAME} team_id=${TEAM_ID:-auto}"

create_payload="$(
  jq -nc \
    --arg name "${CHALLENGE_NAME}" \
    --arg baseline_image "${BASELINE_IMAGE}" \
    --arg checker_image "${CHECKER_IMAGE}" \
    --argjson weight 1 \
    --argjson service_port "${SERVICE_PORT}" \
    --argjson service_subnet_octet "${SERVICE_SUBNET_OCTET}" \
    '{name:$name,baseline_image:$baseline_image,checker_image:$checker_image,weight:$weight,service_port:$service_port,service_subnet_octet:$service_subnet_octet}'
)"

create_response="$(curl_json "challenge create" -X POST "${API_URL}/api/v2/admin/challenges" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "${create_payload}")"
challenge_id="$(printf '%s' "${create_response}" | jq -er '.id')"

echo "created challenge:"
printf '%s\n' "${create_response}" | jq -c '{id,name,service_port,service_subnet_octet}'

echo "validation:"
curl_json "challenge validate" -X POST "${API_URL}/api/v2/admin/challenges/${challenge_id}/validate" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" |
  jq -c '{challenge_id,status,baseline_ssh_contract_ok,checker_contract_ok,message}'

echo "deploy:"
curl_json "challenge deploy" -X POST "${API_URL}/api/v2/admin/challenges/${challenge_id}/deploy" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" |
  jq -c '{job_id,challenge_id,status,queued_team_count,ready_team_count}'

echo "reconcile:"
curl_json "deployment reconcile" -X POST "${API_URL}/api/v2/admin/deployments/reconcile" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" |
  jq -c '{processed_jobs,processed_instances,completed_jobs}'

participant_token="$(
  curl_json "participant authenticate" -X POST "${API_URL}/api/v2/authenticate" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg email "${EMAIL}" --arg password "${PASSWORD}" '{email:$email,password:$password}')" |
    jq -er '.token'
)"

echo "team service state:"
team_service_json="$(
  curl_json "team services read" "${API_URL}/api/v2/team/services" \
    -H "Authorization: Bearer ${participant_token}" |
    jq -ec --argjson challenge_id "${challenge_id}" 'map(select(.challenge_id == $challenge_id)) | first'
)"
printf '%s\n' "${team_service_json}" | jq -c '{challenge_id,name,endpoint,status,checker}'

actual_endpoint="$(printf '%s' "${team_service_json}" | jq -er '.endpoint')"
expected_endpoint_regex="^10\\.80\\.${SERVICE_SUBNET_OCTET}\\.[0-9]+:${SERVICE_PORT}$"
if [[ ! "${actual_endpoint}" =~ ${expected_endpoint_regex} ]]; then
  echo "unexpected endpoint shape: got ${actual_endpoint}, want 10.80.${SERVICE_SUBNET_OCTET}.<team-octet>:${SERVICE_PORT}" >&2
  exit 1
fi
if [[ -n "${TEAM_ID}" ]]; then
  expected_octet="$((TEAM_ID - 90))"
  expected_endpoint="10.80.${SERVICE_SUBNET_OCTET}.${expected_octet}:${SERVICE_PORT}"
  if [[ "${actual_endpoint}" != "${expected_endpoint}" ]]; then
    echo "unexpected endpoint: got ${actual_endpoint}, want ${expected_endpoint}" >&2
    exit 1
  fi
fi

echo "public target map:"
public_target_map="$(
  curl_json "public service map read" "${API_URL}/api/v2/services" \
    -H "Authorization: Bearer ${participant_token}"
)"
if [[ -n "${TEAM_ID}" ]]; then
  printf '%s\n' "${public_target_map}" |
    jq -c --arg challenge_id "${challenge_id}" --arg team_id "${TEAM_ID}" '.[$challenge_id][$team_id]'
else
  printf '%s\n' "${public_target_map}" |
    jq -c --arg challenge_id "${challenge_id}" --arg endpoint "${actual_endpoint}" '.[$challenge_id] | to_entries[] | select(.value[]? == $endpoint) | .value'
fi
if ! printf '%s\n' "${public_target_map}" | jq -e --arg challenge_id "${challenge_id}" --arg endpoint "${actual_endpoint}" '.[$challenge_id] | to_entries | any(.value[]? == $endpoint)' >/dev/null; then
  echo "public service map does not include authenticated team endpoint ${actual_endpoint}" >&2
  exit 1
fi

echo "runtime smoke passed:"
printf '  challenge_id=%s endpoint=%s\n' "${challenge_id}" "${actual_endpoint}"
