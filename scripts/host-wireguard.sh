#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

ACTION="${1:-}"
PROD_ENV="${PROD_ENV:-deploy/compose/prod.env}"

load_env_file "${PROD_ENV}"
load_env_file .env
load_env_file .env.example

WIREGUARD_INTERFACE="${WIREGUARD_GATEWAY_INTERFACE:-wg0}"
WIREGUARD_ADDRESS="${WIREGUARD_SERVER_ADDRESS:-}"
WIREGUARD_LISTEN_PORT="${WIREGUARD_SERVER_LISTEN_PORT:-}"
WIREGUARD_PRIVATE_KEY="${WIREGUARD_SERVER_PRIVATE_KEY:-}"
WIREGUARD_CONFIG_PATH="${WIREGUARD_CONFIG_PATH:-/etc/wireguard/${WIREGUARD_INTERFACE}.conf}"

usage() {
  cat <<EOF
usage: $0 <keygen|render|install|up|down|show|setup>

actions:
  keygen   print a new WireGuard server private/public keypair
  render   print the host wg-quick config derived from env
  install  install the rendered config to ${WIREGUARD_CONFIG_PATH}
  up       bring up ${WIREGUARD_INTERFACE} with wg-quick
  down     bring down ${WIREGUARD_INTERFACE} with wg-quick
  show     show current interface and WireGuard status
  setup    install the config and bring the interface up

env sources:
  PROD_ENV=${PROD_ENV}
  .env
  .env.example
EOF
}

require_wg_env() {
  local missing=()

  [[ -n "${WIREGUARD_ADDRESS}" ]] || missing+=("WIREGUARD_SERVER_ADDRESS")
  [[ -n "${WIREGUARD_LISTEN_PORT}" ]] || missing+=("WIREGUARD_SERVER_LISTEN_PORT")
  [[ -n "${WIREGUARD_PRIVATE_KEY}" ]] || missing+=("WIREGUARD_SERVER_PRIVATE_KEY")

  if [[ "${#missing[@]}" -gt 0 ]]; then
    printf 'missing required WireGuard env: %s\n' "${missing[*]}" >&2
    echo "fill ${PROD_ENV} or export them before running this command" >&2
    exit 1
  fi
}

render_config() {
  require_wg_env
  cat <<EOF
[Interface]
Address = ${WIREGUARD_ADDRESS}
ListenPort = ${WIREGUARD_LISTEN_PORT}
PrivateKey = ${WIREGUARD_PRIVATE_KEY}
SaveConfig = false
EOF
}

install_config() {
  require_bin sudo
  require_bin install

  local tmp
  tmp="$(mktemp)"
  trap 'rm -f "${tmp}"' RETURN

  render_config > "${tmp}"
  sudo install -d -m 700 "$(dirname "${WIREGUARD_CONFIG_PATH}")"
  sudo install -m 600 "${tmp}" "${WIREGUARD_CONFIG_PATH}"
  echo "installed ${WIREGUARD_CONFIG_PATH}"
}

bring_up() {
  require_bin sudo
  require_bin wg-quick
  sudo wg-quick up "${WIREGUARD_INTERFACE}"
}

bring_down() {
  require_bin sudo
  require_bin wg-quick
  sudo wg-quick down "${WIREGUARD_INTERFACE}"
}

show_status() {
  require_bin ip
  require_bin wg
  ip addr show "${WIREGUARD_INTERFACE}" || true
  echo "---"
  sudo wg show "${WIREGUARD_INTERFACE}" || true
}

keygen() {
  require_bin wg
  local private_key public_key
  private_key="$(wg genkey)"
  public_key="$(printf '%s' "${private_key}" | wg pubkey)"
  cat <<EOF
WIREGUARD_SERVER_PRIVATE_KEY=${private_key}
WIREGUARD_SERVER_PUBLIC_KEY=${public_key}
EOF
}

case "${ACTION}" in
  keygen)
    keygen
    ;;
  render)
    render_config
    ;;
  install)
    install_config
    ;;
  up)
    bring_up
    ;;
  down)
    bring_down
    ;;
  show)
    show_status
    ;;
  setup)
    install_config
    bring_up
    ;;
  *)
    usage >&2
    exit 1
    ;;
esac
