#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin docker
require_bin jq

prod_env="${1:-${PROD_ENV:-deploy/compose/prod.env}}"
host_override="${PROD_HOST_OVERRIDE:-deploy/compose/prod.host-enforcement.yml}"
"${ROOT_DIR}/scripts/validate-prod-env.sh" "${prod_env}" >/dev/null

rendered="$(mktemp /tmp/adplatform-prod-host-config.XXXXXX.json)"
trap 'rm -f "${rendered}"' EXIT
docker compose --env-file "${prod_env}" \
  -f deploy/compose/prod.yml \
  -f "${host_override}" \
  config --format json >"${rendered}"

if ! jq -e '
  .services["docker-socket-proxy"] as $proxy |
  $proxy.networks["docker-control"].ipv4_address as $proxy_ip |
  (($proxy.ports // []) | length == 0) and
  (($proxy.networks | keys) == ["docker-control"]) and
  (.services["controller-service"].environment.DOCKER_HOST == ("tcp://" + $proxy_ip + ":2375")) and
  (.services["controller-service"].environment.CONTROLLER_REQUIRE_DISK_QUOTA == "true") and
  (.services["controller-service"].environment.CONTROLLER_SERVICE_DISK_LIMIT_MIB | test("^[1-9][0-9]{0,11}$")) and
  (.services["controller-service"].environment.CONTROLLER_SERVICE_ADDR | test("^(0\\.0\\.0\\.0|:|\\[?::)") | not) and
  (.services["wireguard-gateway"].environment.WIREGUARD_GATEWAY_ADDR | test("^(0\\.0\\.0\\.0|:|\\[?::)") | not) and
  (.services.edge.environment.EDGE_SITE_ADDRESS | startswith("https://"))
' "${rendered}" >/dev/null; then
  echo "rendered host compose configuration violates production isolation requirements" >&2
  exit 1
fi

if ! jq -e '
  .services["controller-service"] as $c |
  if $c.environment.CONTROLLER_CAPTURE_MODE == "guarded" then
    $c.cgroup == "host" and
    (["BPF","PERFMON","NET_ADMIN","SYS_ADMIN"] - $c.cap_add | length == 0) and
    ($c.privileged // false | not) and
    ($c.pid // "" | . != "host") and
    ($c.volumes | any(.target == "/sys/fs/cgroup" and (.read_only // false) == false)) and
    ($c.volumes | any(.target == "/host/proc" and .read_only == true)) and
    ($c.volumes | any(.target == "/host/docker-netns" and .read_only == true and .bind.propagation == "rslave")) and
    $c.depends_on["capture-bootstrap"].condition == "service_completed_successfully" and
    .services["capture-bootstrap"].network_mode == "none" and
    .services["capture-bootstrap"].read_only == true and
    .services["capture-bootstrap"].cap_drop == ["ALL"]
  else true end
' "${rendered}" >/dev/null; then
  echo "rendered compose configuration is missing guarded capture boundaries" >&2
  exit 1
fi

echo "production host compose security validated: ${prod_env}"
