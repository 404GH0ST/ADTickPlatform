#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin psql
require_bin docker

load_default_env_files
PROD_ENV="${PROD_ENV:-${ROOT_DIR}/deploy/compose/prod.env}"
load_env_file_override "${PROD_ENV}"
load_env_file .runtime/backend-stack.env

STACK_MODE="${AD_PLATFORM_STACK_MODE:-postgres}"
CLEAR_TEAMS="${BOOTSTRAP_CLEAR_TEAMS:-false}"
if [[ "${STACK_MODE}" != "postgres" ]]; then
  echo "clean match bootstrap requires a postgres-backed stack; current mode=${STACK_MODE}" >&2
  exit 1
fi

POSTGRES_DSN="${POSTGRES_DSN_HOST_ENFORCEMENT:-${POSTGRES_DSN:-${AD_PLATFORM_POSTGRES_DSN:-}}}"
if [[ -z "${POSTGRES_DSN}" ]]; then
  POSTGRES_PORT="$(
    docker compose -f deploy/compose/dev.yml port postgres 5432 2>/dev/null | awk -F: 'NF { print $NF }' | tail -n1
  )"
  if [[ -z "${POSTGRES_PORT}" ]]; then
    echo "could not resolve Postgres connection details for clean match bootstrap" >&2
    exit 1
  fi
  POSTGRES_DSN="postgres://adplatform:adplatform@127.0.0.1:${POSTGRES_PORT}/adplatform?sslmode=disable"
fi

readarray -t runtime_rows < <(
  psql "${POSTGRES_DSN}" -At -F '|' -c \
    "SELECT COALESCE(container_name, ''), COALESCE(state_volume, '') FROM service_instances ORDER BY team_id, challenge_id" \
    2>/dev/null || true
)

declare -A container_names=()
declare -A volume_names=()

for row in "${runtime_rows[@]:-}"; do
  [[ -n "${row}" ]] || continue
  IFS='|' read -r container_name state_volume <<< "${row}"
  if [[ -n "${container_name}" ]]; then
    container_names["${container_name}"]=1
  fi
  if [[ -n "${state_volume}" ]]; then
    volume_names["${state_volume}"]=1
  fi
done

readarray -t platform_containers < <(docker ps -a --filter label=adplatform.team_id --format '{{.Names}}')
for container_name in "${platform_containers[@]:-}"; do
  [[ -n "${container_name}" ]] || continue
  container_names["${container_name}"]=1
  while IFS= read -r volume_name; do
    [[ -n "${volume_name}" ]] || continue
    volume_names["${volume_name}"]=1
  done < <(
    docker inspect --format '{{range .Mounts}}{{if eq .Type "volume"}}{{println .Name}}{{end}}{{end}}' "${container_name}" 2>/dev/null || true
  )
done

for container_name in "${!container_names[@]}"; do
  docker rm -f "${container_name}" >/dev/null 2>&1 || true
done

for volume_name in "${!volume_names[@]}"; do
  docker volume rm -f "${volume_name}" >/dev/null 2>&1 || true
done

readarray -t platform_networks < <(docker network ls --filter label=adplatform.game_network=true --format '{{.Name}}')
for network_name in "${platform_networks[@]:-}"; do
  [[ -n "${network_name}" ]] || continue
  docker network rm "${network_name}" >/dev/null 2>&1 || true
done

if [[ "${AD_PLATFORM_NETWORK_LAYOUT:-per-service}" == "per-service" ]]; then
  remove_unused_docker_network "${CONTROLLER_DOCKER_NETWORK:-adplatform_game}"
  if [[ "${CHECKER_RUNNER_DOCKER_NETWORK:-${CONTROLLER_DOCKER_NETWORK:-adplatform_game}}" != "${CONTROLLER_DOCKER_NETWORK:-adplatform_game}" ]]; then
    remove_unused_docker_network "${CHECKER_RUNNER_DOCKER_NETWORK:-}"
  fi
fi

psql "${POSTGRES_DSN}" <<'SQL'
BEGIN;

DELETE FROM checker_runs;
DELETE FROM issued_flags;
DELETE FROM game_scheduler_events;
DELETE FROM submitted_flags;
DELETE FROM attack_events;
DELETE FROM audit_logs;
DELETE FROM service_instances;
DELETE FROM deployment_jobs;
DELETE FROM team_service_states;
DELETE FROM game_ticks;
DELETE FROM challenges;
DELETE FROM scoreboard_entries;

INSERT INTO game_scheduler_state (singleton, state, interval_seconds, last_run_at, next_run_at, last_tick_id, last_error, updated_at)
VALUES (TRUE, 'stopped', 60, NULL, NULL, NULL, '', NOW())
ON CONFLICT (singleton) DO UPDATE SET
    state = 'stopped',
    interval_seconds = 60,
    last_run_at = NULL,
    next_run_at = NULL,
    last_tick_id = NULL,
    last_error = '',
    updated_at = NOW();

INSERT INTO game_match_state (singleton, state, started_at, ended_at, updated_at)
VALUES (TRUE, 'not_started', NULL, NULL, NOW())
ON CONFLICT (singleton) DO UPDATE SET
    state = 'not_started',
    started_at = NULL,
    ended_at = NULL,
    scheduled_start_at = NULL,
    scheduled_end_at = NULL,
    schedule_configured = FALSE,
    updated_at = NOW();

ALTER SEQUENCE IF EXISTS checker_runs_id_seq RESTART WITH 1;
ALTER SEQUENCE IF EXISTS game_scheduler_events_id_seq RESTART WITH 1;

COMMIT;
SQL

if [[ "${CLEAR_TEAMS}" == "true" ]]; then
  psql "${POSTGRES_DSN}" <<'SQL'
BEGIN;
DELETE FROM wireguard_peers;
DELETE FROM players;
DELETE FROM teams;
COMMIT;
SQL
else
  psql "${POSTGRES_DSN}" <<'SQL'
BEGIN;
INSERT INTO scoreboard_entries (team_id, rank, team_name, attack_points, defense_points, sla_points, total_points, delta)
SELECT
    t.id,
    ROW_NUMBER() OVER (ORDER BY t.name, t.id),
    t.name,
    0,
    0,
    0,
    0,
    '0'
FROM teams t;
COMMIT;
SQL
fi

echo "clean match bootstrap completed"
if [[ "${CLEAR_TEAMS}" == "true" ]]; then
  echo "  state: platform emptied (teams, players, peers, challenges, runtime, ticks, audit logs)"
else
  echo "  state: challenges cleared, runtime rows cleared, ticks cleared, scoreboard reset to zeroed team rows"
fi
