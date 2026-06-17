#!/usr/bin/env bash
set -euo pipefail

# This script configures deploy/compose/prod.env with a custom domain/IP and port settings.
# It automatically aligns EDGE_SITE_ADDRESS, AD_PLATFORM_PUBLIC_BASE_URL, and WIREGUARD_SERVER_ENDPOINT.
# When scheme is https, it also generates a local self-signed certificate for the edge Caddy proxy.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROD_ENV="${PROD_ENV:-${ROOT_DIR}/deploy/compose/prod.env}"
CADDY_CERT_DIR="${CADDY_CERT_DIR:-${ROOT_DIR}/deploy/caddy/certs}"
CADDY_CERT_FILE="${CADDY_CERT_DIR}/adplatform-selfsigned.crt"
CADDY_KEY_FILE="${CADDY_CERT_DIR}/adplatform-selfsigned.key"
EDGE_CERT_FILE="/etc/caddy/certs/adplatform-selfsigned.crt"
EDGE_KEY_FILE="/etc/caddy/certs/adplatform-selfsigned.key"

# Ensure generate-prod-env.sh has been run to initialize secrets
if [[ ! -f "${PROD_ENV}" ]]; then
  echo "Initializing prod.env first..."
  "${ROOT_DIR}/scripts/generate-prod-env.sh" "${PROD_ENV}"
fi

# Usage helper
show_usage() {
  echo "Usage: $0 <domain_or_ip> [scheme] [wireguard_port]" >&2
  echo "  Example: $0 ctf.example.com https 51820" >&2
  echo "  Example: $0 1.2.3.4 http 51820" >&2
  exit 1
}

# If no arguments provided, prompt interactively
if [[ $# -eq 0 ]]; then
  read -rp "Enter your public domain name or IP address (e.g. ctf.example.com or 1.2.3.4): " DOMAIN_OR_IP
  if [[ -z "${DOMAIN_OR_IP}" ]]; then
    echo "Error: Domain name or IP address is required." >&2
    exit 1
  fi
  read -rp "Enter scheme (http/https) [default: http]: " SCHEME
  SCHEME="${SCHEME:-http}"
  read -rp "Enter WireGuard public port [default: 51820]: " WG_PORT
  WG_PORT="${WG_PORT:-51820}"
else
  DOMAIN_OR_IP="$1"
  SCHEME="${2:-http}"
  WG_PORT="${3:-51820}"
fi

# Normalize scheme
SCHEME="$(echo "${SCHEME}" | tr '[:upper:]' '[:lower:]')"
if [[ "${SCHEME}" != "http" && "${SCHEME}" != "https" ]]; then
  echo "Error: Scheme must be 'http' or 'https'." >&2
  exit 1
fi

is_ip_address() {
  local value="$1"
  [[ "${value}" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ || "${value}" =~ ^[0-9A-Fa-f:]+$ ]]
}

generate_self_signed_cert() {
  local host="$1"
  local san

  if ! command -v openssl >/dev/null 2>&1; then
    echo "Error: openssl is required to generate a self-signed HTTPS certificate." >&2
    exit 1
  fi

  mkdir -p "${CADDY_CERT_DIR}"
  chmod 700 "${CADDY_CERT_DIR}"

  if is_ip_address "${host}"; then
    san="IP:${host}"
  else
    san="DNS:${host}"
  fi

  openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes \
    -keyout "${CADDY_KEY_FILE}" \
    -out "${CADDY_CERT_FILE}" \
    -subj "/CN=${host}" \
    -addext "subjectAltName=${san}" >/dev/null 2>&1

  chmod 600 "${CADDY_KEY_FILE}"
  chmod 644 "${CADDY_CERT_FILE}"
}

# Construct values
edge_site_address="${SCHEME}://${DOMAIN_OR_IP}"
ad_platform_public_url="${SCHEME}://${DOMAIN_OR_IP}"
wireguard_endpoint="${DOMAIN_OR_IP}:${WG_PORT}"
edge_tls_directive=""

if [[ "${SCHEME}" == "https" ]]; then
  generate_self_signed_cert "${DOMAIN_OR_IP}"
  edge_tls_directive="tls ${EDGE_CERT_FILE} ${EDGE_KEY_FILE}"
fi

echo "Updating environment configurations in prod.env..."
echo "  EDGE_SITE_ADDRESS           -> ${edge_site_address}"
echo "  AD_PLATFORM_PUBLIC_BASE_URL -> ${ad_platform_public_url}"
echo "  WIREGUARD_SERVER_ENDPOINT   -> ${wireguard_endpoint}"
if [[ "${SCHEME}" == "https" ]]; then
  echo "  EDGE_TLS_DIRECTIVE          -> ${edge_tls_directive}"
  echo "  Self-signed cert            -> ${CADDY_CERT_FILE}"
else
  echo "  EDGE_TLS_DIRECTIVE          -> disabled"
fi

tmp_file="$(mktemp "${PROD_ENV}.tmp.XXXXXX")"
cleanup() {
  rm -f "${tmp_file}"
}
trap cleanup EXIT

# Read and replace keys in-place
seen_edge_tls_directive=0
while IFS= read -r line || [[ -n "${line}" ]]; do
  if [[ "${line}" != *=* || "${line}" =~ ^[[:space:]]*# ]]; then
    printf '%s\n' "${line}" >>"${tmp_file}"
    continue
  fi

  key="${line%%=*}"
  case "${key}" in
    EDGE_SITE_ADDRESS) value="${edge_site_address}" ;;
    AD_PLATFORM_PUBLIC_BASE_URL) value="${ad_platform_public_url}" ;;
    WIREGUARD_SERVER_ENDPOINT) value="${wireguard_endpoint}" ;;
    EDGE_TLS_DIRECTIVE)
      if [[ "${seen_edge_tls_directive}" -eq 1 ]]; then
        continue
      fi
      value="${edge_tls_directive}"
      seen_edge_tls_directive=1
      ;;
    *)
      printf '%s\n' "${line}" >>"${tmp_file}"
      continue
      ;;
  esac
  printf '%s=%s\n' "${key}" "${value}" >>"${tmp_file}"
done <"${PROD_ENV}"

if [[ "${seen_edge_tls_directive}" -eq 0 ]]; then
  printf 'EDGE_TLS_DIRECTIVE=%s\n' "${edge_tls_directive}" >>"${tmp_file}"
fi

mv "${tmp_file}" "${PROD_ENV}"
chmod 600 "${PROD_ENV}"
trap - EXIT

echo "Successfully updated prod.env!"
