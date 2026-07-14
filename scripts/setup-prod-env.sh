#!/usr/bin/env bash
set -euo pipefail

# This script configures deploy/compose/prod.env with a custom domain/IP and port settings.
# It automatically aligns EDGE_SITE_ADDRESS, AD_PLATFORM_PUBLIC_BASE_URL, and WIREGUARD_SERVER_ENDPOINT.
# Localhost and IP literals receive a local self-signed certificate. Public DNS
# names leave Caddy's TLS directive empty so Caddy can obtain a trusted ACME cert.

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
  echo "  Example: $0 1.2.3.4 https 51820" >&2
  exit 1
}

# If no arguments provided, prompt interactively
if [[ $# -eq 0 ]]; then
  read -rp "Enter your public domain name or IP address (e.g. ctf.example.com or 1.2.3.4): " DOMAIN_OR_IP
  if [[ -z "${DOMAIN_OR_IP}" ]]; then
    echo "Error: Domain name or IP address is required." >&2
    exit 1
  fi
  read -rp "Enter scheme [default: https]: " SCHEME
  SCHEME="${SCHEME:-https}"
  read -rp "Enter WireGuard public port [default: 51820]: " WG_PORT
  WG_PORT="${WG_PORT:-51820}"
else
  DOMAIN_OR_IP="$1"
  SCHEME="${2:-https}"
  WG_PORT="${3:-51820}"
fi

# Normalize scheme
SCHEME="$(echo "${SCHEME}" | tr '[:upper:]' '[:lower:]')"
if [[ "${SCHEME}" != "https" ]]; then
  echo "Error: production deployments require https. Use the development stack for plain HTTP." >&2
  exit 1
fi

is_ipv4_address() {
  local value="$1"
  local octet
  local -a octets
  [[ "${value}" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] || return 1
  IFS=. read -r -a octets <<<"${value}"
  for octet in "${octets[@]}"; do
    ((10#${octet} <= 255)) || return 1
  done
}

is_ipv6_address() {
  local value="$1"
  [[ "${value}" == *:* && "${value}" =~ ^[0-9A-Fa-f:.]+$ ]]
}

is_ip_address() {
  is_ipv4_address "$1" || is_ipv6_address "$1"
}

is_dns_name() {
  local value="$1"
  local label
  local -a labels
  ((${#value} <= 253)) || return 1
  IFS=. read -r -a labels <<<"${value}"
  for label in "${labels[@]}"; do
    [[ -n "${label}" && ${#label} -le 63 ]] || return 1
    [[ "${label}" =~ ^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$ ]] || return 1
  done
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

  if ! openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes \
    -keyout "${CADDY_KEY_FILE}" \
    -out "${CADDY_CERT_FILE}" \
    -subj "/CN=${host}" \
    -addext "subjectAltName=${san}" >/dev/null 2>&1; then
    echo "Error: could not generate a certificate for invalid host ${host}." >&2
    exit 1
  fi

  chmod 600 "${CADDY_KEY_FILE}"
  chmod 644 "${CADDY_CERT_FILE}"
}

if [[ "${DOMAIN_OR_IP}" == \[*\] ]]; then
  DOMAIN_OR_IP="${DOMAIN_OR_IP:1:${#DOMAIN_OR_IP}-2}"
fi
if [[ -z "${DOMAIN_OR_IP}" || "${DOMAIN_OR_IP}" == *[[:space:]/?#]* || "${DOMAIN_OR_IP}" == *://* ]]; then
  echo "Error: domain_or_ip must be a bare DNS name or IP literal." >&2
  exit 1
fi
if [[ "${DOMAIN_OR_IP}" =~ ^[0-9.]+$ ]] && ! is_ipv4_address "${DOMAIN_OR_IP}"; then
  echo "Error: invalid IPv4 address: ${DOMAIN_OR_IP}" >&2
  exit 1
fi
if ! is_ip_address "${DOMAIN_OR_IP}" && ! is_dns_name "${DOMAIN_OR_IP}"; then
  echo "Error: invalid DNS name or IP literal: ${DOMAIN_OR_IP}" >&2
  exit 1
fi
if [[ ! "${WG_PORT}" =~ ^[0-9]+$ ]] || ((10#${WG_PORT} < 1 || 10#${WG_PORT} > 65535)); then
  echo "Error: wireguard_port must be an integer from 1 to 65535." >&2
  exit 1
fi

# Construct URL and endpoint authorities. RFC 3986 requires brackets around
# IPv6 literals when a scheme or port is present.
public_host="${DOMAIN_OR_IP}"
if is_ipv6_address "${DOMAIN_OR_IP}"; then
  public_host="[${DOMAIN_OR_IP}]"
fi

edge_site_address="${SCHEME}://${public_host}"
if is_ip_address "${DOMAIN_OR_IP}"; then
  # Most clients do not send SNI for IP literals. Use a catch-all TLS listener
  # so Caddy can serve the self-signed IP-SAN certificate during handshake.
  edge_site_address="https://:443"
fi
ad_platform_public_url="${SCHEME}://${public_host}"
wireguard_endpoint="${public_host}:${WG_PORT}"
edge_tls_directive=""
certificate_summary="Caddy automatic HTTPS (ACME)"
if is_ip_address "${DOMAIN_OR_IP}" || [[ "${DOMAIN_OR_IP,,}" == "localhost" ]]; then
  generate_self_signed_cert "${DOMAIN_OR_IP}"
  edge_tls_directive="tls ${EDGE_CERT_FILE} ${EDGE_KEY_FILE}"
  certificate_summary="${CADDY_CERT_FILE}"
fi

echo "Updating environment configurations in prod.env..."
echo "  EDGE_SITE_ADDRESS           -> ${edge_site_address}"
echo "  AD_PLATFORM_PUBLIC_BASE_URL -> ${ad_platform_public_url}"
echo "  WIREGUARD_SERVER_ENDPOINT   -> ${wireguard_endpoint}"
echo "  EDGE_TLS_DIRECTIVE          -> ${edge_tls_directive}"
echo "  Certificate source          -> ${certificate_summary}"

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
