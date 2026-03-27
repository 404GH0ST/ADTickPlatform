#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin docker
require_bin wg
require_bin nft
require_bin ip
require_bin ss

PROD_ENV="${PROD_ENV:-deploy/compose/prod.env}"
load_env_file "${PROD_ENV}"

require_noninteractive_root

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "missing required variable in ${PROD_ENV}: ${name}" >&2
    exit 1
  fi
}

port_from_addr() {
  local addr="$1"
  addr="${addr##*:}"
  printf '%s\n' "${addr}"
}

assert_port_free() {
  local port="$1"
  local label="$2"
  if ss -ltnH "( sport = :${port} )" | grep -q .; then
    echo "${label} port ${port} is already in use" >&2
    exit 1
  fi
}

check_host_interface() {
  local iface="$1"
  if ! ip link show "${iface}" >/dev/null 2>&1; then
    echo "required host interface does not exist: ${iface}" >&2
    exit 1
  fi
}

require_env ADMIN_API_TOKEN
require_env TEAM_JWT_SECRET
require_env UNLOCK_PROOF_SECRET
require_env GAME_CORE_FLAG_SECRET
require_env WIREGUARD_SERVER_ENDPOINT
require_env WIREGUARD_SERVER_ADDRESS
require_env WIREGUARD_SERVER_LISTEN_PORT
require_env WIREGUARD_SERVER_ALLOWED_IPS
require_env WIREGUARD_SERVER_PRIVATE_KEY

POSTGRES_LOOPBACK_PORT="${POSTGRES_LOOPBACK_PORT:-15432}"
CHECKER_RUNNER_LOOPBACK_PORT="${CHECKER_RUNNER_LOOPBACK_PORT:-18083}"
CONTROLLER_HOST_PORT="$(port_from_addr "${CONTROLLER_SERVICE_ADDR_HOST:-:18084}")"
WIREGUARD_HOST_PORT="$(port_from_addr "${WIREGUARD_GATEWAY_ADDR_HOST:-:18087}")"
WIREGUARD_INTERFACE="${WIREGUARD_GATEWAY_INTERFACE:-wg0}"

docker compose version >/dev/null
wg help >/dev/null
nft --version >/dev/null

check_host_interface "${WIREGUARD_INTERFACE}"

assert_port_free "${POSTGRES_LOOPBACK_PORT}" "postgres loopback"
assert_port_free "${CHECKER_RUNNER_LOOPBACK_PORT}" "checker-runner loopback"
assert_port_free "${CONTROLLER_HOST_PORT}" "controller-service host"
assert_port_free "${WIREGUARD_HOST_PORT}" "wireguard-gateway host"

echo "host enforcement preflight passed:"
echo "  interface: ${WIREGUARD_INTERFACE}"
echo "  postgres loopback port: ${POSTGRES_LOOPBACK_PORT}"
echo "  checker-runner loopback port: ${CHECKER_RUNNER_LOOPBACK_PORT}"
echo "  controller-service host port: ${CONTROLLER_HOST_PORT}"
echo "  wireguard-gateway host port: ${WIREGUARD_HOST_PORT}"
echo "  root automation: $( [[ "${EUID}" -eq 0 ]] && printf 'running as root' || printf 'sudo NOPASSWD available' )"
