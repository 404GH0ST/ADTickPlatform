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
EMAIL="${AD_PLATFORM_EMAIL:-alpha.captain@example.com}"
PASSWORD="${AD_PLATFORM_PASSWORD:-alpha-secret}"
TEAM_ID="${AD_PLATFORM_TEAM_ID:-101}"
REQUESTED_CHALLENGE_ID="${SMOKE_CHALLENGE_ID:-}"

auth_payload="$(jq -nc --arg email "${EMAIL}" --arg password "${PASSWORD}" '{email:$email,password:$password}')"
token="$(
  curl -fsS -X POST "${API_URL}/api/v2/authenticate" \
    -H 'Content-Type: application/json' \
    -d "${auth_payload}" |
    jq -er '.token'
)"

team_services_json="$(
  curl -fsS "${API_URL}/api/v2/team/services" \
    -H "Authorization: Bearer ${token}"
)"

CHALLENGE_ID="${REQUESTED_CHALLENGE_ID}"
if [[ -z "${CHALLENGE_ID}" ]]; then
  CHALLENGE_ID="$(
    printf '%s\n' "${team_services_json}" |
      jq -er 'first | .challenge_id'
  )"
fi

echo "participant smoke: api=${API_URL} team_id=${TEAM_ID} challenge_id=${CHALLENGE_ID}"

echo "authenticate:"
printf '  token: %s\n' "${token:0:16}..."

echo "challenges:"
curl -fsS "${API_URL}/api/v2/challenges" | jq -c 'map({id,name})'

echo "scoreboard:"
curl -fsS "${API_URL}/api/v2/scoreboard" | jq -c 'map({rank,team,total})'

echo "team services before:"
printf '%s\n' "${team_services_json}" |
  jq -c --argjson challenge_id "${CHALLENGE_ID}" 'map(select(.challenge_id == $challenge_id))'

echo "ssh session:"
curl -fsS -X POST "${API_URL}/api/v2/services/${CHALLENGE_ID}/ssh-session" \
  -H "Authorization: Bearer ${token}" |
  jq -c '{host,port,username,password_present:(.password | length > 0)}'

echo "factory reset:"
curl -fsS -X POST "${API_URL}/api/v2/services/${CHALLENGE_ID}/reset/factory" \
  -H "Authorization: Bearer ${token}" |
  jq -c '{challenge_id,action,unlock_preserved}'

echo "team services after:"
curl -fsS "${API_URL}/api/v2/team/services" \
  -H "Authorization: Bearer ${token}" |
  jq -c --argjson challenge_id "${CHALLENGE_ID}" 'map(select(.challenge_id == $challenge_id) | {challenge_id,unlocked,status,checker,ssh_hint,last_event,reset_cooldown})'
