#!/bin/bash
set -euo pipefail

# This script creates an organizer/admin player account via the Admin API.
# It is intended for bootstrapping production environments or local development.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${ROOT_DIR}/scripts/lib/common.sh"
require_bin curl
require_bin jq

# Configuration from environment with defaults
load_default_env_files
PROD_ENV="${PROD_ENV:-${ROOT_DIR}/deploy/compose/prod.env}"
load_env_file_override "${PROD_ENV}"

API_URL="${AD_PLATFORM_API_URL:-http://localhost:8080}"
PUBLIC_URL="${AD_PLATFORM_PUBLIC_BASE_URL:-}"

if [[ "${API_URL}" == *"api-gateway"* ]] && [[ -n "${PUBLIC_URL}" ]]; then
  API_URL="${PUBLIC_URL}"
fi

ADMIN_TOKEN="${ADMIN_API_TOKEN:-dev-admin-token}"

# User information from environment or positional arguments
DISPLAY_NAME="${1:-${ADMIN_DISPLAY_NAME:-"System Admin"}}"
EMAIL="${2:-${ADMIN_EMAIL:-"admin@example.com"}}"
PASSWORD="${3:-${ADMIN_PASSWORD:-""}}"
WIREGUARD_OUTPUT_DIR="${4:-${ADMIN_WIREGUARD_OUTPUT_DIR:-"${ROOT_DIR}/.runtime/admin-wireguard"}}"
AUTO_RECONCILE="${ADMIN_AUTO_RECONCILE:-true}"

if [[ -z "${PASSWORD}" ]]; then
  echo "Error: ADMIN_PASSWORD is required." >&2
  echo "Usage: $0 [display_name] [email] [password] [wireguard_output_dir]" >&2
  exit 1
fi

echo "creating organizer account: ${EMAIL} (${DISPLAY_NAME})"

RESPONSE=$(curl -s -X POST "${API_URL}/api/v2/admin/players" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"team_id\": 0,
    \"display_name\": \"${DISPLAY_NAME}\",
    \"email\": \"${EMAIL}\",
    \"password\": \"${PASSWORD}\",
    \"role\": \"organizer\"
  }")

STATUS=$(echo "${RESPONSE}" | jq -r '.status')

if [[ "${STATUS}" != "success" ]]; then
  MESSAGE=$(echo "${RESPONSE}" | jq -r '.message // "unknown error"')
  echo "failed to create organizer: ${MESSAGE}" >&2
  echo "response: ${RESPONSE}" >&2
  exit 1
fi

PLAYER_ID="$(echo "${RESPONSE}" | jq -r '.data.id // empty')"
if [[ -z "${PLAYER_ID}" ]]; then
  echo "failed to parse organizer player id from response" >&2
  echo "response: ${RESPONSE}" >&2
  exit 1
fi

echo "fetching WireGuard config for organizer player ${PLAYER_ID}..."
WIREGUARD_RESPONSE="$(curl -s -X GET "${API_URL}/api/v2/admin/players/${PLAYER_ID}/wireguard" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}")"

WIREGUARD_STATUS="$(echo "${WIREGUARD_RESPONSE}" | jq -r '.status')"
if [[ "${WIREGUARD_STATUS}" != "success" ]]; then
  MESSAGE="$(echo "${WIREGUARD_RESPONSE}" | jq -r '.message // "unknown error"')"
  echo "failed to fetch organizer WireGuard config: ${MESSAGE}" >&2
  echo "response: ${WIREGUARD_RESPONSE}" >&2
  exit 1
fi

mkdir -p "${WIREGUARD_OUTPUT_DIR}"
WIREGUARD_FILE_NAME="$(echo "${WIREGUARD_RESPONSE}" | jq -r '.data.download_name // empty')"
if [[ -z "${WIREGUARD_FILE_NAME}" ]]; then
  WIREGUARD_FILE_NAME="organizer-player-${PLAYER_ID}.conf"
fi
WIREGUARD_CONFIG_PATH="${WIREGUARD_OUTPUT_DIR}/${WIREGUARD_FILE_NAME}"
echo "${WIREGUARD_RESPONSE}" | jq -r '.data.config' > "${WIREGUARD_CONFIG_PATH}"

if [[ "${AUTO_RECONCILE}" == "true" ]]; then
  echo "reconciling wireguard/access runtime state..."
  if ! curl -s -X POST "${API_URL}/api/v2/admin/wireguard/reconcile" -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null; then
    echo "warning: wireguard reconcile request failed; run it manually from organizer dashboard or API." >&2
  fi
  if ! curl -s -X POST "${API_URL}/api/v2/admin/access/reconcile" -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null; then
    echo "warning: access reconcile request failed; run it manually from organizer dashboard or API." >&2
  fi
fi

echo "organizer account created successfully."
echo "login at: ${AD_PLATFORM_PUBLIC_BASE_URL:-http://localhost}/login"
echo "wireguard config saved: ${WIREGUARD_CONFIG_PATH}"
