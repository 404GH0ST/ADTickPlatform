#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE="${1:-memory}"

if [[ "${MODE}" != "memory" && "${MODE}" != "postgres" ]]; then
  echo "usage: $0 [memory|postgres]" >&2
  exit 1
fi

cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"
load_default_env_files

mkdir -p .runtime/logs .cache/go-build .cache/gomod
export GOCACHE="${ROOT_DIR}/.cache/go-build"
export GOMODCACHE="${ROOT_DIR}/.cache/gomod"
STACK_ENV_FILE="${ROOT_DIR}/.runtime/backend-stack.env"

export GAME_CORE_INTERNAL_URL="${GAME_CORE_INTERNAL_URL:-http://127.0.0.1:8081}"
export CONTROLLER_INTERNAL_URL="${CONTROLLER_INTERNAL_URL:-http://127.0.0.1:8084}"
export WIREGUARD_GATEWAY_INTERNAL_URL="${WIREGUARD_GATEWAY_INTERNAL_URL:-http://127.0.0.1:8087}"
export SUBMISSION_SERVICE_INTERNAL_URL="${SUBMISSION_SERVICE_INTERNAL_URL:-http://127.0.0.1:8082}"
export SCORING_WORKER_INTERNAL_URL="${SCORING_WORKER_INTERNAL_URL:-http://127.0.0.1:8085}"
export REALTIME_SOURCE_URL="${REALTIME_SOURCE_URL:-http://127.0.0.1:8080}"
export AD_PLATFORM_GAME_NETWORK_SUBNET="${AD_PLATFORM_GAME_NETWORK_SUBNET:-10.80.0.0/16}"
export AD_PLATFORM_NETWORK_LAYOUT="${AD_PLATFORM_NETWORK_LAYOUT:-per-service}"
readonly REQUIRED_SERVICE_PORTS=(8080 8081 8082 8084 8085 8086 8087)

port_in_use() {
  local port="$1"

  if command -v ss >/dev/null 2>&1; then
    ss -H -ltn "sport = :${port}" | grep -q .
    return
  fi

  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1
    return
  fi

  (echo >/dev/tcp/127.0.0.1/"${port}") >/dev/null 2>&1
}

find_free_port() {
  local port="$1"

  while port_in_use "${port}"; do
    port=$((port + 1))
  done

  echo "${port}"
}

ensure_required_ports_free() {
  local port

  for port in "${REQUIRED_SERVICE_PORTS[@]}"; do
    if port_in_use "${port}"; then
      echo "required backend port ${port} is already in use; stop the existing stack or reuse it instead of starting another one" >&2
      exit 1
    fi
  done
}

ensure_docker_network() {
  local network_name="$1"
  local subnet="${2:-}"

  if [[ -z "${network_name}" ]]; then
    return 0
  fi

  if docker network inspect "${network_name}" >/dev/null 2>&1; then
    return 0
  fi

  if [[ -n "${subnet}" ]]; then
    docker network create --subnet "${subnet}" "${network_name}" >/dev/null
    echo "created docker network ${network_name} (${subnet})"
    return 0
  fi

  docker network create "${network_name}" >/dev/null
  echo "created docker network ${network_name}"
}

placeholder_secret() {
  local value="${1:-}"

  [[ -z "${value}" ]] && return 0
  [[ "${value}" == dev-* ]] && return 0
  [[ "${value}" == change-this-* ]] && return 0
  [[ "${value}" == replace-with-* ]] && return 0
  return 1
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
    return
  fi
  od -An -tx1 -N32 /dev/urandom | tr -d ' \n'
}

ensure_runtime_secret() {
  local key="$1"
  local current="${!key:-}"

  if placeholder_secret "${current}"; then
    export "${key}=local-${key,,}-$(random_secret)"
  fi
}

ensure_runtime_secret ADMIN_API_TOKEN
ensure_runtime_secret TEAM_JWT_SECRET
ensure_runtime_secret UNLOCK_PROOF_SECRET
ensure_runtime_secret SSH_CREDENTIAL_SECRET
ensure_runtime_secret GAME_CORE_FLAG_SECRET
ensure_runtime_secret CONTROLLER_INTERNAL_TOKEN
ensure_runtime_secret GAME_CORE_INTERNAL_TOKEN
ensure_runtime_secret SUBMISSION_SERVICE_INTERNAL_TOKEN
ensure_runtime_secret SCORING_WORKER_INTERNAL_TOKEN
ensure_runtime_secret CHECKER_RUNNER_INTERNAL_TOKEN
ensure_runtime_secret WIREGUARD_GATEWAY_INTERNAL_TOKEN
ensure_runtime_secret REALTIME_ADMIN_TOKEN
if placeholder_secret "${REALTIME_SOURCE_ADMIN_TOKEN:-}"; then
  export REALTIME_SOURCE_ADMIN_TOKEN="${ADMIN_API_TOKEN}"
fi

if [[ "${MODE}" == "postgres" ]]; then
  export POSTGRES_HOST_PORT="${POSTGRES_HOST_PORT:-$(find_free_port 15432)}"
  export REDIS_HOST_PORT="${REDIS_HOST_PORT:-$(find_free_port 16379)}"
  export POSTGRES_DSN="postgres://adplatform:adplatform@127.0.0.1:${POSTGRES_HOST_PORT}/adplatform?sslmode=disable"
  export REDIS_ADDR="127.0.0.1:${REDIS_HOST_PORT}"

  if [[ "${POSTGRES_HOST_PORT}" != "15432" ]]; then
    echo "postgres host port 15432 is busy, using ${POSTGRES_HOST_PORT}"
  fi

  if [[ "${REDIS_HOST_PORT}" != "16379" ]]; then
    echo "redis host port 16379 is busy, using ${REDIS_HOST_PORT}"
  fi

  docker compose -f deploy/compose/dev.yml up -d --wait postgres redis
  export API_GATEWAY_STATE_BACKEND=postgres
  export GAME_CORE_STATE_BACKEND=postgres
  export CONTROLLER_STATE_BACKEND=postgres
  export WIREGUARD_GATEWAY_STATE_BACKEND=postgres
else
  export API_GATEWAY_STATE_BACKEND=memory
  export GAME_CORE_STATE_BACKEND=memory
  export CONTROLLER_STATE_BACKEND=memory
  export WIREGUARD_GATEWAY_STATE_BACKEND=memory
fi

ensure_required_ports_free

if [[ "${AD_PLATFORM_NETWORK_LAYOUT}" == "per-service" ]]; then
  remove_unused_docker_network "${CONTROLLER_DOCKER_NETWORK:-adplatform_game}"
  if [[ "${CHECKER_RUNNER_DOCKER_NETWORK:-${CONTROLLER_DOCKER_NETWORK:-adplatform_game}}" != "${CONTROLLER_DOCKER_NETWORK:-adplatform_game}" ]]; then
    remove_unused_docker_network "${CHECKER_RUNNER_DOCKER_NETWORK:-}"
  fi
fi

if [[ "${AD_PLATFORM_NETWORK_LAYOUT}" == "shared" && ( "${CONTROLLER_RUNTIME_MODE:-dry-run}" == "docker" || "${CHECKER_RUNNER_MODE:-dry-run}" == "docker" ) ]]; then
  controller_network="${CONTROLLER_DOCKER_NETWORK:-adplatform_game}"
  checker_network="${CHECKER_RUNNER_DOCKER_NETWORK:-${controller_network}}"
  ensure_docker_network "${controller_network}" "${AD_PLATFORM_GAME_NETWORK_SUBNET}"
  if [[ "${checker_network}" != "${controller_network}" ]]; then
    ensure_docker_network "${checker_network}" "${AD_PLATFORM_GAME_NETWORK_SUBNET}"
  fi
fi

cat > "${STACK_ENV_FILE}" <<EOF
AD_PLATFORM_STACK_MODE=${MODE}
POSTGRES_HOST_PORT=${POSTGRES_HOST_PORT:-}
REDIS_HOST_PORT=${REDIS_HOST_PORT:-}
POSTGRES_DSN=${POSTGRES_DSN:-}
REDIS_ADDR=${REDIS_ADDR:-}
API_GATEWAY_STATE_BACKEND=${API_GATEWAY_STATE_BACKEND}
GAME_CORE_STATE_BACKEND=${GAME_CORE_STATE_BACKEND}
CONTROLLER_STATE_BACKEND=${CONTROLLER_STATE_BACKEND}
WIREGUARD_GATEWAY_STATE_BACKEND=${WIREGUARD_GATEWAY_STATE_BACKEND}
GAME_CORE_INTERNAL_URL=${GAME_CORE_INTERNAL_URL}
CONTROLLER_INTERNAL_URL=${CONTROLLER_INTERNAL_URL}
WIREGUARD_GATEWAY_INTERNAL_URL=${WIREGUARD_GATEWAY_INTERNAL_URL}
SUBMISSION_SERVICE_INTERNAL_URL=${SUBMISSION_SERVICE_INTERNAL_URL}
SCORING_WORKER_INTERNAL_URL=${SCORING_WORKER_INTERNAL_URL}
REALTIME_SOURCE_URL=${REALTIME_SOURCE_URL}
ADMIN_API_TOKEN=${ADMIN_API_TOKEN}
TEAM_JWT_SECRET=${TEAM_JWT_SECRET}
UNLOCK_PROOF_SECRET=${UNLOCK_PROOF_SECRET}
SSH_CREDENTIAL_SECRET=${SSH_CREDENTIAL_SECRET}
GAME_CORE_FLAG_SECRET=${GAME_CORE_FLAG_SECRET}
CONTROLLER_INTERNAL_TOKEN=${CONTROLLER_INTERNAL_TOKEN}
GAME_CORE_INTERNAL_TOKEN=${GAME_CORE_INTERNAL_TOKEN}
SUBMISSION_SERVICE_INTERNAL_TOKEN=${SUBMISSION_SERVICE_INTERNAL_TOKEN}
SCORING_WORKER_INTERNAL_TOKEN=${SCORING_WORKER_INTERNAL_TOKEN}
CHECKER_RUNNER_INTERNAL_TOKEN=${CHECKER_RUNNER_INTERNAL_TOKEN}
WIREGUARD_GATEWAY_INTERNAL_TOKEN=${WIREGUARD_GATEWAY_INTERNAL_TOKEN}
REALTIME_SOURCE_ADMIN_TOKEN=${REALTIME_SOURCE_ADMIN_TOKEN}
REALTIME_ADMIN_TOKEN=${REALTIME_ADMIN_TOKEN}
AD_PLATFORM_API_URL=http://127.0.0.1:8080
AD_PLATFORM_REALTIME_URL=http://127.0.0.1:8086
AD_PLATFORM_NETWORK_LAYOUT=${AD_PLATFORM_NETWORK_LAYOUT}
AD_PLATFORM_GAME_NETWORK_SUBNET=${AD_PLATFORM_GAME_NETWORK_SUBNET}
CONTROLLER_RUNTIME_MODE=${CONTROLLER_RUNTIME_MODE:-dry-run}
CHECKER_RUNNER_MODE=${CHECKER_RUNNER_MODE:-dry-run}
CONTROLLER_DOCKER_NETWORK=${CONTROLLER_DOCKER_NETWORK:-}
CHECKER_RUNNER_DOCKER_NETWORK=${CHECKER_RUNNER_DOCKER_NETWORK:-}
EOF

declare -a PIDS=()

cleanup() {
  local exit_code=$?
  trap - EXIT INT TERM
  rm -f "${STACK_ENV_FILE}"
  for pid in "${PIDS[@]:-}"; do
    kill -- "-${pid}" 2>/dev/null || kill "${pid}" 2>/dev/null || true
  done
  wait 2>/dev/null || true
  exit "${exit_code}"
}

trap cleanup EXIT INT TERM

start_service() {
  local name="$1"
  shift
  local log_file="${ROOT_DIR}/.runtime/logs/${name}.log"
  : > "${log_file}"
  if command -v setsid >/dev/null 2>&1; then
    (
      cd "${ROOT_DIR}"
      exec setsid "$@" >>"${log_file}" 2>&1
    ) &
  else
    (
      cd "${ROOT_DIR}"
      exec "$@" >>"${log_file}" 2>&1
    ) &
  fi
  local pid=$!
  PIDS+=("${pid}")
  echo "started ${name} (pid=${pid}, log=${log_file})"
}

start_service checker-runner go run ./services/checker-runner
start_service game-core go run ./services/game-core
start_service submission-service go run ./services/submission-service
start_service controller-service go run ./services/controller-service
start_service scoring-worker go run ./services/scoring-worker
start_service wireguard-gateway go run ./services/wireguard-gateway
start_service api-gateway go run ./services/api-gateway
start_service realtime-gateway go run ./services/realtime-gateway

wait_for_http "http://127.0.0.1:8081/readyz" 40 "game-core"
wait_for_http "http://127.0.0.1:8082/readyz" 40 "submission-service"
wait_for_http "http://127.0.0.1:8085/readyz" 40 "scoring-worker"
wait_for_http "http://127.0.0.1:8080/readyz" 40 "api-gateway"
wait_for_http "http://127.0.0.1:8080/api/v2/challenges" 40 "api-gateway database path"
wait_for_http "http://127.0.0.1:8086/readyz" 40 "realtime-gateway"

cat <<EOF
backend stack is running (${MODE} mode)

participant api:
  http://127.0.0.1:8080/api/v2/challenges
  http://127.0.0.1:8080/api/v2/scoreboard

realtime:
  http://127.0.0.1:8086/public/v1/scoreboard/stream
  http://127.0.0.1:8086/public/v1/attacks/stream

logs:
  ${ROOT_DIR}/.runtime/logs

press Ctrl+C to stop all backend services.
EOF

wait
