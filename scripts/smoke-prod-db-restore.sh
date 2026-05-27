#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin docker
require_bin curl
require_bin jq
require_bin sha256sum

PROD_ENV="${PROD_ENV:-deploy/compose/prod.env}"
PROD_HOST_OVERRIDE="${PROD_HOST_OVERRIDE:-deploy/compose/prod.host-enforcement.yml}"
COMPOSE_PROJECT="${COMPOSE_PROJECT_NAME:-ad-platform-prod}"

load_env_file "${PROD_ENV}"
ADMIN_TOKEN="$(resolve_admin_api_token "${ROOT_DIR}/.runtime/backend-stack.env")"

POSTGRES_DB="${POSTGRES_DB:-adplatform}"
POSTGRES_USER="${POSTGRES_USER:-adplatform}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-adplatform}"
EDGE_BASE_URL="$(derive_edge_base_url)"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUTPUT_DIR="${RESTORE_DRILL_OUTPUT_DIR:-.runtime/prod-db-restore-${TIMESTAMP}}"
BACKUP_FILE="${OUTPUT_DIR}/postgres-backup.sql"
PRE_GAME_STATUS_FILE="${OUTPUT_DIR}/pre-restore-game-status.json"
POST_GAME_STATUS_FILE="${OUTPUT_DIR}/post-restore-game-status.json"
PRE_SCOREBOARD_FILE="${OUTPUT_DIR}/pre-restore-scoreboard.json"
POST_SCOREBOARD_FILE="${OUTPUT_DIR}/post-restore-scoreboard.json"
PRE_ATTACKS_FILE="${OUTPUT_DIR}/pre-restore-attacks.json"
POST_ATTACKS_FILE="${OUTPUT_DIR}/post-restore-attacks.json"
PRE_COMPOSE_FILE="${OUTPUT_DIR}/pre-restore-compose-ps.txt"
POST_COMPOSE_FILE="${OUTPUT_DIR}/post-restore-compose-ps.txt"

readonly APP_SERVICES=(
  redis
  checker-runner
  game-core
  submission-service
  controller-service
  scoring-worker
  realtime-gateway
  wireguard-gateway
  api-gateway
  web
  edge
)

mkdir -p "${OUTPUT_DIR}"

compose_cmd() {
  docker compose --project-name "${COMPOSE_PROJECT}" \
    --env-file "${PROD_ENV}" \
    -f deploy/compose/prod.yml \
    -f "${PROD_HOST_OVERRIDE}" \
    "$@"
}

postgres_shell() {
  compose_cmd exec -T postgres sh -lc "$1"
}

wait_for_admin_json() {
  local path="$1"
  local attempts="${2:-60}"
  local i

  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      "${EDGE_BASE_URL}${path}" >/dev/null 2>&1; then
      echo "organizer endpoint ready at ${EDGE_BASE_URL}${path}"
      return 0
    fi
    sleep 1
  done

  echo "organizer endpoint did not become ready at ${EDGE_BASE_URL}${path}" >&2
  return 1
}

capture_admin_data() {
  local path="$1"
  local destination="$2"
  curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    "${EDGE_BASE_URL}${path}" > "${destination}"
}

capture_public_data() {
  local path="$1"
  local destination="$2"
  curl -fsS "${EDGE_BASE_URL}${path}" > "${destination}"
}

normalized_game_status() {
  jq -c '{
    match_state: .match.state,
    scheduler_state: .scheduler.state,
    current_tick: (.current_tick.id // 0),
    total_ticks,
    total_checker_runs
  }' "$1"
}

normalized_scoreboard() {
  jq -c 'map({team,attack,defense,sla,total})' "$1"
}

normalized_attacks() {
  jq -c '{
    total_count,
    item_ids: [.items[]?.id]
  }' "$1"
}

echo "production database restore drill: base_url=${EDGE_BASE_URL} output=${OUTPUT_DIR}"

wait_for_http "${EDGE_BASE_URL}/healthz" 60 "edge healthz"
wait_for_http "${EDGE_BASE_URL}/api/v2/challenges" 60 "public challenges"
wait_for_admin_json "/api/v2/admin/game/status" 60

echo "capturing pre-restore control-plane snapshot:"
capture_admin_data "/api/v2/admin/game/status" "${PRE_GAME_STATUS_FILE}"
capture_admin_data "/api/v2/admin/game/scoreboard" "${PRE_SCOREBOARD_FILE}"
capture_public_data "/api/v2/attacks?limit=25" "${PRE_ATTACKS_FILE}"
compose_cmd ps > "${PRE_COMPOSE_FILE}"
cat "${PRE_GAME_STATUS_FILE}"

echo "dumping postgres backup to ${BACKUP_FILE}"
postgres_shell "export PGPASSWORD='${POSTGRES_PASSWORD}'; pg_dump -U '${POSTGRES_USER}' -d '${POSTGRES_DB}' --clean --if-exists --no-owner --no-privileges" > "${BACKUP_FILE}"
sha256sum "${BACKUP_FILE}" > "${BACKUP_FILE}.sha256"

echo "stopping application services for restore:"
compose_cmd stop "${APP_SERVICES[@]}"

echo "dropping and recreating database ${POSTGRES_DB}:"
postgres_shell "export PGPASSWORD='${POSTGRES_PASSWORD}'; psql -v ON_ERROR_STOP=1 -U '${POSTGRES_USER}' -d postgres -c \"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${POSTGRES_DB}' AND pid <> pg_backend_pid();\" -c \"DROP DATABASE IF EXISTS \\\"${POSTGRES_DB}\\\";\" -c \"CREATE DATABASE \\\"${POSTGRES_DB}\\\";\""

echo "restoring postgres backup:"
postgres_shell "export PGPASSWORD='${POSTGRES_PASSWORD}'; psql -v ON_ERROR_STOP=1 -U '${POSTGRES_USER}' -d '${POSTGRES_DB}'" < "${BACKUP_FILE}"

echo "starting stack after restore:"
compose_cmd up -d

wait_for_http "${EDGE_BASE_URL}/healthz" 90 "edge healthz"
wait_for_http "${EDGE_BASE_URL}/api/v2/challenges" 90 "public challenges"
wait_for_admin_json "/api/v2/admin/game/status" 90

echo "capturing post-restore control-plane snapshot:"
capture_admin_data "/api/v2/admin/game/status" "${POST_GAME_STATUS_FILE}"
capture_admin_data "/api/v2/admin/game/scoreboard" "${POST_SCOREBOARD_FILE}"
capture_public_data "/api/v2/attacks?limit=25" "${POST_ATTACKS_FILE}"
compose_cmd ps > "${POST_COMPOSE_FILE}"
cat "${POST_GAME_STATUS_FILE}"

pre_game_summary="$(normalized_game_status "${PRE_GAME_STATUS_FILE}")"
post_game_summary="$(normalized_game_status "${POST_GAME_STATUS_FILE}")"
if [[ "${pre_game_summary}" != "${post_game_summary}" ]]; then
  echo "game status changed across backup/restore:" >&2
  echo "  before=${pre_game_summary}" >&2
  echo "  after=${post_game_summary}" >&2
  exit 1
fi

pre_scoreboard_summary="$(normalized_scoreboard "${PRE_SCOREBOARD_FILE}")"
post_scoreboard_summary="$(normalized_scoreboard "${POST_SCOREBOARD_FILE}")"
if [[ "${pre_scoreboard_summary}" != "${post_scoreboard_summary}" ]]; then
  echo "scoreboard changed across backup/restore" >&2
  exit 1
fi

pre_attacks_summary="$(normalized_attacks "${PRE_ATTACKS_FILE}")"
post_attacks_summary="$(normalized_attacks "${POST_ATTACKS_FILE}")"
if [[ "${pre_attacks_summary}" != "${post_attacks_summary}" ]]; then
  echo "attack feed changed across backup/restore" >&2
  exit 1
fi

cat > "${OUTPUT_DIR}/README.txt" <<EOF
Production database restore drill completed at ${TIMESTAMP}.

Commands run:
- docker compose exec postgres pg_dump
- docker compose stop ${APP_SERVICES[*]}
- docker compose exec postgres psql (drop/create database)
- docker compose up -d

Artifacts in this directory:
- postgres-backup.sql
- postgres-backup.sql.sha256
- pre-restore-game-status.json
- post-restore-game-status.json
- pre-restore-scoreboard.json
- post-restore-scoreboard.json
- pre-restore-attacks.json
- post-restore-attacks.json
- pre-restore-compose-ps.txt
- post-restore-compose-ps.txt
EOF

echo "production database restore drill passed:"
printf '  %s\n' \
  "${BACKUP_FILE}" \
  "${BACKUP_FILE}.sha256" \
  "${OUTPUT_DIR}/README.txt"
