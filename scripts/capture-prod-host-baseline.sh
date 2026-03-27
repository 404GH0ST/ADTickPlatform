#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin docker
require_bin iptables
require_bin nft
require_bin wg
if [[ "${EUID}" -ne 0 ]]; then
  require_bin sudo
fi

PROD_ENV="${PROD_ENV:-deploy/compose/prod.env}"
PROD_HOST_OVERRIDE="${PROD_HOST_OVERRIDE:-deploy/compose/prod.host-enforcement.yml}"
OUTPUT_DIR="${BASELINE_OUTPUT_DIR:-.runtime}"
COMPOSE_PROJECT="${COMPOSE_PROJECT_NAME:-ad-platform-prod}"
WIREGUARD_INTERFACE="${WIREGUARD_GATEWAY_INTERFACE:-wg0}"

mkdir -p "${OUTPUT_DIR}"
load_env_file "${PROD_ENV}"

host_net_cmd() {
  run_as_root_noninteractive "$@"
}

compose_cmd() {
  docker compose --project-name "${COMPOSE_PROJECT}" --env-file "${PROD_ENV}" -f deploy/compose/prod.yml -f "${PROD_HOST_OVERRIDE}" "$@"
}

echo "capturing production host baseline into ${OUTPUT_DIR}"

host_net_cmd iptables -S > "${OUTPUT_DIR}/final-iptables-filter.txt"
if ! host_net_cmd iptables -t raw -S > "${OUTPUT_DIR}/final-iptables-raw.txt"; then
  : > "${OUTPUT_DIR}/final-iptables-raw.txt"
fi
host_net_cmd nft list ruleset > "${OUTPUT_DIR}/final-nft-ruleset.txt"
host_net_cmd wg show "${WIREGUARD_INTERFACE}" > "${OUTPUT_DIR}/final-wg-show.txt"
compose_cmd ps > "${OUTPUT_DIR}/final-compose-ps.txt"

echo "baseline snapshot captured:"
printf '  %s\n' \
  "${OUTPUT_DIR}/final-iptables-filter.txt" \
  "${OUTPUT_DIR}/final-iptables-raw.txt" \
  "${OUTPUT_DIR}/final-nft-ruleset.txt" \
  "${OUTPUT_DIR}/final-wg-show.txt" \
  "${OUTPUT_DIR}/final-compose-ps.txt"
