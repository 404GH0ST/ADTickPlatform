#!/usr/bin/env bash
# Teardown platform access + WireGuard rules via the admin API when reachable.
# Used by: make firewall-cleanup / make down-prod-host-clean
#
# prod-host does not publish api-gateway:8080 on the host — only the edge
# (Caddy / AD_PLATFORM_PUBLIC_BASE_URL / EDGE_SITE_ADDRESS) is reachable.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/common.sh
source "${ROOT_DIR}/scripts/lib/common.sh"

load_default_env_files
PROD_ENV="${PROD_ENV:-${ROOT_DIR}/deploy/compose/prod.env}"
if [[ -f "${PROD_ENV}" ]]; then
  load_env_file_override "${PROD_ENV}"
fi

ADMIN_TOKEN="$(resolve_admin_api_token "${ROOT_DIR}/.runtime/backend-stack.env")"
PUBLIC_URL="${AD_PLATFORM_PUBLIC_BASE_URL:-${EDGE_SITE_ADDRESS:-}}"
API_URL="${AD_PLATFORM_API_URL:-}"

if [[ -z "${API_URL}" || "${API_URL}" == *"api-gateway"* || "${API_URL}" == *"localhost:8080"* || "${API_URL}" == *"127.0.0.1:8080"* ]]; then
  if [[ -n "${PUBLIC_URL}" ]]; then
    API_URL="${PUBLIC_URL}"
  else
    API_URL="http://localhost:8080"
  fi
fi
API_URL="${API_URL%/}"

CURL_TLS_ARGS=()
if [[ "${API_URL}" == https://* ]]; then
  default_admin_ca_cert="${ROOT_DIR}/deploy/caddy/certs/adplatform-selfsigned.crt"
  if [[ "${ADMIN_CURL_INSECURE:-false}" == "true" ]]; then
    CURL_TLS_ARGS=(-k)
  elif [[ -n "${ADMIN_CA_CERT:-}" && -f "${ADMIN_CA_CERT}" ]]; then
    CURL_TLS_ARGS=(--cacert "${ADMIN_CA_CERT}")
  elif [[ "${EDGE_TLS_DIRECTIVE:-}" == *"adplatform-selfsigned.crt"* && -f "${default_admin_ca_cert}" ]]; then
    CURL_TLS_ARGS=(--cacert "${default_admin_ca_cert}")
  fi
fi

echo "cleaning up all platform firewall rules (API: ${API_URL})..."

if ! curl "${CURL_TLS_ARGS[@]}" -fsS --connect-timeout 2 --max-time 5 "${API_URL}/readyz" >/dev/null 2>&1 \
  && ! curl "${CURL_TLS_ARGS[@]}" -fsS --connect-timeout 2 --max-time 5 "${API_URL}/healthz" >/dev/null 2>&1; then
  echo "API not reachable at ${API_URL} (stack already down or wrong URL); skipping API access/wireguard teardown."
  echo "hint: while the stack is up, set AD_PLATFORM_PUBLIC_BASE_URL / EDGE_SITE_ADDRESS to the edge URL."
  echo "cleanup requested."
  exit 0
fi

if curl "${CURL_TLS_ARGS[@]}" -fsS -X POST \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  "${API_URL}/api/v2/admin/access/teardown" >/dev/null; then
  echo "access teardown ok."
else
  echo "access teardown failed (continuing)."
fi

if curl "${CURL_TLS_ARGS[@]}" -fsS -X POST \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  "${API_URL}/api/v2/admin/wireguard/teardown" >/dev/null; then
  echo "wireguard teardown ok."
else
  echo "wireguard teardown failed (continuing)."
fi

echo "cleanup requested."
