#!/usr/bin/env bash
set -euo pipefail

# This script configures deploy/compose/prod.env with a custom domain/IP and port settings.
# It automatically aligns EDGE_SITE_ADDRESS, AD_PLATFORM_PUBLIC_BASE_URL, and WIREGUARD_SERVER_ENDPOINT.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROD_ENV="${ROOT_DIR}/deploy/compose/prod.env"

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

# Construct values
edge_site_address="${SCHEME}://${DOMAIN_OR_IP}"
ad_platform_public_url="${SCHEME}://${DOMAIN_OR_IP}"
wireguard_endpoint="${DOMAIN_OR_IP}:${WG_PORT}"

echo "Updating environment configurations in prod.env..."
echo "  EDGE_SITE_ADDRESS           -> ${edge_site_address}"
echo "  AD_PLATFORM_PUBLIC_BASE_URL -> ${ad_platform_public_url}"
echo "  WIREGUARD_SERVER_ENDPOINT   -> ${wireguard_endpoint}"

tmp_file="$(mktemp "${PROD_ENV}.tmp.XXXXXX")"
cleanup() {
  rm -f "${tmp_file}"
}
trap cleanup EXIT

# Read and replace keys in-place
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
    *)
      printf '%s\n' "${line}" >>"${tmp_file}"
      continue
      ;;
  esac
  printf '%s=%s\n' "${key}" "${value}" >>"${tmp_file}"
done <"${PROD_ENV}"

mv "${tmp_file}" "${PROD_ENV}"
chmod 600 "${PROD_ENV}"
trap - EXIT

echo "Successfully updated prod.env!"
