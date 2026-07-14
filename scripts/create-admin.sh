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

# Prefer the public edge URL for host-side calls. prod-host does not publish
# api-gateway:8080 on the host — only Caddy (EDGE_*) is reachable.
PUBLIC_URL="${AD_PLATFORM_PUBLIC_BASE_URL:-${EDGE_SITE_ADDRESS:-}}"
API_URL="${AD_PLATFORM_API_URL:-}"
if [[ -z "${API_URL}" || "${API_URL}" == *"api-gateway"* || "${API_URL}" == *"localhost:8080"* || "${API_URL}" == *"127.0.0.1:8080"* ]]; then
  if [[ -n "${PUBLIC_URL}" ]]; then
    API_URL="${PUBLIC_URL}"
  else
    API_URL="http://localhost:8080"
  fi
fi
# Strip trailing slash so paths join cleanly.
API_URL="${API_URL%/}"

CURL_TLS_ARGS=()
if [[ "${API_URL}" == https://* ]]; then
  custom_admin_ca_cert="${ADMIN_CA_CERT:-}"
  default_admin_ca_cert="${ROOT_DIR}/deploy/caddy/certs/adplatform-selfsigned.crt"
  if [[ "${ADMIN_CURL_INSECURE:-false}" == "true" ]]; then
    CURL_TLS_ARGS=(-k)
    echo "warning: ADMIN_CURL_INSECURE=true; skipping TLS certificate verification for ${API_URL}" >&2
  elif [[ -n "${custom_admin_ca_cert}" && -f "${custom_admin_ca_cert}" ]]; then
    CURL_TLS_ARGS=(--cacert "${custom_admin_ca_cert}")
    echo "using TLS CA certificate: ${custom_admin_ca_cert}"
  elif [[ "${EDGE_TLS_DIRECTIVE:-}" == *"adplatform-selfsigned.crt"* && -f "${default_admin_ca_cert}" ]]; then
    CURL_TLS_ARGS=(--cacert "${default_admin_ca_cert}")
    echo "using TLS CA certificate: ${default_admin_ca_cert}"
  fi
fi

ADMIN_TOKEN="$(resolve_admin_api_token "${ROOT_DIR}/.runtime/backend-stack.env")"

# User information from environment or positional arguments.
# Empty positional args (e.g. from make) fall through to env/prod.env.
arg_or_default() {
  local arg="${1-}"
  local fallback="${2-}"
  if [[ -n "${arg}" ]]; then
    printf '%s' "${arg}"
  else
    printf '%s' "${fallback}"
  fi
}

DISPLAY_NAME="$(arg_or_default "${1-}" "${ADMIN_DISPLAY_NAME:-Organizer}")"
EMAIL="$(arg_or_default "${2-}" "${ADMIN_EMAIL:-admin@example.com}")"
PASSWORD="$(arg_or_default "${3-}" "${ADMIN_PASSWORD:-}")"
WIREGUARD_OUTPUT_DIR="$(arg_or_default "${4-}" "${ADMIN_WIREGUARD_OUTPUT_DIR:-${ROOT_DIR}/.runtime/admin-wireguard}")"
AUTO_RECONCILE="${ADMIN_AUTO_RECONCILE:-true}"

if [[ -z "${PASSWORD}" ]]; then
  echo "Error: ADMIN_PASSWORD is required." >&2
  echo "Set it in ${PROD_ENV} (make generate-prod-env) or pass: $0 [display_name] [email] [password]" >&2
  exit 1
fi

echo "creating organizer account: ${EMAIL} (${DISPLAY_NAME})"
echo "using API: ${API_URL}"

curl_json() {
  local label="$1"
  shift

  local response_file
  response_file="$(mktemp)"

  local status
  status="$(curl "${CURL_TLS_ARGS[@]}" -sS -o "${response_file}" -w '%{http_code}' "$@")"
  if [[ "${status}" -lt 200 || "${status}" -ge 300 ]]; then
    echo "${label} failed (status=${status}):" >&2
    cat "${response_file}" >&2
    rm -f "${response_file}"
    exit 1
  fi

  cat "${response_file}"
  rm -f "${response_file}"
}

RESPONSE="$(
  curl_json "create organizer" -X POST "${API_URL}/api/v2/admin/players" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{
      \"team_id\": 0,
      \"display_name\": \"${DISPLAY_NAME}\",
      \"email\": \"${EMAIL}\",
      \"password\": \"${PASSWORD}\",
      \"role\": \"organizer\"
    }"
)"

PLAYER_ID="$(echo "${RESPONSE}" | jq -r '.id // empty')"
if [[ -z "${PLAYER_ID}" ]]; then
  echo "failed to parse organizer player id from response" >&2
  echo "response: ${RESPONSE}" >&2
  exit 1
fi

echo "fetching WireGuard config for organizer player ${PLAYER_ID}..."
WIREGUARD_RESPONSE="$(
  curl_json "fetch organizer wireguard" -X GET "${API_URL}/api/v2/admin/players/${PLAYER_ID}/wireguard" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"

mkdir -p "${WIREGUARD_OUTPUT_DIR}"
WIREGUARD_FILE_NAME="$(echo "${WIREGUARD_RESPONSE}" | jq -r '.download_name // empty')"
if [[ -z "${WIREGUARD_FILE_NAME}" ]]; then
  WIREGUARD_FILE_NAME="organizer-player-${PLAYER_ID}.conf"
fi
WIREGUARD_CONFIG_PATH="${WIREGUARD_OUTPUT_DIR}/${WIREGUARD_FILE_NAME}"
echo "${WIREGUARD_RESPONSE}" | jq -r '.config' > "${WIREGUARD_CONFIG_PATH}"

if [[ "${AUTO_RECONCILE}" == "true" ]]; then
  echo "reconciling wireguard/access runtime state..."
  if ! curl "${CURL_TLS_ARGS[@]}" -s -X POST "${API_URL}/api/v2/admin/wireguard/reconcile" -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null; then
    echo "warning: wireguard reconcile request failed; run it manually from organizer dashboard or API." >&2
  fi
  if ! curl "${CURL_TLS_ARGS[@]}" -s -X POST "${API_URL}/api/v2/admin/access/reconcile" -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null; then
    echo "warning: access reconcile request failed; run it manually from organizer dashboard or API." >&2
  fi
fi

echo "organizer account created successfully."
echo "login at: ${AD_PLATFORM_PUBLIC_BASE_URL:-https://localhost}/login"
echo "wireguard config saved: ${WIREGUARD_CONFIG_PATH}"
