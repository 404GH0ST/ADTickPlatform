#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"
load_default_env_files

require_bin curl
require_bin docker
require_bin jq

STACK_ENV_FILE="${ROOT_DIR}/.runtime/backend-stack.env"
STACK_LOG="${ROOT_DIR}/.runtime/logs/organizer-created-stack.log"
STACK_PID=""
STACK_OWNED="false"

NETWORK="${CONTROLLER_DOCKER_NETWORK:-adplatform_game}"
SUBNET="${AD_PLATFORM_GAME_NETWORK_SUBNET:-10.80.0.0/16}"
NETWORK_LAYOUT="${AD_PLATFORM_NETWORK_LAYOUT:-per-service}"
BASELINE_IMAGE="${SAMPLE_CHALLENGE_BASELINE_IMAGE:-adplatform/sample-http:baseline}"
CHECKER_IMAGE="${SAMPLE_CHALLENGE_CHECKER_IMAGE:-adplatform/sample-http-checker:latest}"

ADMIN_TOKEN="${ADMIN_API_TOKEN:-dev-admin-token}"
API_URL="${AD_PLATFORM_API_URL:-http://127.0.0.1:8080}"
READY_URL="${ORGANIZER_SMOKE_READY_URL:-${API_URL%/}/readyz}"
SKIP_STACK_BOOTSTRAP="${ORGANIZER_SMOKE_SKIP_STACK_BOOTSTRAP:-false}"
USE_SCHEDULER="${ORGANIZER_SMOKE_USE_SCHEDULER:-false}"
TARGET_TICKS="${ORGANIZER_SMOKE_TARGET_TICKS:-1}"
SCHEDULER_WAIT_SECONDS="${ORGANIZER_SMOKE_SCHEDULER_WAIT_SECONDS:-60}"
SCHEDULER_INTERVAL_SECONDS="${ORGANIZER_SMOKE_SCHEDULER_INTERVAL_SECONDS:-}"
ARTIFACT_FILE="${ORGANIZER_SMOKE_ARTIFACT_FILE:-}"

cleanup() {
  local exit_code=$?
  trap - EXIT INT TERM
  if [[ "${STACK_OWNED}" == "true" && -n "${STACK_PID}" ]]; then
    kill -- "-${STACK_PID}" 2>/dev/null || kill "${STACK_PID}" 2>/dev/null || true
    wait "${STACK_PID}" 2>/dev/null || true
  fi
  exit "${exit_code}"
}

trap cleanup EXIT INT TERM

curl_json() {
  local label="$1"
  shift

  local response_file
  response_file="$(mktemp)"

  local status
  status="$(curl -sS -o "${response_file}" -w '%{http_code}' "$@")"
  if [[ "${status}" -lt 200 || "${status}" -ge 300 ]]; then
    echo "${label} failed (status=${status}):" >&2
    cat "${response_file}" >&2
    rm -f "${response_file}"
    exit 1
  fi

  cat "${response_file}"
  rm -f "${response_file}"
}

wait_for_target_tick() {
  local target_tick="$1"
  local deadline
  deadline=$((SECONDS + SCHEDULER_WAIT_SECONDS))

  while (( SECONDS < deadline )); do
    local status_json current_tick tick_status
    status_json="$(
      curl_json "game status" "${API_URL}/api/v2/admin/game/status" \
        -H "Authorization: Bearer ${ADMIN_TOKEN}"
    )"
    current_tick="$(printf '%s\n' "${status_json}" | jq -r '.current_tick.id // 0')"
    tick_status="$(printf '%s\n' "${status_json}" | jq -r '.current_tick.status // ""')"
    if [[ "${current_tick}" =~ ^[0-9]+$ ]] && (( current_tick >= target_tick )) && [[ "${tick_status}" == "completed" ]]; then
      printf '%s\n' "${status_json}"
      return 0
    fi
    sleep 1
  done

  echo "scheduler did not reach completed tick ${target_tick} within ${SCHEDULER_WAIT_SECONDS}s" >&2
  return 1
}

mkdir -p .runtime/logs

if [[ "${NETWORK_LAYOUT}" == "shared" ]] && ! docker network inspect "${NETWORK}" >/dev/null 2>&1; then
  docker network create --subnet "${SUBNET}" "${NETWORK}" >/dev/null
fi

echo "building sample challenge images"
docker build -t "${BASELINE_IMAGE}" examples/sample-http-challenge/service >/dev/null
docker build -t "${CHECKER_IMAGE}" examples/sample-http-challenge/checker >/dev/null

if [[ "${SKIP_STACK_BOOTSTRAP}" == "true" ]]; then
  wait_for_http "${READY_URL}" 60 "organizer smoke api"
  echo "reusing externally managed stack at ${API_URL}"
elif http_ready "${READY_URL}"; then
  if [[ -f "${STACK_ENV_FILE}" ]]; then
    load_env_file_override "${STACK_ENV_FILE}"
    API_URL="${AD_PLATFORM_API_URL:-http://127.0.0.1:8080}"
    if [[ "${AD_PLATFORM_STACK_MODE:-}" != "postgres" ]]; then
      echo "existing backend stack is not postgres-backed; organizer-created smoke requires postgres mode" >&2
      exit 1
    fi
    if [[ "${CONTROLLER_RUNTIME_MODE:-}" != "docker" || "${CHECKER_RUNNER_MODE:-}" != "docker" ]]; then
      echo "existing backend stack is not running controller/checker in docker mode; restart it with CONTROLLER_RUNTIME_MODE=docker CHECKER_RUNNER_MODE=docker" >&2
      exit 1
    fi
    echo "reusing active backend stack from ${STACK_ENV_FILE}"
  else
    echo "reusing active backend stack on default ports without ${STACK_ENV_FILE}; assuming local defaults"
  fi
else
  echo "starting clean backend stack in docker mode"
  if command -v setsid >/dev/null 2>&1; then
    (
      cd "${ROOT_DIR}"
      exec setsid env \
        CONTROLLER_RUNTIME_MODE=docker \
        CHECKER_RUNNER_MODE=docker \
        API_GATEWAY_AUTO_SEED=false \
        DATABASE_INCLUDE_SEEDS=false \
        AD_PLATFORM_NETWORK_LAYOUT="${NETWORK_LAYOUT}" \
        CONTROLLER_DOCKER_NETWORK="${NETWORK}" \
        CHECKER_RUNNER_DOCKER_NETWORK="${NETWORK}" \
        AD_PLATFORM_GAME_NETWORK_SUBNET="${SUBNET}" \
        ./scripts/run-backend-stack.sh postgres >>"${STACK_LOG}" 2>&1
    ) &
  else
    (
      cd "${ROOT_DIR}"
      exec env \
        CONTROLLER_RUNTIME_MODE=docker \
        CHECKER_RUNNER_MODE=docker \
        API_GATEWAY_AUTO_SEED=false \
        DATABASE_INCLUDE_SEEDS=false \
        AD_PLATFORM_NETWORK_LAYOUT="${NETWORK_LAYOUT}" \
        CONTROLLER_DOCKER_NETWORK="${NETWORK}" \
        CHECKER_RUNNER_DOCKER_NETWORK="${NETWORK}" \
        AD_PLATFORM_GAME_NETWORK_SUBNET="${SUBNET}" \
        ./scripts/run-backend-stack.sh postgres >>"${STACK_LOG}" 2>&1
    ) &
  fi
  STACK_PID=$!
  STACK_OWNED="true"

  wait_for_http "http://127.0.0.1:8080/readyz" 60
  wait_for_http "http://127.0.0.1:8081/readyz" 60

  if [[ -f "${STACK_ENV_FILE}" ]]; then
    load_env_file_override "${STACK_ENV_FILE}"
    API_URL="${AD_PLATFORM_API_URL:-http://127.0.0.1:8080}"
  fi
fi

echo "resetting platform to an empty organizer-managed state"
BOOTSTRAP_CLEAR_TEAMS=true ./scripts/bootstrap-clean-match.sh >/dev/null

UNIQUE_SUFFIX="$(date +%s)"
TEAM_ONE_NAME="College Alpha ${UNIQUE_SUFFIX}"
TEAM_TWO_NAME="College Beta ${UNIQUE_SUFFIX}"
TEAM_ONE_EMAIL="alpha-${UNIQUE_SUFFIX}@college.local"
TEAM_TWO_EMAIL="beta-${UNIQUE_SUFFIX}@college.local"
TEAM_ONE_PLAYER_EMAIL="captain.alpha.${UNIQUE_SUFFIX}@college.local"
TEAM_TWO_PLAYER_EMAIL="captain.beta.${UNIQUE_SUFFIX}@college.local"
TEAM_ONE_PASSWORD="alpha-${UNIQUE_SUFFIX}-secret"
TEAM_TWO_PASSWORD="beta-${UNIQUE_SUFFIX}-secret"
CHALLENGE_NAME="college-http-${UNIQUE_SUFFIX}"

existing_challenges="$(curl_json "challenge list" "${API_URL}/api/v2/admin/challenges" -H "Authorization: Bearer ${ADMIN_TOKEN}")"
SERVICE_SUBNET_OCTET=""
used_subnets="$(printf '%s' "${existing_challenges}" | jq -r '.[].service_subnet_octet')"
for candidate in $(seq 50 254); do
  if ! printf '%s\n' "${used_subnets}" | grep -qx "${candidate}"; then
    SERVICE_SUBNET_OCTET="${candidate}"
    break
  fi
done
if [[ -z "${SERVICE_SUBNET_OCTET}" ]]; then
  echo "could not allocate a free service_subnet_octet for organizer-created smoke" >&2
  exit 1
fi
SERVICE_PORT="$((30000 + SERVICE_SUBNET_OCTET))"

echo "creating organizer-owned teams and players"
team_one_response="$(curl_json "create team one" -X POST "${API_URL}/api/v2/admin/teams" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg name "${TEAM_ONE_NAME}" --arg email "${TEAM_ONE_EMAIL}" '{name:$name,contact_email:$email}')")"
team_two_response="$(curl_json "create team two" -X POST "${API_URL}/api/v2/admin/teams" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg name "${TEAM_TWO_NAME}" --arg email "${TEAM_TWO_EMAIL}" '{name:$name,contact_email:$email}')")"

team_one_id="$(printf '%s' "${team_one_response}" | jq -er '.id')"
team_two_id="$(printf '%s' "${team_two_response}" | jq -er '.id')"

curl_json "create team one player" -X POST "${API_URL}/api/v2/admin/players" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc \
    --argjson team_id "${team_one_id}" \
    --arg display_name "Captain Alpha ${UNIQUE_SUFFIX}" \
    --arg email "${TEAM_ONE_PLAYER_EMAIL}" \
    --arg password "${TEAM_ONE_PASSWORD}" \
    --arg role "captain" \
    '{team_id:$team_id,display_name:$display_name,email:$email,password:$password,role:$role}')" >/dev/null

curl_json "create team two player" -X POST "${API_URL}/api/v2/admin/players" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc \
    --argjson team_id "${team_two_id}" \
    --arg display_name "Captain Beta ${UNIQUE_SUFFIX}" \
    --arg email "${TEAM_TWO_PLAYER_EMAIL}" \
    --arg password "${TEAM_TWO_PASSWORD}" \
    --arg role "captain" \
    '{team_id:$team_id,display_name:$display_name,email:$email,password:$password,role:$role}')" >/dev/null

echo "creating challenge"
challenge_response="$(curl_json "create challenge" -X POST "${API_URL}/api/v2/admin/challenges" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc \
    --arg name "${CHALLENGE_NAME}" \
    --arg baseline_image "${BASELINE_IMAGE}" \
    --arg checker_image "${CHECKER_IMAGE}" \
    --argjson weight 1 \
    --argjson service_port "${SERVICE_PORT}" \
    --argjson service_subnet_octet "${SERVICE_SUBNET_OCTET}" \
    '{name:$name,baseline_image:$baseline_image,checker_image:$checker_image,weight:$weight,service_port:$service_port,service_subnet_octet:$service_subnet_octet}')")"
challenge_id="$(printf '%s' "${challenge_response}" | jq -er '.id')"

curl_json "validate challenge" -X POST "${API_URL}/api/v2/admin/challenges/${challenge_id}/validate" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null

curl_json "deploy challenge" -X POST "${API_URL}/api/v2/admin/challenges/${challenge_id}/deploy" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null

reconcile_response="$(curl_json "reconcile deployments" -X POST "${API_URL}/api/v2/admin/deployments/reconcile" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}")"
printf '%s\n' "${reconcile_response}" | jq -c '{processed_jobs,processed_instances,completed_jobs}'

team_one_token="$(
  curl_json "authenticate team one" -X POST "${API_URL}/api/v2/authenticate" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg email "${TEAM_ONE_PLAYER_EMAIL}" --arg password "${TEAM_ONE_PASSWORD}" '{email:$email,password:$password}')" |
    jq -er '.token'
)"

team_one_state="$(
  curl_json "read team one services" "${API_URL}/api/v2/team/services" \
    -H "Authorization: Bearer ${team_one_token}" |
    jq -ec --argjson challenge_id "${challenge_id}" 'map(select(.challenge_id == $challenge_id)) | first'
)"
printf '%s\n' "${team_one_state}" | jq -c '{challenge_id,name,endpoint,status,checker}'
team_one_endpoint="$(printf '%s' "${team_one_state}" | jq -er '.endpoint')"

public_services="$(
  curl_json "read service targets" "${API_URL}/api/v2/services" \
    -H "Authorization: Bearer ${team_one_token}"
)"
team_two_endpoint="$(
  printf '%s\n' "${public_services}" |
    jq -er --arg challenge_id "${challenge_id}" --arg team_id "${team_two_id}" '.[$challenge_id][$team_id][0]'
)"

container_name="svc-$(slug_name "${CHALLENGE_NAME}")-team-${team_one_id}"
if ! docker ps -a --format '{{.Names}}' | grep -qx "${container_name}"; then
  echo "expected runtime container ${container_name} was not created" >&2
  exit 1
fi

if ! wait_for_http "http://${team_one_endpoint}/health" 30; then
  docker exec -i "${container_name}" python3 - <<'PY' >/dev/null
import os
from urllib.request import urlopen
port = os.environ.get("AD_PLATFORM_SERVICE_PORT", os.environ.get("PORT", "8080"))
with urlopen(f"http://127.0.0.1:{port}/health", timeout=5) as response:
    if response.status != 200:
        raise SystemExit(1)
PY
fi

unlock_proof="$(
  curl -fsS "http://${team_one_endpoint}/unlock" 2>/dev/null | jq -er '.proof' 2>/dev/null || \
  docker exec -i "${container_name}" python3 - <<'PY'
import json
import os
from urllib.request import urlopen
port = os.environ.get("AD_PLATFORM_SERVICE_PORT", os.environ.get("PORT", "8080"))
with urlopen(f"http://127.0.0.1:{port}/unlock", timeout=5) as response:
    body = json.loads(response.read().decode("utf-8"))
    print(body["proof"])
PY
)"

curl_json "unlock team one service" -X POST "${API_URL}/api/v2/services/${challenge_id}/unlock" \
  -H "Authorization: Bearer ${team_one_token}" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg proof "${unlock_proof}" '{proof:$proof}')" |
  jq -c '{challenge_id,team_id,unlocked}'

curl_json "request ssh credential" -X POST "${API_URL}/api/v2/services/${challenge_id}/ssh-session" \
  -H "Authorization: Bearer ${team_one_token}" |
  jq -c '{host,port,username,password_present:(.password | length > 0)}'

curl_json "start match" -X POST "${API_URL}/api/v2/admin/game/match/start" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null

if [[ "${USE_SCHEDULER}" == "true" ]]; then
  if [[ -n "${SCHEDULER_INTERVAL_SECONDS}" ]]; then
    curl_json "update scheduler interval" -X PUT "${API_URL}/api/v2/admin/game/scheduler/interval" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "$(jq -nc --argjson interval_seconds "${SCHEDULER_INTERVAL_SECONDS}" '{interval_seconds:$interval_seconds}')" >/dev/null
  fi

  curl_json "start scheduler" -X POST "${API_URL}/api/v2/admin/game/scheduler/start" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null

  status_response="$(wait_for_target_tick "${TARGET_TICKS}")"
  printf '%s\n' "${status_response}" | jq -c '{match:.match.state,scheduler:.scheduler.state,current_tick:(.current_tick.id // 0),total_checker_runs}'
  tick_id="$(printf '%s\n' "${status_response}" | jq -er '.current_tick.id')"

  curl_json "stop scheduler" -X POST "${API_URL}/api/v2/admin/game/scheduler/stop" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" >/dev/null
else
  tick_response="$(
    curl_json "advance tick" -X POST "${API_URL}/api/v2/admin/game/ticks/advance" \
      -H "Authorization: Bearer ${ADMIN_TOKEN}"
  )"
  printf '%s\n' "${tick_response}" | jq -c '{id,status,total_checker_runs,successful_checker_runs,failed_checker_runs,skipped_checker_runs}'
  tick_id="$(printf '%s\n' "${tick_response}" | jq -er '.id')"
fi

checker_runs_response="$(
  curl_json "checker runs" "${API_URL}/api/v2/admin/game/checker-runs?tick_id=${tick_id}&challenge_id=${challenge_id}&limit=20" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
printf '%s\n' "${checker_runs_response}" | jq -c '{total_count,has_next}'

if ! printf '%s\n' "${checker_runs_response}" | jq -e '.items | length == 6 and all(.[]; .status == "success")' >/dev/null; then
  echo "checker runs were not all successful for the organizer-created smoke" >&2
  printf '%s\n' "${checker_runs_response}" | jq -c '.items[] | {team_id,phase,status,message}' >&2
  exit 1
fi

stolen_flag="$(
  docker exec -i "${container_name}" python3 - <<PY
import json
from urllib.request import urlopen

endpoint = "${team_two_endpoint}"
with urlopen(f"http://{endpoint}/leak", timeout=5) as response:
    body = json.loads(response.read().decode("utf-8"))
    print(body["flag"])
PY
)"

submit_response="$(
  curl_json "submit stolen flag" -X POST "${API_URL}/api/v2/submit" \
    -H "Authorization: Bearer ${team_one_token}" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg flag "${stolen_flag}" '{flags:[$flag]}')"
)"
printf '%s\n' "${submit_response}" | jq -c '.results[] | {flag,status,detail}'
if ! printf '%s\n' "${submit_response}" | jq -e '.results | length == 1 and .[0].status == "accepted" and .[0].detail == "flag is correct."' >/dev/null; then
  echo "stolen flag submission was not accepted" >&2
  exit 1
fi

scoreboard_response="$(
  curl_json "recompute scoring" -X POST "${API_URL}/api/v2/admin/game/scoring/recompute" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
)"
printf '%s\n' "${scoreboard_response}" | jq -c '.[] | {team,attack,defense,sla,total}'

sqrt_two="$(awk 'BEGIN { printf "%.12f", sqrt(2) }')"
expected_team_one_attack=2
expected_team_one_defense=0
expected_team_one_sla="${sqrt_two}"
expected_team_one_total="$(awk -v sla="${sqrt_two}" 'BEGIN { printf "%.12f", 2 + sla }')"
expected_team_two_attack=0
expected_team_two_defense=-1
expected_team_two_sla="${sqrt_two}"
expected_team_two_total="$(awk -v sla="${sqrt_two}" 'BEGIN { printf "%.12f", -1 + sla }')"

if ! printf '%s\n' "${scoreboard_response}" | jq -e \
  --arg team_one "${TEAM_ONE_NAME}" \
  --arg team_two "${TEAM_TWO_NAME}" \
  --argjson team_one_attack "${expected_team_one_attack}" \
  --argjson team_one_defense "${expected_team_one_defense}" \
  --argjson team_one_sla "${expected_team_one_sla}" \
  --argjson team_one_total "${expected_team_one_total}" \
  --argjson team_two_attack "${expected_team_two_attack}" \
  --argjson team_two_defense "${expected_team_two_defense}" \
  --argjson team_two_sla "${expected_team_two_sla}" \
  --argjson team_two_total "${expected_team_two_total}" '
  def close($left; $right): (($left - $right) | if . < 0 then -. else . end) < 0.000001;
  any(.[]; .team == $team_one and close(.attack; $team_one_attack) and close(.defense; $team_one_defense) and close(.sla; $team_one_sla) and close(.total; $team_one_total))
  and any(.[]; .team == $team_two and close(.attack; $team_two_attack) and close(.defense; $team_two_defense) and close(.sla; $team_two_sla) and close(.total; $team_two_total))
' >/dev/null; then
  echo "unexpected scoreboard after organizer-created attack flow" >&2
  exit 1
fi

printf 'organizer-created flow passed: challenge_id=%s attacker_team_id=%s victim_team_id=%s endpoint=%s\n' \
  "${challenge_id}" "${team_one_id}" "${team_two_id}" "${team_one_endpoint}"

if [[ -n "${ARTIFACT_FILE}" ]]; then
  mkdir -p "$(dirname "${ARTIFACT_FILE}")"
  cat > "${ARTIFACT_FILE}" <<EOF
SMOKE_CHALLENGE_ID=${challenge_id}
SMOKE_TEAM_ONE_ID=${team_one_id}
SMOKE_TEAM_TWO_ID=${team_two_id}
SMOKE_TEAM_ONE_PLAYER_EMAIL=${TEAM_ONE_PLAYER_EMAIL}
SMOKE_TEAM_ONE_PASSWORD=${TEAM_ONE_PASSWORD}
SMOKE_TEAM_TWO_PLAYER_EMAIL=${TEAM_TWO_PLAYER_EMAIL}
SMOKE_TEAM_TWO_PASSWORD=${TEAM_TWO_PASSWORD}
SMOKE_TEAM_ONE_ENDPOINT=${team_one_endpoint}
SMOKE_TEAM_TWO_ENDPOINT=${team_two_endpoint}
EOF
fi
