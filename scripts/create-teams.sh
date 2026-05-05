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

TEAM_COUNT="${1:-${TEAM_COUNT:-}}"
TEAM_PREFIX="${2:-${TEAM_PREFIX:-Team}}"
TEAM_EMAIL_DOMAIN="${3:-${TEAM_EMAIL_DOMAIN:-teams.local}}"
TEAM_START_INDEX="${4:-${TEAM_START_INDEX:-1}}"

usage() {
  cat <<'EOF'
Usage: scripts/create-teams.sh <count> [team_prefix] [email_domain] [start_index]

Examples:
  scripts/create-teams.sh 10
  scripts/create-teams.sh 8 "College Team" college.local
  scripts/create-teams.sh 16 Team teams.local 101

Environment overrides:
  TEAM_COUNT
  TEAM_PREFIX
  TEAM_EMAIL_DOMAIN
  TEAM_START_INDEX
  AD_PLATFORM_API_URL
  AD_PLATFORM_PUBLIC_BASE_URL
  ADMIN_API_TOKEN
EOF
}

if [[ -z "${TEAM_COUNT}" ]]; then
  echo "Error: team count is required." >&2
  usage >&2
  exit 1
fi

if ! [[ "${TEAM_COUNT}" =~ ^[0-9]+$ ]] || (( TEAM_COUNT < 1 )); then
  echo "Error: team count must be a positive integer." >&2
  exit 1
fi

if ! [[ "${TEAM_START_INDEX}" =~ ^[0-9]+$ ]] || (( TEAM_START_INDEX < 1 )); then
  echo "Error: start index must be a positive integer." >&2
  exit 1
fi

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

prefix_slug="$(slug_name "${TEAM_PREFIX}")"
end_index=$((TEAM_START_INDEX + TEAM_COUNT - 1))
pad_width="${#end_index}"
(( pad_width < 2 )) && pad_width=2

echo "creating ${TEAM_COUNT} team(s) via ${API_URL}"

for ((i = 0; i < TEAM_COUNT; i++)); do
  team_index=$((TEAM_START_INDEX + i))
  team_suffix="$(printf "%0${pad_width}d" "${team_index}")"
  team_name="${TEAM_PREFIX} ${team_suffix}"
  contact_email="${prefix_slug}-${team_suffix}@${TEAM_EMAIL_DOMAIN}"

  response="$(
    curl_json "create team ${team_name}" \
      -X POST "${API_URL}/api/v2/admin/teams" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "$(jq -nc --arg name "${team_name}" --arg email "${contact_email}" '{name:$name,contact_email:$email}')"
  )"

  printf '%s\n' "${response}" | jq -c '{id,name,contact_email}'
done

echo "created ${TEAM_COUNT} team(s) successfully."
