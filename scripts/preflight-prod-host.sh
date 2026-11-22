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
require_bin findmnt

PROD_ENV="${PROD_ENV:-deploy/compose/prod.env}"
load_env_file "${PROD_ENV}"
"${ROOT_DIR}/scripts/validate-prod-env.sh" "${PROD_ENV}"

require_noninteractive_root
"${ROOT_DIR}/scripts/preflight-capture-host.sh" "${PROD_ENV}"

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "missing required variable in ${PROD_ENV}: ${name}" >&2
    exit 1
  fi
}

require_secret_env() {
  local name="$1"
  require_env "${name}"
  if is_placeholder_secret "${!name:-}"; then
    echo "placeholder value is not allowed in ${PROD_ENV}: ${name}" >&2
    exit 1
  fi
}

wireguard_endpoint_with_port() {
  local endpoint="$1"

  endpoint="${endpoint#http://}"
  endpoint="${endpoint#https://}"
  if [[ "${endpoint}" == \[*\]:* ]]; then
    return 0
  fi
  if [[ "${endpoint}" == *:* && "${endpoint}" != *:*:* ]]; then
    return 0
  fi
  return 1
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

validate_challenge_storage() {
  local limit="${CONTROLLER_SERVICE_DISK_LIMIT_MIB:-1024}"
  if [[ ! "${limit}" =~ ^[1-9][0-9]{0,11}$ ]]; then
    echo "CONTROLLER_SERVICE_DISK_LIMIT_MIB must specify a positive storage quota in MiB" >&2
    return 1
  fi
  local size=$((limit * 1048576))
  local driver docker_root
  read -r driver docker_root < <(docker info --format '{{.Driver}} {{.DockerRootDir}}')
  if [[ "${driver}" != overlay2 ]] ||
     [[ "$(findmnt -n -o FSTYPE -T "${docker_root}/overlay2")" != xfs ]] ||
     [[ ",$(findmnt -n -o OPTIONS -T "${docker_root}/overlay2")," != *,pquota,* &&
        ",$(findmnt -n -o OPTIONS -T "${docker_root}/overlay2")," != *,prjquota,* ]]; then
    echo "prod-host requires Docker overlay2 on XFS with project quotas for challenge writable layers" >&2
    return 1
  fi

  local probe="adplatform-quota-probe-$(cat /proc/sys/kernel/random/uuid)"
  if ! docker volume create --driver local --opt "size=${size}" "${probe}" >/dev/null; then
    echo "Docker local volumes must support project quotas for challenge state" >&2
    return 1
  fi
  docker volume rm "${probe}" >/dev/null

  local containers container limits volume volume_options
  containers="$(docker ps -aq --filter label=adplatform.team_id)"
  for container in ${containers}; do
    limits="$(docker inspect --format '{{index .HostConfig.StorageOpt "size"}} {{.HostConfig.LogConfig.Type}} {{index .HostConfig.LogConfig.Config "max-size"}} {{index .HostConfig.LogConfig.Config "max-file"}}' "${container}")"
    volume="$(docker inspect --format '{{index .Config.Labels "adplatform.state_volume"}}' "${container}")"
    if [[ "${limits}" != "${size} local 10m 3" || -z "${volume}" ]]; then
      echo "challenge container ${container} requires offline recreation with storage limits" >&2
      return 1
    fi
    volume_options="$(docker volume inspect --format '{{.Driver}} {{json .Options}}' "${volume}")"
    if [[ "${volume_options}" != "local {\"size\":\"${size}\"}" ]]; then
      echo "challenge volume ${volume} requires offline migration to a quota-managed volume" >&2
      return 1
    fi
  done
}

require_secret_env ADMIN_API_TOKEN
require_secret_env CONTROLLER_INTERNAL_TOKEN
require_secret_env GAME_CORE_INTERNAL_TOKEN
require_secret_env SUBMISSION_SERVICE_INTERNAL_TOKEN
require_secret_env SCORING_WORKER_INTERNAL_TOKEN
require_secret_env CHECKER_RUNNER_INTERNAL_TOKEN
require_secret_env WIREGUARD_GATEWAY_INTERNAL_TOKEN
require_secret_env REALTIME_ADMIN_TOKEN
require_secret_env TEAM_JWT_SECRET
require_secret_env SSH_CREDENTIAL_SECRET
require_secret_env GAME_CORE_FLAG_SECRET
require_secret_env POSTGRES_PASSWORD
require_secret_env POSTGRES_DSN
require_secret_env POSTGRES_DSN_HOST_ENFORCEMENT
require_env WIREGUARD_SERVER_ENDPOINT
require_env WIREGUARD_SERVER_ADDRESS
require_env WIREGUARD_SERVER_LISTEN_PORT
require_env WIREGUARD_SERVER_ALLOWED_IPS
require_secret_env WIREGUARD_SERVER_PRIVATE_KEY
if ! wireguard_endpoint_with_port "${WIREGUARD_SERVER_ENDPOINT}"; then
  echo "WIREGUARD_SERVER_ENDPOINT must include host:port in ${PROD_ENV}; use ${WIREGUARD_SERVER_ENDPOINT}:${WIREGUARD_SERVER_LISTEN_PORT}" >&2
  exit 1
fi

DOCKER_BRIDGE_GATEWAY="$(docker network inspect bridge --format '{{(index .IPAM.Config 0).Gateway}}')"
HOST_CONTROL_BIND_ADDRESS="${HOST_CONTROL_BIND_ADDRESS:-}"
case "${HOST_CONTROL_BIND_ADDRESS}" in
  ""|0.0.0.0|::|\[::\])
    echo "HOST_CONTROL_BIND_ADDRESS must be the private Docker bridge gateway, not a wildcard address" >&2
    exit 1
    ;;
esac
if [[ "${HOST_CONTROL_BIND_ADDRESS}" != "${DOCKER_BRIDGE_GATEWAY}" ]]; then
  echo "HOST_CONTROL_BIND_ADDRESS must match Docker bridge gateway ${DOCKER_BRIDGE_GATEWAY}; got ${HOST_CONTROL_BIND_ADDRESS}" >&2
  exit 1
fi

POSTGRES_LOOPBACK_PORT="${POSTGRES_LOOPBACK_PORT:-15432}"
CHECKER_RUNNER_LOOPBACK_PORT="${CHECKER_RUNNER_LOOPBACK_PORT:-18083}"
CONTROLLER_HOST_PORT="${CONTROLLER_SERVICE_PORT:-18084}"
WIREGUARD_HOST_PORT="${WIREGUARD_GATEWAY_PORT:-18087}"
WIREGUARD_INTERFACE="${WIREGUARD_GATEWAY_INTERFACE:-wg0}"

docker compose version >/dev/null
"${ROOT_DIR}/scripts/validate-prod-compose-security.sh" "${PROD_ENV}"
wg help >/dev/null
nft --version >/dev/null
validate_challenge_storage

check_host_interface "${WIREGUARD_INTERFACE}"
internet_interface="${CONTROLLER_INTERNET_INTERFACE:-eth0}"
check_host_interface "${internet_interface}"
default_routes="$(ip -4 -o route show default)"
if [[ -n "${default_routes}" ]] && ! awk -v uplink="${internet_interface}" '
  { for (i = 1; i < NF; i++) if ($i == "dev" && $(i+1) == uplink) found = 1 }
  END { exit !found }
' <<< "${default_routes}"; then
  echo "CONTROLLER_INTERNET_INTERFACE must select an IPv4 default-route interface on this host" >&2
  exit 1
fi

assert_port_free "${POSTGRES_LOOPBACK_PORT}" "postgres loopback"
assert_port_free "${CHECKER_RUNNER_LOOPBACK_PORT}" "checker-runner loopback"
assert_port_free "${CONTROLLER_HOST_PORT}" "controller-service host"
assert_port_free "${WIREGUARD_HOST_PORT}" "wireguard-gateway host"

echo "host enforcement preflight passed:"
echo "  interface: ${WIREGUARD_INTERFACE}"
echo "  internet interface: ${internet_interface}"
echo "  private control bind: ${HOST_CONTROL_BIND_ADDRESS}"
echo "  postgres loopback port: ${POSTGRES_LOOPBACK_PORT}"
echo "  checker-runner loopback port: ${CHECKER_RUNNER_LOOPBACK_PORT}"
echo "  controller-service host port: ${CONTROLLER_HOST_PORT}"
echo "  wireguard-gateway host port: ${WIREGUARD_HOST_PORT}"
echo "  root automation: $( [[ "${EUID}" -eq 0 ]] && printf 'running as root' || printf 'sudo NOPASSWD available' )"
