#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin curl
require_bin jq
require_bin docker

PROD_ENV="${PROD_ENV:-deploy/compose/prod.env}"
PROD_HOST_OVERRIDE="${PROD_HOST_OVERRIDE:-deploy/compose/prod.host-enforcement.yml}"
load_env_file "${PROD_ENV}"

EDGE_BASE_URL="${PROD_EDGE_BASE_URL:-$(derive_edge_base_url)}"
ADMIN_TOKEN="$(resolve_admin_api_token "${ROOT_DIR}/.runtime/backend-stack.env")"
COMPOSE_PROJECT="${COMPOSE_PROJECT_NAME:-ad-platform-prod}"

admin_get() {
  local path="$1"
  curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" "${EDGE_BASE_URL}${path}"
}

compose_cmd() {
  docker compose --project-name "${COMPOSE_PROJECT}" --env-file "${PROD_ENV}" -f deploy/compose/prod.yml -f "${PROD_HOST_OVERRIDE}" "$@"
}

echo "production host recovery smoke: base_url=${EDGE_BASE_URL}"

echo "pre-restart control-plane snapshot:"
pre_game_status="$(admin_get /api/v2/admin/game/status)"
echo "${pre_game_status}" | jq -c '.match as $match | .scheduler as $scheduler | {match_state:$match.state,scheduler_state:$scheduler.state,current_tick:.current_tick.id}'

echo "restarting controller-service, wireguard-gateway, and game-core:"
compose_cmd restart controller-service wireguard-gateway game-core

echo "waiting for edge and organizer endpoints:"
wait_for_http "${EDGE_BASE_URL}/healthz" 60 "edge healthz"
wait_for_http "${EDGE_BASE_URL}/api/v2/challenges" 60 "public challenges"

for attempt in $(seq 1 60); do
  if admin_get /api/v2/admin/wireguard/status >/dev/null 2>&1 &&
    admin_get /api/v2/admin/access/status >/dev/null 2>&1 &&
    admin_get /api/v2/admin/game/status >/dev/null 2>&1; then
    echo "organizer endpoints ready after restart"
    break
  fi
  sleep 1
  if [[ "${attempt}" == "60" ]]; then
    echo "organizer endpoints did not recover within 60s" >&2
    exit 1
  fi
done

echo "re-running host enforcement smoke:"
"${ROOT_DIR}/scripts/smoke-prod-host-enforcement.sh"

echo "post-restart control-plane snapshot:"
post_game_status="$(admin_get /api/v2/admin/game/status)"
echo "${post_game_status}" | jq -c '.match as $match | .scheduler as $scheduler | {match_state:$match.state,scheduler_state:$scheduler.state,current_tick:.current_tick.id}'
admin_get /api/v2/admin/wireguard/status | jq -c '{mode,state,peers_active,revision}'
admin_get /api/v2/admin/access/status | jq -c '{mode,state,policies_total,ssh_open_services,revision}'

echo "host recovery smoke passed:"
printf '  compose_project=%s edge=%s\n' "${COMPOSE_PROJECT}" "${EDGE_BASE_URL}"
